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

func TestSyncRestore(t *testing.T) {
	sourceRoot := t.TempDir()
	secretMarker := "SECRET_MARKER_T006_NEVER_LEAK"

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

	newAppliedProject := func(t *testing.T) (string, *syncfs.ScopedProject, *synccatalog.Catalog, string) {
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
		view, err := Preview(proj, cat, sel, profiles, nil)
		if err != nil || !view.Applicable() {
			t.Fatalf("Preview: err=%v", err)
		}
		res, err := Apply(proj, cat, sel, profiles, view.ID, ApplyOptions{})
		if err != nil || res.Status != StatusApplied {
			t.Fatalf("Apply: err=%v res=%+v", err, res)
		}
		return root, proj, cat, res.OperationID
	}

	t.Run("restore_reverts_update_and_creates_and_second_restore_is_noop", func(t *testing.T) {
		root, proj, _, opID := newAppliedProject(t)

		res, err := Restore(proj, opID, RestoreOptions{})
		if err != nil {
			t.Fatalf("Restore: %v", err)
		}
		if res.Status != StatusRestored || res.OperationID != opID || len(res.Changes) != 3 {
			t.Fatalf("unexpected RestoreResult: %+v", res)
		}
		// AC-08: JSON output must not contain secretMarker.
		rawJSON, _ := json.Marshal(res)
		if strings.Contains(string(rawJSON), secretMarker) {
			t.Fatalf("RestoreResult JSON leaked file content")
		}
		// AGENTS.md restored to exact pre-apply bytes.
		gotAgents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil || string(gotAgents) != foreignAgents {
			t.Fatalf("AGENTS.md not restored: err=%v got=%q", err, string(gotAgents))
		}
		// Created skill files removed.
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "revision-cambios", "SKILL.md")); !os.IsNotExist(err) {
			t.Fatalf("SKILL.md still exists after restore")
		}
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "revision-cambios", "references", "checklist.md")); !os.IsNotExist(err) {
			t.Fatalf("checklist.md still exists after restore")
		}
		// Receipts cleaned for removed/reverted files.
		receipts, err := LoadReceipts(proj)
		if err != nil || len(receipts) != 0 {
			t.Fatalf("expected 0 receipts after full restore, err=%v receipts=%+v", err, receipts)
		}
		// Second restore is noop.
		res2, err := Restore(proj, opID, RestoreOptions{})
		if err != nil || res2.Status != StatusNoOp || len(res2.Changes) != 0 {
			t.Fatalf("expected noop on second restore, err=%v res2=%+v", err, res2)
		}
	})

	t.Run("conflict_preserves_post_edit_and_reconciliation_enables_restore_and_apply", func(t *testing.T) {
		root, proj, cat, opID := newAppliedProject(t)
		editedAgents := foreignAgents + "\nEdicion posterior del usuario.\n"
		if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(editedAgents), 0o644); err != nil {
			t.Fatalf("edit AGENTS.md: %v", err)
		}

		// Restore must fail pre-validation, preserve edited AGENTS.md, and leave SKILL.md untouched.
		res, err := Restore(proj, opID, RestoreOptions{})
		if !errors.Is(err, ErrRestoreConflict) || res.Status != StatusConflict {
			t.Fatalf("expected ErrRestoreConflict, got err=%v res=%+v", err, res)
		}
		gotAgents, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if string(gotAgents) != editedAgents {
			t.Fatalf("conflict restore overwrote post-operation edit")
		}
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "revision-cambios", "SKILL.md")); err != nil {
			t.Fatalf("conflict restore mutated SKILL.md despite pre-validation failure")
		}

		// Reconcile AGENTS.md back to pre-apply backup -> Restore succeeds and removes SKILL.md.
		if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(foreignAgents), 0o644); err != nil {
			t.Fatalf("reconcile AGENTS.md: %v", err)
		}
		res2, err := Restore(proj, opID, RestoreOptions{})
		if err != nil || res2.Status != StatusRestored {
			t.Fatalf("expected Restore after reconciliation to succeed, err=%v res=%+v", err, res2)
		}
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "revision-cambios", "SKILL.md")); !os.IsNotExist(err) {
			t.Fatalf("SKILL.md not removed after reconciled restore")
		}
		// New Apply works cleanly after restore.
		receipts, _ := LoadReceipts(proj)
		vNew, err := Preview(proj, cat, sel, profiles, receipts)
		if err != nil || !vNew.Applicable() {
			t.Fatalf("Preview after restore: err=%v v=%+v", err, vNew)
		}
		appRes, err := Apply(proj, cat, sel, profiles, vNew.ID, ApplyOptions{})
		if err != nil || appRes.Status != StatusApplied {
			t.Fatalf("Apply after restore: err=%v res=%+v", err, appRes)
		}
	})

	t.Run("interrupted_apply_restores_only_written_and_preserves_unwritten_edit", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(foreignAgents), 0o644); err != nil {
			t.Fatalf("write AGENTS.md: %v", err)
		}
		proj, err := syncfs.OpenProject(root)
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
		view, err := Preview(proj, cat, sel, profiles, nil)
		if err != nil || !view.Applicable() {
			t.Fatalf("Preview: err=%v", err)
		}
		// Interrupt after 1st write (.agents/skills/revision-cambios/SKILL.md).
		writes := 0
		appRes, err := Apply(proj, cat, sel, profiles, view.ID, ApplyOptions{
			AfterWrite: func(p string) error {
				writes++
				if writes == 1 {
					return errors.New("crash after first write")
				}
				return nil
			},
		})
		if err == nil || appRes.Status != StatusRecoveryPending {
			t.Fatalf("expected interrupted apply, err=%v res=%+v", err, appRes)
		}
		// Externally edit AGENTS.md (which Gandalf never wrote).
		unwrittenEdit := foreignAgents + "edit on unwritten file\n"
		if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(unwrittenEdit), 0o644); err != nil {
			t.Fatalf("edit unwritten AGENTS.md: %v", err)
		}

		// Restore must revert only the written SKILL.md, preserve AGENTS.md's edit, and clear pending recovery.
		restRes, err := Restore(proj, appRes.OperationID, RestoreOptions{})
		if err != nil || restRes.Status != StatusRestored {
			t.Fatalf("Restore interrupted apply: err=%v res=%+v", err, restRes)
		}
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "revision-cambios", "SKILL.md")); !os.IsNotExist(err) {
			t.Fatalf("written SKILL.md was not reverted")
		}
		gotAgents, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if string(gotAgents) != unwrittenEdit {
			t.Fatalf("unwritten AGENTS.md edit was not preserved: got %q", string(gotAgents))
		}
		pending, err := PendingRecovery(proj)
		if err != nil || pending != nil {
			t.Fatalf("expected pending recovery cleared, err=%v pending=%+v", err, pending)
		}
	})

	t.Run("interrupted_restore_resumes_and_completes_remaining_entries", func(t *testing.T) {
		root, proj, _, opID := newAppliedProject(t)

		// Interrupt after the first restored file.
		writes := 0
		res, err := Restore(proj, opID, RestoreOptions{
			AfterWrite: func(p string) error {
				writes++
				if writes == 1 {
					return errors.New("crash after first restore write")
				}
				return nil
			},
		})
		if err == nil || res.Status != StatusRecoveryPending || len(res.Changes) != 1 {
			t.Fatalf("expected interrupted restore with 1 change, err=%v res=%+v", err, res)
		}
		pending, err := PendingRecovery(proj)
		if err != nil || pending == nil {
			t.Fatalf("expected pending recovery after interruption, err=%v pending=%+v", err, pending)
		}
		op, err := LoadOperation(proj, opID)
		if err != nil || op.Phase != PhaseRestoring {
			t.Fatalf("expected PhaseRestoring after interruption, err=%v op=%+v", err, op)
		}
		// Exactly one of the three targets is back to its pre-apply state.
		restored := 0
		if got, _ := os.ReadFile(filepath.Join(root, "AGENTS.md")); string(got) == foreignAgents {
			restored++
		}
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "revision-cambios", "SKILL.md")); os.IsNotExist(err) {
			restored++
		}
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "revision-cambios", "references", "checklist.md")); os.IsNotExist(err) {
			restored++
		}
		if restored != 1 {
			t.Fatalf("expected exactly 1 restored target after interruption, got %d", restored)
		}

		// Retry resumes and completes the remaining entries.
		res2, err := Restore(proj, opID, RestoreOptions{})
		if err != nil || res2.Status != StatusRestored || len(res2.Changes) != 2 {
			t.Fatalf("expected resumed restore to complete remaining entries, err=%v res2=%+v", err, res2)
		}
		gotAgents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil || string(gotAgents) != foreignAgents {
			t.Fatalf("AGENTS.md not restored after resume: err=%v got=%q", err, string(gotAgents))
		}
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "revision-cambios", "SKILL.md")); !os.IsNotExist(err) {
			t.Fatalf("SKILL.md still exists after resumed restore")
		}
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "revision-cambios", "references", "checklist.md")); !os.IsNotExist(err) {
			t.Fatalf("checklist.md still exists after resumed restore")
		}
		receipts, err := LoadReceipts(proj)
		if err != nil || len(receipts) != 0 {
			t.Fatalf("expected receipts cleared after resumed restore, err=%v receipts=%+v", err, receipts)
		}
		pending2, err := PendingRecovery(proj)
		if err != nil || pending2 != nil {
			t.Fatalf("expected pending recovery cleared after resume, err=%v pending=%+v", err, pending2)
		}
		op2, err := LoadOperation(proj, opID)
		if err != nil || op2.Phase != PhaseRestored {
			t.Fatalf("expected PhaseRestored after resume, err=%v op=%+v", err, op2)
		}
		for _, e := range op2.Entries {
			if e.State != EntryRestored {
				t.Fatalf("entry %q not marked restored after resume: state=%q", e.Path, e.State)
			}
		}
	})

	t.Run("busy_lease_blocks_restore_against_concurrent_apply", func(t *testing.T) {
		root, proj, _, opID := newAppliedProject(t)

		lease, err := proj.TryExclusive("apply-concurrent")
		if err != nil {
			t.Fatalf("TryExclusive: %v", err)
		}
		defer lease.Close()

		res, err := Restore(proj, opID, RestoreOptions{})
		if !errors.Is(err, syncfs.ErrBusy) {
			t.Fatalf("expected syncfs.ErrBusy, got err=%v", err)
		}
		if res.Status != StatusConflict || res.OperationID != opID {
			t.Fatalf("unexpected busy restore result: %+v", res)
		}
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "revision-cambios", "SKILL.md")); err != nil {
			t.Fatalf("busy restore mutated SKILL.md: %v", err)
		}
	})
}
