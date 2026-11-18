package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const routerSentinelSecret = "sentinel-secreto-123"

const routerGlobalConfig = `{
  "router": {
    "attempt_timeout": "45s",
    "executors": [
      {"id": "antigravity", "kind": "cli", "authorized": true},
      {"id": "api-mistral", "kind": "api", "authorized": true, "protocol": "mistral", "base_url": "https://api.mistral.ai/v1", "credential_env": "MISTRAL_API_KEY"}
    ],
    "profiles": [
      {"id": "antigravity/modelo-a", "executor_id": "antigravity", "model": "modelo-a", "efforts": {"low": "low", "medium": "medium", "high": "high"}},
      {"id": "api-mistral/modelo-c", "executor_id": "api-mistral", "model": "modelo-c"}
    ]
  }
}`

// TestRouterPolicy covers the router policy block: defaults, layer rules,
// validation and credential redaction (AC5, AC13, AC14, AC16, AC17).
func TestRouterPolicy(t *testing.T) {
	cases := []struct {
		name        string
		projectJSON string
		globalJSON  string
		env         map[string]string
		wantErr     error
		wantReason  string
		check       func(t *testing.T, cfg EffectiveConfig)
		checkErr    func(t *testing.T, err error)
	}{
		{
			name:        "config antigua sin bloque router carga con defaults",
			projectJSON: `{"project":"proyecto-file"}`,
			check: func(t *testing.T, cfg EffectiveConfig) {
				if len(cfg.Router.Executors) != 0 {
					t.Fatalf("esperaba 0 ejecutores, got %d", len(cfg.Router.Executors))
				}
				if len(cfg.Router.Profiles) != 0 {
					t.Fatalf("esperaba 0 perfiles, got %d", len(cfg.Router.Profiles))
				}
				if cfg.Router.AttemptTimeout != DefaultRouterAttemptTimeout {
					t.Fatalf("plazo esperado %s, got %s", DefaultRouterAttemptTimeout, cfg.Router.AttemptTimeout)
				}
			},
		},
		{
			name:       "global declara ejecutores y perfiles",
			globalJSON: routerGlobalConfig,
			check: func(t *testing.T, cfg EffectiveConfig) {
				if len(cfg.Router.Executors) != 2 {
					t.Fatalf("esperaba 2 ejecutores, got %d", len(cfg.Router.Executors))
				}
				agy := cfg.Router.Executors[0]
				if agy.ID != "antigravity" || agy.Kind != RouterKindCLI || !agy.Authorized {
					t.Fatalf("ejecutor cli inesperado: %+v", agy)
				}
				api := cfg.Router.Executors[1]
				if api.ID != "api-mistral" || api.Kind != RouterKindAPI || !api.Authorized {
					t.Fatalf("ejecutor api inesperado: %+v", api)
				}
				if api.Protocol != "mistral" || api.BaseURL != "https://api.mistral.ai/v1" || api.CredentialEnv != "MISTRAL_API_KEY" {
					t.Fatalf("vinculos de api inesperados: %+v", api)
				}
				if len(cfg.Router.Profiles) != 2 {
					t.Fatalf("esperaba 2 perfiles, got %d", len(cfg.Router.Profiles))
				}
				p1 := cfg.Router.Profiles[0]
				if p1.ID != "antigravity/modelo-a" || p1.ExecutorID != "antigravity" || p1.Model != "modelo-a" {
					t.Fatalf("perfil 1 inesperado: %+v", p1)
				}
				if len(p1.Efforts) != 3 || p1.Efforts[RouterEffortHigh] != "high" {
					t.Fatalf("esfuerzos del perfil 1 inesperados: %+v", p1.Efforts)
				}
				if cfg.Router.AttemptTimeout != 45*time.Second {
					t.Fatalf("plazo esperado 45s, got %s", cfg.Router.AttemptTimeout)
				}
			},
		},
		{
			name:        "proyecto sustituye perfiles y ajusta plazo",
			projectJSON: `{"router":{"attempt_timeout":"30s","profiles":[{"id":"antigravity/modelo-b","executor_id":"antigravity","model":"modelo-b"}]}}`,
			globalJSON:  routerGlobalConfig,
			check: func(t *testing.T, cfg EffectiveConfig) {
				if len(cfg.Router.Profiles) != 1 || cfg.Router.Profiles[0].ID != "antigravity/modelo-b" {
					t.Fatalf("perfiles del proyecto no sustituyeron: %+v", cfg.Router.Profiles)
				}
				if len(cfg.Router.Executors) != 2 {
					t.Fatalf("esperaba 2 ejecutores del global, got %d", len(cfg.Router.Executors))
				}
				if cfg.Router.AttemptTimeout != 30*time.Second {
					t.Fatalf("plazo esperado 30s, got %s", cfg.Router.AttemptTimeout)
				}
			},
		},
		{
			name:        "proyecto vacia la lista de perfiles",
			projectJSON: `{"router":{"profiles":[]}}`,
			globalJSON:  routerGlobalConfig,
			check: func(t *testing.T, cfg EffectiveConfig) {
				if len(cfg.Router.Profiles) != 0 {
					t.Fatalf("esperaba 0 perfiles, got %d", len(cfg.Router.Profiles))
				}
				if len(cfg.Router.Executors) != 2 {
					t.Fatalf("esperaba 2 ejecutores, got %d", len(cfg.Router.Executors))
				}
				if cfg.Router.AttemptTimeout != 45*time.Second {
					t.Fatalf("plazo esperado 45s del global, got %s", cfg.Router.AttemptTimeout)
				}
			},
		},
		{
			name:        "proyecto no puede declarar ejecutores",
			projectJSON: `{"router":{"executors":[{"id":"vibe","kind":"cli","authorized":true}]}}`,
			wantErr:     ErrInvalidConfig,
			wantReason:  "capa global",
		},
		{
			name:       "ids duplicados de ejecutor",
			globalJSON: `{"router":{"executors":[{"id":"dup","kind":"cli"},{"id":"dup","kind":"cli"}]}}`,
			wantErr:    ErrInvalidConfig,
			wantReason: "duplicado",
		},
		{
			name: "ids duplicados de perfil",
			globalJSON: `{"router":{"executors":[{"id":"antigravity","kind":"cli"}],"profiles":[` +
				`{"id":"p/uno","executor_id":"antigravity","model":"m"},{"id":"p/uno","executor_id":"antigravity","model":"m"}]}}`,
			wantErr:    ErrInvalidConfig,
			wantReason: "duplicado",
		},
		{
			name: "perfil referencia ejecutor inexistente",
			globalJSON: `{"router":{"executors":[{"id":"antigravity","kind":"cli"}],` +
				`"profiles":[{"id":"p/uno","executor_id":"desconocido","model":"m"}]}}`,
			wantErr:    ErrInvalidConfig,
			wantReason: "ejecutor desconocido",
		},
		{
			name:       "plazo invalido",
			globalJSON: `{"router":{"attempt_timeout":"0s"}}`,
			wantErr:    ErrInvalidConfig,
			wantReason: "plazo",
		},
		{
			name:       "plazo con formato invalido",
			globalJSON: `{"router":{"attempt_timeout":"abc"}}`,
			wantErr:    ErrInvalidConfig,
			wantReason: "plazo",
		},
		{
			name:       "url sin https",
			globalJSON: `{"router":{"executors":[{"id":"api-x","kind":"api","protocol":"mistral","base_url":"http://api.example"}]}}`,
			wantErr:    ErrInvalidConfig,
			wantReason: "https",
		},
		{
			name:       "url con credenciales",
			globalJSON: `{"router":{"executors":[{"id":"api-x","kind":"api","protocol":"mistral","base_url":"https://user:pass@api.example"}]}}`,
			wantErr:    ErrInvalidConfig,
			wantReason: "credenciales",
		},
		{
			name:       "credencial literal en archivo",
			globalJSON: `{"router":{"executors":[{"id":"api-x","kind":"api","api_key":"` + routerSentinelSecret + `"}]}}`,
			wantErr:    ErrForbiddenCredentialKey,
			checkErr: func(t *testing.T, err error) {
				if strings.Contains(err.Error(), routerSentinelSecret) {
					t.Fatalf("el error revela el centinela: %s", err.Error())
				}
			},
		},
		{
			name:       "credencial especifica de proveedor en archivo",
			globalJSON: `{"router":{"executors":[{"id":"api-x","kind":"api","mistral_api_key":"` + routerSentinelSecret + `"}]}}`,
			wantErr:    ErrForbiddenCredentialKey,
			checkErr: func(t *testing.T, err error) {
				if strings.Contains(err.Error(), routerSentinelSecret) {
					t.Fatalf("el error revela el centinela: %s", err.Error())
				}
			},
		},
		{
			name:       "claves desconocidas en executors y profiles emiten aviso sin bloquear",
			globalJSON: `{"router":{"executors":[{"id":"antigravity","kind":"cli","authorised":true}],"profiles":[{"id":"p/uno","executor_id":"antigravity","model":"m","effort":"high"}]}}`,
			check: func(t *testing.T, cfg EffectiveConfig) {
				joined := strings.Join(cfg.Warnings, "\n")
				if !strings.Contains(joined, "authorised") || !strings.Contains(joined, "effort") {
					t.Fatalf("esperaba avisos de claves desconocidas en executors y profiles, got %v", cfg.Warnings)
				}
			},
		},
		{
			name:       "ejecutor cli con campos de api",
			globalJSON: `{"router":{"executors":[{"id":"antigravity","kind":"cli","base_url":"https://api.example"}]}}`,
			wantErr:    ErrInvalidConfig,
			wantReason: "campos de api",
		},
		{
			name:       "api sin protocolo",
			globalJSON: `{"router":{"executors":[{"id":"api-x","kind":"api","base_url":"https://api.example"}]}}`,
			wantErr:    ErrInvalidConfig,
			wantReason: "protocolo",
		},
		{
			name:       "api sin url",
			globalJSON: `{"router":{"executors":[{"id":"api-x","kind":"api","protocol":"mistral"}]}}`,
			wantErr:    ErrInvalidConfig,
			wantReason: "URL",
		},
		{
			name:       "variable de credencial invalida",
			globalJSON: `{"router":{"executors":[{"id":"api-x","kind":"api","protocol":"mistral","base_url":"https://api.example","credential_env":"mistral_api_key"}]}}`,
			wantErr:    ErrInvalidConfig,
			wantReason: "variable de credencial",
		},
		{
			name:       "esfuerzo desconocido",
			globalJSON: `{"router":{"executors":[{"id":"antigravity","kind":"cli"}],"profiles":[{"id":"p/uno","executor_id":"antigravity","model":"m","efforts":{"turbo":"x"}}]}}`,
			wantErr:    ErrInvalidConfig,
			wantReason: "esfuerzo",
		},
		{
			name:       "autorizacion ausente por defecto es false",
			globalJSON: `{"router":{"executors":[{"id":"antigravity","kind":"cli"}]}}`,
			check: func(t *testing.T, cfg EffectiveConfig) {
				if cfg.Router.Executors[0].Authorized {
					t.Fatal("ejecutor sin campo authorized debe quedar no autorizado")
				}
			},
		},
		{
			name:       "secreto del entorno no aparece en la politica",
			globalJSON: routerGlobalConfig,
			env:        map[string]string{"MISTRAL_API_KEY": routerSentinelSecret},
			check: func(t *testing.T, cfg EffectiveConfig) {
				if cfg.Router.Executors[1].CredentialEnv != "MISTRAL_API_KEY" {
					t.Fatalf("nombre de variable esperado, got %q", cfg.Router.Executors[1].CredentialEnv)
				}
				if strings.Contains(fmt.Sprintf("%+v", cfg.Router), routerSentinelSecret) {
					t.Fatal("la politica efectiva contiene el centinela del entorno")
				}
			},
		},
		{
			name:       "secreto del entorno no aparece en errores",
			globalJSON: `{"router":{"executors":[{"id":"api-x","kind":"api","protocol":"mistral","base_url":"http://api.example"}]}}`,
			env:        map[string]string{"MISTRAL_API_KEY": routerSentinelSecret},
			wantErr:    ErrInvalidConfig,
			checkErr: func(t *testing.T, err error) {
				if strings.Contains(err.Error(), routerSentinelSecret) {
					t.Fatalf("el error revela el centinela: %s", err.Error())
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			projectDir := t.TempDir()
			globalDir := t.TempDir()
			if tc.projectJSON != "" {
				writeFile(t, projectDir, ProjectConfigFileName, tc.projectJSON)
			}
			if tc.globalJSON != "" {
				writeFile(t, globalDir, filepath.Join(GlobalConfigDirName, GlobalConfigFileName), tc.globalJSON)
			}
			input := baseInput(projectDir, globalDir)
			input.Getenv = getenvFunc(tc.env)

			cfg, err := Load(input)
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("esperaba error %v, got nil", tc.wantErr)
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("esperaba error %v, got %v", tc.wantErr, err)
				}
				if tc.wantReason != "" && !strings.Contains(err.Error(), tc.wantReason) {
					t.Fatalf("error %q sin la razon esperada %q", err.Error(), tc.wantReason)
				}
				if tc.checkErr != nil {
					tc.checkErr(t, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if tc.check == nil {
				t.Fatal("caso sin comprobacion")
			}
			tc.check(t, cfg)
		})
	}
}
