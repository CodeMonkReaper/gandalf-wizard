package projectsync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/syncfs"
)

const (
	StatusRestored = "restored"

	ActionRestore = "restore"
	ActionDelete  = "delete"
)

var (
	ErrRestoreConflict = errors.New("restore blocked by post-operation edits")
)

// RestoreOptions configures optional controlled interruption hooks for testing restore recovery.
type RestoreOptions struct {
	BeforeWrite func(path string) error
	AfterWrite  func(path string) error
}

// RestoreResult is the metadata-only result of Restore.
type RestoreResult struct {
	Status      string          `json:"status"`
	Reason      string          `json:"reason,omitempty"`
	ProjectID   string          `json:"project_id"`
	OperationID string          `json:"operation_id,omitempty"`
	Changes     []ChangeSummary `json:"changes,omitempty"`
	Diagnostics []Diagnostic    `json:"diagnostics,omitempty"`
}

// restorePlanItem is the pre-validated restore action for one operation entry.
type restorePlanItem struct {
	index       int
	needsWrite  bool
	cur         syncfs.FileState
	desired     syncfs.FileState
	action      string
	beforeBytes []byte
}

// Restore reverts the files written by operationID within project under an exclusive lease,
// pre-validating all written entries before mutating, preserving post-operation edits on conflict,
// supporting resumable recovery, and updating receipts and pending state.
func Restore(project *syncfs.ScopedProject, operationID string, opts RestoreOptions) (RestoreResult, error) {
	if project == nil {
		return RestoreResult{}, errors.New("projectsync: project is nil")
	}
	if !validOperationID(operationID) {
		return RestoreResult{
			Status:    StatusRejected,
			Reason:    "invalid operation id",
			ProjectID: project.ID(),
		}, fmt.Errorf("projectsync: invalid operation id %q", operationID)
	}

	leaseName := "restore-" + operationID
	if len(leaseName) > 64 {
		leaseName = operationID
	}
	lease, err := project.TryExclusive(leaseName)
	if err != nil {
		return RestoreResult{
			Status:      StatusConflict,
			Reason:      err.Error(),
			ProjectID:   project.ID(),
			OperationID: operationID,
		}, err
	}
	defer lease.Close()

	op, err := LoadOperation(project, operationID)
	if err != nil {
		return RestoreResult{
			Status:      StatusRejected,
			Reason:      err.Error(),
			ProjectID:   project.ID(),
			OperationID: operationID,
		}, err
	}
	if op.ProjectID != project.ID() {
		return RestoreResult{
			Status:      StatusRejected,
			Reason:      "operation belongs to another project",
			ProjectID:   project.ID(),
			OperationID: operationID,
		}, fmt.Errorf("projectsync: operation %q belongs to project %q, not %q", operationID, op.ProjectID, project.ID())
	}
	if op.Phase == PhaseRestored {
		return RestoreResult{Status: StatusNoOp, ProjectID: project.ID(), OperationID: operationID}, nil
	}

	plans, diags, err := planRestore(project, op)
	if err != nil {
		return RestoreResult{
			Status:      StatusRejected,
			Reason:      err.Error(),
			ProjectID:   project.ID(),
			OperationID: operationID,
		}, err
	}
	if len(diags) > 0 {
		return RestoreResult{
			Status:      StatusConflict,
			Reason:      "post-operation edits block restore",
			ProjectID:   project.ID(),
			OperationID: operationID,
			Diagnostics: diags,
		}, ErrRestoreConflict
	}

	recoveryPending := func(reason string, changes []ChangeSummary, err error) (RestoreResult, error) {
		return RestoreResult{
			Status:      StatusRecoveryPending,
			Reason:      reason,
			ProjectID:   project.ID(),
			OperationID: operationID,
			Changes:     changes,
		}, err
	}

	// From here on the operation is mid-restore: keep the phase and the pending
	// marker on disk so PendingRecovery sees an interrupted restore and Apply stays blocked.
	op.Phase = PhaseRestoring
	if err := persistOperation(project, lease, op); err != nil {
		return recoveryPending("restore state persist failed", nil, fmt.Errorf("projectsync: persist restoring phase: %w", err))
	}
	marker, err := json.Marshal(pendingMarker{OperationID: operationID})
	if err != nil {
		return recoveryPending("restore pending marker persist failed", nil, fmt.Errorf("projectsync: encode pending marker: %w", err))
	}
	if err := writeStateFile(project, lease, pendingPath, marker); err != nil {
		return recoveryPending("restore pending marker persist failed", nil, fmt.Errorf("projectsync: write pending marker: %w", err))
	}
	receipts, err := LoadReceipts(project)
	if err != nil {
		return recoveryPending("restore receipts load failed", nil, fmt.Errorf("projectsync: load receipts: %w", err))
	}

	summaries := make([]ChangeSummary, 0, len(plans))
	interrupted := func(reason string, err error) (RestoreResult, error) {
		if perr := persistOperation(project, lease, op); perr != nil {
			return recoveryPending(reason, summaries, fmt.Errorf("projectsync: %s: %w; persisting operation: %v", reason, err, perr))
		}
		return recoveryPending(reason, summaries, err)
	}

	for i := len(plans) - 1; i >= 0; i-- {
		item := plans[i]
		entry := op.Entries[item.index]

		if !item.needsWrite {
			// Already at its pre-operation state: bookkeeping only.
			op.Entries[item.index].State = EntryRestored
			if err := persistOperation(project, lease, op); err != nil {
				return interrupted("restore state persist failed", err)
			}
			receipts = receiptsAfterRestore(receipts, entry, item.beforeBytes)
			if err := persistReceipts(project, lease, receipts); err != nil {
				return interrupted("restore receipts persist failed", err)
			}
			continue
		}

		if opts.BeforeWrite != nil {
			if err := opts.BeforeWrite(entry.Path); err != nil {
				return interrupted("restore interrupted before write", err)
			}
		}
		if _, err := project.Change(lease, entry.Path, item.cur, item.desired); err != nil {
			return interrupted("restore write failed", err)
		}
		op.Entries[item.index].State = EntryRestored
		if err := persistOperation(project, lease, op); err != nil {
			return interrupted("restore state persist failed", err)
		}
		receipts = receiptsAfterRestore(receipts, entry, item.beforeBytes)
		if err := persistReceipts(project, lease, receipts); err != nil {
			return interrupted("restore receipts persist failed", err)
		}
		summaries = append(summaries, ChangeSummary{
			Path:         entry.Path,
			Action:       item.action,
			Targets:      entry.Targets,
			BeforeSHA256: item.cur.SHA256,
			AfterSHA256:  item.desired.SHA256,
		})
		if opts.AfterWrite != nil {
			if err := opts.AfterWrite(entry.Path); err != nil {
				return interrupted("restore interrupted after write", err)
			}
		}
	}

	op.Phase = PhaseRestored
	if err := persistOperation(project, lease, op); err != nil {
		return recoveryPending("restore state persist failed", summaries, fmt.Errorf("projectsync: persist restored phase: %w", err))
	}
	if err := removeStateFile(project, lease, pendingPath); err != nil {
		return RestoreResult{
			Status:      StatusRestored,
			ProjectID:   project.ID(),
			OperationID: operationID,
			Changes:     summaries,
		}, fmt.Errorf("projectsync: restore completed but pending marker cleanup failed: %w", err)
	}
	return RestoreResult{
		Status:      StatusRestored,
		ProjectID:   project.ID(),
		OperationID: operationID,
		Changes:     summaries,
	}, nil
}

// planRestore pre-validates every entry of op against the current filesystem state and
// returns the restore plan without mutating any target file. Entries modified after the
// operation are reported as diagnostics; missing or corrupt backups are reported as errors.
func planRestore(project *syncfs.ScopedProject, op Operation) ([]restorePlanItem, []Diagnostic, error) {
	plans := make([]restorePlanItem, 0, len(op.Entries))
	var diags []Diagnostic
	for i := range op.Entries {
		entry := op.Entries[i]
		if entry.State == EntryRestored {
			continue
		}
		cur, err := project.Observe(entry.Path)
		if err != nil {
			return nil, nil, err
		}
		switch entry.State {
		case EntryPending:
			if !matchesAfterState(entry, cur) {
				// The operation crashed before writing this entry; leave the
				// unwritten file and any external edit on it untouched.
				continue
			}
		case EntryWritten, EntryConflict:
			if matchesBeforeState(entry, cur) {
				plans = append(plans, restorePlanItem{index: i, cur: cur, beforeBytes: cur.Bytes})
				continue
			}
			if !matchesAfterState(entry, cur) {
				diags = append(diags, Diagnostic{
					Code:    "restore_conflict",
					Path:    entry.Path,
					Message: "file was modified after operation",
				})
				continue
			}
		default:
			return nil, nil, fmt.Errorf("projectsync: entry %q has unknown state %q", entry.Path, entry.State)
		}
		item := restorePlanItem{index: i, needsWrite: true, cur: cur}
		if entry.BeforeExists {
			backup, err := project.Observe(entry.BackupPath)
			if err != nil {
				return nil, nil, err
			}
			if !backup.Exists || backup.SHA256 != entry.BeforeSHA256 {
				return nil, nil, fmt.Errorf("projectsync: backup for %q is missing or corrupt", entry.Path)
			}
			item.desired = syncfs.NewFileState(backup.Bytes)
			item.action = ActionRestore
			item.beforeBytes = backup.Bytes
		} else {
			item.desired = syncfs.FileState{}
			item.action = ActionDelete
		}
		plans = append(plans, item)
	}
	return plans, diags, nil
}

// matchesBeforeState reports whether cur equals the entry's pre-operation state.
func matchesBeforeState(entry OperationEntry, cur syncfs.FileState) bool {
	if !entry.BeforeExists {
		return !cur.Exists
	}
	return cur.Exists && cur.SHA256 == entry.BeforeSHA256
}

// matchesAfterState reports whether cur equals the entry's post-operation state.
func matchesAfterState(entry OperationEntry, cur syncfs.FileState) bool {
	if !entry.AfterExists {
		return !cur.Exists
	}
	return cur.Exists && cur.SHA256 == entry.AfterSHA256
}

// receiptsAfterRestore updates receipts for a restored entry: a pre-operation file that
// carried a Gandalf block keeps a receipt pinned to its restored hash; every other entry
// loses its receipt.
func receiptsAfterRestore(receipts []Receipt, entry OperationEntry, beforeBytes []byte) []Receipt {
	if entry.BeforeExists && bytes.Contains(beforeBytes, []byte(BlockStart)) && bytes.Contains(beforeBytes, []byte(BlockEnd)) {
		return upsertReceipt(receipts, Receipt{Path: entry.Path, Owner: OwnerGandalf, AfterSHA256: entry.BeforeSHA256})
	}
	return removeReceipt(receipts, entry.Path)
}

// removeReceipt drops any receipt recorded for path.
func removeReceipt(receipts []Receipt, path string) []Receipt {
	kept := make([]Receipt, 0, len(receipts))
	for _, receipt := range receipts {
		if receipt.Path != path {
			kept = append(kept, receipt)
		}
	}
	return kept
}
