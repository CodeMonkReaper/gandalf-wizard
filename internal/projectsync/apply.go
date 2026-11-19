package projectsync

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/synccatalog"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/syncfs"
)

const (
	// StateDir is the local project state root for sync metadata, operations, backups, and receipts.
	StateDir = ".gandalf/sync"
)

const (
	PhaseApplying        = "applying"
	PhaseApplied         = "applied"
	PhaseRecoveryPending = "recovery_pending"
	PhaseRestoring       = "restoring"
	PhaseRestored        = "restored"
)

const (
	EntryPending  = "pending"
	EntryWritten  = "written"
	EntryRestored = "restored"
	EntryConflict = "conflict"
)

const (
	StatusApplied         = "applied"
	StatusNoOp            = "noop"
	StatusConflict        = "conflict"
	StatusRecoveryPending = "recovery_pending"
	StatusRejected        = "rejected"
)

var (
	ErrUnauthorized    = errors.New("missing explicit view authorization")
	ErrStaleView       = errors.New("authorized view no longer matches current project state")
	ErrRecoveryPending = errors.New("pending recovery blocks new apply operations")
)

const (
	receiptsPath     = StateDir + "/receipts.json"
	pendingPath      = StateDir + "/pending.json"
	stateGitignore   = StateDir + "/.gitignore"
	stateGitignoreIn = "*\n"
	operationsDir    = StateDir + "/operations"
)

// operationIDPattern constrains operation identifiers to a safe, path-free token.
var operationIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// OperationEntry records the before/after metadata, backup path, and progress of one file change.
type OperationEntry struct {
	Path         string   `json:"path"`
	Action       string   `json:"action"`
	Targets      []string `json:"targets"`
	BeforeExists bool     `json:"before_exists"`
	BeforeSHA256 string   `json:"before_sha256,omitempty"`
	BeforeFileID string   `json:"before_file_id,omitempty"`
	BackupPath   string   `json:"backup_path,omitempty"`
	AfterExists  bool     `json:"after_exists"`
	AfterSHA256  string   `json:"after_sha256,omitempty"`
	State        string   `json:"state"`
}

// Operation is the persisted, recoverable journal of one apply/restore operation.
type Operation struct {
	ID        string           `json:"id"`
	ProjectID string           `json:"project_id"`
	ViewID    string           `json:"view_id"`
	Phase     string           `json:"phase"`
	Entries   []OperationEntry `json:"entries"`
}

// ChangeSummary exposes only metadata and hashes (never file contents).
type ChangeSummary struct {
	Path         string   `json:"path"`
	Action       string   `json:"action"`
	Targets      []string `json:"targets"`
	BeforeSHA256 string   `json:"before_sha256,omitempty"`
	AfterSHA256  string   `json:"after_sha256,omitempty"`
}

// ApplyResult is the metadata-only result of Apply.
type ApplyResult struct {
	Status      string          `json:"status"`
	Reason      string          `json:"reason,omitempty"`
	ProjectID   string          `json:"project_id"`
	ViewID      string          `json:"view_id,omitempty"`
	OperationID string          `json:"operation_id,omitempty"`
	Changes     []ChangeSummary `json:"changes,omitempty"`
	Diagnostics []Diagnostic    `json:"diagnostics,omitempty"`
}

// ApplyOptions configures optional operation ID override and controlled interruption hooks for testing.
type ApplyOptions struct {
	OperationID string
	BeforeWrite func(path string) error
	AfterWrite  func(path string) error
}

// pendingMarker is the on-disk pointer to the operation currently in flight.
type pendingMarker struct {
	OperationID string `json:"operation_id"`
}

// validOperationID reports whether id is a safe operation identifier.
func validOperationID(id string) bool {
	if id == "" || strings.Contains(id, "..") {
		return false
	}
	return operationIDPattern.MatchString(id)
}

// operationDir returns the state directory holding one operation's journal and backups.
func operationDir(operationID string) string {
	return operationsDir + "/" + operationID
}

// operationPath returns the journal path for one operation.
func operationPath(operationID string) string {
	return operationDir(operationID) + "/operation.json"
}

// backupPath returns the backup blob path for the change at index i.
func backupPath(operationID string, i int) string {
	return fmt.Sprintf("%s/backups/%04d.bak", operationDir(operationID), i)
}

// newOperationID returns a random, path-safe operation identifier.
func newOperationID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "op-" + hex.EncodeToString(buf), nil
}

// writeStateFile writes data to relPath under the exclusive lease, tolerating an unchanged file.
func writeStateFile(project *syncfs.ScopedProject, lease *syncfs.Lease, relPath string, data []byte) error {
	desired := syncfs.NewFileState(data)
	current, err := project.Observe(relPath)
	if err != nil {
		return err
	}
	if current.Exists && current.SHA256 == desired.SHA256 {
		return nil
	}
	_, err = project.Change(lease, relPath, current, desired)
	return err
}

// removeStateFile deletes relPath under the exclusive lease, tolerating an absent file.
func removeStateFile(project *syncfs.ScopedProject, lease *syncfs.Lease, relPath string) error {
	current, err := project.Observe(relPath)
	if err != nil {
		return err
	}
	if !current.Exists {
		return nil
	}
	_, err = project.Change(lease, relPath, current, syncfs.FileState{})
	return err
}

// persistOperation writes the operation journal under the exclusive lease.
func persistOperation(project *syncfs.ScopedProject, lease *syncfs.Lease, op Operation) error {
	data, err := json.MarshalIndent(op, "", "  ")
	if err != nil {
		return err
	}
	return writeStateFile(project, lease, operationPath(op.ID), append(data, '\n'))
}

// persistReceipts writes the receipt set sorted by path under the exclusive lease.
func persistReceipts(project *syncfs.ScopedProject, lease *syncfs.Lease, receipts []Receipt) error {
	sorted := make([]Receipt, len(receipts))
	copy(sorted, receipts)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	data, err := json.MarshalIndent(sorted, "", "  ")
	if err != nil {
		return err
	}
	return writeStateFile(project, lease, receiptsPath, append(data, '\n'))
}

// upsertReceipt records or replaces the receipt for one path.
func upsertReceipt(receipts []Receipt, receipt Receipt) []Receipt {
	for i := range receipts {
		if receipts[i].Path == receipt.Path {
			receipts[i] = receipt
			return receipts
		}
	}
	return append(receipts, receipt)
}

// LoadReceipts reads the current Gandalf ownership receipts stored under StateDir.
func LoadReceipts(project *syncfs.ScopedProject) ([]Receipt, error) {
	if project == nil {
		return nil, errors.New("projectsync: nil project")
	}
	state, err := project.Observe(receiptsPath)
	if err != nil {
		return nil, err
	}
	if !state.Exists {
		return []Receipt{}, nil
	}
	var receipts []Receipt
	if err := json.Unmarshal(state.Bytes, &receipts); err != nil {
		return nil, err
	}
	if receipts == nil {
		receipts = []Receipt{}
	}
	sort.Slice(receipts, func(i, j int) bool { return receipts[i].Path < receipts[j].Path })
	return receipts, nil
}

// LoadOperation reads a persisted operation journal by ID.
func LoadOperation(project *syncfs.ScopedProject, operationID string) (Operation, error) {
	if project == nil {
		return Operation{}, errors.New("projectsync: nil project")
	}
	if !validOperationID(operationID) {
		return Operation{}, fmt.Errorf("projectsync: invalid operation id %q", operationID)
	}
	relPath := operationPath(operationID)
	state, err := project.Observe(relPath)
	if err != nil {
		return Operation{}, err
	}
	if !state.Exists {
		return Operation{}, os.ErrNotExist
	}
	var op Operation
	if err := json.Unmarshal(state.Bytes, &op); err != nil {
		return Operation{}, err
	}
	return op, nil
}

// PendingRecovery returns the active incomplete operation requiring recovery, if any.
func PendingRecovery(project *syncfs.ScopedProject) (*Operation, error) {
	if project == nil {
		return nil, errors.New("projectsync: nil project")
	}
	state, err := project.Observe(pendingPath)
	if err != nil {
		return nil, err
	}
	if !state.Exists {
		return nil, nil
	}
	var marker pendingMarker
	if err := json.Unmarshal(state.Bytes, &marker); err != nil {
		return nil, err
	}
	if !validOperationID(marker.OperationID) {
		return nil, fmt.Errorf("projectsync: invalid pending operation id %q", marker.OperationID)
	}
	op, err := LoadOperation(project, marker.OperationID)
	if err != nil {
		return nil, err
	}
	switch op.Phase {
	case PhaseApplying, PhaseRecoveryPending, PhaseRestoring:
		return &op, nil
	default:
		return nil, nil
	}
}

// Apply revalidates the authorized preview under exclusive project lock, persists backups
// and operation intent before mutating targets, writes changes, and updates receipts.
func Apply(
	project *syncfs.ScopedProject,
	catalog *synccatalog.Catalog,
	selection synccatalog.Selection,
	profiles []NativeProfile,
	viewID string,
	opts ApplyOptions,
) (ApplyResult, error) {
	if project == nil {
		return ApplyResult{}, errors.New("projectsync: nil project")
	}
	if catalog == nil {
		return ApplyResult{}, errors.New("projectsync: nil catalog")
	}
	projectID := project.ID()

	if strings.TrimSpace(viewID) == "" {
		return ApplyResult{
			Status:    StatusRejected,
			Reason:    "missing view authorization",
			ProjectID: projectID,
		}, ErrUnauthorized
	}

	opID := strings.TrimSpace(opts.OperationID)
	if opID != "" {
		if !validOperationID(opID) {
			return ApplyResult{
				Status:    StatusRejected,
				Reason:    "invalid operation id",
				ProjectID: projectID,
				ViewID:    viewID,
			}, fmt.Errorf("projectsync: invalid operation id %q", opts.OperationID)
		}
	} else {
		generated, err := newOperationID()
		if err != nil {
			return ApplyResult{
				Status:    StatusRejected,
				Reason:    "cannot generate operation id",
				ProjectID: projectID,
				ViewID:    viewID,
			}, err
		}
		opID = generated
	}

	lease, err := project.TryExclusive(opID)
	if err != nil {
		return ApplyResult{
			Status:    StatusConflict,
			Reason:    err.Error(),
			ProjectID: projectID,
			ViewID:    viewID,
		}, err
	}
	defer func() { _ = lease.Close() }()

	pending, err := PendingRecovery(project)
	if err != nil {
		return ApplyResult{
			Status:    StatusRejected,
			Reason:    err.Error(),
			ProjectID: projectID,
			ViewID:    viewID,
		}, err
	}
	if pending != nil {
		return ApplyResult{
			Status:      StatusRecoveryPending,
			Reason:      "pending recovery exists",
			ProjectID:   projectID,
			ViewID:      viewID,
			OperationID: pending.ID,
		}, ErrRecoveryPending
	}

	existingOp, err := project.Observe(operationPath(opID))
	if err != nil {
		return ApplyResult{
			Status:    StatusRejected,
			Reason:    err.Error(),
			ProjectID: projectID,
			ViewID:    viewID,
		}, err
	}
	if existingOp.Exists {
		return ApplyResult{
			Status:      StatusConflict,
			Reason:      "operation id already exists",
			ProjectID:   projectID,
			ViewID:      viewID,
			OperationID: opID,
		}, fmt.Errorf("%w: operation %q already exists", syncfs.ErrConflict, opID)
	}

	receipts, err := LoadReceipts(project)
	if err != nil {
		return ApplyResult{
			Status:    StatusRejected,
			Reason:    err.Error(),
			ProjectID: projectID,
			ViewID:    viewID,
		}, err
	}

	view, err := Preview(project, catalog, selection, profiles, receipts)
	if err != nil {
		return ApplyResult{
			Status:    StatusRejected,
			Reason:    err.Error(),
			ProjectID: projectID,
			ViewID:    viewID,
		}, err
	}
	if !view.Applicable() {
		return ApplyResult{
			Status:      StatusRejected,
			Reason:      "preview has blocking diagnostics",
			ProjectID:   projectID,
			ViewID:      viewID,
			Diagnostics: view.Diagnostics,
		}, ErrStaleView
	}
	if view.ID != viewID {
		return ApplyResult{
			Status:    StatusConflict,
			Reason:    "view_id does not match current project state",
			ProjectID: projectID,
			ViewID:    viewID,
		}, ErrStaleView
	}
	if len(view.Changes) == 0 {
		return ApplyResult{
			Status:    StatusNoOp,
			ProjectID: projectID,
			ViewID:    view.ID,
		}, nil
	}

	summaries := make([]ChangeSummary, 0, len(view.Changes))
	entries := make([]OperationEntry, 0, len(view.Changes))
	for i, ch := range view.Changes {
		targets := make([]string, len(ch.Targets))
		copy(targets, ch.Targets)
		summaries = append(summaries, ChangeSummary{
			Path:         ch.Path,
			Action:       ch.Action,
			Targets:      targets,
			BeforeSHA256: ch.Before.SHA256,
			AfterSHA256:  ch.After.SHA256,
		})
		entry := OperationEntry{
			Path:         ch.Path,
			Action:       ch.Action,
			Targets:      targets,
			BeforeExists: ch.Before.Exists,
			BeforeSHA256: ch.Before.SHA256,
			BeforeFileID: ch.Before.FileID,
			AfterExists:  ch.After.Exists,
			AfterSHA256:  ch.After.SHA256,
			State:        EntryPending,
		}
		if ch.Before.Exists {
			entry.BackupPath = backupPath(opID, i)
		}
		entries = append(entries, entry)
	}

	op := Operation{
		ID:        opID,
		ProjectID: projectID,
		ViewID:    view.ID,
		Phase:     PhaseApplying,
		Entries:   entries,
	}

	recoveryResult := func(reason string, cause error) (ApplyResult, error) {
		op.Phase = PhaseRecoveryPending
		if persistErr := persistOperation(project, lease, op); persistErr != nil {
			return ApplyResult{
				Status:      StatusRecoveryPending,
				Reason:      reason,
				ProjectID:   projectID,
				ViewID:      view.ID,
				OperationID: opID,
				Changes:     summaries,
			}, errors.Join(cause, persistErr)
		}
		return ApplyResult{
			Status:      StatusRecoveryPending,
			Reason:      reason,
			ProjectID:   projectID,
			ViewID:      view.ID,
			OperationID: opID,
			Changes:     summaries,
		}, cause
	}

	rejectPreWrite := func(reason string, cause error) (ApplyResult, error) {
		return ApplyResult{
			Status:    StatusRejected,
			Reason:    reason,
			ProjectID: projectID,
			ViewID:    view.ID,
		}, cause
	}

	// Persist intent before the first target write: owner-only ACL, git exclusion, backups, journal, pending marker.
	if err := project.RestrictDir(lease, StateDir); err != nil {
		return rejectPreWrite("cannot restrict sync state directory", err)
	}
	if err := writeStateFile(project, lease, stateGitignore, []byte(stateGitignoreIn)); err != nil {
		return rejectPreWrite("cannot persist state gitignore", err)
	}
	for i, ch := range view.Changes {
		if !ch.Before.Exists {
			continue
		}
		if err := writeStateFile(project, lease, backupPath(opID, i), ch.Before.Bytes); err != nil {
			return rejectPreWrite("cannot persist backup", err)
		}
	}
	if err := persistOperation(project, lease, op); err != nil {
		return rejectPreWrite("cannot persist operation journal", err)
	}
	marker, err := json.Marshal(pendingMarker{OperationID: opID})
	if err != nil {
		return rejectPreWrite("cannot encode pending marker", err)
	}
	if err := writeStateFile(project, lease, pendingPath, append(marker, '\n')); err != nil {
		return rejectPreWrite("cannot persist pending marker", err)
	}

	for i, ch := range view.Changes {
		if opts.BeforeWrite != nil {
			if err := opts.BeforeWrite(ch.Path); err != nil {
				return recoveryResult("interrupted before write", err)
			}
		}
		if _, err := project.Change(lease, ch.Path, ch.Before, ch.After); err != nil {
			return recoveryResult("write conflict or failure", err)
		}
		op.Entries[i].State = EntryWritten
		if err := persistOperation(project, lease, op); err != nil {
			return recoveryResult("cannot persist operation journal", err)
		}
		receipts = upsertReceipt(receipts, Receipt{
			Path:        ch.Path,
			Owner:       OwnerGandalf,
			AfterSHA256: ch.After.SHA256,
		})
		if err := persistReceipts(project, lease, receipts); err != nil {
			return recoveryResult("cannot persist receipts", err)
		}
		if opts.AfterWrite != nil {
			if err := opts.AfterWrite(ch.Path); err != nil {
				return recoveryResult("interrupted after write", err)
			}
		}
	}

	op.Phase = PhaseApplied
	if err := persistOperation(project, lease, op); err != nil {
		return recoveryResult("cannot persist operation journal", err)
	}
	if err := removeStateFile(project, lease, pendingPath); err != nil {
		return recoveryResult("cannot clear pending marker", err)
	}

	return ApplyResult{
		Status:      StatusApplied,
		ProjectID:   projectID,
		ViewID:      view.ID,
		OperationID: opID,
		Changes:     summaries,
	}, nil
}
