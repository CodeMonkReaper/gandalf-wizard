package projectsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/synccatalog"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/syncfs"
)

func TestSyncPreview(t *testing.T) {
	projectRoot := t.TempDir()
	sourceRoot := t.TempDir()

	// Prepare local sources for reference example: skill + rule + pattern.
	skillDir := filepath.Join(sourceRoot, "revision-cambios")
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Revision Cambios\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "checklist.md"), []byte("Check diff\n"), 0o644); err != nil {
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

	// Pre-existing AGENTS.md with foreign instructions that must be preserved.
	foreignAgents := "# Proyecto Local\nReglas propias del equipo.\n"
	if err := os.WriteFile(filepath.Join(projectRoot, "AGENTS.md"), []byte(foreignAgents), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	proj, err := syncfs.OpenProject(projectRoot)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	defer proj.Close()

	cat := synccatalog.New(proj)
	if _, err := cat.Register(synccatalog.KindRule, "evidencia-de-pruebas", "1.0.0", rulePath, synccatalog.RegisterOptions{Origin: "local:rule"}); err != nil {
		t.Fatalf("register rule: %v", err)
	}
	if _, err := cat.Register(synccatalog.KindSkill, "revision-cambios", "1.0.0", skillDir, synccatalog.RegisterOptions{
		Origin:       "local:skill",
		Dependencies: []synccatalog.Dependency{{Kind: synccatalog.KindRule, ID: "evidencia-de-pruebas", Version: "1.0.0"}},
	}); err != nil {
		t.Fatalf("register skill: %v", err)
	}
	if _, err := cat.Register(synccatalog.KindPattern, "revision-por-pares", "1.0.0", patternPath, synccatalog.RegisterOptions{Origin: "local:pattern"}); err != nil {
		t.Fatalf("register pattern: %v", err)
	}

	sel := synccatalog.Selection{
		Targets: []string{"codex", "vibe"},
		Components: []synccatalog.ComponentRef{
			{Kind: synccatalog.KindSkill, ID: "revision-cambios", Version: "1.0.0"},
			{Kind: synccatalog.KindRule, ID: "evidencia-de-pruebas", Version: "1.0.0"},
			{Kind: synccatalog.KindPattern, ID: "revision-por-pares", Version: "1.0.0"},
		},
	}
	profiles := DefaultVerifiedProfiles()

	t.Run("deterministic_shared_projection_and_read_only", func(t *testing.T) {
		v1, err := Preview(proj, cat, sel, profiles, nil)
		if err != nil {
			t.Fatalf("Preview v1: %v", err)
		}
		v2, err := Preview(proj, cat, sel, profiles, nil)
		if err != nil {
			t.Fatalf("Preview v2: %v", err)
		}
		if !v1.Applicable() || v1.ID == "" || v1.ID != v2.ID {
			t.Fatalf("expected deterministic applicable view ID: v1=%+v v2=%+v", v1.ID, v2.ID)
		}
		// Expect 3 file changes: AGENTS.md (update), .agents/skills/revision-cambios/SKILL.md (create), references/checklist.md (create).
		// Pattern is reference-only and does not produce a file change.
		if len(v1.Changes) != 3 {
			t.Fatalf("expected 3 changes, got %d: %+v", len(v1.Changes), v1.Changes)
		}
		for _, ch := range v1.Changes {
			if len(ch.Targets) != 2 || ch.Targets[0] != "codex" || ch.Targets[1] != "vibe" {
				t.Fatalf("expected shared targets [codex vibe] for %s, got %v", ch.Path, ch.Targets)
			}
			if ch.Path == "AGENTS.md" {
				if ch.Action != ActionUpdate {
					t.Fatalf("expected AGENTS.md update, got %s", ch.Action)
				}
				afterText := string(ch.After.Bytes)
				if !strings.Contains(afterText, foreignAgents) || !strings.Contains(afterText, BlockStart) || !strings.Contains(afterText, BlockEnd) {
					t.Fatalf("AGENTS.md projection did not preserve foreign bytes and markers")
				}
			}
		}
		// Verify Preview did not write any files.
		if _, err := os.Stat(filepath.Join(projectRoot, ".agents", "skills")); !os.IsNotExist(err) {
			t.Fatalf("Preview mutated .agents/skills")
		}
		gotAgents, err := os.ReadFile(filepath.Join(projectRoot, "AGENTS.md"))
		if err != nil || string(gotAgents) != foreignAgents {
			t.Fatalf("Preview mutated AGENTS.md")
		}
	})

	t.Run("empty_selection_zero_changes", func(t *testing.T) {
		emptySel := synccatalog.Selection{Targets: []string{"codex", "vibe"}, Components: []synccatalog.ComponentRef{}}
		v, err := Preview(proj, cat, emptySel, profiles, nil)
		if err != nil {
			t.Fatalf("Preview empty: %v", err)
		}
		if !v.Applicable() || len(v.Changes) != 0 {
			t.Fatalf("expected applicable empty view with 0 changes, got %+v", v)
		}
	})

	t.Run("unverified_target_blocks_preview", func(t *testing.T) {
		badSel := synccatalog.Selection{Targets: []string{"codex", "unknown-cli"}, Components: sel.Components}
		v, err := Preview(proj, cat, badSel, profiles, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Applicable() || len(v.Diagnostics) == 0 {
			t.Fatalf("expected blocking diagnostic for unverified target, got %+v", v)
		}
	})

	t.Run("receipt_allows_update_and_noop_when_already_applied", func(t *testing.T) {
		appRoot := t.TempDir()
		if err := os.WriteFile(filepath.Join(appRoot, "AGENTS.md"), []byte(foreignAgents), 0o644); err != nil {
			t.Fatalf("write AGENTS.md: %v", err)
		}
		appProj, err := syncfs.OpenProject(appRoot)
		if err != nil {
			t.Fatalf("OpenProject app: %v", err)
		}
		defer appProj.Close()
		appCat := synccatalog.New(appProj)
		if _, err := appCat.Register(synccatalog.KindRule, "evidencia-de-pruebas", "1.0.0", rulePath, synccatalog.RegisterOptions{Origin: "local:rule"}); err != nil {
			t.Fatalf("register rule: %v", err)
		}
		if _, err := appCat.Register(synccatalog.KindSkill, "revision-cambios", "1.0.0", skillDir, synccatalog.RegisterOptions{
			Origin:       "local:skill",
			Dependencies: []synccatalog.Dependency{{Kind: synccatalog.KindRule, ID: "evidencia-de-pruebas", Version: "1.0.0"}},
		}); err != nil {
			t.Fatalf("register skill: %v", err)
		}
		if _, err := appCat.Register(synccatalog.KindPattern, "revision-por-pares", "1.0.0", patternPath, synccatalog.RegisterOptions{Origin: "local:pattern"}); err != nil {
			t.Fatalf("register pattern: %v", err)
		}

		// First preview to learn the planned after-state bytes and hashes.
		v1, err := Preview(appProj, appCat, sel, profiles, nil)
		if err != nil {
			t.Fatalf("Preview v1: %v", err)
		}
		if !v1.Applicable() {
			t.Fatalf("expected applicable view, got %+v", v1.Diagnostics)
		}
		var receipts []Receipt
		for _, ch := range v1.Changes {
			dst := filepath.Join(appRoot, filepath.FromSlash(ch.Path))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", ch.Path, err)
			}
			if err := os.WriteFile(dst, ch.After.Bytes, 0o644); err != nil {
				t.Fatalf("write %s: %v", ch.Path, err)
			}
			receipts = append(receipts, Receipt{Path: ch.Path, Owner: OwnerGandalf, AfterSHA256: ch.After.SHA256})
		}
		// With matching receipts and applied content on disk, everything is a no-op.
		v2, err := Preview(appProj, appCat, sel, profiles, receipts)
		if err != nil {
			t.Fatalf("Preview v2: %v", err)
		}
		if !v2.Applicable() {
			t.Fatalf("expected applicable view with receipts, got %+v", v2.Diagnostics)
		}
		if len(v2.Changes) != 0 {
			t.Fatalf("expected 0 changes when receipts match and content already applied, got %+v", v2.Changes)
		}

		// Updating rule to 2.0.0 with valid receipt must plan an update to AGENTS.md while preserving foreign text.
		rule2Path := filepath.Join(sourceRoot, "evidencia-v2.md")
		if err := os.WriteFile(rule2Path, []byte("Indicar pruebas y resultados.\n"), 0o644); err != nil {
			t.Fatalf("write rule v2: %v", err)
		}
		if _, err := appCat.Register(synccatalog.KindRule, "evidencia-de-pruebas", "2.0.0", rule2Path, synccatalog.RegisterOptions{Origin: "local:rule-v2"}); err != nil {
			t.Fatalf("register rule v2: %v", err)
		}
		selV2 := synccatalog.Selection{
			Targets:    []string{"codex", "vibe"},
			Components: []synccatalog.ComponentRef{{Kind: synccatalog.KindRule, ID: "evidencia-de-pruebas", Version: "2.0.0"}},
		}
		v3, err := Preview(appProj, appCat, selV2, profiles, receipts)
		if err != nil {
			t.Fatalf("Preview v3: %v", err)
		}
		if !v3.Applicable() || len(v3.Changes) != 1 || v3.Changes[0].Path != "AGENTS.md" || v3.Changes[0].Action != ActionUpdate {
			t.Fatalf("expected single AGENTS.md update with receipt, got applicable=%v changes=%+v diags=%+v", v3.Applicable(), v3.Changes, v3.Diagnostics)
		}
		if !strings.Contains(string(v3.Changes[0].After.Bytes), foreignAgents) || !strings.Contains(string(v3.Changes[0].After.Bytes), "evidencia-de-pruebas@2.0.0") {
			t.Fatalf("updated AGENTS.md did not preserve foreign bytes or include new rule version")
		}
	})

	t.Run("ambiguous_agents_markers_block_preview", func(t *testing.T) {
		ambRoot := t.TempDir()
		ambiguous := foreignAgents + BlockStart + "\n" + BlockStart + "\n" + BlockEnd + "\n"
		if err := os.WriteFile(filepath.Join(ambRoot, "AGENTS.md"), []byte(ambiguous), 0o644); err != nil {
			t.Fatalf("write ambiguous AGENTS.md: %v", err)
		}
		ambProj, err := syncfs.OpenProject(ambRoot)
		if err != nil {
			t.Fatalf("OpenProject amb: %v", err)
		}
		defer ambProj.Close()
		ambCat := synccatalog.New(ambProj)
		if _, err := ambCat.Register(synccatalog.KindRule, "evidencia-de-pruebas", "1.0.0", rulePath, synccatalog.RegisterOptions{Origin: "local:rule"}); err != nil {
			t.Fatalf("register rule amb: %v", err)
		}
		ambSel := synccatalog.Selection{
			Targets:    []string{"codex"},
			Components: []synccatalog.ComponentRef{{Kind: synccatalog.KindRule, ID: "evidencia-de-pruebas", Version: "1.0.0"}},
		}
		v, err := Preview(ambProj, ambCat, ambSel, profiles, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Applicable() {
			t.Fatalf("expected marker_conflict diagnostic, got applicable view")
		}
		found := false
		for _, d := range v.Diagnostics {
			if d.Code == "marker_conflict" && d.Path == "AGENTS.md" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected marker_conflict for AGENTS.md, got %+v", v.Diagnostics)
		}
	})

	t.Run("precedence_conflict_blocks_preview", func(t *testing.T) {
		precRoot := t.TempDir()
		if err := os.WriteFile(filepath.Join(precRoot, "AGENTS.override.md"), []byte("override\n"), 0o644); err != nil {
			t.Fatalf("write override: %v", err)
		}
		vibeSkillDir := filepath.Join(precRoot, ".vibe", "skills", "revision-cambios")
		if err := os.MkdirAll(vibeSkillDir, 0o755); err != nil {
			t.Fatalf("mkdir vibe skill: %v", err)
		}
		if err := os.WriteFile(filepath.Join(vibeSkillDir, "SKILL.md"), []byte("# Vibe copy\n"), 0o644); err != nil {
			t.Fatalf("write vibe SKILL.md: %v", err)
		}
		precProj, err := syncfs.OpenProject(precRoot)
		if err != nil {
			t.Fatalf("OpenProject prec: %v", err)
		}
		defer precProj.Close()
		precCat := synccatalog.New(precProj)
		if _, err := precCat.Register(synccatalog.KindRule, "evidencia-de-pruebas", "1.0.0", rulePath, synccatalog.RegisterOptions{Origin: "local:rule"}); err != nil {
			t.Fatalf("register rule prec: %v", err)
		}
		if _, err := precCat.Register(synccatalog.KindSkill, "revision-cambios", "1.0.0", skillDir, synccatalog.RegisterOptions{Origin: "local:skill"}); err != nil {
			t.Fatalf("register skill prec: %v", err)
		}
		if _, err := precCat.Register(synccatalog.KindPattern, "revision-por-pares", "1.0.0", patternPath, synccatalog.RegisterOptions{Origin: "local:pattern"}); err != nil {
			t.Fatalf("register pattern prec: %v", err)
		}
		v, err := Preview(precProj, precCat, sel, profiles, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Applicable() {
			t.Fatalf("expected precedence_conflict diagnostics, got applicable view")
		}
		codes := map[string]bool{}
		for _, d := range v.Diagnostics {
			if d.Code == "precedence_conflict" {
				codes[d.Target+"|"+d.Path] = true
			}
		}
		if !codes["codex|AGENTS.override.md"] {
			t.Fatalf("expected precedence_conflict for codex AGENTS.override.md, got %+v", v.Diagnostics)
		}
		if !codes["vibe|.vibe/skills/revision-cambios/SKILL.md"] {
			t.Fatalf("expected precedence_conflict for vibe skill, got %+v", v.Diagnostics)
		}
	})

	t.Run("different_project_or_selection_changes_view_id", func(t *testing.T) {
		v1, err := Preview(proj, cat, sel, profiles, nil)
		if err != nil {
			t.Fatalf("Preview v1: %v", err)
		}
		otherSel := synccatalog.Selection{
			Targets: []string{"codex"},
			Components: []synccatalog.ComponentRef{
				{Kind: synccatalog.KindSkill, ID: "revision-cambios", Version: "1.0.0"},
				{Kind: synccatalog.KindRule, ID: "evidencia-de-pruebas", Version: "1.0.0"},
			},
		}
		v2, err := Preview(proj, cat, otherSel, profiles, nil)
		if err != nil {
			t.Fatalf("Preview v2: %v", err)
		}
		if v1.ID == "" || v2.ID == "" || v1.ID == v2.ID {
			t.Fatalf("expected different non-empty view IDs for different selections: %q vs %q", v1.ID, v2.ID)
		}
		otherRoot := t.TempDir()
		otherProj, err := syncfs.OpenProject(otherRoot)
		if err != nil {
			t.Fatalf("OpenProject other: %v", err)
		}
		defer otherProj.Close()
		otherCat := synccatalog.New(otherProj)
		if _, err := otherCat.Register(synccatalog.KindRule, "evidencia-de-pruebas", "1.0.0", rulePath, synccatalog.RegisterOptions{Origin: "local:rule"}); err != nil {
			t.Fatalf("register rule other: %v", err)
		}
		if _, err := otherCat.Register(synccatalog.KindSkill, "revision-cambios", "1.0.0", skillDir, synccatalog.RegisterOptions{Origin: "local:skill"}); err != nil {
			t.Fatalf("register skill other: %v", err)
		}
		v3, err := Preview(otherProj, otherCat, sel, profiles, nil)
		if err != nil {
			t.Fatalf("Preview v3: %v", err)
		}
		if v1.ID == v3.ID {
			t.Fatalf("expected different view IDs across projects: %q", v1.ID)
		}
	})

	t.Run("ownership_conflict_without_receipt", func(t *testing.T) {
		conflictDir := filepath.Join(projectRoot, ".agents", "skills", "revision-cambios")
		if err := os.MkdirAll(conflictDir, 0o755); err != nil {
			t.Fatalf("mkdir conflict: %v", err)
		}
		defer os.RemoveAll(filepath.Join(projectRoot, ".agents"))
		if err := os.WriteFile(filepath.Join(conflictDir, "SKILL.md"), []byte("# Revision Cambios\n"), 0o644); err != nil {
			t.Fatalf("write foreign skill: %v", err)
		}
		// Same content, but NO receipt -> must still block as ownership conflict.
		v, err := Preview(proj, cat, sel, profiles, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Applicable() || len(v.Diagnostics) == 0 {
			t.Fatalf("expected ownership conflict diagnostic without receipt, got %+v", v)
		}
	})
}
