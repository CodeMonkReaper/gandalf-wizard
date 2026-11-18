package antigravity

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

const fakeBin = `C:\fake\agy.exe`

var testEnv = []string{
	"PATH=C:\\Windows\\System32",
	"SYSTEMROOT=C:\\Windows",
	"MISTRAL_API_KEY=secret-mistral-value",
	"ROMERO_API_KEY=secret-romero-value",
	"GITHUB_TOKEN=secret-token-value",
}

func cliExecutor() config.RouterExecutor {
	return config.RouterExecutor{ID: "antigravity", Kind: config.RouterKindCLI, Authorized: true}
}

func apiExecutor() config.RouterExecutor {
	return config.RouterExecutor{ID: "api-x", Kind: config.RouterKindAPI, Authorized: true, Protocol: "mistral", BaseURL: "https://api.example"}
}

func execution(effort string) router.Execution {
	return router.Execution{
		Executor: cliExecutor(),
		Profile: config.RouterProfile{
			ID:         "antigravity/gemini-4-argon-medium",
			ExecutorID: "antigravity",
			Model:      "gemini-4-argon-medium",
			Efforts:    map[string]string{"medium": "medium"},
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

func successEnvelope(text string) []byte {
	b, _ := json.Marshal(map[string]string{"status": "SUCCESS", "response": text})
	return b
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

// buildHelper compiles a fake agy that ignores its arguments and hangs, so
// the real process runner can prove cancellation and cleanup semantics.
func buildHelper(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte(helperSrc), 0o644); err != nil {
		t.Fatalf("escribir helper: %v", err)
	}
	bin := filepath.Join(dir, "fake-agy")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	out, err := exec.Command("go", "build", "-o", bin, src).CombinedOutput()
	if err != nil {
		t.Fatalf("compilar helper: %v: %s", err, out)
	}
	return bin
}

// TestAdapter covers the Antigravity adapter port (AC2, AC5, AC6, AC11-AC13, AC15, AC18).
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
						return []byte("1.5.10\n"), nil
					}
					return nil, errors.New("llamada inesperada")
				}}
				a := testAdapter(f)
				p := a.Probe(context.Background(), cliExecutor())
				if !p.Installed || p.Auth != router.AuthReady || p.TextOnly != router.TextOnlyEnforced {
					t.Fatalf("sondeo inesperado: %+v", p)
				}
				if !strings.Contains(p.Reason, "1.5.10") {
					t.Fatalf("la version no figura en el motivo: %q", p.Reason)
				}
			},
		},
		{
			name: "probe sin sesion omite version",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) {
					if hasFlag(cmd.Args[1:], "--version") {
						return []byte("1.5.10\n"), nil
					}
					return nil, errors.New("llamada inesperada")
				}}
				a := testAdapter(f)
				a.Session = func() router.AuthState { return router.AuthMissing }
				p := a.Probe(context.Background(), cliExecutor())
				if p.Auth != router.AuthMissing {
					t.Fatalf("sesion inesperada: %+v", p)
				}
				if p.Reason != "" {
					t.Fatalf("la version no debe enmascarar el motivo de falta de sesion: %q", p.Reason)
				}
			},
		},
		{
			name: "probe con solo texto sin demostrar omite version",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) {
					if hasFlag(cmd.Args[1:], "--version") {
						return []byte("1.5.10\n"), nil
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
			name: "probe no autorizado no lanza version",
			run: func(t *testing.T) {
				f := &fakeRunner{}
				a := testAdapter(f)
				ex := cliExecutor()
				ex.Authorized = false
				p := a.Probe(context.Background(), ex)
				if f.count() != 0 {
					t.Fatalf("probe lanzó proceso en ejecutor no autorizado (%d llamadas)", f.count())
				}
				if p.Reason != "" {
					t.Fatalf("probe no autorizado no debe informar versión: %q", p.Reason)
				}
			},
		},
		{
			name: "discover requiere solo texto demostrado",
			run: func(t *testing.T) {
				f := &fakeRunner{}
				a := testAdapter(f)
				a.TextOnly = func() router.TextOnlyState { return router.TextOnlyUnproven }
				_, err := a.Discover(context.Background(), cliExecutor())
				if !errors.Is(err, router.ErrDiscoveryUnavailable) {
					t.Fatalf("esperaba ErrDiscoveryUnavailable, got %v", err)
				}
				if f.count() != 0 {
					t.Fatal("el descubrimiento invocó el binario sin modalidad demostrada")
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
			name: "discover parsea modelos con esfuerzos",
			run: func(t *testing.T) {
				out := "gemini-4-argon-medium\tGemini 4 Argon (Medium)\n" +
					"gemini-4-argon-high\tGemini 4 Argon (High)\n" +
					"gemini-4-argon-medium\tGemini 4 Argon (Medium)\n" +
					"gemini-pro-orchestrator-g1-pro\tOrchestrator (Argon)\n" +
					"Fetching available models...\n"
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) { return []byte(out), nil }}
				a := testAdapter(f)
				cs, err := a.Discover(context.Background(), cliExecutor())
				if err != nil {
					t.Fatalf("descubrimiento inesperado: %v", err)
				}
				if len(cs) != 3 {
					t.Fatalf("esperaba 3 candidatos tras dedup, got %d: %+v", len(cs), cs)
				}
				if cs[0].Model != "gemini-4-argon-medium" || cs[0].Efforts["medium"] != "medium" {
					t.Fatalf("candidato 0 inesperado: %+v", cs[0])
				}
				if cs[1].Model != "gemini-4-argon-high" || cs[1].Efforts["high"] != "high" {
					t.Fatalf("candidato 1 inesperado: %+v", cs[1])
				}
				if cs[2].Model != "gemini-pro-orchestrator-g1-pro" || len(cs[2].Efforts) != 0 {
					t.Fatalf("candidato 2 inesperado: %+v", cs[2])
				}
			},
		},
		{
			name: "discover falla cuando el proceso falla",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) { return nil, errors.New("boom") }}
				a := testAdapter(f)
				_, err := a.Discover(context.Background(), cliExecutor())
				if err == nil || errors.Is(err, router.ErrDiscoveryUnavailable) {
					t.Fatalf("esperaba fallo de descubrimiento, got %v", err)
				}
			},
		},
		{
			name: "execute bloquea sin solo texto demostrado",
			run: func(t *testing.T) {
				f := &fakeRunner{}
				a := testAdapter(f)
				a.TextOnly = func() router.TextOnlyState { return router.TextOnlyUnproven }
				_, err := a.Execute(context.Background(), execution("medium"))
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
			name: "execute construye la invocacion admitida",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) {
					if _, err := os.Stat(cmd.Dir); err != nil {
						t.Errorf("el directorio aislado no existía durante la ejecución: %v", err)
					}
					if cmd.Cancel == nil {
						t.Error("el comando no configura Cancel para terminar al cancelar")
					}
					if cmd.WaitDelay <= 0 {
						t.Error("el comando no configura WaitDelay para esperar el árbol")
					}
					return successEnvelope("  texto final \n"), nil
				}}
				a := testAdapter(f)
				ex := execution("medium")
				text, err := a.Execute(context.Background(), ex)
				if err != nil {
					t.Fatalf("ejecución inesperada: %v", err)
				}
				if text != "texto final" {
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
				if !hasPair(args, "-p", ex.Task) {
					t.Fatalf("falta la tarea con argumentos separados: %v", args)
				}
				if !hasPair(args, "--output-format", "json") {
					t.Fatalf("falta --output-format json: %v", args)
				}
				if !hasPair(args, "--model", "gemini-4-argon-medium") {
					t.Fatalf("falta el modelo seleccionado: %v", args)
				}
				if !hasPair(args, "--effort", "medium") {
					t.Fatalf("falta el esfuerzo nativo: %v", args)
				}
				if !hasPair(args, "--settings", textOnlySettings) {
					t.Fatalf("falta la denegación de herramientas: %v", args)
				}
				if !hasFlag(args, "--disable-slash-commands") {
					t.Fatalf("falta --disable-slash-commands: %v", args)
				}
				if !hasFlag(args, "--sandbox") {
					t.Fatalf("falta --sandbox: %v", args)
				}
				if hasFlag(args, "--dangerously-skip-permissions") {
					t.Fatal("la invocación incluye una opción de autoaprobación")
				}
				if envHas(cmd.Env, "MISTRAL_API_KEY") || envHas(cmd.Env, "ROMERO_API_KEY") || envHas(cmd.Env, "GITHUB_TOKEN") {
					t.Fatalf("el entorno del hijo filtra credenciales: %v", cmd.Env)
				}
				if !envHas(cmd.Env, "PATH=") || !envHas(cmd.Env, "SYSTEMROOT=") {
					t.Fatalf("el entorno del hijo perdió variables del sistema: %v", cmd.Env)
				}
				if _, err := os.Stat(cmd.Dir); !os.IsNotExist(err) {
					t.Fatalf("el directorio aislado no se limpió: %q", cmd.Dir)
				}
			},
		},
		{
			name: "execute sin esfuerzo no envia flag",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) { return successEnvelope("ok"), nil }}
				a := testAdapter(f)
				if _, err := a.Execute(context.Background(), execution("")); err != nil {
					t.Fatalf("ejecución inesperada: %v", err)
				}
				if f.last() == nil {
					t.Fatal("el adaptador no invocó el binario")
				}
				if hasFlag(f.last().Args[1:], "--effort") {
					t.Fatal("se envió --effort sin esfuerzo declarado")
				}
			},
		},
		{
			name: "execute refleja el plazo solo cuando existe",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) { return successEnvelope("ok"), nil }}
				a := testAdapter(f)
				if _, err := a.Execute(context.Background(), execution("")); err != nil {
					t.Fatalf("ejecución sin plazo inesperada: %v", err)
				}
				if f.count() == 0 {
					t.Fatal("el adaptador no invocó el binario")
				}
				if hasFlag(f.last().Args[1:], "--print-timeout") {
					t.Fatal("se envió --print-timeout sin plazo")
				}
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				defer cancel()
				if _, err := a.Execute(ctx, execution("")); err != nil {
					t.Fatalf("ejecución con plazo inesperada: %v", err)
				}
				if f.count() < 2 {
					t.Fatal("el adaptador no invocó el binario dos veces")
				}
				args := f.last().Args[1:]
				if !hasFlag(args, "--print-timeout") {
					t.Fatalf("falta --print-timeout con plazo: %v", args)
				}
			},
		},
		{
			name: "execute mapea estado de error del proveedor",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) {
					return []byte(`{"status":"ERROR","error":"internal"}`), nil
				}}
				a := testAdapter(f)
				_, err := a.Execute(context.Background(), execution(""))
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureExecutorError {
					t.Fatalf("esperaba SafeFailure executor-error, got %v", err)
				}
			},
		},
		{
			name: "execute detecta autenticacion requerida",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) {
					return []byte(`{"status":"ERROR","error":"authentication required"}`), nil
				}}
				a := testAdapter(f)
				_, err := a.Execute(context.Background(), execution(""))
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureUnavailable {
					t.Fatalf("esperaba SafeFailure unavailable, got %v", err)
				}
			},
		},
		{
			name: "execute trata salida mala",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) { return []byte("esto no es json"), nil }}
				a := testAdapter(f)
				_, err := a.Execute(context.Background(), execution(""))
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureInvalidOutput {
					t.Fatalf("esperaba SafeFailure invalid-output, got %v", err)
				}
			},
		},
		{
			name: "execute trata salida no cero",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) {
					return []byte("garbage"), &exec.ExitError{}
				}}
				a := testAdapter(f)
				_, err := a.Execute(context.Background(), execution(""))
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureExecutorError {
					t.Fatalf("esperaba SafeFailure executor-error, got %v", err)
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
					t.Fatal("sin directorio aislado registrado")
				}
				if _, statErr := os.Stat(dirs[0]); !os.IsNotExist(statErr) {
					t.Fatalf("el directorio aislado no se limpió: %q", dirs[0])
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
					t.Fatal("sin directorio aislado registrado")
				}
				if _, statErr := os.Stat(dirs[0]); !os.IsNotExist(statErr) {
					t.Fatalf("el directorio aislado no se limpió: %q", dirs[0])
				}
			},
		},
		{
			name: "verify textonly demostrado",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) {
					args := cmd.Args[1:]
					i := indexOf(args, "-p")
					if i < 0 || !strings.Contains(args[i+1], "sentinel-") {
						t.Errorf("la verificación no pide el centinela: %v", args)
					}
					return successEnvelope("listo"), nil
				}}
				a := testAdapter(f)
				state, reason := a.VerifyTextOnly(context.Background(), cliExecutor(), "gemini-4-argon-medium")
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
					return successEnvelope("listo"), nil
				}}
				a := testAdapter(f)
				state, reason := a.VerifyTextOnly(context.Background(), cliExecutor(), "gemini-4-argon-medium")
				if state != router.TextOnlyUnproven {
					t.Fatalf("esperaba unproven ante efecto centinela, got %s (%s)", state, reason)
				}
			},
		},
		{
			name: "verify textonly sin exito",
			run: func(t *testing.T) {
				f := &fakeRunner{response: func(cmd *exec.Cmd) ([]byte, error) { return nil, errors.New("boom") }}
				a := testAdapter(f)
				state, _ := a.VerifyTextOnly(context.Background(), cliExecutor(), "gemini-4-argon-medium")
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

func indexOf(args []string, flag string) int {
	for i, a := range args {
		if a == flag {
			return i
		}
	}
	return -1
}
