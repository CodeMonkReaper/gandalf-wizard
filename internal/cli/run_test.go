package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/romero"
)

type captured struct {
	stdout *bytes.Buffer
	stderr *bytes.Buffer
	deps   Dependencies
}

func newCaptured(t *testing.T) *captured {
	t.Helper()
	c := &captured{
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}
	c.deps = Dependencies{
		Version:       "0.1.0",
		WorkDir:       t.TempDir(),
		UserConfigDir: t.TempDir(),
		Stdout:        c.stdout,
		Stderr:        c.stderr,
		Getenv:        func(string) string { return "" },
		LookPath:      func(string) (string, error) { return "", errors.New("not found") },
		DefaultCaller: func() string { return "tester" },
	}
	return c
}

func (c *captured) withProjectFile(t *testing.T, project string) {
	t.Helper()
	path := filepath.Join(c.deps.WorkDir, config.ProjectConfigFileName)
	if err := os.WriteFile(path, []byte(`{"project":"`+project+`"}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func (c *captured) withAPIKey() {
	c.deps.Getenv = func(key string) string {
		if key == config.EnvKeyRomeroAPIKey {
			return "sentinel-key"
		}
		return ""
	}
}

func faroReadPort() *romero.RomeroFakeAdapter {
	return &romero.RomeroFakeAdapter{
		Records: []romero.FakeMemoryRecord{
			{
				MemoryID:       "M1",
				ProjectID:      "faro",
				Summary:        "Conservamos los informes para comparar entregas.",
				EpistemicState: "observed",
				Status:         "active",
				SourceRef:      "F1",
				MatchQueries:   []string{"informes"},
			},
		},
	}
}

func TestCLIDispatch_HelpAndUsage(t *testing.T) {
	t.Run("help sale 0 y lista subcomandos", func(t *testing.T) {
		c := newCaptured(t)
		code := Run(context.Background(), []string{"help"}, c.deps)
		if code != ExitOK {
			t.Errorf("exit = %d, want 0", code)
		}
		for _, sub := range []string{"version", "config show", "config init", "doctor", "memory query"} {
			if !strings.Contains(c.stdout.String(), sub) {
				t.Errorf("help no lista %q: %q", sub, c.stdout.String())
			}
		}
	})

	t.Run("--help equivalente", func(t *testing.T) {
		c := newCaptured(t)
		if code := Run(context.Background(), []string{"--help"}, c.deps); code != ExitOK {
			t.Errorf("exit = %d, want 0", code)
		}
	})

	t.Run("sin argumentos sale 2 con uso en stderr", func(t *testing.T) {
		c := newCaptured(t)
		code := Run(context.Background(), nil, c.deps)
		if code != ExitUsageError {
			t.Errorf("exit = %d, want 2", code)
		}
		if c.stderr.Len() == 0 {
			t.Error("stderr vacío; debe mostrar el uso")
		}
	})

	t.Run("subcomando desconocido sale 2", func(t *testing.T) {
		c := newCaptured(t)
		code := Run(context.Background(), []string{"frobnicate"}, c.deps)
		if code != ExitUsageError {
			t.Errorf("exit = %d, want 2", code)
		}
		if !strings.Contains(c.stderr.String(), "frobnicate") {
			t.Errorf("stderr debe citar el subcomando: %q", c.stderr.String())
		}
	})

	t.Run("flag inválido sale 2", func(t *testing.T) {
		c := newCaptured(t)
		code := Run(context.Background(), []string{"version", "--no-existe"}, c.deps)
		if code != ExitUsageError {
			t.Errorf("exit = %d, want 2", code)
		}
	})
}

func TestCLIDispatch_Version(t *testing.T) {
	t.Run("humano", func(t *testing.T) {
		c := newCaptured(t)
		code := Run(context.Background(), []string{"version"}, c.deps)
		if code != ExitOK || c.stdout.String() != "gandalf 0.1.0\n" {
			t.Errorf("exit/stdout = %d/%q", code, c.stdout.String())
		}
	})

	t.Run("json", func(t *testing.T) {
		c := newCaptured(t)
		code := Run(context.Background(), []string{"version", "--json"}, c.deps)
		if code != ExitOK || !strings.Contains(c.stdout.String(), `"command": "version"`) {
			t.Errorf("exit/stdout = %d/%q", code, c.stdout.String())
		}
	})
}

func TestCLIDispatch_ConfigShow(t *testing.T) {
	t.Run("tabla con origen y credencial presente", func(t *testing.T) {
		c := newCaptured(t)
		c.withProjectFile(t, "faro")
		c.withAPIKey()

		code := Run(context.Background(), []string{"config", "show"}, c.deps)
		if code != ExitOK {
			t.Errorf("exit = %d, want 0 (stderr: %q)", code, c.stderr.String())
		}
		out := c.stdout.String()
		for _, want := range []string{"| project | faro | proyecto |", "credencial (ROMERO_API_KEY) | presente | entorno |"} {
			if !strings.Contains(out, want) {
				t.Errorf("salida sin %q: %q", want, out)
			}
		}
		if strings.Contains(out, "sentinel-key") {
			t.Error("config show filtra la credencial")
		}
	})

	t.Run("config inválida sale 3 sin llamar a ROMERO", func(t *testing.T) {
		c := newCaptured(t)
		if err := os.WriteFile(filepath.Join(c.deps.WorkDir, config.ProjectConfigFileName), []byte(`{"project":`), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		c.deps.ReadPort = &romero.RomeroFakeAdapter{}

		code := Run(context.Background(), []string{"config", "show"}, c.deps)
		if code != ExitConfigError {
			t.Errorf("exit = %d, want 3", code)
		}
		if !strings.Contains(c.stderr.String(), config.ProjectConfigFileName) {
			t.Errorf("stderr debe citar el archivo: %q", c.stderr.String())
		}
		if got := c.deps.ReadPort.(*romero.RomeroFakeAdapter).Operations(); len(got) != 0 {
			t.Errorf("config show no debe llamar a ROMERO; llamadas: %v", got)
		}
	})

	t.Run("subcomando config desconocido sale 2", func(t *testing.T) {
		c := newCaptured(t)
		if code := Run(context.Background(), []string{"config", "frob"}, c.deps); code != ExitUsageError {
			t.Errorf("exit = %d, want 2", code)
		}
	})
}

func TestCLIDispatch_ConfigInit(t *testing.T) {
	t.Run("crea y sale 0", func(t *testing.T) {
		c := newCaptured(t)
		code := Run(context.Background(), []string{"config", "init", "--project", "faro"}, c.deps)
		if code != ExitOK {
			t.Errorf("exit = %d, want 0 (stderr: %q)", code, c.stderr.String())
		}
		if _, err := os.Stat(filepath.Join(c.deps.WorkDir, config.ProjectConfigFileName)); err != nil {
			t.Errorf("archivo no creado: %v", err)
		}
	})

	t.Run("existente sale 3 sin modificar", func(t *testing.T) {
		c := newCaptured(t)
		c.withProjectFile(t, "original")
		code := Run(context.Background(), []string{"config", "init", "--project", "faro"}, c.deps)
		if code != ExitConfigError {
			t.Errorf("exit = %d, want 3", code)
		}
		data, _ := os.ReadFile(filepath.Join(c.deps.WorkDir, config.ProjectConfigFileName))
		if string(data) != `{"project":"original"}` {
			t.Errorf("archivo modificado: %q", string(data))
		}
	})

	t.Run("id inválido sale 3 sin crear", func(t *testing.T) {
		c := newCaptured(t)
		code := Run(context.Background(), []string{"config", "init", "--project", "in valid!"}, c.deps)
		if code != ExitConfigError {
			t.Errorf("exit = %d, want 3", code)
		}
		if _, err := os.Stat(filepath.Join(c.deps.WorkDir, config.ProjectConfigFileName)); !os.IsNotExist(err) {
			t.Error("no debía crear el archivo")
		}
	})

	t.Run("sin --project sale 2", func(t *testing.T) {
		c := newCaptured(t)
		if code := Run(context.Background(), []string{"config", "init"}, c.deps); code != ExitUsageError {
			t.Errorf("exit = %d, want 2", code)
		}
	})
}

func TestCLIDispatch_Doctor(t *testing.T) {
	okStatus := &romero.RomeroFakeStatusAdapter{Status: romero.SystemStatus{Status: "ok", Version: "1.2.3", ActiveProfile: "lite"}}

	t.Run("todo OK sale 0", func(t *testing.T) {
		c := newCaptured(t)
		c.withProjectFile(t, "faro")
		c.withAPIKey()
		c.deps.StatusPort = okStatus

		code := Run(context.Background(), []string{"doctor"}, c.deps)
		if code != ExitOK {
			t.Errorf("exit = %d, want 0 (stderr: %q)", code, c.stderr.String())
		}
		if !strings.Contains(c.stdout.String(), "Resultado: fallos=0 avisos=0") {
			t.Errorf("salida doctor = %q", c.stdout.String())
		}
		if !strings.Contains(c.stdout.String(), "global: no encontrado") {
			t.Errorf("sin archivo global el detalle debe decir «no encontrado»: %q", c.stdout.String())
		}
	})

	t.Run("con fallos sale 7 sin cortocircuito", func(t *testing.T) {
		c := newCaptured(t)
		if err := os.WriteFile(filepath.Join(c.deps.WorkDir, config.ProjectConfigFileName), []byte(`{"project":`), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		c.deps.StatusPort = &romero.RomeroFakeStatusAdapter{Err: romero.ErrAccessDenied}

		code := Run(context.Background(), []string{"doctor"}, c.deps)
		if code != ExitDoctorFailed {
			t.Errorf("exit = %d, want 7", code)
		}
		out := c.stdout.String()
		if !strings.Contains(out, "[FAIL] credencial") || !strings.Contains(out, "[FAIL] romero.conexión") {
			t.Errorf("doctor debe reportar credencial y conexión en FAIL: %q", out)
		}
		if !strings.Contains(out, "[FAIL] proyecto") || !strings.Contains(out, "[FAIL] configuración") {
			t.Errorf("sin config el doctor sigue reportando todos los checks: %q", out)
		}
	})

	t.Run("json", func(t *testing.T) {
		c := newCaptured(t)
		c.withProjectFile(t, "faro")
		c.withAPIKey()
		c.deps.StatusPort = okStatus

		code := Run(context.Background(), []string{"doctor", "--json"}, c.deps)
		if code != ExitOK {
			t.Errorf("exit = %d, want 0", code)
		}
		if !strings.Contains(c.stdout.String(), `"command": "doctor"`) {
			t.Errorf("json doctor = %q", c.stdout.String())
		}
	})
}

func TestCLIDispatch_MemoryQuery(t *testing.T) {
	t.Run("ok con tabla de referencia", func(t *testing.T) {
		c := newCaptured(t)
		c.withProjectFile(t, "faro")
		c.deps.ReadPort = faroReadPort()

		code := Run(context.Background(), []string{"memory", "query", "¿Para qué conservamos los informes de prueba?"}, c.deps)
		if code != ExitOK {
			t.Errorf("exit = %d, want 0 (stderr: %q)", code, c.stderr.String())
		}
		out := c.stdout.String()
		if !strings.Contains(out, "Proyecto: faro") || !strings.Contains(out, "| Conservamos los informes para comparar entregas. | observed | vigente | M1 | F1 |") {
			t.Errorf("tabla = %q", out)
		}
	})

	t.Run("sin pregunta sale 2", func(t *testing.T) {
		c := newCaptured(t)
		c.deps.ReadPort = faroReadPort()
		if code := Run(context.Background(), []string{"memory", "query"}, c.deps); code != ExitUsageError {
			t.Errorf("exit = %d, want 2", code)
		}
	})

	t.Run("sin proyecto sale 3 con cero llamadas", func(t *testing.T) {
		c := newCaptured(t)
		fake := &romero.RomeroFakeAdapter{}
		c.deps.ReadPort = fake

		code := Run(context.Background(), []string{"memory", "query", "pregunta"}, c.deps)
		if code != ExitConfigError {
			t.Errorf("exit = %d, want 3", code)
		}
		if ops := fake.Operations(); len(ops) != 0 {
			t.Errorf("sin proyecto no debe llamar a ROMERO; llamadas: %v", ops)
		}
		if !strings.Contains(c.stderr.String(), "Identifica el proyecto activo") {
			t.Errorf("stderr = %q", c.stderr.String())
		}
	})

	t.Run("denegación sale 4 sin contenido", func(t *testing.T) {
		c := newCaptured(t)
		c.withProjectFile(t, "faro")
		c.deps.ReadPort = &romero.RomeroFakeAdapter{DeniedProjects: map[string]bool{"faro": true}}

		code := Run(context.Background(), []string{"memory", "query", "pregunta"}, c.deps)
		if code != ExitAccessDenied {
			t.Errorf("exit = %d, want 4", code)
		}
		if strings.Contains(c.stdout.String(), "M1") || strings.Contains(c.stdout.String(), "Conservamos") {
			t.Errorf("denegación filtra contenido: %q", c.stdout.String())
		}
	})

	t.Run("parcial sale 5 e incompleta sale 6", func(t *testing.T) {
		c := newCaptured(t)
		c.withProjectFile(t, "faro")
		c.deps.ReadPort = &romero.RomeroFakeAdapter{
			Records: []romero.FakeMemoryRecord{
				{MemoryID: "M1", ProjectID: "faro", Summary: "Uno.", EpistemicState: "observed", Status: "active", SourceRef: "F1"},
				{MemoryID: "M2", ProjectID: "faro", Summary: "Dos.", EpistemicState: "observed", Status: "active", SourceRef: "F2"},
			},
			InterruptDetailIDs: map[string]bool{"M2": true},
		}
		// M1 confirma y M2 interrumpe el detalle: consulta parcial con una entrada.
		if code := Run(context.Background(), []string{"memory", "query", "uno"}, c.deps); code != ExitPartialQuery {
			t.Errorf("exit = %d, want 5", code)
		}
		if !strings.Contains(c.stdout.String(), "Consulta parcial") {
			t.Errorf("stdout = %q", c.stdout.String())
		}

		c2 := newCaptured(t)
		c2.withProjectFile(t, "faro")
		c2.deps.ReadPort = &romero.RomeroFakeAdapter{InterruptSearch: true}
		if code := Run(context.Background(), []string{"memory", "query", "x"}, c2.deps); code != ExitIncompleteQuery {
			t.Errorf("exit = %d, want 6", code)
		}
		if !strings.Contains(c2.stdout.String(), "Consulta incompleta") {
			t.Errorf("stdout = %q", c2.stdout.String())
		}
	})

	t.Run("json estable", func(t *testing.T) {
		c := newCaptured(t)
		c.withProjectFile(t, "faro")
		c.deps.ReadPort = faroReadPort()

		code := Run(context.Background(), []string{"memory", "query", "--json", "informes"}, c.deps)
		if code != ExitOK {
			t.Errorf("exit = %d, want 0", code)
		}
		if !strings.Contains(c.stdout.String(), `"schema_version": "1"`) || !strings.Contains(c.stdout.String(), `"memory_ref": "M1"`) {
			t.Errorf("json = %q", c.stdout.String())
		}
	})
}
