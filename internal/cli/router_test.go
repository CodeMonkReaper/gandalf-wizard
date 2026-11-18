package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router"
)

// fakeRouterAdapter records invocations and answers with canned behavior.
type fakeRouterAdapter struct {
	mu       sync.Mutex
	invoked  []string
	probe    router.Probe
	response func(context.Context, router.Execution) (string, error)
}

func (f *fakeRouterAdapter) Probe(ctx context.Context, ex config.RouterExecutor) router.Probe {
	return f.probe
}

func (f *fakeRouterAdapter) Discover(ctx context.Context, ex config.RouterExecutor) ([]router.Candidate, error) {
	return nil, router.ErrDiscoveryUnavailable
}

func (f *fakeRouterAdapter) Execute(ctx context.Context, execution router.Execution) (string, error) {
	f.mu.Lock()
	f.invoked = append(f.invoked, execution.Profile.ID)
	f.mu.Unlock()
	if f.response == nil {
		return "texto de respuesta", nil
	}
	return f.response(ctx, execution)
}

func (f *fakeRouterAdapter) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.invoked)
}

func okProbe() router.Probe {
	return router.Probe{Installed: true, Auth: router.AuthReady, TextOnly: router.TextOnlyEnforced}
}

// withRouterConfig writes the global executors and project profiles.
func withRouterConfig(t *testing.T, c *captured, executors, profiles string) {
	t.Helper()
	globalDir := filepath.Join(c.deps.UserConfigDir, "gandalf")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "config.json"), []byte(executors), 0o600); err != nil {
		t.Fatalf("WriteFile global: %v", err)
	}
	if err := os.WriteFile(filepath.Join(c.deps.WorkDir, ".gandalf.json"), []byte(profiles), 0o600); err != nil {
		t.Fatalf("WriteFile project: %v", err)
	}
}

const fiveExecutors = `{"router":{"executors":[
  {"id":"ok1","kind":"cli","authorized":true},
  {"id":"ok2","kind":"cli","authorized":true},
  {"id":"bad","kind":"cli","authorized":true},
  {"id":"noauth","kind":"cli","authorized":false},
  {"id":"unavail","kind":"cli","authorized":true}
]}}`

const fiveProfiles = `{"project":"demo","router":{"profiles":[
  {"id":"ok1/modelo-a","executor_id":"ok1","model":"modelo-a","efforts":{"low":"low","medium":"medium","high":"high"}},
  {"id":"ok2/modelo-b","executor_id":"ok2","model":"modelo-b"},
  {"id":"bad/modelo-c","executor_id":"bad","model":"modelo-c"},
  {"id":"noauth/modelo-d","executor_id":"noauth","model":"modelo-d"},
  {"id":"unavail/modelo-e","executor_id":"unavail","model":"modelo-e"}
]}}`

// registryWith wires fake adapters for the five test executors.
func registryWith(ok1, ok2, bad, unavail *fakeRouterAdapter) *router.Registry {
	reg := router.NewRegistry()
	reg.Register("ok1", ok1)
	reg.Register("ok2", ok2)
	reg.Register("bad", bad)
	reg.Register("unavail", unavail)
	reg.Register("noauth", &fakeRouterAdapter{probe: okProbe()})
	return reg
}

func standardRegistry() *router.Registry {
	return registryWith(
		&fakeRouterAdapter{probe: okProbe()},
		&fakeRouterAdapter{probe: okProbe()},
		&fakeRouterAdapter{probe: okProbe()},
		&fakeRouterAdapter{probe: router.Probe{Installed: false, Reason: "ejecutor no instalado"}},
	)
}

func runCLI(t *testing.T, c *captured, args ...string) int {
	t.Helper()
	c.stdout.Reset()
	c.stderr.Reset()
	return Run(context.Background(), args, c.deps)
}

func parseJSON(t *testing.T, out string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("salida no json: %v\n%s", err, out)
	}
	return m
}

// TestRouterCLI covers the router commands (AC1-AC12, AC15, AC17-AC19).
func TestRouterCLI(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "help muestra los comandos del router",
			run: func(t *testing.T) {
				c := newCaptured(t)
				if code := runCLI(t, c, "help"); code != ExitOK {
					t.Fatalf("exit inesperado: %d", code)
				}
				for _, want := range []string{"models list", "route", "run"} {
					if !strings.Contains(c.stdout.String(), want) {
						t.Fatalf("la ayuda no menciona %q:\n%s", want, c.stdout.String())
					}
				}
			},
		},
		{
			name: "models list tabla",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, fiveProfiles)
				c.deps.RouterRegistry = standardRegistry()
				if code := runCLI(t, c, "models", "list"); code != ExitOK {
					t.Fatalf("exit inesperado: %d stderr=%s", code, c.stderr.String())
				}
				out := c.stdout.String()
				for _, want := range []string{"ok1/modelo-a", "ok2/modelo-b", "disponible", "no autorizado", "no disponible", "declarado", "ok1", "modelo-a"} {
					if !strings.Contains(out, want) {
						t.Fatalf("la tabla no contiene %q:\n%s", want, out)
					}
				}
			},
		},
		{
			name: "models list json",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, fiveProfiles)
				c.deps.RouterRegistry = standardRegistry()
				if code := runCLI(t, c, "models", "list", "--json"); code != ExitOK {
					t.Fatalf("exit inesperado: %d", code)
				}
				m := parseJSON(t, c.stdout.String())
				if m["schema_version"] != "1" || m["command"] != "models list" {
					t.Fatalf("envoltorio inesperado: %v", m)
				}
				profiles, ok := m["profiles"].([]any)
				if !ok || len(profiles) != 5 {
					t.Fatalf("perfiles inesperados: %v", m["profiles"])
				}
				first, _ := profiles[0].(map[string]any)
				if first["id"] != "ok1/modelo-a" || first["executor"] != "ok1" || first["model"] != "modelo-a" {
					t.Fatalf("primer perfil inesperado: %v", first)
				}
				if first["authorized"] != true || first["availability"] != "disponible" || first["origin"] != "declarado" {
					t.Fatalf("estado del primer perfil inesperado: %v", first)
				}
				efforts, _ := first["efforts"].([]any)
				if len(efforts) != 3 {
					t.Fatalf("esfuerzos inesperados: %v", first["efforts"])
				}
				var noauth map[string]any
				for _, p := range profiles {
					if p.(map[string]any)["id"] == "noauth/modelo-d" {
						noauth = p.(map[string]any)
					}
				}
				if noauth == nil || noauth["authorized"] != false || noauth["availability"] != "no autorizado" {
					t.Fatalf("perfil no autorizado inesperado: %v", noauth)
				}
			},
		},
		{
			name: "models list sin router configurado",
			run: func(t *testing.T) {
				c := newCaptured(t)
				if code := runCLI(t, c, "models", "list", "--json"); code != ExitOK {
					t.Fatalf("exit inesperado: %d stderr=%s", code, c.stderr.String())
				}
				m := parseJSON(t, c.stdout.String())
				profiles, _ := m["profiles"].([]any)
				if len(profiles) != 0 {
					t.Fatalf("esperaba perfiles vacíos: %v", profiles)
				}
			},
		},
		{
			name: "models list error de configuracion",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, `{"router":{"executors":[{"id":"api-x","kind":"api","authorized":true}]}}`, fiveProfiles)
				if code := runCLI(t, c, "models", "list"); code != ExitConfigError {
					t.Fatalf("esperaba exit 3, got %d", code)
				}
			},
		},
		{
			name: "route elige y explica",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, fiveProfiles)
				c.deps.RouterRegistry = standardRegistry()
				if code := runCLI(t, c, "route", "--complexity", "alta", "Revisar el diseño"); code != ExitOK {
					t.Fatalf("exit inesperado: %d stderr=%s", code, c.stderr.String())
				}
				out := c.stdout.String()
				for _, want := range []string{"Perfil elegido: ok1/modelo-a", "esfuerzo: high", "complejidad alta", "Alternativas", "ok2/modelo-b", "no autorizado", "Fallback previsto: ok2/modelo-b"} {
					if !strings.Contains(out, want) {
						t.Fatalf("la explicación no contiene %q:\n%s", want, out)
					}
				}
			},
		},
		{
			name: "route determinista",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, fiveProfiles)
				c.deps.RouterRegistry = standardRegistry()
				runCLI(t, c, "route", "Revisar el diseño")
				first := c.stdout.String()
				runCLI(t, c, "route", "Revisar el diseño")
				if first != c.stdout.String() {
					t.Fatalf("route no es determinista:\n%s\n%s", first, c.stdout.String())
				}
			},
		},
		{
			name: "route json",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, fiveProfiles)
				c.deps.RouterRegistry = standardRegistry()
				if code := runCLI(t, c, "route", "--json", "--complexity", "alta", "Revisar el diseño"); code != ExitOK {
					t.Fatalf("exit inesperado: %d", code)
				}
				m := parseJSON(t, c.stdout.String())
				if m["schema_version"] != "1" || m["command"] != "route" {
					t.Fatalf("envoltorio inesperado: %v", m)
				}
				if m["profile"] != "ok1/modelo-a" || m["effort"] != "high" {
					t.Fatalf("elección inesperada: %v", m)
				}
				criteria, _ := m["criteria"].(string)
				if !strings.Contains(criteria, "complejidad alta") || !strings.Contains(criteria, "riesgo medio") {
					t.Fatalf("criterios inesperados: %q", criteria)
				}
				alts, _ := m["alternatives"].([]any)
				if len(alts) == 0 {
					t.Fatalf("sin alternativas: %v", m)
				}
				if m["fallback"] != "ok2/modelo-b" {
					t.Fatalf("fallback inesperado: %v", m["fallback"])
				}
			},
		},
		{
			name: "route rechaza complejidad invalida",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, fiveProfiles)
				if code := runCLI(t, c, "route", "--complexity", "turbo", "tarea"); code != ExitUsageError {
					t.Fatalf("esperaba exit 2, got %d", code)
				}
			},
		},
		{
			name: "route rechaza riesgo invalido",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, fiveProfiles)
				if code := runCLI(t, c, "route", "--risk", "mucho", "tarea"); code != ExitUsageError {
					t.Fatalf("esperaba exit 2, got %d", code)
				}
			},
		},
		{
			name: "route sin tarea",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, fiveProfiles)
				if code := runCLI(t, c, "route"); code != ExitUsageError {
					t.Fatalf("esperaba exit 2, got %d", code)
				}
			},
		},
		{
			name: "route sin elegibles",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, `{"project":"demo","router":{"profiles":[
  {"id":"noauth/modelo-d","executor_id":"noauth","model":"modelo-d"},
  {"id":"unavail/modelo-e","executor_id":"unavail","model":"modelo-e"}
]}}`)
				reg := router.NewRegistry()
				reg.Register("noauth", &fakeRouterAdapter{probe: okProbe()})
				reg.Register("unavail", &fakeRouterAdapter{probe: router.Probe{Installed: false}})
				c.deps.RouterRegistry = reg
				if code := runCLI(t, c, "route", "tarea"); code != ExitNoEligible {
					t.Fatalf("esperaba exit 8, got %d stderr=%s", code, c.stderr.String())
				}
				if !strings.Contains(c.stdout.String(), "no autorizado") {
					t.Fatalf("faltan los motivos de descarte:\n%s", c.stdout.String())
				}
			},
		},
		{
			name: "run ok json",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, fiveProfiles)
				c.deps.RouterRegistry = standardRegistry()
				if code := runCLI(t, c, "run", "--json", "Revisar el diseño"); code != ExitOK {
					t.Fatalf("exit inesperado: %d stderr=%s", code, c.stderr.String())
				}
				m := parseJSON(t, c.stdout.String())
				if m["schema_version"] != "1" || m["command"] != "run" || m["status"] != "ok" {
					t.Fatalf("envoltorio inesperado: %v", m)
				}
				if m["profile"] != "ok1/modelo-a" || m["effort"] != "medium" {
					t.Fatalf("elección inesperada: %v", m)
				}
				attempts, _ := m["attempts"].([]any)
				if len(attempts) != 1 {
					t.Fatalf("intentos inesperados: %v", m["attempts"])
				}
				a := attempts[0].(map[string]any)
				if a["outcome"] != "ok" || a["profile"] != "ok1/modelo-a" {
					t.Fatalf("intento inesperado: %v", a)
				}
				if m["output"] != "texto de respuesta" {
					t.Fatalf("salida inesperada: %v", m["output"])
				}
			},
		},
		{
			name: "run fallback",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, fiveProfiles)
				ok1 := &fakeRouterAdapter{probe: okProbe(), response: func(context.Context, router.Execution) (string, error) {
					return "", &router.SafeFailure{Kind: router.FailureExecutorError, Reason: "el ejecutor terminó con error"}
				}}
				ok2 := &fakeRouterAdapter{probe: okProbe()}
				reg := router.NewRegistry()
				reg.Register("ok1", ok1)
				reg.Register("ok2", ok2)
				reg.Register("bad", &fakeRouterAdapter{probe: okProbe()})
				reg.Register("unavail", &fakeRouterAdapter{probe: router.Probe{Installed: false}})
				reg.Register("noauth", &fakeRouterAdapter{probe: okProbe()})
				c.deps.RouterRegistry = reg
				if code := runCLI(t, c, "run", "--json", "Revisar el diseño"); code != ExitOK {
					t.Fatalf("exit inesperado: %d stderr=%s", code, c.stderr.String())
				}
				m := parseJSON(t, c.stdout.String())
				if m["status"] != "ok" || m["profile"] != "ok2/modelo-b" {
					t.Fatalf("resultado inesperado: %v", m)
				}
				attempts, _ := m["attempts"].([]any)
				if len(attempts) != 2 {
					t.Fatalf("esperaba 2 intentos: %v", attempts)
				}
				if attempts[0].(map[string]any)["outcome"] != "failed" || attempts[1].(map[string]any)["outcome"] != "ok" {
					t.Fatalf("intentos inesperados: %v", attempts)
				}
				expl, _ := m["explanation"].(string)
				if !strings.Contains(expl, "fallback") {
					t.Fatalf("la explicación no menciona el fallback: %q", expl)
				}
			},
		},
		{
			name: "run agotado",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, `{"project":"demo","router":{"profiles":[
  {"id":"ok1/modelo-a","executor_id":"ok1","model":"modelo-a"},
  {"id":"ok2/modelo-b","executor_id":"ok2","model":"modelo-b"}
]}}`)
				fail := &fakeRouterAdapter{probe: okProbe(), response: func(context.Context, router.Execution) (string, error) {
					return "", &router.SafeFailure{Kind: router.FailureExecutorError, Reason: "el ejecutor terminó con error"}
				}}
				reg := router.NewRegistry()
				reg.Register("ok1", fail)
				reg.Register("ok2", fail)
				c.deps.RouterRegistry = reg
				if code := runCLI(t, c, "run", "--json", "tarea"); code != ExitExhausted {
					t.Fatalf("esperaba exit 9, got %d", code)
				}
				m := parseJSON(t, c.stdout.String())
				if m["status"] != "exhausted" {
					t.Fatalf("estado inesperado: %v", m["status"])
				}
				attempts, _ := m["attempts"].([]any)
				if len(attempts) != 2 {
					t.Fatalf("esperaba los 2 intentos: %v", attempts)
				}
			},
		},
		{
			name: "run sin elegibles no ejecuta",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, `{"project":"demo","router":{"profiles":[
  {"id":"noauth/modelo-d","executor_id":"noauth","model":"modelo-d"}
]}}`)
				adapter := &fakeRouterAdapter{probe: okProbe()}
				reg := router.NewRegistry()
				reg.Register("noauth", adapter)
				c.deps.RouterRegistry = reg
				if code := runCLI(t, c, "run", "tarea"); code != ExitNoEligible {
					t.Fatalf("esperaba exit 8, got %d", code)
				}
				if adapter.count() != 0 {
					t.Fatal("se invocó un ejecutor sin perfiles elegibles")
				}
			},
		},
		{
			name: "run manual ok",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, fiveProfiles)
				ok1 := &fakeRouterAdapter{probe: okProbe()}
				ok2 := &fakeRouterAdapter{probe: okProbe()}
				reg := router.NewRegistry()
				reg.Register("ok1", ok1)
				reg.Register("ok2", ok2)
				reg.Register("bad", &fakeRouterAdapter{probe: okProbe()})
				reg.Register("unavail", &fakeRouterAdapter{probe: router.Probe{Installed: false}})
				reg.Register("noauth", &fakeRouterAdapter{probe: okProbe()})
				c.deps.RouterRegistry = reg
				if code := runCLI(t, c, "run", "--json", "--profile", "ok2/modelo-b", "tarea"); code != ExitOK {
					t.Fatalf("exit inesperado: %d stderr=%s", code, c.stderr.String())
				}
				m := parseJSON(t, c.stdout.String())
				if m["profile"] != "ok2/modelo-b" {
					t.Fatalf("perfil manual inesperado: %v", m["profile"])
				}
				attempts, _ := m["attempts"].([]any)
				if len(attempts) != 1 {
					t.Fatalf("el modo manual hizo más de un intento: %v", attempts)
				}
				if ok1.count() != 0 {
					t.Fatal("el modo manual invocó otro perfil")
				}
			},
		},
		{
			name: "run manual no elegible no sustituye",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, fiveProfiles)
				ok1 := &fakeRouterAdapter{probe: okProbe()}
				reg := router.NewRegistry()
				reg.Register("ok1", ok1)
				reg.Register("ok2", &fakeRouterAdapter{probe: okProbe()})
				reg.Register("bad", &fakeRouterAdapter{probe: okProbe()})
				reg.Register("unavail", &fakeRouterAdapter{probe: router.Probe{Installed: false}})
				reg.Register("noauth", &fakeRouterAdapter{probe: okProbe()})
				c.deps.RouterRegistry = reg
				if code := runCLI(t, c, "run", "--profile", "noauth/modelo-d", "tarea"); code != ExitNoEligible {
					t.Fatalf("esperaba exit 8, got %d", code)
				}
				if ok1.count() != 0 {
					t.Fatal("el perfil manual no elegible fue sustituido por otro")
				}
				if !strings.Contains(c.stdout.String(), "no autorizado") {
					t.Fatalf("falta el motivo del descarte manual:\n%s", c.stdout.String())
				}
			},
		},
		{
			name: "run cancelado",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, fiveProfiles)
				c.deps.RouterRegistry = standardRegistry()
				c.stdout.Reset()
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				code := Run(ctx, []string{"run", "--json", "tarea"}, c.deps)
				if code != ExitCanceled {
					t.Fatalf("esperaba exit 10, got %d", code)
				}
				m := parseJSON(t, c.stdout.String())
				if m["status"] != "canceled" {
					t.Fatalf("estado inesperado: %v", m["status"])
				}
			},
		},
		{
			name: "run redacta credenciales",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c,
					`{"router":{"executors":[{"id":"leak","kind":"api","authorized":true,"protocol":"mistral","base_url":"https://api.example","credential_env":"TEST_ROUTER_KEY"}]}}`,
					`{"project":"demo","router":{"profiles":[{"id":"leak/modelo-x","executor_id":"leak","model":"modelo-x"}]}}`)
				c.deps.Getenv = func(key string) string {
					if key == "TEST_ROUTER_KEY" {
						return "sentinel-router-key-123"
					}
					return ""
				}
				leak := &fakeRouterAdapter{probe: okProbe(), response: func(context.Context, router.Execution) (string, error) {
					return "respuesta con sentinel-router-key-123 dentro", nil
				}}
				reg := router.NewRegistry()
				reg.Register("leak", leak)
				c.deps.RouterRegistry = reg
				if code := runCLI(t, c, "run", "--json", "tarea"); code != ExitOK {
					t.Fatalf("exit inesperado: %d stderr=%s", code, c.stderr.String())
				}
				if strings.Contains(c.stdout.String(), "sentinel-router-key-123") {
					t.Fatalf("la salida filtra la credencial:\n%s", c.stdout.String())
				}
			},
		},
		{
			name: "doctor json con ejecutores",
			run: func(t *testing.T) {
				c := newCaptured(t)
				withRouterConfig(t, c, fiveExecutors, fiveProfiles)
				c.deps.RouterRegistry = standardRegistry()
				if code := runCLI(t, c, "doctor", "--json"); code != ExitDoctorFailed && code != ExitOK {
					t.Fatalf("exit inesperado: %d", code)
				}
				m := parseJSON(t, c.stdout.String())
				executors, ok := m["executors"].([]any)
				if !ok || len(executors) != 5 {
					t.Fatalf("ejecutores inesperados: %v", m["executors"])
				}
				first, _ := executors[0].(map[string]any)
				if first["id"] != "ok1" || first["installed"] != true || first["authorized"] != true || first["auth"] != "ready" {
					t.Fatalf("diagnóstico inesperado: %v", first)
				}
			},
		},
		{
			name: "doctor sin ejecutores conserva la forma",
			run: func(t *testing.T) {
				c := newCaptured(t)
				c.withProjectFile(t, "demo")
				if code := runCLI(t, c, "doctor", "--json"); code != ExitDoctorFailed && code != ExitOK {
					t.Fatalf("exit inesperado: %d", code)
				}
				m := parseJSON(t, c.stdout.String())
				if _, present := m["executors"]; present {
					t.Fatalf("sin ejecutores configurados no debe existir la clave executors: %v", m)
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
