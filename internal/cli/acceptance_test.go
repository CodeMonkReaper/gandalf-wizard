package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/romero"
)

const sentinelKey = "SENTINEL-ROMERO-KEY-7f3a"

// faroServer levanta un ROMERO simulado con el dataset Faro (M1, F1) y
// un contador de peticiones recibidas.
func faroServer(t *testing.T, opts ...func(*faroServerConfig)) (*httptest.Server, *int32) {
	t.Helper()
	cfg := faroServerConfig{status: http.StatusOK, searchStatus: http.StatusOK, block: false}
	for _, opt := range opts {
		opt(&cfg)
	}
	var requests int32
	// release despierta a los handlers bloqueados antes de que el cleanup
	// cierre el servidor; el contexto del cliente puede no propagar la
	// desconexión hasta el lado servidor.
	release := make(chan struct{})
	blockUntilRelease := func(r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/memories/search", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if cfg.block {
			blockUntilRelease(r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(cfg.searchStatus)
		if cfg.searchStatus != http.StatusOK {
			w.Write([]byte(`{"error":"denied"}`))
			return
		}
		if cfg.emptySearch {
			w.Write([]byte(`{"candidates":[],"degraded_mode":false,"total_found":0}`))
			return
		}
		w.Write([]byte(`{"candidates":[{"id":"M1","summary":"Conservamos los informes para comparar entregas.","status":"active","epistemic_status":"observed"}],"degraded_mode":false,"total_found":1}`))
	})
	mux.HandleFunc("/api/v1/memories/M1", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if cfg.block {
			blockUntilRelease(r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"memory":{"ID":"M1","ProjectID":"faro","Status":"active","EpistemicStatus":"observed","PrimarySourceID":"F1"}}`))
	})
	mux.HandleFunc("/api/v1/system/status", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if cfg.block {
			blockUntilRelease(r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(cfg.status)
		if cfg.status == http.StatusOK {
			w.Write([]byte(`{"status":"ok","version":"1.2.3","database":"up","active_profile":"lite"}`))
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	return server, &requests
}

type faroServerConfig struct {
	status       int
	searchStatus int
	block        bool
	emptySearch  bool
}

// acceptanceDeps construye dependencias con credencial centinela y ROMERO real vía httptest.
func acceptanceDeps(t *testing.T, server *httptest.Server) *captured {
	t.Helper()
	c := newCaptured(t)
	c.deps.Getenv = func(key string) string {
		if key == config.EnvKeyRomeroAPIKey {
			return sentinelKey
		}
		return ""
	}
	c.deps.HTTPClient = server.Client()
	c.deps.LookPath = func(name string) (string, error) {
		return "", fmt.Errorf("not found: %s", name)
	}
	return c
}

func snapshotTreeAcceptance(t *testing.T, root string) string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		paths = append(paths, rel+"|"+strconv.FormatBool(d.IsDir())+"|"+d.Type().String())
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	sort.Strings(paths)
	return strings.Join(paths, "\n")
}

func TestAcceptance_AC1_to_AC13(t *testing.T) {
	t.Run("AC1 enrutamiento, ayuda y códigos 0/2", func(t *testing.T) {
		c := newCaptured(t)
		if code := Run(context.Background(), []string{"--help"}, c.deps); code != ExitOK {
			t.Errorf("--help exit = %d, want 0", code)
		}
		for _, sub := range []string{"version", "config show", "config init", "doctor", "memory query"} {
			if !strings.Contains(c.stdout.String(), sub) {
				t.Errorf("--help no lista %q", sub)
			}
		}

		c2 := newCaptured(t)
		if code := Run(context.Background(), []string{"desconocido"}, c2.deps); code != ExitUsageError || c2.stderr.Len() == 0 {
			t.Errorf("desconocido = %d, want 2 con stderr", code)
		}

		c3 := newCaptured(t)
		if code := Run(context.Background(), nil, c3.deps); code != ExitUsageError {
			t.Errorf("sin args exit = %d, want 2", code)
		}

		c4 := newCaptured(t)
		if code := Run(context.Background(), []string{"version"}, c4.deps); code != ExitOK || c4.stdout.String() != "gandalf 0.1.0\n" {
			t.Errorf("version = %d/%q, want 0/gandalf 0.1.0", code, c4.stdout.String())
		}
	})

	t.Run("AC2 precedencia flag > entorno > proyecto > global > predeterminado", func(t *testing.T) {
		c := newCaptured(t)
		c.withProjectFile(t, "proj-proj")
		globalDir := c.deps.UserConfigDir
		if err := os.MkdirAll(filepath.Join(globalDir, "gandalf"), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(globalDir, "gandalf", "config.json"), []byte(`{"project":"global-proj"}`), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		withEnv := func(kv map[string]string) func(string) string {
			return func(key string) string {
				if key == config.EnvKeyRomeroAPIKey {
					return sentinelKey
				}
				return kv[key]
			}
		}

		steps := []struct {
			name    string
			args    []string
			env     map[string]string
			wantRow string
		}{
			{"flag gana sobre todas", []string{"config", "show", "--project", "flag-proj"}, map[string]string{config.EnvKeyProject: "env-proj"}, "| project | flag-proj | flag |"},
			{"entorno gana sobre archivos", []string{"config", "show"}, map[string]string{config.EnvKeyProject: "env-proj"}, "| project | env-proj | entorno |"},
			{"proyecto gana sobre global", []string{"config", "show"}, nil, "| project | proj-proj | proyecto |"},
			{"global gana sobre predeterminado", []string{"config", "show"}, nil, "| project | proj-proj | proyecto |"},
		}
		for _, step := range steps {
			t.Run(step.name, func(t *testing.T) {
				c.deps.Getenv = withEnv(step.env)
				code := Run(context.Background(), step.args, c.deps)
				if code != ExitOK {
					t.Fatalf("exit = %d (stderr %q)", code, c.stderr.String())
				}
				if !strings.Contains(c.stdout.String(), step.wantRow) {
					t.Errorf("salida sin %q: %q", step.wantRow, c.stdout.String())
				}
			})
		}

		// Sin archivo de proyecto ni entorno: gana global.
		c.deps.Getenv = withEnv(nil)
		if err := os.Remove(filepath.Join(c.deps.WorkDir, config.ProjectConfigFileName)); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if code := Run(context.Background(), []string{"config", "show"}, c.deps); code != ExitOK {
			t.Fatalf("exit = %d", code)
		}
		if !strings.Contains(c.stdout.String(), "| project | global-proj | global |") {
			t.Errorf("sin proyecto ni entorno debe ganar global: %q", c.stdout.String())
		}

		// Sin nada: predeterminado (vacío).
		if err := os.Remove(filepath.Join(globalDir, "gandalf", "config.json")); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if code := Run(context.Background(), []string{"config", "show"}, c.deps); code != ExitOK {
			t.Fatalf("exit = %d", code)
		}
		if !strings.Contains(c.stdout.String(), "| project |  | predeterminado |") {
			t.Errorf("sin capas debe mostrar predeterminado: %q", c.stdout.String())
		}
	})

	t.Run("AC3 credencial centinela ausente de toda salida", func(t *testing.T) {
		server, _ := faroServer(t)
		c := acceptanceDeps(t, server)
		c.withProjectFile(t, "faro")

		runs := [][]string{
			{"help"},
			{"version"},
			{"version", "--json"},
			{"config", "show"},
			{"config", "show", "--json"},
			{"config", "init", "--project", "faro2"},
			{"config", "init", "--project", "faro"},
			{"doctor"},
			{"doctor", "--json"},
			{"memory", "query", "informes"},
			{"memory", "query", "--json", "informes"},
		}
		for _, args := range runs {
			c.stdout.Reset()
			c.stderr.Reset()
			Run(context.Background(), args, c.deps)
			if got := c.stdout.String() + c.stderr.String(); strings.Contains(got, sentinelKey) {
				t.Errorf("fuga de credencial en %v: %q", args, got)
			}
		}

		// Rutas de error también.
		deniedServer, _ := faroServer(t, func(cfg *faroServerConfig) { cfg.searchStatus = http.StatusUnauthorized })
		c.deps.HTTPClient = deniedServer.Client()
		errorRuns := [][]string{
			{"memory", "query", "informes", "--romero-url", deniedServer.URL},
		}
		for _, args := range errorRuns {
			c.stdout.Reset()
			c.stderr.Reset()
			Run(context.Background(), args, c.deps)
			if got := c.stdout.String() + c.stderr.String(); strings.Contains(got, sentinelKey) {
				t.Errorf("fuga de credencial en error %v: %q", args, got)
			}
		}

		// config show informa presencia, nunca el valor.
		c.deps.HTTPClient = server.Client()
		c.stdout.Reset()
		Run(context.Background(), []string{"config", "show"}, c.deps)
		if !strings.Contains(c.stdout.String(), "credencial (ROMERO_API_KEY) | presente | entorno |") {
			t.Errorf("config show debe informar «presente»: %q", c.stdout.String())
		}
	})

	t.Run("AC4 config init exclusivo y validado", func(t *testing.T) {
		c := newCaptured(t)
		if code := Run(context.Background(), []string{"config", "init", "--project", "faro"}, c.deps); code != ExitOK {
			t.Fatalf("exit = %d, want 0", code)
		}
		data, err := os.ReadFile(filepath.Join(c.deps.WorkDir, config.ProjectConfigFileName))
		if err != nil || string(data) != `{"project":"faro"}` {
			t.Fatalf("archivo = %q, %v", string(data), err)
		}

		if code := Run(context.Background(), []string{"config", "init", "--project", "faro"}, c.deps); code != ExitConfigError {
			t.Errorf("exit = %d, want 3", code)
		}
		data, _ = os.ReadFile(filepath.Join(c.deps.WorkDir, config.ProjectConfigFileName))
		if string(data) != `{"project":"faro"}` {
			t.Errorf("archivo modificado: %q", string(data))
		}

		c2 := newCaptured(t)
		if code := Run(context.Background(), []string{"config", "init", "--project", "in valid!"}, c2.deps); code != ExitConfigError {
			t.Errorf("exit = %d, want 3", code)
		}
		if _, err := os.Stat(filepath.Join(c2.deps.WorkDir, config.ProjectConfigFileName)); !os.IsNotExist(err) {
			t.Error("no debía crear archivo con id inválido")
		}
	})

	t.Run("AC5 config inválida detiene con 3 y cero llamadas ROMERO", func(t *testing.T) {
		server, requests := faroServer(t)
		c := acceptanceDeps(t, server)

		cases := []struct {
			name    string
			content string
		}{
			{"JSON inválido", `{"project":`},
			{"URL inválida", `{"romero":{"base_url":"ftp://x"}}`},
			{"timeout inválido", `{"romero":{"timeout":"abc"}}`},
			{"clave de credencial", `{"api_key":"no-decir"}`},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				c2 := newCaptured(t)
				c2.deps.Getenv = c.deps.Getenv
				c2.deps.HTTPClient = server.Client()
				if err := os.WriteFile(filepath.Join(c2.deps.WorkDir, config.ProjectConfigFileName), []byte(tc.content), 0o600); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
				before := atomic.LoadInt32(requests)
				if code := Run(context.Background(), []string{"config", "show"}, c2.deps); code != ExitConfigError {
					t.Errorf("config show exit = %d, want 3", code)
				}
				if !strings.Contains(c2.stderr.String(), config.ProjectConfigFileName) {
					t.Errorf("stderr debe citar el archivo: %q", c2.stderr.String())
				}
				if code := Run(context.Background(), []string{"memory", "query", "pregunta"}, c2.deps); code != ExitConfigError {
					t.Errorf("memory query exit = %d, want 3", code)
				}
				if after := atomic.LoadInt32(requests); after != before {
					t.Errorf("llamadas a ROMERO con config inválida: %d → %d", before, after)
				}
			})
		}

		t.Run("clave desconocida avisa sin bloquear", func(t *testing.T) {
			c3 := newCaptured(t)
			c3.deps.Getenv = c.deps.Getenv
			if err := os.WriteFile(filepath.Join(c3.deps.WorkDir, config.ProjectConfigFileName), []byte(`{"project":"faro","extra":"x"}`), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			if code := Run(context.Background(), []string{"config", "show"}, c3.deps); code != ExitOK {
				t.Errorf("exit = %d, want 0", code)
			}
			if !strings.Contains(c3.stdout.String(), "Aviso") || !strings.Contains(c3.stdout.String(), "extra") {
				t.Errorf("debe avisar de la clave desconocida: %q", c3.stdout.String())
			}
		})
	})

	t.Run("AC6 doctor todo OK con ROMERO simulado", func(t *testing.T) {
		server, _ := faroServer(t)
		c := acceptanceDeps(t, server)
		c.withProjectFile(t, "faro")
		if err := os.WriteFile(filepath.Join(c.deps.WorkDir, ".rsc.json"), []byte(`{"version":1}`), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		c.deps.LookPath = func(name string) (string, error) {
			if name == "codex" || name == "vibe" {
				return "/bin/" + name, nil
			}
			return "", fmt.Errorf("not found")
		}

		code := Run(context.Background(), []string{"doctor", "--romero-url", server.URL}, c.deps)
		if code != ExitOK {
			t.Fatalf("exit = %d, want 0 (salida: %q)", code, c.stdout.String())
		}
		out := c.stdout.String()
		if strings.Count(out, "[OK]") != 6 || !strings.Contains(out, "Resultado: fallos=0 avisos=0") {
			t.Errorf("doctor = %q", out)
		}
	})

	t.Run("AC7 doctor con fallos ejecuta los 6 y sale 7", func(t *testing.T) {
		server, _ := faroServer(t, func(cfg *faroServerConfig) {
			cfg.status = http.StatusUnauthorized
			cfg.searchStatus = http.StatusUnauthorized
		})
		c := acceptanceDeps(t, server)

		code := Run(context.Background(), []string{"doctor", "--romero-url", server.URL}, c.deps)
		if code != ExitDoctorFailed {
			t.Fatalf("exit = %d, want 7", code)
		}
		out := c.stdout.String()
		for _, name := range []string{"configuración", "proyecto", "romero.url", "credencial", "romero.conexión", "harness"} {
			if !strings.Contains(out, name) {
				t.Errorf("check %s ausente: %q", name, out)
			}
		}
		if !strings.Contains(out, "→ ") {
			t.Errorf("los FAIL deben incluir remedio: %q", out)
		}
	})

	t.Run("AC8 doctor detecta harnesses sin escribir nada", func(t *testing.T) {
		server, _ := faroServer(t)
		c := acceptanceDeps(t, server)
		c.withProjectFile(t, "faro")
		if err := os.WriteFile(filepath.Join(c.deps.WorkDir, ".rsc.json"), []byte(`{"version":1}`), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		c.deps.LookPath = func(name string) (string, error) {
			if name == "codex" {
				return "/bin/codex", nil
			}
			return "", fmt.Errorf("not found")
		}
		before := snapshotTreeAcceptance(t, c.deps.WorkDir)

		code := Run(context.Background(), []string{"doctor", "--romero-url", server.URL}, c.deps)
		if code != ExitOK {
			t.Fatalf("exit = %d, want 0 (salida: %q)", code, c.stdout.String())
		}
		out := c.stdout.String()
		if !strings.Contains(out, "codex: detectado") || !strings.Contains(out, "vibe: no detectado") || !strings.Contains(out, "antigravity: no detectado") || !strings.Contains(out, "rsc: presente") {
			t.Errorf("detección harnesses = %q", out)
		}
		if after := snapshotTreeAcceptance(t, c.deps.WorkDir); after != before {
			t.Error("doctor modificó el árbol de archivos")
		}
	})

	t.Run("AC9 tabla de referencia 1 byte a byte", func(t *testing.T) {
		server, _ := faroServer(t)
		c := acceptanceDeps(t, server)
		c.withProjectFile(t, "faro")

		code := Run(context.Background(), []string{"memory", "query", "--romero-url", server.URL, "¿Para qué conservamos los informes de prueba?"}, c.deps)
		if code != ExitOK {
			t.Fatalf("exit = %d, want 0 (stderr: %q)", code, c.stderr.String())
		}
		want := "Proyecto: faro\n\n" +
			"| Resumen atribuido a ROMERO | Estado declarado | Vigencia | Referencia del recuerdo | Referencia de la fuente original |\n" +
			"| --- | --- | --- | --- | --- |\n" +
			"| Conservamos los informes para comparar entregas. | observed | vigente | M1 | F1 |\n"
		if c.stdout.String() != want {
			t.Errorf("tabla = %q, want %q", c.stdout.String(), want)
		}
	})

	t.Run("AC10 JSON de referencia 2 y entries siempre lista", func(t *testing.T) {
		server, _ := faroServer(t)
		c := acceptanceDeps(t, server)
		c.withProjectFile(t, "faro")

		code := Run(context.Background(), []string{"memory", "query", "--json", "--romero-url", server.URL, "¿Para qué conservamos los informes de prueba?"}, c.deps)
		if code != ExitOK {
			t.Fatalf("exit = %d, want 0", code)
		}
		var payload struct {
			SchemaVersion string `json:"schema_version"`
			Command       string `json:"command"`
			Project       string `json:"project"`
			Status        string `json:"status"`
			Entries       []struct {
				Summary   string  `json:"summary"`
				SourceRef *string `json:"source_ref"`
			} `json:"entries"`
		}
		if err := json.Unmarshal(c.stdout.Bytes(), &payload); err != nil {
			t.Fatalf("json inválido: %v", err)
		}
		if payload.SchemaVersion != "1" || payload.Command != "memory query" || payload.Project != "faro" || payload.Status != "ok" {
			t.Errorf("payload = %+v", payload)
		}
		if len(payload.Entries) != 1 || payload.Entries[0].Summary != "Conservamos los informes para comparar entregas." {
			t.Errorf("entries = %+v", payload.Entries)
		}
		if payload.Entries[0].SourceRef == nil || *payload.Entries[0].SourceRef != "F1" {
			t.Errorf("source_ref = %+v, want F1", payload.Entries[0].SourceRef)
		}

		// Consulta sin coincidencias: entries debe serializar [] y no null.
		emptyServer, _ := faroServer(t, func(cfg *faroServerConfig) { cfg.emptySearch = true })
		c.stdout.Reset()
		Run(context.Background(), []string{"memory", "query", "--json", "--romero-url", emptyServer.URL, "sin-coincidencias-exactas-xyz"}, c.deps)
		if !strings.Contains(c.stdout.String(), `"entries": []`) {
			t.Errorf("entries vacío debe ser []: %q", c.stdout.String())
		}
	})

	t.Run("AC11 estados con etiquetas y códigos aprobados", func(t *testing.T) {
		server, _ := faroServer(t)
		c := acceptanceDeps(t, server)
		c.withProjectFile(t, "faro")

		// ok → 0
		if code := Run(context.Background(), []string{"memory", "query", "--romero-url", server.URL, "informes"}, c.deps); code != ExitOK {
			t.Errorf("ok exit = %d, want 0", code)
		}
		// empty → 0 con etiqueta
		emptyServer, _ := faroServer(t, func(cfg *faroServerConfig) { cfg.emptySearch = true })
		c.stdout.Reset()
		if code := Run(context.Background(), []string{"memory", "query", "--romero-url", emptyServer.URL, "zzz-sin-coincidencias"}, c.deps); code != ExitOK {
			t.Errorf("empty exit = %d, want 0", code)
		}
		if !strings.Contains(c.stdout.String(), romero.LabelNoMatches) {
			t.Errorf("empty debe mostrar %q: %q", romero.LabelNoMatches, c.stdout.String())
		}

		// access_denied → 4 sin contenido ni IDs
		deniedServer, _ := faroServer(t, func(cfg *faroServerConfig) { cfg.searchStatus = http.StatusUnauthorized })
		c.deps.HTTPClient = deniedServer.Client()
		c.stdout.Reset()
		if code := Run(context.Background(), []string{"memory", "query", "--romero-url", deniedServer.URL, "informes"}, c.deps); code != ExitAccessDenied {
			t.Errorf("access_denied exit = %d, want 4", code)
		}
		if got := c.stdout.String(); strings.Contains(got, "M1") || strings.Contains(got, "Conservamos") || strings.Contains(got, "F1") {
			t.Errorf("denegación filtra contenido: %q", got)
		}
		if !strings.Contains(c.stdout.String(), "Acceso denegado") {
			t.Errorf("denegación debe mostrar la etiqueta: %q", c.stdout.String())
		}
		c.deps.HTTPClient = server.Client()

		// partial → 5 e incomplete → 6 vía adaptador fake (estados de red no simulables con un 200 limpio)
		c.deps.ReadPort = &romero.RomeroFakeAdapter{
			Records: []romero.FakeMemoryRecord{
				{MemoryID: "M1", ProjectID: "faro", Summary: "Uno.", EpistemicState: "observed", Status: "active", SourceRef: "F1"},
				{MemoryID: "M2", ProjectID: "faro", Summary: "Dos.", EpistemicState: "observed", Status: "active", SourceRef: "F2"},
			},
			InterruptDetailIDs: map[string]bool{"M2": true},
		}
		c.stdout.Reset()
		if code := Run(context.Background(), []string{"memory", "query", "uno"}, c.deps); code != ExitPartialQuery {
			t.Errorf("partial exit = %d, want 5", code)
		}
		if !strings.Contains(c.stdout.String(), romero.LabelPartial) {
			t.Errorf("partial debe mostrar %q: %q", romero.LabelPartial, c.stdout.String())
		}

		c.deps.ReadPort = &romero.RomeroFakeAdapter{InterruptSearch: true}
		c.stdout.Reset()
		if code := Run(context.Background(), []string{"memory", "query", "x"}, c.deps); code != ExitIncompleteQuery {
			t.Errorf("incomplete exit = %d, want 6", code)
		}
		if !strings.Contains(c.stdout.String(), romero.LabelIncomplete) {
			t.Errorf("incomplete debe mostrar %q: %q", romero.LabelIncomplete, c.stdout.String())
		}
		c.deps.ReadPort = nil

		// scope_required → 3: la CLI lo garantiza antes de llamar a ROMERO con proyecto vacío (cubierto en AC12);
		// su etiqueta canónica se verifica aquí a nivel de presentador.
		view := romero.Present(romero.MemoryQueryOutcome{Status: romero.StatusScopeRequired})
		if view.StatusText != "Identifica el proyecto activo antes de consultar ROMERO" {
			t.Errorf("etiqueta scope_required = %q", view.StatusText)
		}
	})

	t.Run("AC12 sin proyecto: cero peticiones y salida 3", func(t *testing.T) {
		server, requests := faroServer(t)
		c := acceptanceDeps(t, server)

		before := atomic.LoadInt32(requests)
		code := Run(context.Background(), []string{"memory", "query", "--romero-url", server.URL, "pregunta"}, c.deps)
		if code != ExitConfigError {
			t.Errorf("exit = %d, want 3", code)
		}
		if after := atomic.LoadInt32(requests); after != before {
			t.Errorf("peticiones recibidas: %d → %d, deben ser 0", before, after)
		}
		if !strings.Contains(c.stderr.String(), "Identifica el proyecto activo") {
			t.Errorf("stderr = %q", c.stderr.String())
		}
	})

	t.Run("AC13 timeout 1s termina en menos de 3s con código 6", func(t *testing.T) {
		server, _ := faroServer(t, func(cfg *faroServerConfig) { cfg.block = true })
		c := acceptanceDeps(t, server)
		c.withProjectFile(t, "faro")

		start := time.Now()
		code := Run(context.Background(), []string{"memory", "query", "--romero-url", server.URL, "--timeout", "1s", "pregunta"}, c.deps)
		elapsed := time.Since(start)

		if code != ExitIncompleteQuery {
			t.Errorf("exit = %d, want 6", code)
		}
		if elapsed >= 3*time.Second {
			t.Errorf("duración = %v, want < 3s", elapsed)
		}
		if !strings.Contains(c.stdout.String(), romero.LabelIncomplete) {
			t.Errorf("stdout = %q, debe decir %q", c.stdout.String(), romero.LabelIncomplete)
		}
	})
}
