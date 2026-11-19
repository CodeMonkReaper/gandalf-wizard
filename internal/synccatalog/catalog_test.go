package synccatalog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/syncfs"
)

func TestSyncCatalog(t *testing.T) {
	projectRoot := t.TempDir()
	sourceRoot := t.TempDir()

	// Create local sources for skill (directory with SKILL.md + reference), rule (Markdown file), and pattern (Markdown file).
	skillDir := filepath.Join(sourceRoot, "revision-cambios")
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	skillBody := []byte("# Revision Cambios\nCheck diff before merge.\n")
	skillRef := []byte("Checklist reference\n")
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), skillBody, 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "checklist.md"), skillRef, 0o644); err != nil {
		t.Fatalf("write checklist.md: %v", err)
	}

	rulePath := filepath.Join(sourceRoot, "evidencia-de-pruebas.md")
	ruleBody := []byte("# Evidencia\nIndicar que pruebas se ejecutaron.\n")
	if err := os.WriteFile(rulePath, ruleBody, 0o644); err != nil {
		t.Fatalf("write rule: %v", err)
	}

	patternPath := filepath.Join(sourceRoot, "revision-por-pares.md")
	patternBody := []byte("# Patron Revision\nDos pares de ojos.\n")
	if err := os.WriteFile(patternPath, patternBody, 0o644); err != nil {
		t.Fatalf("write pattern: %v", err)
	}

	proj, err := syncfs.OpenProject(projectRoot)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	defer proj.Close()

	cat := New(proj)

	t.Run("register_three_kinds_idempotent_and_conflict", func(t *testing.T) {
		skillEntry, err := cat.Register(KindSkill, "revision-cambios", "1.0.0", skillDir, RegisterOptions{
			Origin: "local:ejemplo/skill",
			Dependencies: []Dependency{
				{Kind: KindRule, ID: "evidencia-de-pruebas", Version: "1.0.0"},
			},
		})
		if err != nil {
			t.Fatalf("register skill: %v", err)
		}
		if skillEntry.SHA256 == "" || skillEntry.Origin != "local:ejemplo/skill" || len(skillEntry.Resources) != 2 {
			t.Fatalf("unexpected skill entry: %+v", skillEntry)
		}

		// Idempotent re-register with identical content.
		again, err := cat.Register(KindSkill, "revision-cambios", "1.0.0", skillDir, RegisterOptions{
			Origin: "local:ejemplo/skill",
			Dependencies: []Dependency{
				{Kind: KindRule, ID: "evidencia-de-pruebas", Version: "1.0.0"},
			},
		})
		if err != nil {
			t.Fatalf("idempotent register skill: %v", err)
		}
		if again.SHA256 != skillEntry.SHA256 {
			t.Fatalf("sha mismatch: got %q want %q", again.SHA256, skillEntry.SHA256)
		}

		if _, err := cat.Register(KindRule, "evidencia-de-pruebas", "1.0.0", rulePath, RegisterOptions{
			Origin: "local:ejemplo/rule",
		}); err != nil {
			t.Fatalf("register rule: %v", err)
		}

		if _, err := cat.Register(KindPattern, "revision-por-pares", "1.0.0", patternPath, RegisterOptions{
			Origin: "local:ejemplo/pattern",
		}); err != nil {
			t.Fatalf("register pattern: %v", err)
		}

		// Mutate source after registration: re-registering same version must conflict.
		if err := os.WriteFile(rulePath, []byte("# Evidencia cambiada\n"), 0o644); err != nil {
			t.Fatalf("mutate rule source: %v", err)
		}
		if _, err := cat.Register(KindRule, "evidencia-de-pruebas", "1.0.0", rulePath, RegisterOptions{
			Origin: "local:ejemplo/rule",
		}); !errors.Is(err, ErrConflict) {
			t.Fatalf("expected ErrConflict on mutated source, got %v", err)
		}

		entries, err := cat.List()
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}
	})

	t.Run("resolve_uses_fixed_bytes_checks_deps_and_pattern_ref", func(t *testing.T) {
		selJSON := `{
  "targets": ["codex", "vibe"],
  "components": [
    {"kind": "skill", "id": "revision-cambios", "version": "1.0.0"},
    {"kind": "rule", "id": "evidencia-de-pruebas", "version": "1.0.0"},
    {"kind": "pattern", "id": "revision-por-pares", "version": "1.0.0"}
  ]
}`
		if err := os.WriteFile(filepath.Join(projectRoot, SelectionFile), []byte(selJSON), 0o644); err != nil {
			t.Fatalf("write selection: %v", err)
		}
		sel, err := LoadSelection(proj)
		if err != nil {
			t.Fatalf("LoadSelection: %v", err)
		}
		resolved, err := cat.Resolve(sel)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if len(resolved) != 3 {
			t.Fatalf("expected 3 resolved entries, got %d", len(resolved))
		}
		// Verify rule still has original fixed bytes despite mutated source.
		var foundRule, foundPattern bool
		for _, e := range resolved {
			if e.Kind == KindRule && e.ID == "evidencia-de-pruebas" {
				foundRule = true
				if len(e.Resources) != 1 || string(e.Resources[0].Bytes) != string(ruleBody) {
					t.Fatalf("rule did not preserve fixed bytes: %+v", e.Resources)
				}
			}
			if e.Kind == KindPattern && e.ID == "revision-por-pares" {
				foundPattern = true
				if e.Version != "1.0.0" || e.Origin != "local:ejemplo/pattern" {
					t.Fatalf("unexpected pattern entry: %+v", e)
				}
			}
		}
		if !foundRule || !foundPattern {
			t.Fatalf("missing resolved rule (%v) or pattern (%v)", foundRule, foundPattern)
		}

		// Missing dependency in selection must fail with DiagnosticError.
		missingDepSel := Selection{
			Targets: []string{"codex"},
			Components: []ComponentRef{
				{Kind: KindSkill, ID: "revision-cambios", Version: "1.0.0"},
			},
		}
		_, err = cat.Resolve(missingDepSel)
		var diagErr *DiagnosticError
		if !errors.As(err, &diagErr) || len(diagErr.Diagnostics) == 0 {
			t.Fatalf("expected DiagnosticError for missing dependency, got %v", err)
		}
	})

	t.Run("empty_selection_resolves_empty", func(t *testing.T) {
		emptyJSON := `{"targets": ["codex", "vibe"], "components": []}`
		if err := os.WriteFile(filepath.Join(projectRoot, SelectionFile), []byte(emptyJSON), 0o644); err != nil {
			t.Fatalf("write empty selection: %v", err)
		}
		sel, err := LoadSelection(proj)
		if err != nil {
			t.Fatalf("LoadSelection empty: %v", err)
		}
		resolved, err := cat.Resolve(sel)
		if err != nil {
			t.Fatalf("Resolve empty: %v", err)
		}
		if len(resolved) != 0 {
			t.Fatalf("expected 0 resolved entries, got %d", len(resolved))
		}
	})

	t.Run("tampered_catalog_blob_integrity_error", func(t *testing.T) {
		blobPath := filepath.Join(projectRoot, ".gandalf", "sync", "catalog", "rule", "evidencia-de-pruebas", "1.0.0", "files", "evidencia-de-pruebas.md")
		if _, err := os.Stat(blobPath); err != nil {
			t.Fatalf("blob missing before tamper: %v", err)
		}
		if err := os.WriteFile(blobPath, []byte("# Evidencia manipulada\n"), 0o644); err != nil {
			t.Fatalf("tamper blob: %v", err)
		}
		sel := Selection{
			Targets: []string{"codex"},
			Components: []ComponentRef{
				{Kind: KindRule, ID: "evidencia-de-pruebas", Version: "1.0.0"},
			},
		}
		_, err := cat.Resolve(sel)
		var diagErr *DiagnosticError
		if !errors.As(err, &diagErr) {
			t.Fatalf("expected DiagnosticError after tampering, got %v", err)
		}
		foundIntegrity := false
		for _, d := range diagErr.Diagnostics {
			if d.Code == "integrity_error" {
				foundIntegrity = true
			}
		}
		if !foundIntegrity {
			t.Fatalf("expected integrity_error diagnostic, got %+v", diagErr.Diagnostics)
		}
	})

	t.Run("invalid_kind_rejected", func(t *testing.T) {
		if err := validateKind(ComponentKind("plugin")); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput for unknown kind, got %v", err)
		}
	})

	t.Run("invalid_id_rejected", func(t *testing.T) {
		for _, bad := range []string{"", "..", ".hidden", "with space", "a/b"} {
			if err := validateID(bad); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput for id %q, got %v", bad, err)
			}
		}
		if err := validateID("valid-id_1.0"); err != nil {
			t.Fatalf("unexpected error for valid id: %v", err)
		}
	})

	t.Run("invalid_version_rejected", func(t *testing.T) {
		for _, bad := range []string{"", "..", "1..0", "v/1"} {
			if err := validateVersion(bad); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput for version %q, got %v", bad, err)
			}
		}
		if err := validateVersion("1.0.0"); err != nil {
			t.Fatalf("unexpected error for valid version: %v", err)
		}
	})

	t.Run("selection_unknown_field_rejected", func(t *testing.T) {
		badSel := []byte(`{"targets": ["codex"], "components": [{"kind": "skill", "id": "x", "version": "1.0.0", "bogus": true}]}`)
		if err := os.WriteFile(filepath.Join(projectRoot, SelectionFile), badSel, 0o644); err != nil {
			t.Fatalf("write bad selection: %v", err)
		}
		if _, err := LoadSelection(proj); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput for unknown field, got %v", err)
		}
	})

	t.Run("dependency_version_mismatch", func(t *testing.T) {
		sel := Selection{
			Targets: []string{"codex"},
			Components: []ComponentRef{
				{Kind: KindSkill, ID: "revision-cambios", Version: "1.0.0"},
				{Kind: KindRule, ID: "evidencia-de-pruebas", Version: "9.9.9"},
			},
		}
		_, err := cat.Resolve(sel)
		var diagErr *DiagnosticError
		if !errors.As(err, &diagErr) {
			t.Fatalf("expected DiagnosticError, got %v", err)
		}
		found := false
		for _, d := range diagErr.Diagnostics {
			if d.Code == "version_mismatch" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected version_mismatch diagnostic, got %+v", diagErr.Diagnostics)
		}
	})
}
