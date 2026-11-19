package syncnative

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	loopbackModel = "gandalf-native-probe"
	probePrompt   = "Reply with the synthetic acknowledgement."
	probeReply    = "Gandalf native probe acknowledgement."
)

type nativeObservation struct {
	skillContext    string
	ruleContext     string
	blockedRequests int32
}

func captureProbe(t *testing.T, target, version, project string) nativeObservation {
	t.Helper()
	home := t.TempDir()
	profile := filepath.Join(home, target)
	bin := nativeBinary(t, target)

	// Catch auxiliary HTTP(S) requests locally, including CONNECT. The
	// listener has no forwarding path and never records hosts or payloads.
	var blockedRequests atomic.Int32
	denyProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		blockedRequests.Add(1)
		http.Error(w, "only the loopback provider is available", http.StatusForbidden)
	}))
	t.Cleanup(denyProxy.Close)

	var providerCalls atomic.Int32
	requests := make(chan modelCall, 1)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != providerPath(target) {
			http.NotFound(w, req)
			return
		}
		providerCalls.Add(1)
		for _, header := range []string{"Authorization", "Cookie", "X-API-Key"} {
			if req.Header.Get(header) != "" {
				http.Error(w, "credentials are forbidden in the test request", http.StatusBadRequest)
				return
			}
		}
		var call modelCall
		if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 2<<20)).Decode(&call); err != nil {
			http.Error(w, "invalid test request", http.StatusBadRequest)
			return
		}
		select {
		case requests <- call:
		default:
		}
		writeProbeReply(w, target, call.Stream)
	}))
	t.Cleanup(provider.Close)

	env := nativeEnvironment(home, profile, target, denyProxy.URL)
	var config, versionPrefix string
	var args []string
	if target == "codex" {
		config = fmt.Sprintf(codexConfig, provider.URL, filepath.ToSlash(project))
		versionPrefix = "codex-cli "
		args = []string{"-a", "never", "exec", "--ephemeral", "--skip-git-repo-check", "--sandbox", "read-only", "--json", probePrompt}
	} else {
		config = fmt.Sprintf(vibeConfig, provider.URL)
		versionPrefix = "vibe "
		args = []string{"--trust", "--agent", "ask", "--enabled-tools", "skill", "--max-turns", "1", "--output", "json", "--prompt", probePrompt}
	}
	writeNativeFixture(t, profile, "config.toml", config)

	out, err := runNative(t, bin, project, env, "--version")
	if err != nil {
		t.Fatalf("%s --version failed: %v", target, err)
	}
	if strings.TrimSpace(string(out)) != versionPrefix+version {
		t.Fatalf("installed %s version is unverified for this test, expected %s", target, version)
	}

	out, err = runNative(t, bin, project, env, args...)
	if err != nil {
		t.Fatalf("%s session failed: %v (provider requests: %d)", target, err, providerCalls.Load())
	}
	if !strings.Contains(string(out), probeReply) {
		t.Fatalf("%s session did not deliver the synthetic acknowledgement", target)
	}
	if count := providerCalls.Load(); count != 1 {
		t.Fatalf("%s loopback model requests = %d, want 1", target, count)
	}

	select {
	case call := <-requests:
		if call.Model != loopbackModel {
			t.Fatalf("%s did not use the synthetic model", target)
		}
		context := call.context()
		if !strings.Contains(context, probePrompt) {
			t.Fatalf("%s loopback request omitted the synthetic user prompt", target)
		}
		return nativeObservation{
			skillContext:    context + "\n" + string(call.Tools),
			ruleContext:     context,
			blockedRequests: blockedRequests.Load(),
		}
	default:
		t.Fatalf("%s did not send a valid loopback model request", target)
		return nativeObservation{}
	}
}

func nativeBinary(t *testing.T, target string) string {
	t.Helper()
	name := target
	if override := os.Getenv("GANDALF_SYNC_" + strings.ToUpper(target) + "_BIN"); override != "" {
		name = override
	}
	bin, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("%s executable not found", target)
	}
	if runtime.GOOS != "windows" || target != "codex" || !strings.EqualFold(filepath.Ext(bin), ".cmd") {
		return bin
	}
	// An npm-installed Codex has a Windows command shim. Launch its native
	// executable directly so Go can own the process and cancel it on timeout.
	platform := "x64"
	if runtime.GOARCH == "arm64" {
		platform = "arm64"
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(bin), "node_modules", "@openai", "codex",
		"node_modules", "@openai", "codex-win32-"+platform, "vendor", "*", "bin", "codex.exe"))
	if err != nil || len(matches) != 1 {
		t.Fatal("set GANDALF_SYNC_CODEX_BIN to the installed native codex.exe")
	}
	return matches[0]
}

func nativeEnvironment(home, profile, target, proxy string) []string {
	env := []string{
		"HOME=" + home, "USERPROFILE=" + home,
		"APPDATA=" + home, "LOCALAPPDATA=" + home,
		"XDG_CONFIG_HOME=" + home, "XDG_DATA_HOME=" + home, "XDG_CACHE_HOME=" + home,
		"TMP=" + home, "TEMP=" + home, "TMPDIR=" + home,
		"HTTP_PROXY=" + proxy, "HTTPS_PROXY=" + proxy, "ALL_PROXY=" + proxy,
		"NO_PROXY=127.0.0.1", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0",
		strings.ToUpper(target) + "_HOME=" + profile,
	}
	for _, key := range []string{"PATH", "PATHEXT", "SystemRoot", "WINDIR", "COMSPEC", "OS", "PROGRAMFILES", "PROGRAMFILES(X86)"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func runNative(t *testing.T, bin, project string, env []string, args ...string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = project, env, time.Second
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	// The profile and requests remain in the test. Report only a status,
	// never child output, since it can include the prompt or configuration.
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return nil, fmt.Errorf("exit code %d", exit.ExitCode())
	}
	if err != nil {
		return nil, errors.New("could not run native executable")
	}
	return out, nil
}

type modelCall struct {
	Model    string          `json:"model"`
	Input    []promptMessage `json:"input"`
	Messages []promptMessage `json:"messages"`
	Tools    json.RawMessage `json:"tools"`
	Stream   bool            `json:"stream"`
}

type promptMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

func (c modelCall) context() string {
	var texts []string
	for _, message := range append(c.Input, c.Messages...) {
		switch message.Role {
		case "system", "developer", "user":
		default:
			continue
		}
		var plain string
		if json.Unmarshal(message.Content, &plain) == nil {
			texts = append(texts, plain)
			continue
		}
		var parts []struct{ Text string }
		if json.Unmarshal(message.Content, &parts) == nil {
			for _, part := range parts {
				texts = append(texts, part.Text)
			}
		}
	}
	return strings.Join(texts, "\n")
}

func providerPath(target string) string {
	if target == "codex" {
		return "/codex/v1/responses"
	}
	return "/vibe/v1/chat/completions"
}

func writeProbeReply(w http.ResponseWriter, target string, stream bool) {
	if target == "vibe" && !stream {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, fmt.Sprintf(`{"id":"probe","object":"chat.completion","created":1,"model":%q,"choices":[{"index":0,"message":{"role":"assistant","content":%q},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`, loopbackModel, probeReply))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	if target == "codex" {
		item := fmt.Sprintf(`{"id":"msg_probe","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":%q,"annotations":[]}]}`, probeReply)
		_, _ = fmt.Fprintf(w, "data: %s\n\ndata: %s\n\ndata: %s\n\n",
			`{"type":"response.created","response":{"id":"resp_probe","output":[]}}`,
			`{"type":"response.output_item.done","output_index":0,"item":`+item+`}`,
			`{"type":"response.completed","response":{"id":"resp_probe","status":"completed","output":[`+item+`],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`)
		return
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\ndata: %s\n\ndata: [DONE]\n\n",
		fmt.Sprintf(`{"id":"probe","object":"chat.completion.chunk","created":1,"model":%q,"choices":[{"index":0,"delta":{"role":"assistant","content":%q},"finish_reason":null}]}`, loopbackModel, probeReply),
		fmt.Sprintf(`{"id":"probe","object":"chat.completion.chunk","created":1,"model":%q,"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`, loopbackModel))
}

const codexConfig = `model = "gandalf-native-probe"
model_provider = "gandalf-probe"
check_for_update_on_startup = false

[analytics]
enabled = false

[features]
apps = false
plugins = false
remote_plugin = false

[model_providers.gandalf-probe]
name = "Gandalf loopback probe"
base_url = "%s/codex/v1"
wire_api = "responses"
request_max_retries = 0
stream_max_retries = 0

[projects.%q]
trust_level = "trusted"
`

const vibeConfig = `active_model = "gandalf-native-probe"
enable_telemetry = false
enable_update_checks = false
experimental_enable_model_catalog = false

[[providers]]
name = "gandalf-probe"
api_base = "%s/vibe/v1"
api_key_env_var = ""
backend = "generic"

[[models]]
name = "gandalf-native-probe"
alias = "gandalf-native-probe"
provider = "gandalf-probe"
`
