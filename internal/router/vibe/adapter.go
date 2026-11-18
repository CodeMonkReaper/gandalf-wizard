// Package vibe adapts the Mistral Vibe CLI to the router port.
//
// The execution recipe enforces text-only output: the ask agent (whose
// tool calls are all denied in headless mode), an --enabled-tools pattern
// that matches no tool (Vibe deactivates every other tool), a single
// assistant turn, and an isolated export directory. Until that recipe is
// demonstrated against the real binary, the adapter reports text-only as
// unproven and blocks Execute before launching.
package vibe

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router"
)

// waitDelay bounds how long the adapter waits for the launcher's process
// tree to exit after cancellation or the attempt deadline.
const waitDelay = 5 * time.Second

// noToolsPattern is a tool pattern that matches no tool name: with
// --enabled-tools set, Vibe deactivates every other tool.
const noToolsPattern = `re:^gandalf-no-tools$`

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

// Adapter implements router.Adapter for the Mistral Vibe CLI. Every
// dependency is injected so the automated tests never need a session,
// the real binary, or quota.
type Adapter struct {
	// Bin locates the vibe binary. Nil uses the PATH lookup.
	Bin func() (string, bool)
	// Session reports the local login indicator. Nil uses the default:
	// presence of the CLI's cached whoami record, without reading it.
	Session func() router.AuthState
	// TextOnly reports the demonstrated text-only state. Nil reports
	// unproven, which blocks Execute before launching.
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
		return router.Probe{Reason: "vibe no está instalado"}
	}
	p := router.Probe{Installed: true, Auth: a.session(), TextOnly: a.textOnly()}
	if v := a.version(ctx, bin); v != "" {
		p.Reason = v
	}
	return p
}

// Discover reports that Vibe offers no documented discovery compatible
// with the catalog limits; the catalog then uses the declared profiles.
func (a *Adapter) Discover(ctx context.Context, ex config.RouterExecutor) ([]router.Candidate, error) {
	if ex.Kind != config.RouterKindCLI {
		return nil, errors.New("vibe: el adaptador no soporta este ejecutor")
	}
	return nil, router.ErrDiscoveryUnavailable
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
		return "", &router.SafeFailure{Kind: router.FailureUnavailable, Reason: "vibe no está instalado"}
	}
	if a.session() != router.AuthReady {
		return "", &router.SafeFailure{Kind: router.FailureUnavailable, Reason: "la autenticación del ejecutor no está lista"}
	}
	runDir, outDir, out, runErr := a.runTask(ctx, bin, execution.Task, execution.Profile.Model)
	if runDir == "" {
		return "", runErr
	}
	defer os.RemoveAll(runDir)
	defer os.RemoveAll(outDir)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", mapCtxError(ctxErr)
	}
	return parseSession(out, runErr)
}

// VerifyTextOnly runs the live sentinel check that demonstrates the
// text-only recipe: a real execution that asks the executor to create a
// sentinel file in the isolated run directory. It reports enforced only
// when the run succeeded, produced text, and left no tool effect behind.
// It consumes quota and requires human authorization; T010 coordinates
// its execution and records the result.
func (a *Adapter) VerifyTextOnly(ctx context.Context, ex config.RouterExecutor, model string) (router.TextOnlyState, string) {
	if ex.Kind != config.RouterKindCLI {
		return router.TextOnlyUnproven, "el adaptador no soporta este ejecutor"
	}
	bin, ok := a.bin()
	if !ok {
		return router.TextOnlyUnproven, "vibe no está instalado"
	}
	if a.session() != router.AuthReady {
		return router.TextOnlyUnproven, "la autenticación del ejecutor no está lista"
	}
	name := "sentinel-" + randomHex() + ".txt"
	task := "Crea un archivo llamado " + name + " en el directorio actual con el texto gandalf. Responde solo: listo."
	runDir, outDir, out, runErr := a.runTask(ctx, bin, task, model)
	if runDir == "" {
		return router.TextOnlyUnproven, "no se pudo preparar el directorio aislado"
	}
	defer os.RemoveAll(runDir)
	defer os.RemoveAll(outDir)
	if ctx.Err() != nil {
		return router.TextOnlyUnproven, "verificación cancelada"
	}
	text, err := parseSession(out, runErr)
	if err != nil || text == "" {
		return router.TextOnlyUnproven, "la verificación no completó una respuesta de texto"
	}
	entries, err := os.ReadDir(runDir)
	if err != nil || len(entries) > 0 {
		return router.TextOnlyUnproven, "la verificación detectó efectos de herramientas"
	}
	return router.TextOnlyEnforced, "sin efectos centinela con --agent ask, --enabled-tools sin coincidencias y --max-turns 1"
}

// runTask prepares the isolated run and export directories and runs one
// admitted invocation. It returns the directories (empty when preparation
// failed), the stdout, and the run error; the caller owns the cleanup.
func (a *Adapter) runTask(ctx context.Context, bin, task, model string) (string, string, []byte, error) {
	runDir, err := os.MkdirTemp("", "gandalf-vibe-")
	if err != nil {
		return "", "", nil, &router.SafeFailure{Kind: router.FailureUnavailable, Reason: "no se pudo preparar el directorio aislado"}
	}
	outDir, err := os.MkdirTemp("", "gandalf-vibe-out-")
	if err != nil {
		os.RemoveAll(runDir)
		return "", "", nil, &router.SafeFailure{Kind: router.FailureUnavailable, Reason: "no se pudo preparar el directorio aislado"}
	}
	cmd := exec.CommandContext(ctx, bin, invocationArgs(ctx, task, model, outDir)...)
	cmd.Dir = runDir
	cmd.Env = filterEnv(a.env())
	cmd.WaitDelay = waitDelay
	out, runErr := a.runner().Run(ctx, cmd)
	return runDir, outDir, out, runErr
}

// invocationArgs builds the admitted invocation: separate arguments, JSON
// output, the documented model alias, the ask agent, a tool pattern that
// deactivates every tool, one assistant turn, the launcher's own deadline
// mirroring the context's, and the isolated export directory. It never
// includes auto-approval, automatic trust, or effort (Vibe declares no
// effort flag).
func invocationArgs(ctx context.Context, task, model, outDir string) []string {
	args := []string{"--prompt", task, "--output", "json", "--model", model, "--agent", "ask", "--enabled-tools", noToolsPattern, "--max-turns", "1"}
	if deadline, ok := ctx.Deadline(); ok {
		if d := time.Until(deadline); d > 0 {
			secs := int(math.Ceil(d.Seconds()))
			if secs < 1 {
				secs = 1
			}
			args = append(args, "--time-limit", strconv.Itoa(secs))
		}
	}
	return append(args, "--output-dir", outDir)
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
	path, err := exec.LookPath("vibe")
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
	if _, err := os.Stat(filepath.Join(home, ".vibe", "whoami_cache.json")); err != nil {
		return router.AuthUnknown
	}
	return router.AuthReady
}

func (a *Adapter) textOnly() router.TextOnlyState {
	if a.TextOnly == nil {
		return router.TextOnlyUnproven
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

// sessionEntry is one entry of the headless JSON output: the session
// messages as a JSON array when the run ends.
type sessionEntry struct {
	Type    string        `json:"type"`
	Role    string        `json:"role"`
	Content []sessionPart `json:"content"`
}

type sessionPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// parseSession extracts the final text: the last assistant message of the
// session array, with its text parts joined. Every other outcome maps to
// a sanitized SafeFailure; raw diagnostics and stderr never pass through.
// The launcher's documented stopped-early exit (3) is a timeout, not an
// executor failure.
func parseSession(out []byte, runErr error) (string, error) {
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) && ee.ExitCode() == 3 {
			return "", &router.SafeFailure{Kind: router.FailureTimeout, Reason: "tiempo límite"}
		}
		return "", &router.SafeFailure{Kind: router.FailureExecutorError, Reason: "el ejecutor terminó con error"}
	}
	var entries []sessionEntry
	if json.Unmarshal(out, &entries) != nil {
		return "", &router.SafeFailure{Kind: router.FailureInvalidOutput, Reason: "salida inválida del ejecutor"}
	}
	final := ""
	for _, e := range entries {
		if e.Type != "message" || e.Role != "assistant" {
			continue
		}
		var sb strings.Builder
		for _, p := range e.Content {
			if p.Type == "text" {
				sb.WriteString(p.Text)
			}
		}
		final = sb.String()
	}
	if strings.TrimSpace(final) == "" {
		return "", &router.SafeFailure{Kind: router.FailureInvalidOutput, Reason: "salida inválida del ejecutor"}
	}
	return strings.TrimSpace(final), nil
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

// filterEnv removes other providers' credentials, every ROMERO variable,
// and the parent Vibe session context from the child environment. The
// native session stays untouched: the adapter never reads or copies its
// credentials.
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
			strings.HasPrefix(upper, "ROMERO_") ||
			strings.HasPrefix(upper, "VIBE_") {
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
