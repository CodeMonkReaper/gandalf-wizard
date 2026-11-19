package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncCLI(t *testing.T) {
	projectRoot := t.TempDir()
	sourceRoot := t.TempDir()
	secretMarker := "SECRET_MARKER_T007_CLI_NEVER_LEAK"

	skillDir := filepath.Join(sourceRoot, "revision-cambios")
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Revision\n"+secretMarker+"\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "checklist.md"), []byte("Checklist\n"), 0o644); err != nil {
		t.Fatalf("write ref: %v", err)
	}
	rulePath := filepath.Join(sourceRoot, "evidencia-de-pruebas.md")
	if err := os.WriteFile(rulePath, []byte("Indicar pruebas ejecutadas.\n"), 0o644); err != nil {
		t.Fatalf("write rule: %v", err)
	}
	patternPath := filepath.Join(sourceRoot, "revision-por-pares.md")
	if err := os.WriteFile(patternPath, []byte("Patron revision.\n"), 0o644); err != nil {
		t.Fatalf("write pattern: %v", err)
	}
	foreignAgents := "# Proyecto CLI\nInstrucciones previas " + secretMarker + "\n"
	if err := os.WriteFile(filepath.Join(projectRoot, "AGENTS.md"), []byte(foreignAgents), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	selJSON := `{
  "targets": ["codex", "vibe"],
  "components": [
    {"kind": "skill", "id": "revision-cambios", "version": "1.0.0"},
    {"kind": "rule", "id": "evidencia-de-pruebas", "version": "1.0.0"},
    {"kind": "pattern", "id": "revision-por-pares", "version": "1.0.0"}
  ]
}`
	if err := os.WriteFile(filepath.Join(projectRoot, ".gandalf-sync.json"), []byte(selJSON), 0o644); err != nil {
		t.Fatalf("write selection: %v", err)
	}

	invoke := func(args ...string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), args, Dependencies{
			Version: "test",
			WorkDir: projectRoot,
			Stdout:  &stdout,
			Stderr:  &stderr,
		})
		outStr, errStr := stdout.String(), stderr.String()
		if strings.Contains(outStr, secretMarker) || strings.Contains(errStr, secretMarker) {
			t.Fatalf("CLI output leaked secret marker on args %v", args)
		}
		return code, outStr, errStr
	}

	t.Run("catalog_register_and_list_json_and_text", func(t *testing.T) {
		code, out, errOut := invoke("catalog", "register", "--json",
			"--kind", "rule", "--id", "evidencia-de-pruebas", "--version", "1.0.0",
			"--source", rulePath, "--origin", "local:rule")
		if code != ExitOK {
			t.Fatalf("register rule failed: code=%d out=%s err=%s", code, out, errOut)
		}
		code, out, errOut = invoke("catalog", "register", "--json",
			"--kind", "skill", "--id", "revision-cambios", "--version", "1.0.0",
			"--source", skillDir, "--origin", "local:skill", "--dep", "rule:evidencia-de-pruebas@1.0.0")
		if code != ExitOK {
			t.Fatalf("register skill failed: code=%d out=%s err=%s", code, out, errOut)
		}
		code, out, errOut = invoke("catalog", "register", "--json",
			"--kind", "pattern", "--id", "revision-por-pares", "--version", "1.0.0",
			"--source", patternPath, "--origin", "local:pattern")
		if code != ExitOK {
			t.Fatalf("register pattern failed: code=%d out=%s err=%s", code, out, errOut)
		}

		code, out, errOut = invoke("catalog", "list", "--json")
		if code != ExitOK {
			t.Fatalf("catalog list json failed: code=%d err=%s", code, errOut)
		}
		var payload struct {
			SchemaVersion string `json:"schema_version"`
			Status        string `json:"status"`
			Components    []struct {
				Kind    string `json:"kind"`
				ID      string `json:"id"`
				Version string `json:"version"`
				Origin  string `json:"origin"`
				SHA256  string `json:"sha256"`
			} `json:"components"`
		}
		if err := json.Unmarshal([]byte(out), &payload); err != nil {
			t.Fatalf("decode catalog list json: %v (out=%s)", err, out)
		}
		if payload.SchemaVersion != syncSchemaVersion || len(payload.Components) != 3 {
			t.Fatalf("unexpected catalog list payload: %+v", payload)
		}

		code, textOut, errOut := invoke("catalog", "list")
		if code != ExitOK || !strings.Contains(textOut, "revision-cambios") {
			t.Fatalf("catalog list text failed: code=%d out=%s err=%s", code, textOut, errOut)
		}
	})

	t.Run("sync_preview_apply_authorization_and_restore", func(t *testing.T) {
		// Apply without --view-id must fail with ExitUsageError.
		code, _, _ := invoke("sync", "apply", "--json")
		if code != ExitUsageError {
			t.Fatalf("expected ExitUsageError without --view-id, got %d", code)
		}

		// Preview JSON returns view_id and 3 changes.
		code, out, errOut := invoke("sync", "preview", "--json")
		if code != ExitOK {
			t.Fatalf("sync preview failed: code=%d out=%s err=%s", code, out, errOut)
		}
		var prev struct {
			SchemaVersion string   `json:"schema_version"`
			Status        string   `json:"status"`
			ViewID        string   `json:"view_id"`
			Targets       []string `json:"targets"`
			Changes       []struct {
				Path   string `json:"path"`
				Action string `json:"action"`
			} `json:"changes"`
		}
		if err := json.Unmarshal([]byte(out), &prev); err != nil {
			t.Fatalf("decode preview json: %v", err)
		}
		if prev.SchemaVersion != syncSchemaVersion || prev.ViewID == "" || len(prev.Changes) != 3 {
			t.Fatalf("unexpected preview payload: %+v", prev)
		}

		// Apply with wrong view_id fails with non-zero exit code.
		code, _, _ = invoke("sync", "apply", "--json", "--view-id", "stale-view-id")
		if code == ExitOK {
			t.Fatalf("expected non-zero exit for stale view-id")
		}

		// Apply with authorized view_id succeeds and returns operation_id.
		code, out, errOut = invoke("sync", "apply", "--json", "--view-id", prev.ViewID)
		if code != ExitOK {
			t.Fatalf("sync apply failed: code=%d out=%s err=%s", code, out, errOut)
		}
		var app struct {
			Status      string `json:"status"`
			OperationID string `json:"operation_id"`
		}
		if err := json.Unmarshal([]byte(out), &app); err != nil || app.OperationID == "" || app.Status != "applied" {
			t.Fatalf("unexpected apply payload: err=%v out=%s", err, out)
		}

		// Restore with operation_id reverts cleanly.
		code, out, errOut = invoke("sync", "restore", "--json", "--operation-id", app.OperationID)
		if code != ExitOK {
			t.Fatalf("sync restore failed: code=%d out=%s err=%s", code, out, errOut)
		}
		gotAgents, err := os.ReadFile(filepath.Join(projectRoot, "AGENTS.md"))
		if err != nil || string(gotAgents) != foreignAgents {
			t.Fatalf("AGENTS.md not restored by CLI: err=%v", err)
		}
	})

	t.Run("usage_errors_for_unknown_subcommands_and_missing_flags", func(t *testing.T) {
		cases := []struct {
			name string
			args []string
		}{
			{"catalog_without_subcommand", []string{"catalog"}},
			{"catalog_unknown_subcommand", []string{"catalog", "frobnicate"}},
			{"catalog_register_missing_flags", []string{"catalog", "register", "--json"}},
			{"catalog_register_missing_source", []string{"catalog", "register", "--json",
				"--kind", "rule", "--id", "x", "--version", "1.0.0"}},
			{"catalog_register_invalid_kind", []string{"catalog", "register", "--json",
				"--kind", "widget", "--id", "x", "--version", "1.0.0", "--source", rulePath}},
			{"catalog_register_invalid_dep", []string{"catalog", "register", "--json",
				"--kind", "rule", "--id", "x", "--version", "1.0.0", "--source", rulePath,
				"--dep", "not-a-dependency"}},
			{"sync_without_subcommand", []string{"sync"}},
			{"sync_unknown_subcommand", []string{"sync", "frobnicate"}},
			{"sync_restore_without_operation_id", []string{"sync", "restore", "--json"}},
		}
		for _, tc := range cases {
			code, out, errOut := invoke(tc.args...)
			if code != ExitUsageError {
				t.Fatalf("%s: expected ExitUsageError, got %d (out=%s err=%s)", tc.name, code, out, errOut)
			}
		}
	})

	t.Run("text_output_mode_never_leaks_contents", func(t *testing.T) {
		code, out, errOut := invoke("sync", "preview")
		if code != ExitOK {
			t.Fatalf("text preview failed: code=%d out=%s err=%s", code, out, errOut)
		}
		if strings.Contains(out, "{") || strings.Contains(out, "schema_version") {
			t.Fatalf("text preview emitted JSON: %s", out)
		}
		if !strings.Contains(out, "preview") {
			t.Fatalf("text preview missing summary line: %s", out)
		}

		code, jsonOut, errOut := invoke("sync", "preview", "--json")
		if code != ExitOK {
			t.Fatalf("json preview failed: code=%d out=%s err=%s", code, jsonOut, errOut)
		}
		var prev struct {
			ViewID string `json:"view_id"`
		}
		if err := json.Unmarshal([]byte(jsonOut), &prev); err != nil || prev.ViewID == "" {
			t.Fatalf("decode preview view id: err=%v out=%s", err, jsonOut)
		}

		code, out, errOut = invoke("sync", "apply", "--view-id", prev.ViewID)
		if code != ExitOK {
			t.Fatalf("text apply failed: code=%d out=%s err=%s", code, out, errOut)
		}
		if strings.Contains(out, "{") || strings.Contains(out, "schema_version") {
			t.Fatalf("text apply emitted JSON: %s", out)
		}
		if !strings.Contains(out, "applied") {
			t.Fatalf("text apply missing status: %s", out)
		}

		var opID string
		if _, after, ok := strings.Cut(out, "(operation "); ok {
			if id, _, ok := strings.Cut(after, ")"); ok {
				opID = strings.TrimSpace(id)
			}
		}
		if opID == "" {
			t.Fatalf("text apply did not include operation id: %s", out)
		}

		code, out, errOut = invoke("sync", "restore", "--operation-id", opID)
		if code != ExitOK {
			t.Fatalf("text restore failed: code=%d out=%s err=%s", code, out, errOut)
		}
		if strings.Contains(out, "{") || strings.Contains(out, "schema_version") {
			t.Fatalf("text restore emitted JSON: %s", out)
		}
		if !strings.Contains(out, "restored") {
			t.Fatalf("text restore missing status: %s", out)
		}
	})
}
