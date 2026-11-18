package cli

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router/direct"
)

// TestRouterAcceptance verifies the public contract of router-02 (AC1–AC19)
// through cli.Run with fake executors and injected clients: exit codes, JSON
// payloads, invocation counts, and ordering. No test consumes quota, uses
// real credentials, or touches the user's project.
func TestRouterAcceptance(t *testing.T) {
	t.Run("AC1 catalogo con autorizados y no autorizados", func(t *testing.T) {
		c := newCaptured(t)
		withRouterConfig(t, c, fiveExecutors, fiveProfiles)
		c.deps.RouterRegistry = standardRegistry()
		if code := runCLI(t, c, "models", "list", "--json"); code != ExitOK {
			t.Fatalf("exit inesperado: %d stderr=%s", code, c.stderr.String())
		}
		m := parseJSON(t, c.stdout.String())
		profiles, _ := m["profiles"].([]any)
		if len(profiles) != 5 {
			t.Fatalf("esperaba 5 perfiles: %v", m["profiles"])
		}
		first, _ := profiles[0].(map[string]any)
		if first["id"] != "ok1/modelo-a" || first["executor"] != "ok1" || first["model"] != "modelo-a" {
			t.Fatalf("primer perfil inesperado: %v", first)
		}
		if first["availability"] != "disponible" || first["origin"] != "declarado" || first["authorized"] != true {
			t.Fatalf("estado del primer perfil inesperado: %v", first)
		}
		efforts, _ := first["efforts"].([]any)
		if len(efforts) != 3 {
			t.Fatalf("esperaba 3 esfuerzos: %v", first["efforts"])
		}
		var noauth map[string]any
		for _, p := range profiles {
			if pm, _ := p.(map[string]any); pm["id"] == "noauth/modelo-d" {
				noauth = pm
			}
		}
		if noauth == nil || noauth["authorized"] != false || noauth["availability"] != "no autorizado" {
			t.Fatalf("perfil no autorizado inesperado: %v", noauth)
		}
	})

	t.Run("AC2 binario y credencial ausentes", func(t *testing.T) {
		c := newCaptured(t)
		executors := `{"router":{"executors":[
  {"id":"nobin","kind":"cli","authorized":true},
  {"id":"nokey","kind":"api","authorized":true,"protocol":"mistral","base_url":"https://api.example","credential_env":"ABSENT_KEY"}
]}}`
		profiles := `{"project":"demo","router":{"profiles":[
  {"id":"nobin/modelo-n","executor_id":"nobin","model":"modelo-n"},
  {"id":"nokey/modelo-k","executor_id":"nokey","model":"modelo-k"}
]}}`
		withRouterConfig(t, c, executors, profiles)
		reg := router.NewRegistry()
		reg.Register("nobin", &fakeRouterAdapter{probe: router.Probe{Installed: false, Reason: "ejecutor no instalado"}})
		reg.Register("nokey", &direct.Adapter{Getenv: func(string) string { return "" }})
		c.deps.RouterRegistry = reg
		if code := runCLI(t, c, "models", "list", "--json"); code != ExitOK {
			t.Fatalf("exit inesperado: %d stderr=%s", code, c.stderr.String())
		}
		m := parseJSON(t, c.stdout.String())
		profilesOut, _ := m["profiles"].([]any)
		if len(profilesOut) != 2 {
			t.Fatalf("esperaba 2 perfiles: %v", m["profiles"])
		}
		nobin, _ := profilesOut[0].(map[string]any)
		if nobin["availability"] != "no disponible" || nobin["reason"] != "ejecutor no instalado" {
			t.Fatalf("binario ausente inesperado: %v", nobin)
		}
		nokey, _ := profilesOut[1].(map[string]any)
		if nokey["availability"] != "no disponible" || nokey["reason"] != "falta la credencial" {
			t.Fatalf("credencial ausente inesperada: %v", nokey)
		}
	})

	t.Run("AC3 misma entrada produce la misma salida de route", func(t *testing.T) {
		c := newCaptured(t)
		withRouterConfig(t, c, fiveExecutors, fiveProfiles)
		c.deps.RouterRegistry = standardRegistry()
		runCLI(t, c, "route", "Revisar el diseño")
		first := c.stdout.String()
		runCLI(t, c, "route", "Revisar el diseño")
		if first != c.stdout.String() {
			t.Fatalf("route no es determinista:\n%s\n%s", first, c.stdout.String())
		}
	})

	t.Run("AC4 route explica criterio alternativas y fallback", func(t *testing.T) {
		c := newCaptured(t)
		withRouterConfig(t, c, fiveExecutors, fiveProfiles)
		c.deps.RouterRegistry = standardRegistry()
		if code := runCLI(t, c, "route", "--complexity", "alta", "Revisar el diseño"); code != ExitOK {
			t.Fatalf("exit inesperado: %d stderr=%s", code, c.stderr.String())
		}
		out := c.stdout.String()
		for _, want := range []string{
			"Perfil elegido: ok1/modelo-a",
			"esfuerzo: high",
			"complejidad alta",
			"ok2/modelo-b",
			"noauth/modelo-d",
			"no autorizado",
			"unavail/modelo-e",
			"no disponible",
			"Fallback previsto: ok2/modelo-b",
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("la explicación no contiene %q:\n%s", want, out)
			}
		}
	})

	t.Run("AC5 no autorizado nunca elegido ni invocado", func(t *testing.T) {
		c := newCaptured(t)
		withRouterConfig(t, c, fiveExecutors, fiveProfiles)
		noauth := &fakeRouterAdapter{probe: okProbe()}
		reg := router.NewRegistry()
		reg.Register("ok1", &fakeRouterAdapter{probe: okProbe()})
		reg.Register("ok2", &fakeRouterAdapter{probe: okProbe()})
		reg.Register("bad", &fakeRouterAdapter{probe: okProbe()})
		reg.Register("unavail", &fakeRouterAdapter{probe: router.Probe{Installed: false}})
		reg.Register("noauth", noauth)
		c.deps.RouterRegistry = reg
		if code := runCLI(t, c, "route", "tarea"); code != ExitOK {
			t.Fatalf("route exit inesperado: %d stderr=%s", code, c.stderr.String())
		}
		if strings.Contains(c.stdout.String(), "Perfil elegido: noauth/modelo-d") {
			t.Fatalf("route eligió un no autorizado:\n%s", c.stdout.String())
		}
		if code := runCLI(t, c, "run", "tarea"); code != ExitOK {
			t.Fatalf("run exit inesperado: %d stderr=%s", code, c.stderr.String())
		}
		if noauth.count() != 0 {
			t.Fatalf("run invocó al ejecutor no autorizado %d veces", noauth.count())
		}
	})

	t.Run("AC6 run ok con un solo intento", func(t *testing.T) {
		c := newCaptured(t)
		withRouterConfig(t, c, fiveExecutors, fiveProfiles)
		c.deps.RouterRegistry = standardRegistry()
		if code := runCLI(t, c, "run", "--json", "tarea"); code != ExitOK {
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
			t.Fatalf("esperaba 1 intento: %v", m["attempts"])
		}
		a, _ := attempts[0].(map[string]any)
		if a["profile"] != "ok1/modelo-a" || a["outcome"] != "ok" {
			t.Fatalf("intento inesperado: %v", a)
		}
		if m["output"] != "texto de respuesta" {
			t.Fatalf("salida inesperada: %v", m["output"])
		}
	})

	t.Run("AC7 fallback al segundo perfil", func(t *testing.T) {
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
		if code := runCLI(t, c, "run", "--json", "tarea"); code != ExitOK {
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
		first, _ := attempts[0].(map[string]any)
		second, _ := attempts[1].(map[string]any)
		if first["profile"] != "ok1/modelo-a" || first["outcome"] != "failed" || first["reason"] != "el ejecutor terminó con error" {
			t.Fatalf("primer intento inesperado: %v", first)
		}
		if second["profile"] != "ok2/modelo-b" || second["outcome"] != "ok" {
			t.Fatalf("segundo intento inesperado: %v", second)
		}
		expl, _ := m["explanation"].(string)
		if !strings.Contains(expl, "fallback") {
			t.Fatalf("la explicación no menciona el fallback: %q", expl)
		}
	})

	t.Run("AC8 agotamiento con codigo 9", func(t *testing.T) {
		c := newCaptured(t)
		executors := `{"router":{"executors":[
  {"id":"ok1","kind":"cli","authorized":true},
  {"id":"ok2","kind":"cli","authorized":true}
]}}`
		profiles := `{"project":"demo","router":{"profiles":[
  {"id":"ok1/modelo-a","executor_id":"ok1","model":"modelo-a"},
  {"id":"ok2/modelo-b","executor_id":"ok2","model":"modelo-b"}
]}}`
		withRouterConfig(t, c, executors, profiles)
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
		for _, a := range attempts {
			am, _ := a.(map[string]any)
			if am["outcome"] != "failed" {
				t.Fatalf("intentos inesperados: %v", attempts)
			}
		}
	})

	t.Run("AC9 sin elegibles codigo 8 sin invocar", func(t *testing.T) {
		c := newCaptured(t)
		executors := `{"router":{"executors":[
  {"id":"noauth","kind":"cli","authorized":false},
  {"id":"unavail","kind":"cli","authorized":true}
]}}`
		profiles := `{"project":"demo","router":{"profiles":[
  {"id":"noauth/modelo-d","executor_id":"noauth","model":"modelo-d"},
  {"id":"unavail/modelo-e","executor_id":"unavail","model":"modelo-e"}
]}}`
		withRouterConfig(t, c, executors, profiles)
		noauth := &fakeRouterAdapter{probe: okProbe()}
		unavail := &fakeRouterAdapter{probe: router.Probe{Installed: false}}
		reg := router.NewRegistry()
		reg.Register("noauth", noauth)
		reg.Register("unavail", unavail)
		c.deps.RouterRegistry = reg
		if code := runCLI(t, c, "route", "tarea"); code != ExitNoEligible {
			t.Fatalf("route esperaba exit 8, got %d", code)
		}
		if code := runCLI(t, c, "run", "tarea"); code != ExitNoEligible {
			t.Fatalf("run esperaba exit 8, got %d", code)
		}
		if noauth.count() != 0 || unavail.count() != 0 {
			t.Fatalf("se invocó un ejecutor sin perfiles elegibles: noauth=%d unavail=%d", noauth.count(), unavail.count())
		}
		out := c.stdout.String()
		if !strings.Contains(out, "no autorizado") || !strings.Contains(out, "no disponible") {
			t.Fatalf("faltan los motivos de descarte:\n%s", out)
		}
	})

	t.Run("AC10 manual inelegible sin sustitucion", func(t *testing.T) {
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
			t.Fatal("el perfil manual inelegible fue sustituido por otro")
		}
		if !strings.Contains(c.stdout.String(), "no autorizado") {
			t.Fatalf("falta el motivo del descarte manual:\n%s", c.stdout.String())
		}
	})

	t.Run("AC11 cancelacion codigo 10 sin fallback", func(t *testing.T) {
		c := newCaptured(t)
		executors := `{"router":{"executors":[
  {"id":"uno","kind":"cli","authorized":true},
  {"id":"dos","kind":"cli","authorized":true}
]}}`
		profiles := `{"project":"demo","router":{"profiles":[
  {"id":"uno/modelo-1","executor_id":"uno","model":"modelo-1"},
  {"id":"dos/modelo-2","executor_id":"dos","model":"modelo-2"}
]}}`
		withRouterConfig(t, c, executors, profiles)
		started := make(chan struct{})
		var startedOnce sync.Once
		block := func(ctx context.Context, _ router.Execution) (string, error) {
			startedOnce.Do(func() { close(started) })
			<-ctx.Done()
			return "", ctx.Err()
		}
		uno := &fakeRouterAdapter{probe: okProbe(), response: block}
		dos := &fakeRouterAdapter{probe: okProbe(), response: block}
		reg := router.NewRegistry()
		reg.Register("uno", uno)
		reg.Register("dos", dos)
		c.deps.RouterRegistry = reg

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-started
			cancel()
		}()
		code := Run(ctx, []string{"run", "--json", "tarea"}, c.deps)
		if code != ExitCanceled {
			t.Fatalf("esperaba exit 10, got %d", code)
		}
		m := parseJSON(t, c.stdout.String())
		if m["status"] != "canceled" {
			t.Fatalf("estado inesperado: %v", m["status"])
		}
		attempts, _ := m["attempts"].([]any)
		if len(attempts) != 1 {
			t.Fatalf("esperaba 1 intento: %v", attempts)
		}
		a, _ := attempts[0].(map[string]any)
		if a["profile"] != "uno/modelo-1" || a["outcome"] != "canceled" {
			t.Fatalf("intento inesperado: %v", a)
		}
		if dos.count() != 0 {
			t.Fatal("la cancelación lanzó el fallback")
		}
	})

	t.Run("AC12 esfuerzo omitido y explicado", func(t *testing.T) {
		c := newCaptured(t)
		withRouterConfig(t, c, fiveExecutors, fiveProfiles)
		var sentEffort string
		ok2 := &fakeRouterAdapter{probe: okProbe(), response: func(_ context.Context, e router.Execution) (string, error) {
			sentEffort = e.Effort
			return "respuesta sin esfuerzo", nil
		}}
		reg := router.NewRegistry()
		reg.Register("ok1", &fakeRouterAdapter{probe: okProbe()})
		reg.Register("ok2", ok2)
		reg.Register("bad", &fakeRouterAdapter{probe: okProbe()})
		reg.Register("unavail", &fakeRouterAdapter{probe: router.Probe{Installed: false}})
		reg.Register("noauth", &fakeRouterAdapter{probe: okProbe()})
		c.deps.RouterRegistry = reg
		if code := runCLI(t, c, "run", "--json", "--complexity", "alta", "--profile", "ok2/modelo-b", "tarea"); code != ExitOK {
			t.Fatalf("exit inesperado: %d stderr=%s", code, c.stderr.String())
		}
		m := parseJSON(t, c.stdout.String())
		if m["effort"] != "" {
			t.Fatalf("esperaba esfuerzo vacío: %v", m["effort"])
		}
		attempts, _ := m["attempts"].([]any)
		if len(attempts) != 1 {
			t.Fatalf("esperaba 1 intento: %v", attempts)
		}
		a, _ := attempts[0].(map[string]any)
		if a["effort"] != "" {
			t.Fatalf("el intento declara esfuerzo: %v", a)
		}
		if sentEffort != "" {
			t.Fatalf("se envió un esfuerzo no declarado: %q", sentEffort)
		}
		expl, _ := m["explanation"].(string)
		if !strings.Contains(expl, "sin esfuerzo") {
			t.Fatalf("la explicación no indica la omisión: %q", expl)
		}
	})

	t.Run("AC13 centinela ausente en salidas y archivos", func(t *testing.T) {
		const secret = "sentinel-acceptance-key"
		c := newCaptured(t)
		executors := `{"router":{"executors":[
  {"id":"leak","kind":"api","authorized":true,"protocol":"mistral","base_url":"https://api.example","credential_env":"TEST_ROUTER_KEY"}
]}}`
		profiles := `{"project":"demo","router":{"profiles":[
  {"id":"leak/modelo-x","executor_id":"leak","model":"modelo-x"}
]}}`
		withRouterConfig(t, c, executors, profiles)
		c.deps.Getenv = func(key string) string {
			if key == "TEST_ROUTER_KEY" {
				return secret
			}
			return ""
		}
		fail := false
		leak := &fakeRouterAdapter{probe: okProbe(), response: func(context.Context, router.Execution) (string, error) {
			if fail {
				return "", &router.SafeFailure{Kind: router.FailureExecutorError, Reason: "fallo con sentinel-acceptance-key dentro"}
			}
			return "respuesta con sentinel-acceptance-key dentro", nil
		}}
		reg := router.NewRegistry()
		reg.Register("leak", leak)
		c.deps.RouterRegistry = reg

		if code := runCLI(t, c, "models", "list", "--json"); code != ExitOK {
			t.Fatalf("models list exit inesperado: %d", code)
		}
		assertNoSecret(t, c, secret)
		if code := runCLI(t, c, "route", "tarea"); code != ExitOK {
			t.Fatalf("route exit inesperado: %d", code)
		}
		assertNoSecret(t, c, secret)
		if code := runCLI(t, c, "run", "--json", "tarea"); code != ExitOK {
			t.Fatalf("run exit inesperado: %d", code)
		}
		if !strings.Contains(c.stdout.String(), "[redactado]") {
			t.Fatalf("la salida exitosa no redactó el centinela:\n%s", c.stdout.String())
		}
		assertNoSecret(t, c, secret)
		fail = true
		if code := runCLI(t, c, "run", "--json", "tarea"); code != ExitExhausted {
			t.Fatalf("run fallido exit inesperado: %d", code)
		}
		assertNoSecret(t, c, secret)
		if code := runCLI(t, c, "doctor", "--json"); code != ExitDoctorFailed {
			t.Fatalf("doctor exit inesperado: %d", code)
		}
		assertNoSecret(t, c, secret)
	})

	t.Run("AC14 configuracion con credencial da 3", func(t *testing.T) {
		const secret = "sk-acceptance-123"
		c := newCaptured(t)
		globalDir := filepath.Join(c.deps.UserConfigDir, "gandalf")
		if err := os.MkdirAll(globalDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		globalJSON := `{"router":{"executors":[{"id":"api-x","kind":"api","api_key":"` + secret + `"}]}}`
		if err := os.WriteFile(filepath.Join(globalDir, "config.json"), []byte(globalJSON), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if code := runCLI(t, c, "models", "list"); code != ExitConfigError {
			t.Fatalf("esperaba exit 3, got %d", code)
		}
		if strings.Contains(c.stderr.String(), secret) {
			t.Fatalf("el error revela la credencial:\n%s", c.stderr.String())
		}
	})

	t.Run("AC15 doctor informa sin cuota ni escrituras", func(t *testing.T) {
		c := newCaptured(t)
		withRouterConfig(t, c, fiveExecutors, fiveProfiles)
		reg := router.NewRegistry()
		adapters := map[string]*countingAdapter{}
		for _, id := range []string{"ok1", "ok2", "bad", "noauth", "unavail"} {
			probe := okProbe()
			if id == "unavail" {
				probe = router.Probe{Installed: false, Reason: "ejecutor no instalado"}
			}
			a := &countingAdapter{inner: &fakeRouterAdapter{probe: probe}}
			adapters[id] = a
			reg.Register(id, a)
		}
		c.deps.RouterRegistry = reg

		before := snapshotTreeAcceptance(t, c.deps.WorkDir) + "\n--\n" + snapshotTreeAcceptance(t, c.deps.UserConfigDir)
		if code := runCLI(t, c, "doctor", "--json"); code != ExitDoctorFailed {
			t.Fatalf("doctor exit inesperado: %d stderr=%s", code, c.stderr.String())
		}
		after := snapshotTreeAcceptance(t, c.deps.WorkDir) + "\n--\n" + snapshotTreeAcceptance(t, c.deps.UserConfigDir)
		if before != after {
			t.Fatal("doctor escribió archivos fuera de lo previsto")
		}
		m := parseJSON(t, c.stdout.String())
		executors, _ := m["executors"].([]any)
		if len(executors) != 5 {
			t.Fatalf("esperaba 5 ejecutores: %v", m["executors"])
		}
		first, _ := executors[0].(map[string]any)
		if first["id"] != "ok1" || first["installed"] != true || first["authorized"] != true || first["auth"] != "ready" {
			t.Fatalf("diagnóstico inesperado: %v", first)
		}
		for id, a := range adapters {
			executes, discoveries := a.counts()
			if executes != 0 || discoveries != 0 {
				t.Fatalf("doctor invocó a %s: execute=%d discover=%d", id, executes, discoveries)
			}
		}
	})

	t.Run("AC16 proveedor compatible solo por configuracion", func(t *testing.T) {
		c := newCaptured(t)
		executors := `{"router":{"executors":[
  {"id":"api-other","kind":"api","authorized":true,"protocol":"mistral","base_url":"https://api.other.example","credential_env":"OTHER_API_KEY"}
]}}`
		profiles := `{"project":"demo","router":{"profiles":[
  {"id":"api-other/modelo-x","executor_id":"api-other","model":"modelo-x"}
]}}`
		withRouterConfig(t, c, executors, profiles)
		var gotURL, gotAuth string
		transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotURL = req.URL.String()
			gotAuth = req.Header.Get("Authorization")
			body := `{"choices":[{"message":{"content":"respuesta del proveedor compatible"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{},
			}, nil
		})
		reg := router.NewRegistry()
		reg.Register("api-other", &direct.Adapter{Transport: transport, Getenv: func(string) string { return "sentinel-other-key" }})
		c.deps.RouterRegistry = reg

		if code := runCLI(t, c, "models", "list", "--json"); code != ExitOK {
			t.Fatalf("models list exit inesperado: %d stderr=%s", code, c.stderr.String())
		}
		m := parseJSON(t, c.stdout.String())
		profilesOut, _ := m["profiles"].([]any)
		if len(profilesOut) != 1 {
			t.Fatalf("esperaba 1 perfil: %v", m["profiles"])
		}
		row, _ := profilesOut[0].(map[string]any)
		if row["id"] != "api-other/modelo-x" || row["executor"] != "api:mistral" || row["availability"] != "disponible" {
			t.Fatalf("perfil del proveedor compatible inesperado: %v", row)
		}
		if code := runCLI(t, c, "run", "--json", "tarea"); code != ExitOK {
			t.Fatalf("run exit inesperado: %d stderr=%s", code, c.stderr.String())
		}
		m = parseJSON(t, c.stdout.String())
		if m["status"] != "ok" || m["profile"] != "api-other/modelo-x" {
			t.Fatalf("resultado inesperado: %v", m)
		}
		if m["output"] != "respuesta del proveedor compatible" {
			t.Fatalf("salida inesperada: %v", m["output"])
		}
		if gotURL != "https://api.other.example/chat/completions" {
			t.Fatalf("URL inesperada: %s", gotURL)
		}
		if gotAuth != "Bearer sentinel-other-key" {
			t.Fatalf("autorización inesperada: %s", gotAuth)
		}
	})

	t.Run("AC17 foundation y codigos 0-7 en verde", func(t *testing.T) {
		c := newCaptured(t)
		if code := runCLI(t, c, "version"); code != ExitOK {
			t.Fatalf("version esperaba exit 0, got %d", code)
		}
		if !strings.Contains(c.stdout.String(), "gandalf 0.1.0") {
			t.Fatalf("salida de version inesperada: %s", c.stdout.String())
		}
		if code := runCLI(t, c, "comando-desconocido"); code != ExitUsageError {
			t.Fatalf("comando desconocido esperaba exit 2, got %d", code)
		}
		if code := runCLI(t, c, "doctor"); code != ExitDoctorFailed {
			t.Fatalf("doctor sin entorno esperaba exit 7, got %d", code)
		}
	})

	t.Run("AC18 timeout corta el primer intento antes del segundo", func(t *testing.T) {
		c := newCaptured(t)
		executors := `{"router":{"executors":[
  {"id":"slow","kind":"cli","authorized":true},
  {"id":"fast","kind":"cli","authorized":true}
]}}`
		profiles := `{"project":"demo","router":{"attempt_timeout":"200ms","profiles":[
  {"id":"slow/modelo-a","executor_id":"slow","model":"modelo-a"},
  {"id":"fast/modelo-b","executor_id":"fast","model":"modelo-b"}
]}}`
		withRouterConfig(t, c, executors, profiles)
		log := &eventLog{}
		slow := &fakeRouterAdapter{probe: okProbe(), response: func(ctx context.Context, _ router.Execution) (string, error) {
			log.add("slow-start")
			<-ctx.Done()
			log.add("slow-end")
			return "", ctx.Err()
		}}
		fast := &fakeRouterAdapter{probe: okProbe(), response: func(context.Context, router.Execution) (string, error) {
			log.add("fast-start")
			return "respuesta rápida", nil
		}}
		reg := router.NewRegistry()
		reg.Register("slow", slow)
		reg.Register("fast", fast)
		c.deps.RouterRegistry = reg

		if code := runCLI(t, c, "run", "--json", "tarea"); code != ExitOK {
			t.Fatalf("exit inesperado: %d stderr=%s", code, c.stderr.String())
		}
		m := parseJSON(t, c.stdout.String())
		if m["status"] != "ok" || m["profile"] != "fast/modelo-b" {
			t.Fatalf("resultado inesperado: %v", m)
		}
		attempts, _ := m["attempts"].([]any)
		if len(attempts) != 2 {
			t.Fatalf("esperaba 2 intentos: %v", attempts)
		}
		first, _ := attempts[0].(map[string]any)
		second, _ := attempts[1].(map[string]any)
		if first["profile"] != "slow/modelo-a" || first["outcome"] != "failed" || first["reason"] != "tiempo límite" {
			t.Fatalf("primer intento inesperado: %v", first)
		}
		if second["profile"] != "fast/modelo-b" || second["outcome"] != "ok" {
			t.Fatalf("segundo intento inesperado: %v", second)
		}
		events := log.snapshot()
		want := []string{"slow-start", "slow-end", "fast-start"}
		if !reflect.DeepEqual(events, want) {
			t.Fatalf("orden de eventos inesperado: %v", events)
		}
	})

	t.Run("AC19 riesgo alto descarta el no verificado", func(t *testing.T) {
		c := newCaptured(t)
		withRouterConfig(t, c, fiveExecutors, fiveProfiles)
		c.deps.RouterRegistry = standardRegistry()
		if code := runCLI(t, c, "route", "--risk", "alto", "tarea"); code != ExitNoEligible {
			t.Fatalf("esperaba exit 8, got %d", code)
		}
		out := c.stdout.String()
		if !strings.Contains(out, "Perfil elegido: ninguno") {
			t.Fatalf("esperaba perfil ninguno:\n%s", out)
		}
		if !strings.Contains(out, "ok1/modelo-a") || !strings.Contains(out, "no verificado") {
			t.Fatalf("falta el descarte por no verificado:\n%s", out)
		}
	})
}

// roundTripFunc adapts a function to http.RoundTripper for injected clients.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// countingAdapter wraps a fake adapter and counts its Execute and Discover
// calls: the acceptance suite proves that catalog and doctor never invoke
// an executor. Probe stays uncounted because probing is the local,
// quota-free path.
type countingAdapter struct {
	inner       *fakeRouterAdapter
	mu          sync.Mutex
	executes    int
	discoveries int
}

func (a *countingAdapter) Probe(ctx context.Context, ex config.RouterExecutor) router.Probe {
	return a.inner.Probe(ctx, ex)
}

func (a *countingAdapter) Discover(ctx context.Context, ex config.RouterExecutor) ([]router.Candidate, error) {
	a.mu.Lock()
	a.discoveries++
	a.mu.Unlock()
	return a.inner.Discover(ctx, ex)
}

func (a *countingAdapter) Execute(ctx context.Context, execution router.Execution) (string, error) {
	a.mu.Lock()
	a.executes++
	a.mu.Unlock()
	return a.inner.Execute(ctx, execution)
}

func (a *countingAdapter) counts() (int, int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.executes, a.discoveries
}

// eventLog records ordered adapter events across goroutines.
type eventLog struct {
	mu     sync.Mutex
	events []string
}

func (l *eventLog) add(event string) {
	l.mu.Lock()
	l.events = append(l.events, event)
	l.mu.Unlock()
}

func (l *eventLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.events...)
}

// assertNoSecret fails when the sentinel appears in stdout, stderr, or any
// file of the temporary environment.
func assertNoSecret(t *testing.T, c *captured, secret string) {
	t.Helper()
	if strings.Contains(c.stdout.String(), secret) {
		t.Fatalf("stdout filtra el centinela:\n%s", c.stdout.String())
	}
	if strings.Contains(c.stderr.String(), secret) {
		t.Fatalf("stderr filtra el centinela:\n%s", c.stderr.String())
	}
	for path, content := range fileContents(t, c.deps.WorkDir, c.deps.UserConfigDir) {
		if strings.Contains(content, secret) {
			t.Fatalf("el archivo %s filtra el centinela", path)
		}
	}
}

// fileContents reads every file under the given roots as path→content.
func fileContents(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			tree[path] = string(raw)
			return nil
		})
		if err != nil {
			t.Fatalf("WalkDir %s: %v", root, err)
		}
	}
	return tree
}
