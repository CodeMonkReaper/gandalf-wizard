package projectsync

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/synccatalog"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/syncfs"
)

func TestSyncApply(t *testing.T) {
	sourceRoot := t.TempDir()
	secretMarker := "SECRET_MARKER_T005_NEVER_LEAK"

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
	if err := os.WriteFile(patternPath, []byte("Patron.\n"), 0o644); err != nil {
		t.Fatalf("write pattern: %v", err)
	}

	foreignAgents := "# Proyecto Local\nContenido ajeno " + secretMarker + "\n"
	sel := synccatalog.Selection{
		Targets: []string{"codex", "vibe"},
		Components: []synccatalog.ComponentRef{
			{Kind: synccatalog.KindSkill, ID: "revision-cambios", Version: "1.0.0"},
			{Kind: synccatalog.KindRule, ID: "evidencia-de-pruebas", Version: "1.0.0"},
			{Kind: synccatalog.KindPattern, ID: "revision-por-pares", Version: "1.0.0"},
		},
	}
	profiles := DefaultVerifiedProfiles()

	newProject := func(t *testing.T) (string, *syncfs.ScopedProject, *synccatalog.Catalog) {
		t.Helper()
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(foreignAgents), 0o644); err != nil {
			t.Fatalf("write AGENTS.md: %v", err)
		}
		proj, err := syncfs.OpenProject(root)
		if err != nil {
			t.Fatalf("OpenProject: %v", err)
		}
		t.Cleanup(func() { _ = proj.Close() })
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
		return root, proj, cat
	}

	t.Run("rejects_missing_or_stale_view_without_mutating", func(t *testing.T) {
		root, proj, cat := newProject(t)
		view, err := Preview(proj, cat, sel, profiles, nil)
		if err != nil || !view.Applicable() {
			t.Fatalf("Preview: err=%v view=%+v", err, view)
		}

		// Empty viewID rejected.
		if _, err := Apply(proj, cat, sel, profiles, "", ApplyOptions{}); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized for empty viewID, got %v", err)
		}
		// Wrong viewID rejected.
		if _, err := Apply(proj, cat, sel, profiles, "wrong-view-id", ApplyOptions{}); !errors.Is(err, ErrStaleView) {
			t.Fatalf("expected ErrStaleView for mismatched viewID, got %v", err)
		}
		// Edit AGENTS.md between preview and apply -> stale view rejected, zero files written.
		edited := foreignAgents + "edit externo\n"
		if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(edited), 0o644); err != nil {
			t.Fatalf("edit AGENTS.md: %v", err)
		}
		if _, err := Apply(proj, cat, sel, profiles, view.ID, ApplyOptions{}); !errors.Is(err, ErrStaleView) {
			t.Fatalf("expected ErrStaleView after external edit, got %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, ".agents")); !os.IsNotExist(err) {
			t.Fatalf("stale apply created .agents")
		}
		gotAgents, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if string(gotAgents) != edited {
			t.Fatalf("stale apply modified AGENTS.md")
		}
	})

	t.Run("apply_creates_backup_gitignore_preserves_foreign_and_second_view_is_noop", func(t *testing.T) {
		root, proj, cat := newProject(t)
		view, err := Preview(proj, cat, sel, profiles, nil)
		if err != nil || !view.Applicable() {
			t.Fatalf("Preview: err=%v", err)
		}
		res, err := Apply(proj, cat, sel, profiles, view.ID, ApplyOptions{})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if res.Status != StatusApplied || res.OperationID == "" || len(res.Changes) != 3 {
			t.Fatalf("unexpected ApplyResult: %+v", res)
		}
		// AC-08: JSON output must not contain secretMarker.
		rawJSON, _ := json.Marshal(res)
		if strings.Contains(string(rawJSON), secretMarker) {
			t.Fatalf("ApplyResult JSON leaked file content")
		}
		// Git exclusion for private sync state.
		gi, err := os.ReadFile(filepath.Join(root, ".gandalf", "sync", ".gitignore"))
		if err != nil || !strings.Contains(string(gi), "*") {
			t.Fatalf("expected .gandalf/sync/.gitignore excluding state, err=%v content=%q", err, string(gi))
		}
		// Foreign bytes preserved in AGENTS.md.
		gotAgents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil || !strings.Contains(string(gotAgents), foreignAgents) || !strings.Contains(string(gotAgents), BlockStart) {
			t.Fatalf("AGENTS.md missing foreign bytes or managed block")
		}
		// Operation & backup persisted.
		op, err := LoadOperation(proj, res.OperationID)
		if err != nil || op.Phase != PhaseApplied || len(op.Entries) != 3 {
			t.Fatalf("unexpected operation: err=%v op=%+v", err, op)
		}
		// Second preview with loaded receipts has 0 changes; applying it is noop with no operation created.
		receipts, err := LoadReceipts(proj)
		if err != nil || len(receipts) != 3 {
			t.Fatalf("LoadReceipts: err=%v receipts=%+v", err, receipts)
		}
		v2, err := Preview(proj, cat, sel, profiles, receipts)
		if err != nil || !v2.Applicable() || len(v2.Changes) != 0 {
			t.Fatalf("expected 0-change second preview, err=%v v2=%+v", err, v2)
		}
		res2, err := Apply(proj, cat, sel, profiles, v2.ID, ApplyOptions{})
		if err != nil {
			t.Fatalf("Apply noop: %v", err)
		}
		if res2.Status != StatusNoOp || res2.OperationID != "" {
			t.Fatalf("expected noop without operation ID, got %+v", res2)
		}
	})

	t.Run("interruption_after_first_write_leaves_recovery_pending_and_blocks_apply", func(t *testing.T) {
		root, proj, cat := newProject(t)
		view, err := Preview(proj, cat, sel, profiles, nil)
		if err != nil || !view.Applicable() {
			t.Fatalf("Preview: err=%v", err)
		}
		injected := errors.New("simulated crash after first write")
		writes := 0
		res, err := Apply(proj, cat, sel, profiles, view.ID, ApplyOptions{
			AfterWrite: func(p string) error {
				writes++
				if writes == 1 {
					return injected
				}
				return nil
			},
		})
		if err == nil {
			t.Fatalf("expected error on simulated crash, got result=%+v", res)
		}
		if res.Status != StatusRecoveryPending || res.OperationID == "" {
			t.Fatalf("expected recovery_pending with operation ID, got %+v", res)
		}
		pending, err := PendingRecovery(proj)
		if err != nil || pending == nil || pending.ID != res.OperationID || pending.Phase != PhaseRecoveryPending {
			t.Fatalf("unexpected PendingRecovery: err=%v pending=%+v", err, pending)
		}
		// New Apply must be blocked by ErrRecoveryPending.
		if _, err := Apply(proj, cat, sel, profiles, view.ID, ApplyOptions{}); !errors.Is(err, ErrRecoveryPending) {
			t.Fatalf("expected ErrRecoveryPending on subsequent Apply, got %v", err)
		}
		_ = root
	})

	t.Run("interruption_before_first_write_preserves_targets_and_marks_recovery_pending", func(t *testing.T) {
		root, proj, cat := newProject(t)
		view, err := Preview(proj, cat, sel, profiles, nil)
		if err != nil || !view.Applicable() {
			t.Fatalf("Preview: err=%v", err)
		}
		injected := errors.New("simulated crash before first write")
		res, err := Apply(proj, cat, sel, profiles, view.ID, ApplyOptions{
			BeforeWrite: func(p string) error {
				return injected
			},
		})
		if !errors.Is(err, injected) {
			t.Fatalf("expected injected error, got %v", err)
		}
		if res.Status != StatusRecoveryPending || res.OperationID == "" {
			t.Fatalf("expected recovery_pending with operation ID, got %+v", res)
		}
		// No target file was modified.
		if _, err := os.Stat(filepath.Join(root, ".agents")); !os.IsNotExist(err) {
			t.Fatalf("interrupted apply before first write created .agents")
		}
		gotAgents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil || string(gotAgents) != foreignAgents {
			t.Fatalf("interrupted apply before first write modified AGENTS.md")
		}
		op, err := LoadOperation(proj, res.OperationID)
		if err != nil || op.Phase != PhaseRecoveryPending {
			t.Fatalf("expected persisted recovery_pending op, err=%v op=%+v", err, op)
		}
		for _, e := range op.Entries {
			if e.State != EntryPending {
				t.Fatalf("expected all entries pending, got %+v", e)
			}
		}
	})

	t.Run("busy_lease_blocks_concurrent_apply", func(t *testing.T) {
		_, proj, cat := newProject(t)
		view, err := Preview(proj, cat, sel, profiles, nil)
		if err != nil || !view.Applicable() {
			t.Fatalf("Preview: err=%v", err)
		}
		lease, err := proj.TryExclusive("other-writer")
		if err != nil {
			t.Fatalf("TryExclusive: %v", err)
		}
		defer lease.Close()

		res, err := Apply(proj, cat, sel, profiles, view.ID, ApplyOptions{})
		if !errors.Is(err, syncfs.ErrBusy) {
			t.Fatalf("expected syncfs.ErrBusy, got err=%v res=%+v", err, res)
		}
	})

	t.Run("external_edit_mid_apply_stops_remaining_writes_and_preserves_edited_file", func(t *testing.T) {
		root, proj, cat := newProject(t)
		view, err := Preview(proj, cat, sel, profiles, nil)
		if err != nil || !view.Applicable() {
			t.Fatalf("Preview: err=%v", err)
		}
		if len(view.Changes) < 3 {
			t.Fatalf("expected at least 3 changes, got %d", len(view.Changes))
		}
		// View changes are sorted by path:
		// [0] .agents/skills/revision-cambios/SKILL.md
		// [1] .agents/skills/revision-cambios/references/checklist.md
		// [2] AGENTS.md
		externalEdit := foreignAgents + "external concurrent edit\n"
		res, err := Apply(proj, cat, sel, profiles, view.ID, ApplyOptions{
			BeforeWrite: func(p string) error {
				if p == "AGENTS.md" {
					return os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(externalEdit), 0o644)
				}
				return nil
			},
		})
		if !errors.Is(err, syncfs.ErrConflict) {
			t.Fatalf("expected syncfs.ErrConflict on external edit mid-apply, got err=%v res=%+v", err, res)
		}
		if res.Status != StatusRecoveryPending || res.OperationID == "" {
			t.Fatalf("expected recovery_pending with operation ID, got %+v", res)
		}
		// External edit to AGENTS.md must be preserved.
		gotAgents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil || string(gotAgents) != externalEdit {
			t.Fatalf("external edit to AGENTS.md was overwritten: got %q", string(gotAgents))
		}
		// Operation records first two entries written and AGENTS.md still pending.
		op, err := LoadOperation(proj, res.OperationID)
		if err != nil || op.Phase != PhaseRecoveryPending {
			t.Fatalf("LoadOperation: err=%v op=%+v", err, op)
		}
		var writtenCount, pendingCount int
		for _, e := range op.Entries {
			switch e.State {
			case EntryWritten:
				writtenCount++
			case EntryPending:
				pendingCount++
			}
		}
		if writtenCount != 2 || pendingCount != 1 {
			t.Fatalf("expected 2 written and 1 pending, got written=%d pending=%d entries=%+v", writtenCount, pendingCount, op.Entries)
		}
	})
}
