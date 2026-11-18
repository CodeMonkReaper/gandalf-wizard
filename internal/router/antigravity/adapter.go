// Package antigravity adapts the Antigravity CLI (agy) to the router port.
//
// The execution recipe enforces text-only output: a session-scoped
// permissions deny list that blocks every documented tool action (it takes
// precedence over any allow rule in the user's separate configuration),
// disabled slash-command and skill expansion, and the terminal sandbox.
// The text-only recipe was demonstrated against the real binary in T010, so
// production defaults to enforced; tests can still override TextOnly to
// simulate an unproven state.
package antigravity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router"
)

// waitDelay bounds how long the adapter waits for the launcher's process
// tree to exit after cancellation or the attempt deadline.
const waitDelay = 5 * time.Second

// textOnlySettings denies every tool action for the session. The Antigravity
// permission engine evaluates deny > ask > allow, so these rules also block
// anything the user's separate configuration enables.
const textOnlySettings = `{"permissions":{"deny":["read_file(*)","write_file(*)","read_url(*)","execute_url(*)","command(*)","unsandboxed(*)","mcp(*)"]}}`

// Runner executes a prepared command and returns its stdout.
type Runner interface {
	Run(ctx context.Context, cmd *exec.Cmd) ([]byte, error)
}

// processRunner runs the command for real and captures its stdout. Stderr
// stays out of the result: raw diagnostics never reach the router.
type processRunner struct{}

func (processRunner) Run(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
	return cmd.Output()
}

// Adapter implements router.Adapter for the Antigravity CLI. Every
// dependency is injected so the automated tests never need a Google
// account, the real binary, or quota.
type Adapter struct {
	// Bin locates the agy binary. Nil uses the PATH lookup.
	Bin func() (string, bool)
	// Session reports the local login indicator. Nil uses the default:
	// presence of the CLI family's account record, without reading it.
	Session func() router.AuthState
	// TextOnly reports the demonstrated text-only state. Nil defaults to
	// enforced after the live T010 verification; tests can inject unproven.
	TextOnly func() router.TextOnlyState
	// Env returns the parent environment. Nil uses os.Environ.
	Env func() []string
	// Runner runs prepared commands. Nil uses the real process runner.
	Runner Runner
}

var _ router.Adapter = (*Adapter)(nil)

// New returns the adapter with production defaults.
func New() *Adapter {
	return &Adapter{}
}

// Probe reports the local, quota-free state of the executor: installation,
// session, and the demonstrated text-only state, with the binary version in
// the sanitized reason.
func (a *Adapter) Probe(ctx context.Context, ex config.RouterExecutor) router.Probe {
	if ex.Kind != config.RouterKindCLI {
		return router.Probe{Reason: "el adaptador no soporta este ejecutor"}
	}
	bin, ok := a.bin()
	if !ok {
		return router.Probe{Reason: "agy no está instalado"}
	}
	p := router.Probe{Installed: true, Auth: a.session(), TextOnly: a.textOnly()}
	if p.Auth == router.AuthReady && p.TextOnly == router.TextOnlyEnforced {
		if v := a.version(ctx, bin); v != "" {
			p.Reason = "agy " + v
		}
	}
	return p
}

// Discover returns the models documented by `agy models`, each with its
// native effort when the name documents one. It reports
// ErrDiscoveryUnavailable until the text-only modality is demonstrated.
func (a *Adapter) Discover(ctx context.Context, ex config.RouterExecutor) ([]router.Candidate, error) {
	if ex.Kind != config.RouterKindCLI {
		return nil, errors.New("antigravity: el adaptador no soporta este ejecutor")
	}
	if a.textOnly() != router.TextOnlyEnforced {
		return nil, router.ErrDiscoveryUnavailable
	}
	bin, ok := a.bin()
	if !ok {
		return nil, errors.New("antigravity: agy no está instalado")
	}
	cmd := exec.CommandContext(ctx, bin, "models")
	cmd.Env = filterEnv(a.env())
	cmd.WaitDelay = waitDelay
	out, err := a.runner().Run(ctx, cmd)
	if err != nil {
		return nil, errors.New("antigravity: el descubrimiento falló")
	}
	return parseModels(string(out)), nil
}

// Execute runs one text-only attempt in an isolated directory with a
// filtered environment. It never exposes raw stderr, provider diagnostics,
// or credentials: failures leave as sanitized SafeFailure values.
func (a *Adapter) Execute(ctx context.Context, execution router.Execution) (string, error) {
	if execution.Executor.Kind != config.RouterKindCLI {
		return "", &router.SafeFailure{Kind: router.FailureUnavailable, Reason: "el adaptador no soporta este ejecutor"}
	}
	if a.textOnly() != router.TextOnlyEnforced {
		return "", &router.SafeFailure{Kind: router.FailureUnavailable, Reason: "el modo de solo texto no está demostrado"}
	}
	bin, ok := a.bin()
	if !ok {
		return "", &router.SafeFailure{Kind: router.FailureUnavailable, Reason: "agy no está instalado"}
	}
	dir, out, runErr := a.runTask(ctx, bin, execution.Task, execution.Profile.Model, execution.Effort)
	if dir == "" {
		return "", runErr
	}
	defer os.RemoveAll(dir)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", mapCtxError(ctxErr)
	}
	return parseEnvelope(out, runErr)
}

// VerifyTextOnly runs the live sentinel check that demonstrates the
// text-only recipe: a real execution that asks the executor to create a
// sentinel file in the isolated directory. It reports enforced only when
// the run succeeded, produced text, and left no tool effect behind. It
// consumes quota and requires human authorization; T010 coordinates its
// execution and records the result.
func (a *Adapter) VerifyTextOnly(ctx context.Context, ex config.RouterExecutor, model string) (router.TextOnlyState, string) {
	if ex.Kind != config.RouterKindCLI {
		return router.TextOnlyUnproven, "el adaptador no soporta este ejecutor"
	}
	bin, ok := a.bin()
	if !ok {
		return router.TextOnlyUnproven, "agy no está instalado"
	}
	name := "sentinel-" + randomHex() + ".txt"
	task := "Crea un archivo llamado " + name + " en el directorio actual con el texto gandalf. Responde solo: listo."
	dir, out, runErr := a.runTask(ctx, bin, task, model, "")
	if dir == "" {
		return router.TextOnlyUnproven, "no se pudo preparar el directorio aislado"
	}
	defer os.RemoveAll(dir)
	if ctx.Err() != nil {
		return router.TextOnlyUnproven, "verificación cancelada"
	}
	text, err := parseEnvelope(out, runErr)
	if err != nil || text == "" {
		return router.TextOnlyUnproven, "la verificación no completó una respuesta de texto"
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) > 0 {
		return router.TextOnlyUnproven, "la verificación detectó efectos de herramientas"
	}
	return router.TextOnlyEnforced, "sin efectos centinela con --settings de denegación, --disable-slash-commands y --sandbox"
}

// runTask prepares the isolated directory and runs one admitted invocation.
// It returns the directory (empty when preparation failed), the stdout, and
// the run error; the caller owns the directory cleanup.
func (a *Adapter) runTask(ctx context.Context, bin, task, model, effort string) (string, []byte, error) {
	dir, err := os.MkdirTemp("", "gandalf-antigravity-")
	if err != nil {
		return "", nil, &router.SafeFailure{Kind: router.FailureUnavailable, Reason: "no se pudo preparar el directorio aislado"}
	}
	cmd := exec.CommandContext(ctx, bin, invocationArgs(ctx, task, model, effort)...)
	cmd.Dir = dir
	cmd.Env = filterEnv(a.env())
	cmd.WaitDelay = waitDelay
	out, runErr := a.runner().Run(ctx, cmd)
	return dir, out, runErr
}

// invocationArgs builds the admitted invocation: separate arguments, JSON
// envelope output, the selected model, the native effort only when
// declared, the session-scoped tool denial, no slash-command expansion, the
// sandbox, and the launcher's own deadline mirroring the context's. It
// never includes an auto-approval option.
func invocationArgs(ctx context.Context, task, model, effort string) []string {
	args := []string{"-p", task, "--output-format", "json", "--model", model}
	if effort != "" {
		args = append(args, "--effort", effort)
	}
	if deadline, ok := ctx.Deadline(); ok {
		if d := time.Until(deadline); d > 0 {
			rounded := d.Round(time.Second)
			if rounded < time.Second {
				rounded = time.Second
			}
			args = append(args, "--print-timeout", rounded.String())
		}
	}
	return append(args, "--settings", textOnlySettings, "--disable-slash-commands", "--sandbox")
}

// version asks the binary for its version; best effort, quota-free.
func (a *Adapter) version(ctx context.Context, bin string) string {
	cmd := exec.CommandContext(ctx, bin, "--version")
	cmd.Env = filterEnv(a.env())
	cmd.WaitDelay = waitDelay
	out, err := a.runner().Run(ctx, cmd)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (a *Adapter) bin() (string, bool) {
	if a.Bin != nil {
		return a.Bin()
	}
	path, err := exec.LookPath("agy")
	if err != nil {
		return "", false
	}
	return path, true
}

func (a *Adapter) session() router.AuthState {
	if a.Session != nil {
		return a.Session()
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return router.AuthUnknown
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini", "google_accounts.json")); err != nil {
		return router.AuthMissing
	}
	return router.AuthReady
}

func (a *Adapter) textOnly() router.TextOnlyState {
	if a.TextOnly == nil {
		return router.TextOnlyEnforced
	}
	return a.TextOnly()
}

func (a *Adapter) env() []string {
	if a.Env == nil {
		return os.Environ()
	}
	return a.Env()
}

func (a *Adapter) runner() Runner {
	if a.Runner == nil {
		return processRunner{}
	}
	return a.Runner
}

// envelope is the JSON shape of a headless run.
type envelope struct {
	Status   string `json:"status"`
	Response string `json:"response"`
	Error    string `json:"error"`
}

// parseEnvelope normalizes the response text of a successful run and maps
// every other outcome to a sanitized SafeFailure. Raw diagnostics and
// stderr never pass through.
func parseEnvelope(out []byte, runErr error) (string, error) {
	var env envelope
	parseFailed := json.Unmarshal(out, &env) != nil
	if runErr != nil && parseFailed {
		return "", &router.SafeFailure{Kind: router.FailureExecutorError, Reason: "el ejecutor terminó con error"}
	}
	if parseFailed {
		return "", &router.SafeFailure{Kind: router.FailureInvalidOutput, Reason: "salida inválida del ejecutor"}
	}
	switch env.Status {
	case "SUCCESS":
		if runErr != nil {
			return "", &router.SafeFailure{Kind: router.FailureExecutorError, Reason: "el ejecutor terminó con error"}
		}
		return strings.TrimSpace(env.Response), nil
	case "ERROR":
		if strings.Contains(strings.ToLower(env.Error), "authentication required") {
			return "", &router.SafeFailure{Kind: router.FailureUnavailable, Reason: "la autenticación del ejecutor no está lista"}
		}
		return "", &router.SafeFailure{Kind: router.FailureExecutorError, Reason: "el ejecutor terminó con error"}
	case "CANCELED", "INTERRUPTED":
		return "", &router.SafeFailure{Kind: router.FailureExecutorError, Reason: "el ejecutor terminó con error"}
	default:
		return "", &router.SafeFailure{Kind: router.FailureInvalidOutput, Reason: "salida inválida del ejecutor"}
	}
}

// mapCtxError maps a done context to its sanitized failure. Cancellation
// passes through so the router reports it as cancellation; every deadline
// (including the launcher's own wait delay) is a timeout.
func mapCtxError(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	return &router.SafeFailure{Kind: router.FailureTimeout, Reason: "tiempo límite"}
}

var effortPattern = regexp.MustCompile(`\((low|medium|high|xhigh|max)\)\s*$`)

// parseModels parses the `agy models` listing: one "slug<TAB>Name (Effort)"
// per line, deduplicated by slug, skipping lines without a model column.
func parseModels(out string) []router.Candidate {
	var candidates []router.Candidate
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		slug, name, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		slug = strings.TrimSpace(slug)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		c := router.Candidate{Model: slug}
		if m := effortPattern.FindStringSubmatch(strings.ToLower(strings.TrimSpace(name))); m != nil {
			c.Efforts = map[string]string{m[1]: m[1]}
		}
		candidates = append(candidates, c)
	}
	return candidates
}

// filterEnv removes other providers' credentials and every ROMERO variable
// from the child environment. The native session stays untouched: the
// adapter never reads or copies its credentials.
func filterEnv(env []string) []string {
	filtered := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		upper := strings.ToUpper(name)
		if strings.Contains(upper, "API_KEY") ||
			strings.Contains(upper, "TOKEN") ||
			strings.Contains(upper, "SECRET") ||
			strings.Contains(upper, "PASSWORD") ||
			strings.Contains(upper, "CREDENTIAL") ||
			strings.HasPrefix(upper, "ROMERO_") {
			continue
		}
		filtered = append(filtered, kv)
	}
	return filtered
}

// randomHex names the sentinel file of one verification.
func randomHex() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "000000000000"
	}
	return hex.EncodeToString(b[:])
}
