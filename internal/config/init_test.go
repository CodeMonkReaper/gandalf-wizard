package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInitProjectConfig(t *testing.T) {
	t.Run("crea .gandalf.json exclusivo con project", func(t *testing.T) {
		dir := t.TempDir()

		path, err := InitProjectConfig(dir, "faro")
		if err != nil {
			t.Fatalf("InitProjectConfig: error inesperado: %v", err)
		}
		if path != filepath.Join(dir, ProjectConfigFileName) {
			t.Errorf("path = %q, want %q", path, filepath.Join(dir, ProjectConfigFileName))
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(data) != `{"project":"faro"}` {
			t.Errorf("contenido = %q, want {\"project\":\"faro\"}", string(data))
		}
		st, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
			t.Errorf("permisos = %v, want 0600", st.Mode().Perm())
		}
	})

	t.Run("nunca sobrescribe un archivo existente", func(t *testing.T) {
		dir := t.TempDir()
		existing := filepath.Join(dir, ProjectConfigFileName)
		original := `{"project":"original"}`
		if err := os.WriteFile(existing, []byte(original), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		_, err := InitProjectConfig(dir, "faro")
		if err == nil {
			t.Fatal("InitProjectConfig: se esperaba error por configuración existente")
		}
		var cfgErr *ConfigError
		if !errors.As(err, &cfgErr) {
			t.Fatalf("InitProjectConfig: error %v no es *ConfigError", err)
		}
		if !errors.Is(err, ErrConfigExists) {
			t.Errorf("InitProjectConfig: error no envuelve ErrConfigExists: %v", err)
		}
		if cfgErr.Path != existing {
			t.Errorf("Path = %q, want %q", cfgErr.Path, existing)
		}
		data, err := os.ReadFile(existing)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(data) != original {
			t.Errorf("el archivo existente fue modificado: %q", string(data))
		}
	})

	t.Run("rechaza id inválido sin crear archivo", func(t *testing.T) {
		for _, id := range []string{"", "in valid!", " leading", "trailing ", "-start", ".dot"} {
			dir := t.TempDir()
			_, err := InitProjectConfig(dir, id)
			if err == nil {
				t.Fatalf("InitProjectConfig(%q): se esperaba error", id)
			}
			if !errors.Is(err, ErrInvalidProjectID) {
				t.Errorf("InitProjectConfig(%q): error no envuelve ErrInvalidProjectID: %v", id, err)
			}
			if _, statErr := os.Stat(filepath.Join(dir, ProjectConfigFileName)); !os.IsNotExist(statErr) {
				t.Errorf("InitProjectConfig(%q): no debía crear el archivo", id)
			}
		}
	})
}
