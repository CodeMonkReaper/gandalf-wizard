package doctor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router"
)

// TestRouterDoctor covers the executor diagnosis added by router-02 (AC15, AC17).
func TestRouterDoctor(t *testing.T) {
	t.Run("sin ejecutores conserva la forma anterior", func(t *testing.T) {
		report := Run(context.Background(), Input{})
		if report.Executors != nil {
			t.Fatalf("sin ejecutores el reporte no debe diagnosticar: %v", report.Executors)
		}
		raw, err := json.Marshal(report)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if json.Unmarshal(raw, &map[string]any{}) != nil {
			t.Fatal("el reporte no es json")
		}
		if string(raw) == "" || containsKey(raw, "executors") {
			t.Fatalf("la forma anterior cambió: %s", raw)
		}
	})

	t.Run("diagnostica ejecutores locales sin fallar", func(t *testing.T) {
		input := Input{
			RouterExecutors: []router.ProbeView{
				{
					Executor: config.RouterExecutor{ID: "antigravity", Kind: config.RouterKindCLI, Authorized: true},
					Probe:    router.Probe{Installed: true, Auth: router.AuthReady, TextOnly: router.TextOnlyEnforced, Reason: "agy 1.5.10"},
				},
				{
					Executor: config.RouterExecutor{ID: "api-mistral", Kind: config.RouterKindAPI, Authorized: true, Protocol: "mistral", BaseURL: "https://api.mistral.ai/v1", CredentialEnv: "MISTRAL_API_KEY"},
					Probe:    router.Probe{Installed: true, Auth: router.AuthMissing, TextOnly: router.TextOnlyEnforced},
				},
				{
					Executor: config.RouterExecutor{ID: "vibe", Kind: config.RouterKindCLI, Authorized: false},
					Probe:    router.Probe{Installed: false, Auth: router.AuthUnknown, TextOnly: router.TextOnlyUnproven},
				},
			},
		}
		report := Run(context.Background(), input)
		if len(report.Executors) != 3 {
			t.Fatalf("ejecutores inesperados: %v", report.Executors)
		}
		first := report.Executors[0]
		if first.ID != "antigravity" || !first.Installed || !first.Authorized || first.Auth != string(router.AuthReady) || first.TextOnly != string(router.TextOnlyEnforced) {
			t.Fatalf("diagnóstico inesperado: %+v", first)
		}
		second := report.Executors[1]
		if second.Auth != string(router.AuthMissing) {
			t.Fatalf("autenticación inesperada: %+v", second)
		}
		third := report.Executors[2]
		if third.Authorized || third.Installed {
			t.Fatalf("estado no autorizado inesperado: %+v", third)
		}
		baseline := Run(context.Background(), Input{})
		if report.Failures != baseline.Failures {
			t.Fatalf("el diagnóstico de ejecutores no debe sumar fallos: %d (base %d)", report.Failures, baseline.Failures)
		}
	})

	t.Run("estado desconocido se informa como tal", func(t *testing.T) {
		input := Input{
			RouterExecutors: []router.ProbeView{
				{
					Executor: config.RouterExecutor{ID: "x", Kind: config.RouterKindCLI, Authorized: true},
					Probe:    router.Probe{Installed: true, Auth: router.AuthUnknown, TextOnly: router.TextOnlyUnproven},
				},
			},
		}
		report := Run(context.Background(), input)
		if len(report.Executors) == 0 {
			t.Fatal("el doctor no diagnosticó los ejecutores")
		}
		if report.Executors[0].Auth != string(router.AuthUnknown) {
			t.Fatalf("esperaba auth unknown: %+v", report.Executors[0])
		}
	})
}

// containsKey reports whether the raw JSON object carries a top-level key.
func containsKey(raw []byte, key string) bool {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	_, ok := m[key]
	return ok
}
