package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func getenvFunc(kv map[string]string) func(string) string {
	return func(key string) string { return kv[key] }
}

func baseInput(dir, userConfigDir string) LoadInput {
	return LoadInput{
		WorkDir:       dir,
		UserConfigDir: userConfigDir,
		Getenv:        getenvFunc(nil),
		DefaultCaller: func() string {
			return "osuser"
		},
	}
}

func TestLoadConfig_LayerPrecedence(t *testing.T) {
	projectDir := t.TempDir()
	globalDir := t.TempDir()
	projectFile := writeFile(t, projectDir, ProjectConfigFileName, `{"project":"proyecto-file","caller":"proyecto-caller","romero":{"base_url":"http://proyecto.example","timeout":"3s"}}`)
	globalFile := writeFile(t, globalDir, filepath.Join(GlobalConfigDirName, GlobalConfigFileName), `{"project":"global-file","caller":"global-caller","romero":{"base_url":"http://global.example","timeout":"4s"}}`)

	cases := []struct {
		name          string
		workDir       string
		userConfigDir string
		overrides     FlagOverrides
		env           map[string]string
		wantProject   string
		wantPOrigin   Origin
		wantCaller    string
		wantCOrigin   Origin
		wantURL       string
		wantUOrigin   Origin
		wantTimeout   time.Duration
		wantTOrigin   Origin
	}{
		{
			name:          "defaults when nothing is defined",
			workDir:       t.TempDir(),
			userConfigDir: t.TempDir(),
			wantProject:   "",
			wantPOrigin:   OriginDefault,
			wantCaller:    "osuser",
			wantCOrigin:   OriginDefault,
			wantURL:       DefaultBaseURL,
			wantUOrigin:   OriginDefault,
			wantTimeout:   DefaultTimeout,
			wantTOrigin:   OriginDefault,
		},
		{
			name:          "global file wins over defaults",
			workDir:       t.TempDir(),
			userConfigDir: globalDir,
			env:           map[string]string{},
			wantProject:   "global-file",
			wantPOrigin:   OriginGlobal,
			wantCaller:    "global-caller",
			wantCOrigin:   OriginGlobal,
			wantURL:       "http://global.example",
			wantUOrigin:   OriginGlobal,
			wantTimeout:   4 * time.Second,
			wantTOrigin:   OriginGlobal,
		},
		{
			name:        "project file wins over global",
			workDir:     projectDir,
			wantProject: "proyecto-file",
			wantPOrigin: OriginProject,
			wantCaller:  "proyecto-caller",
			wantCOrigin: OriginProject,
			wantURL:     "http://proyecto.example",
			wantUOrigin: OriginProject,
			wantTimeout: 3 * time.Second,
			wantTOrigin: OriginProject,
		},
		{
			name: "environment wins over files",
			env: map[string]string{
				EnvKeyProject:       "env-project",
				EnvKeyCaller:        "env-caller",
				EnvKeyRomeroBaseURL: "http://env.example",
				EnvKeyRomeroTimeout: "2s",
			},
			wantProject: "env-project",
			wantPOrigin: OriginEnv,
			wantCaller:  "env-caller",
			wantCOrigin: OriginEnv,
			wantURL:     "http://env.example",
			wantUOrigin: OriginEnv,
			wantTimeout: 2 * time.Second,
			wantTOrigin: OriginEnv,
		},
		{
			name: "flags win over everything",
			overrides: FlagOverrides{
				Project:       "flag-project",
				Caller:        "flag-caller",
				RomeroBaseURL: "http://flag.example",
				RomeroTimeout: "1s",
			},
			env: map[string]string{
				EnvKeyProject:       "env-project",
				EnvKeyCaller:        "env-caller",
				EnvKeyRomeroBaseURL: "http://env.example",
				EnvKeyRomeroTimeout: "2s",
			},
			wantProject: "flag-project",
			wantPOrigin: OriginFlag,
			wantCaller:  "flag-caller",
			wantCOrigin: OriginFlag,
			wantURL:     "http://flag.example",
			wantUOrigin: OriginFlag,
			wantTimeout: 1 * time.Second,
			wantTOrigin: OriginFlag,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := baseInput(tc.workDir, tc.userConfigDir)
			input.Getenv = getenvFunc(tc.env)
			input.Overrides = tc.overrides

			cfg, err := Load(input)
			if err != nil {
				t.Fatalf("Load: error inesperado: %v", err)
			}
			if cfg.Project.Value != tc.wantProject || cfg.Project.Origin != tc.wantPOrigin {
				t.Errorf("project = %q (%q), want %q (%q)", cfg.Project.Value, cfg.Project.Origin, tc.wantProject, tc.wantPOrigin)
			}
			if cfg.Caller.Value != tc.wantCaller || cfg.Caller.Origin != tc.wantCOrigin {
				t.Errorf("caller = %q (%q), want %q (%q)", cfg.Caller.Value, cfg.Caller.Origin, tc.wantCaller, tc.wantCOrigin)
			}
			if cfg.RomeroBaseURL.Value != tc.wantURL || cfg.RomeroBaseURL.Origin != tc.wantUOrigin {
				t.Errorf("romero.base_url = %q (%q), want %q (%q)", cfg.RomeroBaseURL.Value, cfg.RomeroBaseURL.Origin, tc.wantURL, tc.wantUOrigin)
			}
			if cfg.RomeroTimeout.Value != tc.wantTimeout || cfg.RomeroTimeout.Origin != tc.wantTOrigin {
				t.Errorf("romero.timeout = %v (%q), want %v (%q)", cfg.RomeroTimeout.Value, cfg.RomeroTimeout.Origin, tc.wantTimeout, tc.wantTOrigin)
			}
			if tc.wantPOrigin == OriginProject && cfg.ProjectConfigPath != projectFile {
				t.Errorf("ProjectConfigPath = %q, want %q", cfg.ProjectConfigPath, projectFile)
			}
			if tc.wantPOrigin == OriginGlobal && cfg.GlobalConfigPath != globalFile {
				t.Errorf("GlobalConfigPath = %q, want %q", cfg.GlobalConfigPath, globalFile)
			}
		})
	}
}

func TestLoadConfig_ProjectDiscoveryUpward(t *testing.T) {
	root := t.TempDir()
	near := filepath.Join(root, "near")
	if err := os.MkdirAll(filepath.Join(root, "near", "deep"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, root, ProjectConfigFileName, `{"project":"root-project"}`)
	nearFile := writeFile(t, near, ProjectConfigFileName, `{"project":"near-project"}`)

	cfg, err := Load(baseInput(filepath.Join(root, "near", "deep"), t.TempDir()))
	if err != nil {
		t.Fatalf("Load: error inesperado: %v", err)
	}
	if cfg.Project.Value != "near-project" || cfg.Project.Origin != OriginProject {
		t.Errorf("project = %q (%q), want near-project (proyecto)", cfg.Project.Value, cfg.Project.Origin)
	}
	if cfg.ProjectConfigPath != nearFile {
		t.Errorf("ProjectConfigPath = %q, want %q", cfg.ProjectConfigPath, nearFile)
	}

	cfg, err = Load(baseInput(near, t.TempDir()))
	if err != nil {
		t.Fatalf("Load: error inesperado: %v", err)
	}
	if cfg.Project.Value != "near-project" {
		t.Errorf("project = %q, want near-project", cfg.Project.Value)
	}
}

func TestLoadConfig_RejectsForbiddenCredentialKeys(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"root api_key", `{"project":"faro","api_key":"supersecret"}`},
		{"nested romero token", `{"romero":{"base_url":"http://x.example","token":"supersecret"}}`},
		{"case-insensitive API_KEY", `{"ROMERO":{"API_KEY":"supersecret"}}`},
		{"password in nested object", `{"extra":{"password":"supersecret"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeFile(t, dir, ProjectConfigFileName, tc.content)

			_, err := Load(baseInput(dir, t.TempDir()))
			if err == nil {
				t.Fatal("Load: se esperaba error por clave de credencial")
			}
			var cfgErr *ConfigError
			if !errors.As(err, &cfgErr) {
				t.Fatalf("Load: error %v no es *ConfigError", err)
			}
			if !errors.Is(err, ErrForbiddenCredentialKey) {
				t.Errorf("Load: error no envuelve ErrForbiddenCredentialKey: %v", err)
			}
			if cfgErr.Path != path {
				t.Errorf("Path = %q, want %q", cfgErr.Path, path)
			}
			if !strings.Contains(cfgErr.Reason, "credencial no permitida en archivo de configuración") {
				t.Errorf("Reason = %q, debe citar el motivo exacto", cfgErr.Reason)
			}
			if strings.Contains(err.Error(), "supersecret") {
				t.Errorf("el error filtra el valor del secreto: %v", err)
			}
		})
	}
}

func TestLoadConfig_RejectsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, ProjectConfigFileName, `{"project":`)

	_, err := Load(baseInput(dir, t.TempDir()))
	if err == nil {
		t.Fatal("Load: se esperaba error por JSON inválido")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("Load: error %v no es *ConfigError", err)
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("Load: error no envuelve ErrInvalidConfig: %v", err)
	}
	if cfgErr.Path != path {
		t.Errorf("Path = %q, want %q", cfgErr.Path, path)
	}
	if !strings.Contains(cfgErr.Reason, "JSON inválido") {
		t.Errorf("Reason = %q, debe citar JSON inválido", cfgErr.Reason)
	}
}

func TestLoadConfig_RejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name    string
		content string
		reason  string
	}{
		{"invalid project id", `{"project":"in valid!"}`, "identificador de proyecto inválido"},
		{"empty project id", `{"project":""}`, "identificador de proyecto inválido"},
		{"invalid base_url scheme", `{"romero":{"base_url":"ftp://x.example"}}`, "URL de ROMERO inválida"},
		{"invalid base_url host", `{"romero":{"base_url":"http://"}}`, "URL de ROMERO inválida"},
		{"invalid timeout", `{"romero":{"timeout":"abc"}}`, "tiempo límite de ROMERO inválido"},
		{"zero timeout", `{"romero":{"timeout":"0s"}}`, "tiempo límite de ROMERO inválido"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeFile(t, dir, ProjectConfigFileName, tc.content)

			_, err := Load(baseInput(dir, t.TempDir()))
			if err == nil {
				t.Fatal("Load: se esperaba error de valor inválido")
			}
			var cfgErr *ConfigError
			if !errors.As(err, &cfgErr) {
				t.Fatalf("Load: error %v no es *ConfigError", err)
			}
			if !errors.Is(err, ErrInvalidConfig) {
				t.Errorf("Load: error no envuelve ErrInvalidConfig: %v", err)
			}
			if cfgErr.Path != path {
				t.Errorf("Path = %q, want %q", cfgErr.Path, path)
			}
			if !strings.Contains(cfgErr.Reason, tc.reason) {
				t.Errorf("Reason = %q, debe citar %q", cfgErr.Reason, tc.reason)
			}
		})
	}
}

func TestLoadConfig_RejectsInvalidEnvAndFlagValues(t *testing.T) {
	t.Run("invalid env project", func(t *testing.T) {
		input := baseInput(t.TempDir(), t.TempDir())
		input.Getenv = getenvFunc(map[string]string{EnvKeyProject: "in valid!"})
		if _, err := Load(input); err == nil {
			t.Fatal("Load: se esperaba error por proyecto inválido en entorno")
		}
	})
	t.Run("invalid env timeout", func(t *testing.T) {
		input := baseInput(t.TempDir(), t.TempDir())
		input.Getenv = getenvFunc(map[string]string{EnvKeyRomeroTimeout: "abc"})
		if _, err := Load(input); err == nil {
			t.Fatal("Load: se esperaba error por timeout inválido en entorno")
		}
	})
	t.Run("invalid flag url", func(t *testing.T) {
		input := baseInput(t.TempDir(), t.TempDir())
		input.Overrides.RomeroBaseURL = "not-a-url"
		if _, err := Load(input); err == nil {
			t.Fatal("Load: se esperaba error por URL inválida en flag")
		}
	})
}

func TestLoadConfig_UnknownKeyWarnsWithoutBlocking(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, ProjectConfigFileName, `{"project":"faro","unknown_key":"valor","romero":{"base_url":"http://x.example","extra":"y"}}`)

	cfg, err := Load(baseInput(dir, t.TempDir()))
	if err != nil {
		t.Fatalf("Load: la clave desconocida no debe bloquear: %v", err)
	}
	if cfg.Project.Value != "faro" {
		t.Errorf("project = %q, want faro", cfg.Project.Value)
	}
	joined := strings.Join(cfg.Warnings, "\n")
	if !strings.Contains(joined, "unknown_key") || !strings.Contains(joined, path) {
		t.Errorf("Warnings = %v, deben citar la clave y el archivo", cfg.Warnings)
	}
	if !strings.Contains(joined, "extra") {
		t.Errorf("Warnings = %v, deben citar la clave anidada desconocida", cfg.Warnings)
	}
}

func TestLoadConfig_APIKeyPresenceFromEnvOnly(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		input := baseInput(t.TempDir(), t.TempDir())
		input.Getenv = getenvFunc(map[string]string{EnvKeyRomeroAPIKey: "sentinel-key"})
		cfg, err := Load(input)
		if err != nil {
			t.Fatalf("Load: error inesperado: %v", err)
		}
		if !cfg.APIKeyPresent {
			t.Error("APIKeyPresent = false, want true")
		}
		if cfg.APIKeyOrigin != OriginEnv {
			t.Errorf("APIKeyOrigin = %q, want entorno", cfg.APIKeyOrigin)
		}
	})
	t.Run("absent", func(t *testing.T) {
		cfg, err := Load(baseInput(t.TempDir(), t.TempDir()))
		if err != nil {
			t.Fatalf("Load: error inesperado: %v", err)
		}
		if cfg.APIKeyPresent {
			t.Error("APIKeyPresent = true, want false")
		}
	})
}

func TestLoadConfig_EmptyEnvValuesFallThrough(t *testing.T) {
	input := baseInput(t.TempDir(), t.TempDir())
	input.Getenv = getenvFunc(map[string]string{
		EnvKeyProject:       "",
		EnvKeyCaller:        "",
		EnvKeyRomeroBaseURL: "",
		EnvKeyRomeroTimeout: "",
	})
	cfg, err := Load(input)
	if err != nil {
		t.Fatalf("Load: variables vacías no deben fallar: %v", err)
	}
	if cfg.Project.Value != "" || cfg.Project.Origin != OriginDefault {
		t.Errorf("project = %q (%q), want vacío (predeterminado)", cfg.Project.Value, cfg.Project.Origin)
	}
	if cfg.RomeroBaseURL.Value != DefaultBaseURL {
		t.Errorf("romero.base_url = %q, want %q", cfg.RomeroBaseURL.Value, DefaultBaseURL)
	}
}
