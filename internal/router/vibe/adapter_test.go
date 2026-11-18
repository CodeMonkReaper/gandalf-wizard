package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router"
)

const fakeBin = `C:\fake\vibe.exe`

var testEnv = []string{
	"PATH=C:\\Windows\\System32",
	"SYSTEMROOT=C:\\Windows",
	"MISTRAL_API_KEY=secret-mistral-value",
	"ROMERO_API_KEY=secret-romero-value",
	"VIBE_SESSION=1",
	"VIBE_APP_SERVER_BIN=C:\\fake\\app-server.exe",
}

func cliExecutor() config.RouterExecutor {
	return config.RouterExecutor{ID: "vibe", Kind: config.RouterKindCLI, Authorized: true}
}

func apiExecutor() config.RouterExecutor {
	return config.RouterExecutor{ID: "api-x", Kind: config.RouterKindAPI, Authorized: true, Protocol: "mistral", BaseURL: "https://api.example"}
}

func execution(effort string) router.Execution {
	return router.Execution{
		Executor: cliExecutor(),
		Profile: config.RouterProfile{
			ID:         "vibe/mistral-large",
			ExecutorID: "vibe",
			Model:      "mistral-large-latest",
		},
		Effort: effort,
		Task:   "Resume el diseño del módulo de pagos",
	}
}

// fakeRunner records every prepared command and answers with canned output.
type fakeRunner struct {
	mu       sync.Mutex
	cmds     []*exec.Cmd
	response func(*exec.Cmd) ([]byte, error)
}

func (f *fakeRunner) Run(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
	f.mu.Lock()
	f.cmds = append(f.cmds, cmd)
	f.mu.Unlock()
	if f.response != nil {
		return f.response(cmd)
	}
	return nil, nil
}

func (f *fakeRunner) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.cmds)
}

func (f *fakeRunner) last() *exec.Cmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.cmds) == 0 {
		return nil
	}
	return f.cmds[len(f.cmds)-1]
}

// recordingRunner decorates a runner and records the isolated directories.
type recordingRunner struct {
	inner Runner
	mu    sync.Mutex
	dirs  []string
}

func (r *recordingRunner) Run(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
	r.mu.Lock()
	r.dirs = append(r.dirs, cmd.Dir)
	r.mu.Unlock()
	return r.inner.Run(ctx, cmd)
}

func (r *recordingRunner) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.dirs))
	copy(out, r.dirs)
	return out
}

func testAdapter(runner Runner) *Adapter {
	return &Adapter{
		Bin:      func() (string, bool) { return fakeBin, true },
		Session:  func() router.AuthState { return router.AuthReady },
		TextOnly: func() router.TextOnlyState { return router.TextOnlyEnforced },
		Env:      func() []string { return testEnv },
		Runner:   runner,
	}
}

// sessionMessages is a canned headless output: a JSON array of session
// entries where the last assistant message carries the final text.
func sessionMessages(final string) []byte {
	text, err := json.Marshal(final)
	if err != nil {
		panic(err)
	}
	return []byte(`[
  {"type":"message","role":"user","content":[{"type":"text","text":"tarea"}]},
  {"type":"reasoning","text":"pensando"},
  {"type":"message","role":"assistant","content":[{"type":"text","text":` + string(text) + `}]}
]`)
}

func hasPair(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func valueOf(args []string, flag string) (string, bool) {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1], true
		}
	}
	return "", false
}

func envHas(env []string, prefix string) bool {
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return true
		}
	}
	return false
}

const helperSrc = `package main

import "time"

func main() { time.Sleep(300 * time.Second) }
`

const exitSrc = `package main

import "os"

func main() { os.Exit(3) }
`

// buildHelperSrc compiles a fake vibe from src under the given name, so the
// real process runner can prove exit-code, cancellation and cleanup
// semantics without a session or quota.
func buildHelperSrc(t *testing.T, src, name string) string {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "main.go")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatalf("escribir helper: %v", err)
	}
	bin := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	out, err := exec.Command("go", "build", "-o", bin, file).CombinedOutput()
	if err != nil {
		t.Fatalf("compilar helper: %v: %s", err, out)
	}
	return bin
}

// buildHelper compiles a fake vibe that ignores its arguments and hangs.
func buildHelper(t *testing.T) string {
	return buildHelperSrc(t, helperSrc, "fake-vibe")
}

// TestAdapter covers the Mistral Vibe adapter port (AC2, AC5, AC6, AC11-AC13, AC15, AC18).
func TestAdapter(t *testing.T) {
	helper := buildHelper(t)

	cases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "probe rechaza ejecutor no cli",
			run: func(t *testing.T) {
				f := &fakeRunner{}
				a := testAdapter(f)
				p := a.Probe(context.Background(), apiExecutor())
				if p.Installed {
					t.Fatal("un ejecutor api no debe reportarse instalado por este adaptador")
				}
				if p.Reason == "" {
					t.Fatal("falta el motivo de rechazo")
				}
				if f.count() != 0 {
					t.Fatal("el sondeo invocó el binario para un ejecutor no soportado")
				}
			},
		},
		{
			name: "probe reporta no instalado",
			run: func(t *testing.T) {
				a := testAdapter(&fakeRunner{})
				a.Bin = func() (string, bool) { return "", false }
				p := a.Probe(context.Background(), cliExecutor())
				if p.Installed || p.Reason == "" {
					t.Fatalf("sondeo sin binario inesperado: %+v", p)
				}
			},
		},
		{
			name: "probe reporta instalado con version y sesion",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) {
					if hasFlag(cmd.Args[1:], "--version") {
						return []byte("vibe 3.1.2\n"), nil
					}
					return nil, errors.New("llamada inesperada")
				}}
				a := testAdapter(f)
				p := a.Probe(context.Background(), cliExecutor())
				if !p.Installed || p.Auth != router.AuthReady || p.TextOnly != router.TextOnlyEnforced {
					t.Fatalf("sondeo inesperado: %+v", p)
				}
				if !strings.Contains(p.Reason, "3.1.2") {
					t.Fatalf("la version no figura en el motivo: %q", p.Reason)
				}
			},
		},
		{
			name: "probe sin sesion reporta unknown y omite version",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) {
					if hasFlag(cmd.Args[1:], "--version") {
						return []byte("vibe 3.1.2\n"), nil
					}
					return nil, errors.New("llamada inesperada")
				}}
				a := testAdapter(f)
				a.Session = func() router.AuthState { return router.AuthUnknown }
				p := a.Probe(context.Background(), cliExecutor())
				if p.Auth != router.AuthUnknown {
					t.Fatalf("sesion inesperada: %+v", p)
				}
				if p.Reason != "" {
					t.Fatalf("la version no debe enmascarar el motivo de autenticacion desconocida: %q", p.Reason)
				}
			},
		},
		{
			name: "probe con solo texto sin demostrar omite version",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) {
					if hasFlag(cmd.Args[1:], "--version") {
						return []byte("vibe 3.1.2\n"), nil
					}
					return nil, errors.New("llamada inesperada")
				}}
				a := testAdapter(f)
				a.TextOnly = func() router.TextOnlyState { return router.TextOnlyUnproven }
				p := a.Probe(context.Background(), cliExecutor())
				if p.TextOnly != router.TextOnlyUnproven || p.Reason != "" {
					t.Fatalf("sondeo sin demostrar inesperado: %+v", p)
				}
			},
		},
		{
			name: "textonly por defecto es enforced tras verificacion T010",
			run: func(t *testing.T) {
				a := New()
				if a.textOnly() != router.TextOnlyEnforced {
					t.Fatalf("esperaba TextOnlyEnforced por defecto, got %s", a.textOnly())
				}
			},
		},
		{
			name: "discover siempre unavailable sin descubrimiento documentado",
			run: func(t *testing.T) {
				f := &fakeRunner{}
				a := testAdapter(f)
				_, err := a.Discover(context.Background(), cliExecutor())
				if !errors.Is(err, router.ErrDiscoveryUnavailable) {
					t.Fatalf("esperaba ErrDiscoveryUnavailable, got %v", err)
				}
				if f.count() != 0 {
					t.Fatal("el descubrimiento invocó el binario sin descubrimiento documentado")
				}
			},
		},
		{
			name: "discover rechaza ejecutor no cli",
			run: func(t *testing.T) {
				f := &fakeRunner{}
				a := testAdapter(f)
				if _, err := a.Discover(context.Background(), apiExecutor()); err == nil {
					t.Fatal("esperaba error para ejecutor no soportado")
				}
				if f.count() != 0 {
					t.Fatal("el descubrimiento invocó el binario para un ejecutor no soportado")
				}
			},
		},
		{
			name: "execute bloquea sin solo texto demostrado",
			run: func(t *testing.T) {
				f := &fakeRunner{}
				a := testAdapter(f)
				a.TextOnly = func() router.TextOnlyState { return router.TextOnlyUnproven }
				_, err := a.Execute(context.Background(), execution(""))
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureUnavailable {
					t.Fatalf("esperaba SafeFailure unavailable, got %v", err)
				}
				if f.count() != 0 {
					t.Fatal("Execute invocó el binario sin modalidad demostrada")
				}
			},
		},
		{
			name: "execute rechaza ejecutor no cli",
			run: func(t *testing.T) {
				f := &fakeRunner{}
				a := testAdapter(f)
				ex := execution("")
				ex.Executor = apiExecutor()
				_, err := a.Execute(context.Background(), ex)
				var sf *router.SafeFailure
				if !errors.As(err, &sf) {
					t.Fatalf("esperaba SafeFailure, got %v", err)
				}
				if f.count() != 0 {
					t.Fatal("Execute invocó el binario para un ejecutor no soportado")
				}
			},
		},
		{
			name: "execute reporta no instalado",
			run: func(t *testing.T) {
				a := testAdapter(&fakeRunner{})
				a.Bin = func() (string, bool) { return "", false }
				_, err := a.Execute(context.Background(), execution(""))
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureUnavailable {
					t.Fatalf("esperaba SafeFailure unavailable, got %v", err)
				}
			},
		},
		{
			name: "execute sin sesion no lanza",
			run: func(t *testing.T) {
				f := &fakeRunner{}
				a := testAdapter(f)
				a.Session = func() router.AuthState { return router.AuthUnknown }
				_, err := a.Execute(context.Background(), execution(""))
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureUnavailable {
					t.Fatalf("esperaba SafeFailure unavailable, got %v", err)
				}
				if f.count() != 0 {
					t.Fatal("Execute lanzó el binario sin sesión comprobada")
				}
			},
		},
		{
			name: "execute construye la invocacion admitida",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) {
					if _, err := os.Stat(cmd.Dir); err != nil {
						t.Errorf("el directorio de ejecución no existía: %v", err)
					}
					outDir, ok := valueOf(cmd.Args[1:], "--output-dir")
					if !ok {
						t.Error("falta --output-dir")
					} else if _, err := os.Stat(outDir); err != nil {
						t.Errorf("el directorio de export no existía: %v", err)
					}
					if cmd.Cancel == nil {
						t.Error("el comando no configura Cancel para terminar al cancelar")
					}
					if cmd.WaitDelay <= 0 {
						t.Error("el comando no configura WaitDelay para esperar el árbol")
					}
					return sessionMessages("  respuesta final \n"), nil
				}}
				a := testAdapter(f)
				ex := execution("")
				text, err := a.Execute(context.Background(), ex)
				if err != nil {
					t.Fatalf("ejecución inesperada: %v", err)
				}
				if text != "respuesta final" {
					t.Fatalf("texto no normalizado: %q", text)
				}
				cmd := f.last()
				if cmd == nil {
					t.Fatal("el adaptador no invocó el binario")
				}
				if cmd.Path != fakeBin {
					t.Fatalf("ruta del binario inesperada: %q", cmd.Path)
				}
				args := cmd.Args[1:]
				if !hasPair(args, "--prompt", ex.Task) {
					t.Fatalf("falta la tarea con argumentos separados: %v", args)
				}
				if !hasPair(args, "--output", "json") {
					t.Fatalf("falta --output json: %v", args)
				}
				if !hasPair(args, "--model", "mistral-large-latest") {
					t.Fatalf("falta el alias de modelo: %v", args)
				}
				if !hasPair(args, "--agent", "ask") {
					t.Fatalf("falta el agente ask: %v", args)
				}
				if !hasPair(args, "--enabled-tools", noToolsPattern) {
					t.Fatalf("falta el patrón sin herramientas: %v", args)
				}
				if !hasPair(args, "--max-turns", "1") {
					t.Fatalf("falta el tope de turnos: %v", args)
				}
				if hasFlag(args, "--auto-approve") || hasFlag(args, "--smart-approve") || hasFlag(args, "--trust") {
					t.Fatalf("la invocación incluye autoaprobación o confianza automática: %v", args)
				}
				if envHas(cmd.Env, "MISTRAL_API_KEY") || envHas(cmd.Env, "ROMERO_API_KEY") {
					t.Fatalf("el entorno del hijo filtra credenciales: %v", cmd.Env)
				}
				if envHas(cmd.Env, "VIBE_") {
					t.Fatalf("el entorno del hijo hereda contexto de sesión Vibe: %v", cmd.Env)
				}
				if !envHas(cmd.Env, "PATH=") || !envHas(cmd.Env, "SYSTEMROOT=") {
					t.Fatalf("el entorno del hijo perdió variables del sistema: %v", cmd.Env)
				}
				outDir, _ := valueOf(args, "--output-dir")
				if outDir == "" || outDir == cmd.Dir {
					t.Fatalf("el directorio de export debe ser distinto y aislado: %q vs %q", outDir, cmd.Dir)
				}
				if _, err := os.Stat(cmd.Dir); !os.IsNotExist(err) {
					t.Fatalf("el directorio de ejecución no se limpió: %q", cmd.Dir)
				}
				if _, err := os.Stat(outDir); !os.IsNotExist(err) {
					t.Fatalf("el directorio de export no se limpió: %q", outDir)
				}
			},
		},
		{
			name: "execute sin plazo no envia time-limit",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) { return sessionMessages("ok"), nil }}
				a := testAdapter(f)
				if _, err := a.Execute(context.Background(), execution("")); err != nil {
					t.Fatalf("ejecución sin plazo inesperada: %v", err)
				}
				if f.count() == 0 {
					t.Fatal("el adaptador no invocó el binario")
				}
				if hasFlag(f.last().Args[1:], "--time-limit") {
					t.Fatal("se envió --time-limit sin plazo")
				}
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				defer cancel()
				if _, err := a.Execute(ctx, execution("")); err != nil {
					t.Fatalf("ejecución con plazo inesperada: %v", err)
				}
				if !hasFlag(f.last().Args[1:], "--time-limit") {
					t.Fatal("falta --time-limit con plazo")
				}
			},
		},
		{
			name: "execute omite el esfuerzo sin flag en vibe",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) { return sessionMessages("ok"), nil }}
				a := testAdapter(f)
				if _, err := a.Execute(context.Background(), execution("high")); err != nil {
					t.Fatalf("ejecución con esfuerzo inesperada: %v", err)
				}
				if f.last() == nil {
					t.Fatal("el adaptador no invocó el binario")
				}
				args := f.last().Args[1:]
				for _, flag := range []string{"--effort", "-e", "--effort-level"} {
					if hasFlag(args, flag) {
						t.Fatalf("vibe no declara esfuerzo pero se envió %s: %v", flag, args)
					}
				}
			},
		},
		{
			name: "execute extrae el ultimo mensaje asistente",
			run: func(t *testing.T) {
				out := `[
  {"type":"message","role":"assistant","content":[{"type":"text","text":"primera"}]},
  {"type":"message","role":"user","content":[{"type":"text","text":"tarea"}]},
  {"type":"message","role":"assistant","content":[{"type":"text","text":"segunda "},{"type":"text","text":"parte"}]}
]`
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) { return []byte(out), nil }}
				a := testAdapter(f)
				text, err := a.Execute(context.Background(), execution(""))
				if err != nil {
					t.Fatalf("ejecución inesperada: %v", err)
				}
				if text != "segunda parte" {
					t.Fatalf("texto final inesperado: %q", text)
				}
			},
		},
		{
			name: "execute trata salida invalida",
			run: func(t *testing.T) {
				for _, out := range []string{"esto no es json", "[]", `[{"type":"message","role":"user"}]`} {
					f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) { return []byte(out), nil }}
					a := testAdapter(f)
					_, err := a.Execute(context.Background(), execution(""))
					var sf *router.SafeFailure
					if !errors.As(err, &sf) || sf.Kind != router.FailureInvalidOutput {
						t.Fatalf("salida %q: esperaba invalid-output, got %v", out, err)
					}
				}
			},
		},
		{
			name: "execute trata salida no cero",
			run: func(t *testing.T) {
				for _, name := range []string{"arranque", "api", "limite"} {
					f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) {
						return nil, errors.New("exit status 2")
					}}
					a := testAdapter(f)
					_, err := a.Execute(context.Background(), execution(""))
					var sf *router.SafeFailure
					if !errors.As(err, &sf) || sf.Kind != router.FailureExecutorError {
						t.Fatalf("caso %s: esperaba executor-error, got %v", name, err)
					}
				}
			},
		},
		{
			name: "execute trata limite propio del ejecutor",
			run: func(t *testing.T) {
				rec := &recordingRunner{inner: processRunner{}}
				a := testAdapter(rec)
				a.Bin = func() (string, bool) { return buildHelperSrc(t, exitSrc, "fake-vibe-exit"), true }
				_, err := a.Execute(context.Background(), execution(""))
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureTimeout {
					t.Fatalf("esperaba SafeFailure timeout por salida 3, got %v", err)
				}
			},
		},
		{
			name: "execute cancelacion termina y limpia",
			run: func(t *testing.T) {
				rec := &recordingRunner{inner: processRunner{}}
				a := testAdapter(rec)
				a.Bin = func() (string, bool) { return helper, true }
				ctx, cancel := context.WithCancel(context.Background())
				time.AfterFunc(150*time.Millisecond, cancel)
				start := time.Now()
				_, err := a.Execute(ctx, execution(""))
				elapsed := time.Since(start)
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("esperaba context.Canceled, got %v", err)
				}
				if elapsed > 10*time.Second {
					t.Fatalf("la cancelación no terminó a tiempo: %s", elapsed)
				}
				dirs := rec.recorded()
				if len(dirs) == 0 {
					t.Fatal("sin directorio de ejecución registrado")
				}
				if _, statErr := os.Stat(dirs[0]); !os.IsNotExist(statErr) {
					t.Fatalf("el directorio de ejecución no se limpió: %q", dirs[0])
				}
			},
		},
		{
			name: "execute plazo vencido termina y limpia",
			run: func(t *testing.T) {
				rec := &recordingRunner{inner: processRunner{}}
				a := testAdapter(rec)
				a.Bin = func() (string, bool) { return helper, true }
				ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
				defer cancel()
				start := time.Now()
				_, err := a.Execute(ctx, execution(""))
				elapsed := time.Since(start)
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureTimeout {
					t.Fatalf("esperaba SafeFailure timeout, got %v", err)
				}
				if elapsed > 10*time.Second {
					t.Fatalf("el plazo no terminó a tiempo: %s", elapsed)
				}
				dirs := rec.recorded()
				if len(dirs) == 0 {
					t.Fatal("sin directorio de ejecución registrado")
				}
				if _, statErr := os.Stat(dirs[0]); !os.IsNotExist(statErr) {
					t.Fatalf("el directorio de ejecución no se limpió: %q", dirs[0])
				}
			},
		},
		{
			name: "verify textonly demostrado",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) {
					args := cmd.Args[1:]
					v, ok := valueOf(args, "--prompt")
					if !ok || !strings.Contains(v, "sentinel-") {
						t.Errorf("la verificación no pide el centinela: %v", args)
					}
					return sessionMessages("listo"), nil
				}}
				a := testAdapter(f)
				state, reason := a.VerifyTextOnly(context.Background(), cliExecutor(), "mistral-large-latest")
				if state != router.TextOnlyEnforced {
					t.Fatalf("esperaba enforced, got %s (%s)", state, reason)
				}
				if reason == "" {
					t.Fatal("falta el mecanismo demostrado")
				}
				if f.last() == nil {
					t.Fatal("la verificación no invocó el binario")
				}
				if _, err := os.Stat(f.last().Dir); !os.IsNotExist(err) {
					t.Fatal("la verificación no limpió su directorio")
				}
			},
		},
		{
			name: "verify textonly detecta efecto centinela",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) {
					if err := os.WriteFile(filepath.Join(cmd.Dir, "sentinel-efecto.txt"), []byte("x"), 0o644); err != nil {
						t.Errorf("preparar efecto centinela: %v", err)
					}
					return sessionMessages("listo"), nil
				}}
				a := testAdapter(f)
				state, reason := a.VerifyTextOnly(context.Background(), cliExecutor(), "mistral-large-latest")
				if state != router.TextOnlyUnproven {
					t.Fatalf("esperaba unproven ante efecto centinela, got %s (%s)", state, reason)
				}
			},
		},
		{
			name: "verify textonly demostrado tras bloqueo en turno 1 (exit 3) y confirmacion de texto",
			run: func(t *testing.T) {
				exitBin := buildHelperSrc(t, exitSrc, "fake-vibe-exit")
				calls := 0
				inner := processRunner{}
				r := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) {
					calls++
					if calls == 1 {
						args := cmd.Args[1:]
						v, ok := valueOf(args, "--prompt")
						if !ok || !strings.Contains(v, "sentinel-") {
							t.Errorf("la primera llamada no pide el centinela: %v", args)
						}
						exitCmd := exec.CommandContext(context.Background(), exitBin)
						return inner.Run(context.Background(), exitCmd)
					}
					args := cmd.Args[1:]
					v, ok := valueOf(args, "--prompt")
					if !ok || v != "Responde solo con la palabra: listo." {
						t.Errorf("la segunda llamada no confirma solo texto: %v", args)
					}
					return sessionMessages("listo"), nil
				}}
				a := testAdapter(r)
				state, reason := a.VerifyTextOnly(context.Background(), cliExecutor(), "zai-glm-5-3-flash")
				if state != router.TextOnlyEnforced {
					t.Fatalf("esperaba enforced tras bloqueo + confirmacion, got %s (%s)", state, reason)
				}
				if calls != 2 {
					t.Fatalf("esperaba 2 invocaciones en VerifyTextOnly, got %d", calls)
				}
				if !strings.Contains(reason, "intento bloqueado en turno único") {
					t.Fatalf("motivo inesperado: %q", reason)
				}
			},
		},
		{
			name: "verify textonly sin exito",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) { return nil, errors.New("boom") }}
				a := testAdapter(f)
				state, _ := a.VerifyTextOnly(context.Background(), cliExecutor(), "mistral-large-latest")
				if state != router.TextOnlyUnproven {
					t.Fatalf("esperaba unproven sin éxito, got %s", state)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t)
		})
	}
}
