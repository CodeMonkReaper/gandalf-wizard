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

	t.Run("unlinked_pending_with_stage", func(t *testing.T) {
		root, proj, _, opID := newAppliedProject(t)

		// Simulate a crash between unlink and rename for the pre-existing AGENTS.md:
		// the target is gone, the staged bytes are still on disk, and the entry is pending.
		op, err := LoadOperation(proj, opID)
		if err != nil {
			t.Fatalf("LoadOperation: %v", err)
		}
		var agentsIdx = -1
		for i, e := range op.Entries {
			if e.Path == "AGENTS.md" {
				agentsIdx = i
			}
		}
		if agentsIdx < 0 {
			t.Fatalf("AGENTS.md entry missing from operation")
		}
		agentsEntry := op.Entries[agentsIdx]
		if !agentsEntry.BeforeExists {
			t.Fatalf("expected AGENTS.md to pre-exist")
		}
		afterBytes, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil {
			t.Fatalf("read applied AGENTS.md: %v", err)
		}
		stageName := ".gandalf-sync-" + opID + "-0123456789abcdef0123456789abcdef.tmp"
		if err := os.WriteFile(filepath.Join(root, stageName), afterBytes, 0o644); err != nil {
			t.Fatalf("write stage file: %v", err)
		}
		if err := os.Remove(filepath.Join(root, "AGENTS.md")); err != nil {
			t.Fatalf("unlink AGENTS.md: %v", err)
		}
		op.Entries[agentsIdx].State = EntryPending
		lease, err := proj.TryExclusive("test-unlinked-pending")
		if err != nil {
			t.Fatalf("TryExclusive: %v", err)
		}
		if err := persistOperation(proj, lease, op); err != nil {
			_ = lease.Close()
			t.Fatalf("persistOperation: %v", err)
		}
		if err := lease.Close(); err != nil {
			t.Fatalf("lease.Close: %v", err)
		}

		res, err := Restore(proj, opID, RestoreOptions{})
		if err != nil || res.Status != StatusRestored {
			t.Fatalf("expected Restore to recover unlinked pending entry, err=%v res=%+v", err, res)
		}
		gotAgents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil || string(gotAgents) != foreignAgents {
			t.Fatalf("AGENTS.md not restored from backup: err=%v got=%q", err, string(gotAgents))
		}
		if _, err := os.Stat(filepath.Join(root, stageName)); !os.IsNotExist(err) {
			t.Fatalf("stage file %q was not cleaned up", stageName)
		}
	})

	t.Run("armed_stage_cleanup_preserves_target", func(t *testing.T) {
		root, proj, _, opID := newAppliedProject(t)

		// Target still intact but externally edited: the pending entry must not be
		// reverted, yet the leftover stage file must be removed.
		editedAgents := foreignAgents + "\nedicion externa tras el crash\n"
		if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(editedAgents), 0o644); err != nil {
			t.Fatalf("edit AGENTS.md: %v", err)
		}
		stageName := ".gandalf-sync-" + opID + "-fedcba9876543210fedcba9876543210.tmp"
		if err := os.WriteFile(filepath.Join(root, stageName), []byte("staged bytes\n"), 0o644); err != nil {
			t.Fatalf("write stage file: %v", err)
		}
		op, err := LoadOperation(proj, opID)
		if err != nil {
			t.Fatalf("LoadOperation: %v", err)
		}
		for i := range op.Entries {
			if op.Entries[i].Path == "AGENTS.md" {
				op.Entries[i].State = EntryPending
			}
		}
		lease, err := proj.TryExclusive("test-armed-stage")
		if err != nil {
			t.Fatalf("TryExclusive: %v", err)
		}
		if err := persistOperation(proj, lease, op); err != nil {
			_ = lease.Close()
			t.Fatalf("persistOperation: %v", err)
		}
		if err := lease.Close(); err != nil {
			t.Fatalf("lease.Close: %v", err)
		}

		res, err := Restore(proj, opID, RestoreOptions{})
		if err != nil || res.Status != StatusRestored {
			t.Fatalf("expected Restore to succeed, err=%v res=%+v", err, res)
		}
		gotAgents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil || string(gotAgents) != editedAgents {
			t.Fatalf("external edit to AGENTS.md was not preserved: err=%v got=%q", err, string(gotAgents))
		}
		if _, err := os.Stat(filepath.Join(root, stageName)); !os.IsNotExist(err) {
			t.Fatalf("stage file %q was not cleaned up", stageName)
		}
	})

	t.Run("missing_pending_without_stage", func(t *testing.T) {
		root, proj, _, opID := newAppliedProject(t)

		// Target removed externally with no stage file: the deletion is external and
		// must be preserved, so AGENTS.md stays absent.
		if err := os.Remove(filepath.Join(root, "AGENTS.md")); err != nil {
			t.Fatalf("unlink AGENTS.md: %v", err)
		}
		op, err := LoadOperation(proj, opID)
		if err != nil {
			t.Fatalf("LoadOperation: %v", err)
		}
		for i := range op.Entries {
			if op.Entries[i].Path == "AGENTS.md" {
				op.Entries[i].State = EntryPending
			}
		}
		lease, err := proj.TryExclusive("test-missing-pending")
		if err != nil {
			t.Fatalf("TryExclusive: %v", err)
		}
		if err := persistOperation(proj, lease, op); err != nil {
			_ = lease.Close()
			t.Fatalf("persistOperation: %v", err)
		}
		if err := lease.Close(); err != nil {
			t.Fatalf("lease.Close: %v", err)
		}

		res, err := Restore(proj, opID, RestoreOptions{})
		if err != nil || res.Status != StatusRestored {
			t.Fatalf("expected Restore to succeed, err=%v res=%+v", err, res)
		}
		if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
			t.Fatalf("externally deleted AGENTS.md was recreated")
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

	t.Run("skill_update_restore_preserves_receipt_for_subsequent_preview_and_apply", func(t *testing.T) {
		root, proj, cat, _ := newAppliedProject(t)

		// Register skill v2.0.0 and apply an update over the v1.0.0 skill.
		skillV2Dir := filepath.Join(t.TempDir(), "revision-cambios-v2")
		if err := os.MkdirAll(filepath.Join(skillV2Dir, "references"), 0o755); err != nil {
			t.Fatalf("mkdir skill v2: %v", err)
		}
		if err := os.WriteFile(filepath.Join(skillV2Dir, "SKILL.md"), []byte("# Revision v2\n"), 0o644); err != nil {
			t.Fatalf("write SKILL.md v2: %v", err)
		}
		if err := os.WriteFile(filepath.Join(skillV2Dir, "references", "checklist.md"), []byte("Checklist v2\n"), 0o644); err != nil {
			t.Fatalf("write checklist v2: %v", err)
		}
		if _, err := cat.Register(synccatalog.KindSkill, "revision-cambios", "2.0.0", skillV2Dir, synccatalog.RegisterOptions{
			Origin:       "local:skill-v2",
			Dependencies: []synccatalog.Dependency{{Kind: synccatalog.KindRule, ID: "evidencia-de-pruebas", Version: "1.0.0"}},
		}); err != nil {
			t.Fatalf("register skill v2: %v", err)
		}
		selV2 := synccatalog.Selection{
			Targets: []string{"codex", "vibe"},
			Components: []synccatalog.ComponentRef{
				{Kind: synccatalog.KindSkill, ID: "revision-cambios", Version: "2.0.0"},
				{Kind: synccatalog.KindRule, ID: "evidencia-de-pruebas", Version: "1.0.0"},
				{Kind: synccatalog.KindPattern, ID: "revision-por-pares", Version: "1.0.0"},
			},
		}
		receipts, err := LoadReceipts(proj)
		if err != nil {
			t.Fatalf("LoadReceipts: %v", err)
		}
		v2, err := Preview(proj, cat, selV2, profiles, receipts)
		if err != nil || !v2.Applicable() || len(v2.Changes) != 2 {
			t.Fatalf("Preview v2: err=%v applicable=%v changes=%d diags=%+v", err, v2.Applicable(), len(v2.Changes), v2.Diagnostics)
		}
		op2Res, err := Apply(proj, cat, selV2, profiles, v2.ID, ApplyOptions{})
		if err != nil || op2Res.Status != StatusApplied {
			t.Fatalf("Apply v2: err=%v res=%+v", err, op2Res)
		}

		// Restore op2 back to v1.0.0 bytes -> skill ownership receipts must remain pinned to v1.0.0.
		restRes, err := Restore(proj, op2Res.OperationID, RestoreOptions{})
		if err != nil || restRes.Status != StatusRestored {
			t.Fatalf("Restore op2: err=%v res=%+v", err, restRes)
		}
		gotSkill, err := os.ReadFile(filepath.Join(root, ".agents", "skills", "revision-cambios", "SKILL.md"))
		if err != nil || !strings.Contains(string(gotSkill), "# Revision\n") {
			t.Fatalf("SKILL.md not restored to v1.0.0: err=%v got=%q", err, string(gotSkill))
		}
		receiptsAfter, err := LoadReceipts(proj)
		if err != nil {
			t.Fatalf("LoadReceipts after restore: %v", err)
		}
		// Previewing v1.0.0 must be applicable with 0 changes (no ownership_conflict).
		v1Check, err := Preview(proj, cat, sel, profiles, receiptsAfter)
		if err != nil || !v1Check.Applicable() || len(v1Check.Changes) != 0 {
			t.Fatalf("expected 0-change applicable preview after restoring skill update, err=%v v=%+v", err, v1Check)
		}
		// Re-applying v2.0.0 must also succeed without ownership_conflict.
		v2Again, err := Preview(proj, cat, selV2, profiles, receiptsAfter)
		if err != nil || !v2Again.Applicable() || len(v2Again.Changes) != 2 {
			t.Fatalf("expected applicable v2 preview after restoring skill update, err=%v v=%+v", err, v2Again)
		}
		op3Res, err := Apply(proj, cat, selV2, profiles, v2Again.ID, ApplyOptions{})
		if err != nil || op3Res.Status != StatusApplied {
			t.Fatalf("re-Apply v2 after restore: err=%v res=%+v", err, op3Res)
		}
	})

	t.Run("restore_blocked_when_different_operation_is_pending_recovery", func(t *testing.T) {
		_, proj, cat, op1ID := newAppliedProject(t)

		// Start a second operation (rule v2.0.0) and interrupt it so pending.json points to op2.
		ruleV2Path := filepath.Join(t.TempDir(), "rule-v2.md")
		if err := os.WriteFile(ruleV2Path, []byte("Regla v2.\n"), 0o644); err != nil {
			t.Fatalf("write rule v2: %v", err)
		}
		if _, err := cat.Register(synccatalog.KindRule, "evidencia-de-pruebas", "2.0.0", ruleV2Path, synccatalog.RegisterOptions{Origin: "local:rule-v2"}); err != nil {
			t.Fatalf("register rule v2: %v", err)
		}
		receipts, _ := LoadReceipts(proj)
		selV2 := synccatalog.Selection{
			Targets:    []string{"codex", "vibe"},
			Components: []synccatalog.ComponentRef{{Kind: synccatalog.KindRule, ID: "evidencia-de-pruebas", Version: "2.0.0"}},
		}
		v2, err := Preview(proj, cat, selV2, profiles, receipts)
		if err != nil || !v2.Applicable() {
			t.Fatalf("Preview v2: err=%v v=%+v", err, v2)
		}
		op2Res, err := Apply(proj, cat, selV2, profiles, v2.ID, ApplyOptions{
			AfterWrite: func(p string) error { return errors.New("crash mid op2") },
		})
		if err == nil || op2Res.Status != StatusRecoveryPending {
			t.Fatalf("expected interrupted op2, err=%v res=%+v", err, op2Res)
		}

		// Attempting to restore op1 while op2 is pending recovery must fail with ErrRecoveryPending
		// and preserve pending.json pointing to op2.
		res, err := Restore(proj, op1ID, RestoreOptions{})
		if !errors.Is(err, ErrRecoveryPending) || res.Status != StatusRecoveryPending || res.OperationID != op2Res.OperationID {
			t.Fatalf("expected ErrRecoveryPending referencing op2 (%s), got err=%v res=%+v", op2Res.OperationID, err, res)
		}
		pending, err := PendingRecovery(proj)
		if err != nil || pending == nil || pending.ID != op2Res.OperationID {
			t.Fatalf("pending.json was clobbered: err=%v pending=%+v", err, pending)
		}
	})
}
