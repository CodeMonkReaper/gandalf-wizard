// Package store provides os.Root-confined persistence, atomic checkpoints,
// redacted event logs, cooperative cancellation markers, and OS run locking.
package store

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/gates"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
)

var (
	// ErrUnsafePath indicates path traversal, reserved names, symlink/junction escapes, or unsafe path lengths.
	ErrUnsafePath = errors.New("unsafe workflow store path")
	// ErrRunBusy indicates another process or handle currently holds the exclusive run lock.
	ErrRunBusy = errors.New("workflow run lock is busy")
	// ErrRunNotFound indicates the requested workflow run directory or state does not exist.
	ErrRunNotFound = errors.New("workflow run not found")
	// ErrCorruptState indicates state.json failed JSON decoding or checkpoint digest verification.
	ErrCorruptState = errors.New("workflow run state is corrupt")
)

const (
	// MaxSafePathLen reserves headroom below the Windows 260-char MAX_PATH limit.
	MaxSafePathLen = 248
	// maxWindowsTempPathLen bounds temporary atomic staging file paths under Windows MAX_PATH.
	maxWindowsTempPathLen = 259
	// MaxSummaryBytes bounds stored redacted artifact summaries.
	MaxSummaryBytes = 240
)

// ArtifactRef records metadata and integrity hash of a persisted redacted step output.
type ArtifactRef struct {
	Name           string `json:"name"`
	RelPath        string `json:"rel_path"`
	SHA256         string `json:"sha256"`
	SizeBytes      int64  `json:"size_bytes"`
	Summary        string `json:"summary"`
	SecretDetected bool   `json:"secret_detected,omitempty"`
	JSON           bool   `json:"json,omitempty"`
}

// AttemptRecord captures one execution attempt for a step or correction loop.
type AttemptRecord struct {
	Iteration  int           `json:"iteration"`
	Status     string        `json:"status"`
	Diagnosis  string        `json:"diagnosis,omitempty"`
	GateStatus string        `json:"gate_status,omitempty"`
	Error      string        `json:"error,omitempty"`
	Artifacts  []ArtifactRef `json:"artifacts,omitempty"`
	StartedAt  string        `json:"started_at,omitempty"`
	EndedAt    string        `json:"ended_at,omitempty"`
}

// StepRecord persists the current status, attempts, artifacts, and diagnostics of one step.
type StepRecord struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Optional  bool            `json:"optional,omitempty"`
	Status    string          `json:"status"`
	Iteration int             `json:"iteration"`
	Attempts  []AttemptRecord `json:"attempts,omitempty"`
	Artifacts []ArtifactRef   `json:"artifacts,omitempty"`
	Warnings  []string        `json:"warnings,omitempty"`
	Error     string          `json:"error,omitempty"`
	// CorrectionCycleStart indexes the first attempt of the current bounded
	// correction cycle so a manual resume can grant a fresh budget without
	// discarding the audit trail of earlier cycles.
	CorrectionCycleStart int `json:"correction_cycle_start,omitempty"`
}

// GateRecord persists the latest deterministic outcome of a quality_gate step.
type GateRecord struct {
	StepID      string   `json:"step_id"`
	Target      string   `json:"target"`
	Prompt      string   `json:"prompt,omitempty"`
	Status      string   `json:"status"`
	Informative bool     `json:"informative,omitempty"`
	Retryable   bool     `json:"retryable,omitempty"`
	Violations  []string `json:"violations,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
	CheckedAt   string   `json:"checked_at,omitempty"`
}

// ApprovalRecord binds a human approval/rejection decision to a specific checkpoint digest.
type ApprovalRecord struct {
	GateID           string `json:"gate_id"`
	Decision         string `json:"decision"`
	Reason           string `json:"reason,omitempty"`
	CheckpointDigest string `json:"checkpoint_digest"`
	DecidedAt        string `json:"decided_at"`
	DecidedBy        string `json:"decided_by,omitempty"`
}

// ObjectiveVerdictRecord persists the closing objective evaluation in RunState.
type ObjectiveVerdictRecord struct {
	Status         string   `json:"status"`
	MetCriteria    []string `json:"met_criteria,omitempty"`
	FailedCriteria []string `json:"failed_criteria,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

// RunState represents the canonical persisted state of a workflow run in state.json.
type RunState struct {
	SchemaVersion    string                  `json:"schema_version"`
	RunID            string                  `json:"run_id"`
	WorkflowID       string                  `json:"workflow_id"`
	WorkflowVersion  string                  `json:"workflow_version"`
	ObjectiveID      string                  `json:"objective_id,omitempty"`
	ObjectiveVersion string                  `json:"objective_version,omitempty"`
	DefinitionPath   string                  `json:"definition_path"`
	DefinitionSHA256 string                  `json:"definition_sha256"`
	StateRevision    int64                   `json:"state_revision"`
	CheckpointDigest string                  `json:"checkpoint_digest"`
	Status           string                  `json:"status"`
	CreatedAt        string                  `json:"created_at"`
	UpdatedAt        string                  `json:"updated_at"`
	Steps            map[string]StepRecord   `json:"steps"`
	Gates            map[string]GateRecord   `json:"gates,omitempty"`
	Approvals        []ApprovalRecord        `json:"approvals,omitempty"`
	PendingGates     []string                `json:"pending_gates,omitempty"`
	Warnings         []string                `json:"warnings,omitempty"`
	Error            string                  `json:"error,omitempty"`
	ObjectiveVerdict *ObjectiveVerdictRecord `json:"objective_verdict,omitempty"`
}

// RunEvent represents a single redacted append-only event written to events.jsonl.
type RunEvent struct {
	Timestamp string `json:"timestamp"`
	Seq       int64  `json:"seq"`
	EventType string `json:"event_type"`
	StepID    string `json:"step_id,omitempty"`
	Iteration int    `json:"iteration,omitempty"`
	Status    string `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
}

type cancelPayload struct {
	Reason      string `json:"reason"`
	RequestedAt string `json:"requested_at"`
}

// WorkflowStore confines all runtime state to <projectRoot>/.gandalf/workflows via os.Root.
type WorkflowStore struct {
	mu            sync.Mutex
	rootDir       string
	workflowsPath string
	root          *os.Root
	secretValues  []string
}

// RunLock holds an OS-level exclusive non-blocking lock on a run's run.lock file.
type RunLock struct {
	mu   sync.Mutex
	file *os.File
}

// OpenRoot opens and validates the confined .gandalf/workflows store under projectRoot.
func OpenRoot(projectRoot string) (*WorkflowStore, error) {
	return openRoot(projectRoot, true)
}

// OpenExistingRoot opens the workflow store without creating any directories.
func OpenExistingRoot(projectRoot string) (*WorkflowStore, error) {
	return openRoot(projectRoot, false)
}

func openRoot(projectRoot string, create bool) (*WorkflowStore, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return nil, fmt.Errorf("%w: empty project root", ErrUnsafePath)
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsafePath, err)
	}
	absRoot = filepath.Clean(absRoot)

	info, err := os.Lstat(absRoot)
	if err != nil || !info.IsDir() || isReparseOrSymlink(info) {
		return nil, fmt.Errorf("%w: project root must be a local real directory", ErrUnsafePath)
	}

	resolved, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsafePath, err)
	}
	resolved = filepath.Clean(resolved)
	if !equalCleanPath(absRoot, resolved) {
		return nil, fmt.Errorf("%w: project root traverses symlink or junction", ErrUnsafePath)
	}

	outerRoot, err := os.OpenRoot(absRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsafePath, err)
	}
	defer func() {
		if outerRoot != nil {
			_ = outerRoot.Close()
		}
	}()

	for _, rel := range []string{".gandalf", ".gandalf/workflows"} {
		if err := openSafeDir(outerRoot, absRoot, rel, create); err != nil {
			return nil, err
		}
	}

	wfRoot, err := outerRoot.OpenRoot(".gandalf/workflows")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsafePath, err)
	}
	_ = outerRoot.Close()
	outerRoot = nil

	workflowsPath := filepath.Join(absRoot, ".gandalf", "workflows")
	if err := openSafeDir(wfRoot, workflowsPath, "runs", create); err != nil {
		_ = wfRoot.Close()
		return nil, err
	}

	return &WorkflowStore{
		rootDir:       absRoot,
		workflowsPath: workflowsPath,
		root:          wfRoot,
	}, nil
}

func equalCleanPath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func ensureSafeDir(root *os.Root, basePath, rel string) error {
	return openSafeDir(root, basePath, rel, true)
}

func openSafeDir(root *os.Root, basePath, rel string, create bool) error {
	full := filepath.Join(basePath, filepath.FromSlash(rel))
	if len(full) > MaxSafePathLen {
		return fmt.Errorf("%w: directory path exceeds safe length", ErrUnsafePath)
	}
	info, err := root.Lstat(rel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if !create {
				return ErrRunNotFound
			}
			if mkErr := root.Mkdir(rel, 0o700); mkErr != nil && !errors.Is(mkErr, os.ErrExist) {
				return fmt.Errorf("%w: mkdir %s: %v", ErrUnsafePath, rel, mkErr)
			}
			info, err = root.Lstat(rel)
			if err != nil {
				return fmt.Errorf("%w: stat created %s: %v", ErrUnsafePath, rel, err)
			}
		} else {
			return fmt.Errorf("%w: lstat %s: %v", ErrUnsafePath, rel, err)
		}
	}
	if !info.IsDir() || isReparseOrSymlink(info) {
		return fmt.Errorf("%w: %s is not a confined directory", ErrUnsafePath, rel)
	}
	return nil
}

// Close releases the underlying os.Root handle.
func (s *WorkflowStore) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root == nil {
		return nil
	}
	err := s.root.Close()
	s.root = nil
	return err
}

// SetSecretValues configures explicit credential values to redact before persisting outputs or logs.
func (s *WorkflowStore) SetSecretValues(values []string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			cp = append(cp, v)
		}
	}
	s.secretValues = cp
}

func (s *WorkflowStore) snapshotSecrets() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.secretValues) == 0 {
		return nil
	}
	return append([]string(nil), s.secretValues...)
}

// CreateRun allocates a new compact run ID, initializes state.json, and returns the initial RunState.
func (s *WorkflowStore) CreateRun(def schema.ValidatedWorkflow) (string, *RunState, error) {
	if s == nil || s.root == nil {
		return "", nil, errors.New("workflow store is closed")
	}
	runID, err := generateCompactRunID()
	if err != nil {
		return "", nil, fmt.Errorf("generate run id: %w", err)
	}
	if err := validateSegment(runID); err != nil {
		return "", nil, err
	}

	runRel := path.Join("runs", runID)
	artifactsRel := path.Join(runRel, "artifacts")
	stateRel := path.Join(runRel, "state.json")
	if err := s.checkSafeTarget(stateRel); err != nil {
		return "", nil, err
	}

	if err := s.root.Mkdir(runRel, 0o700); err != nil {
		return "", nil, fmt.Errorf("create run dir: %w", err)
	}
	if err := s.root.Mkdir(artifactsRel, 0o700); err != nil {
		_ = s.root.Remove(runRel)
		return "", nil, fmt.Errorf("create artifacts dir: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	steps := make(map[string]StepRecord, len(def.Definition.Steps))
	for _, st := range def.Definition.Steps {
		steps[st.ID] = StepRecord{
			ID:       st.ID,
			Kind:     string(st.Kind),
			Optional: st.Optional,
			Status:   "pending",
		}
	}

	state := &RunState{
		SchemaVersion:    schema.SchemaVersion,
		RunID:            runID,
		WorkflowID:       def.Definition.ID,
		WorkflowVersion:  def.Definition.Version,
		ObjectiveID:      def.Definition.Objective.ID,
		ObjectiveVersion: def.Definition.Objective.Version,
		DefinitionPath:   def.DefinitionPath,
		DefinitionSHA256: def.DefinitionSHA256,
		StateRevision:    1,
		Status:           "created",
		CreatedAt:        now,
		UpdatedAt:        now,
		Steps:            steps,
	}
	state.CheckpointDigest = ComputeCheckpointDigest(state)

	data, err := marshalStateJSON(state)
	if err != nil {
		return "", nil, err
	}
	if err := s.writeAtomic(stateRel, data); err != nil {
		return "", nil, err
	}
	return runID, state, nil
}

func generateCompactRunID() (string, error) {
	var rnd [4]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return "", err
	}
	ts := strconv.FormatInt(time.Now().UTC().Unix(), 36)
	return fmt.Sprintf("wf-%s-%s", ts, hex.EncodeToString(rnd[:])), nil
}

// AcquireExclusive opens and acquires a non-blocking OS exclusive lock on run.lock.
func (s *WorkflowStore) AcquireExclusive(runID string) (*RunLock, error) {
	if s == nil {
		return nil, errors.New("workflow store is nil")
	}
	if err := s.ensureRunDir(runID); err != nil {
		return nil, err
	}
	lockRel := path.Join("runs", runID, "run.lock")
	if err := s.checkSafeTarget(lockRel); err != nil {
		return nil, err
	}
	if err := rejectNonRegularIfExists(s.root, lockRel); err != nil {
		return nil, err
	}

	f, err := s.root.OpenFile(lockRel, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open run lock: %w", err)
	}
	if err := acquireFileLock(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &RunLock{file: f}, nil
}

// Close releases the OS exclusive lock and closes run.lock safely.
func (l *RunLock) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	f := l.file
	l.file = nil
	relErr := releaseFileLock(f)
	closeErr := f.Close()
	return errors.Join(relErr, closeErr)
}

// SaveArtifact redacts and atomically writes an output artifact under the run's artifacts/ directory.
func (s *WorkflowStore) SaveArtifact(runID, stepID string, iter int, name, content string) (ArtifactRef, error) {
	return s.saveArtifact(runID, stepID, iter, name, content, false)
}

// SaveJSONArtifact redacts values while preserving the shape of JSON evidence.
func (s *WorkflowStore) SaveJSONArtifact(runID, stepID string, iter int, name, content string) (ArtifactRef, error) {
	return s.saveArtifact(runID, stepID, iter, name, content, true)
}

func (s *WorkflowStore) saveArtifact(runID, stepID string, iter int, name, content string, structured bool) (ArtifactRef, error) {
	if s == nil {
		return ArtifactRef{}, errors.New("workflow store is nil")
	}
	if err := s.ensureRunDir(runID); err != nil {
		return ArtifactRef{}, err
	}
	if err := validateSegment(stepID); err != nil {
		return ArtifactRef{}, err
	}
	if err := validateSegment(name); err != nil {
		return ArtifactRef{}, err
	}
	if iter < 0 {
		return ArtifactRef{}, fmt.Errorf("%w: negative iteration", ErrUnsafePath)
	}

	if err := ensureSafeDir(s.root, s.workflowsPath, path.Join("runs", runID, "artifacts")); err != nil {
		return ArtifactRef{}, err
	}

	// A tuple hash distinguishes delimiter boundaries and letter case on Windows.
	key := fmt.Sprintf("%s\x00%d\x00%s", stepID, iter, name)
	digest := sha256.Sum256([]byte(key))
	fileName := fmt.Sprintf("%x.txt", digest[:30])
	relUnderRun := path.Join("artifacts", fileName)
	targetRel := path.Join("runs", runID, relUnderRun)
	if err := s.checkSafeTarget(targetRel); err != nil {
		return ArtifactRef{}, err
	}

	secrets := s.snapshotSecrets()
	var clean string
	var found bool
	if structured {
		var err error
		clean, found, err = gates.RedactJSON(content, secrets)
		if err != nil {
			return ArtifactRef{}, err
		}
	} else {
		clean = redactText(content, secrets)
		found = gates.HasSecrets(content, secrets)
	}
	sum := sha256.Sum256([]byte(clean))

	if err := s.writeAtomic(targetRel, []byte(clean)); err != nil {
		return ArtifactRef{}, err
	}

	return ArtifactRef{
		Name:           name,
		RelPath:        relUnderRun,
		SHA256:         hex.EncodeToString(sum[:]),
		SizeBytes:      int64(len(clean)),
		Summary:        buildRedactedSummary(clean),
		SecretDetected: found,
		JSON:           structured,
	}, nil
}

func buildRedactedSummary(content string) string {
	singleLine := strings.Join(strings.Fields(content), " ")
	if len(singleLine) <= MaxSummaryBytes {
		return singleLine
	}
	// Trim cleanly at UTF-8 rune boundary leaving room for "...".
	limit := MaxSummaryBytes - 3
	n := 0
	for i := range singleLine {
		if i > limit {
			break
		}
		n = i
	}
	if n == 0 {
		return ""
	}
	return singleLine[:n] + "..."
}

// ReadArtifact safely reads a previously persisted artifact within the run's root.
func (s *WorkflowStore) ReadArtifact(runID, relPath string) (string, error) {
	if s == nil {
		return "", errors.New("workflow store is nil")
	}
	if err := s.ensureRunDir(runID); err != nil {
		return "", err
	}
	parts, err := splitAndValidateRelPath(relPath)
	if err != nil {
		return "", err
	}
	if len(parts) < 2 || parts[0] != "artifacts" {
		return "", fmt.Errorf("%w: artifact must reside under artifacts/", ErrUnsafePath)
	}
	targetRel := path.Join("runs", runID, path.Join(parts...))
	info, err := s.root.Lstat(targetRel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%w: artifact not found", ErrUnsafePath)
		}
		return "", fmt.Errorf("%w: %v", ErrUnsafePath, err)
	}
	if !info.Mode().IsRegular() || isReparseOrSymlink(info) {
		return "", fmt.Errorf("%w: invalid artifact file", ErrUnsafePath)
	}
	raw, err := s.root.ReadFile(targetRel)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnsafePath, err)
	}
	return string(raw), nil
}

// AppendEvent redacts and appends one event line to the run's events.jsonl log.
func (s *WorkflowStore) AppendEvent(runID string, ev RunEvent) error {
	if s == nil {
		return errors.New("workflow store is nil")
	}
	if err := s.ensureRunDir(runID); err != nil {
		return err
	}
	secrets := s.snapshotSecrets()
	if strings.TrimSpace(ev.Timestamp) == "" {
		ev.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	ev.Message = redactText(ev.Message, secrets)

	line, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	line = append(line, '\n')

	eventsRel := path.Join("runs", runID, "events.jsonl")
	if err := s.checkSafeTarget(eventsRel); err != nil {
		return err
	}
	if err := rejectNonRegularIfExists(s.root, eventsRel); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root == nil {
		return errors.New("workflow store is closed")
	}
	f, err := s.root.OpenFile(eventsRel, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open events log: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("write event: %w", err)
	}
	return f.Sync()
}

// ReadEvents reads and decodes all events recorded in the run's events.jsonl log.
func (s *WorkflowStore) ReadEvents(runID string) ([]RunEvent, error) {
	if s == nil {
		return nil, errors.New("workflow store is nil")
	}
	if err := s.ensureRunDir(runID); err != nil {
		return nil, err
	}
	eventsRel := path.Join("runs", runID, "events.jsonl")
	if err := rejectNonRegularIfExists(s.root, eventsRel); err != nil {
		return nil, err
	}
	raw, err := s.root.ReadFile(eventsRel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read events: %w", err)
	}

	var events []RunEvent
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev RunEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			return nil, fmt.Errorf("%w: corrupt event line: %v", ErrCorruptState, err)
		}
		events = append(events, ev)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan events: %w", err)
	}
	return events, nil
}

// LoadRun reads state.json and verifies its structure and canonical checkpoint digest.
func (s *WorkflowStore) LoadRun(runID string) (*RunState, error) {
	if s == nil {
		return nil, errors.New("workflow store is nil")
	}
	if err := s.ensureRunDir(runID); err != nil {
		return nil, err
	}
	stateRel := path.Join("runs", runID, "state.json")
	info, err := s.root.Lstat(stateRel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: missing state.json", ErrCorruptState)
		}
		return nil, fmt.Errorf("%w: %v", ErrUnsafePath, err)
	}
	if !info.Mode().IsRegular() || isReparseOrSymlink(info) {
		return nil, fmt.Errorf("%w: unsafe state.json", ErrUnsafePath)
	}
	return readStateFile(s.root, stateRel, runID)
}

// WriteCancelRequest writes cancel.req without requiring RunLock for cooperative cancellation.
func (s *WorkflowStore) WriteCancelRequest(runID, reason string) error {
	if s == nil {
		return errors.New("workflow store is nil")
	}
	if err := s.ensureRunDir(runID); err != nil {
		return err
	}
	secrets := s.snapshotSecrets()
	payload := cancelPayload{
		Reason:      redactText(reason, secrets),
		RequestedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal cancel request: %w", err)
	}
	return s.writeAtomic(path.Join("runs", runID, "cancel.req"), append(data, '\n'))
}

// HasCancelRequest reports whether cancel.req exists for the run and returns its redacted reason.
func (s *WorkflowStore) HasCancelRequest(runID string) (bool, string, error) {
	if s == nil {
		return false, "", errors.New("workflow store is nil")
	}
	if err := s.ensureRunDir(runID); err != nil {
		return false, "", err
	}
	cancelRel := path.Join("runs", runID, "cancel.req")
	if err := rejectNonRegularIfExists(s.root, cancelRel); err != nil {
		return false, "", err
	}
	raw, err := s.root.ReadFile(cancelRel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("read cancel request: %w", err)
	}
	var payload cancelPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return true, redactText(strings.TrimSpace(string(raw)), s.snapshotSecrets()), nil
	}
	return true, redactText(payload.Reason, s.snapshotSecrets()), nil
}

func (s *WorkflowStore) ensureRunDir(runID string) error {
	if s.root == nil {
		return errors.New("workflow store is closed")
	}
	if err := validateSegment(runID); err != nil {
		return err
	}
	runRel := path.Join("runs", runID)
	info, err := s.root.Lstat(runRel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrRunNotFound
		}
		return fmt.Errorf("%w: %v", ErrUnsafePath, err)
	}
	if isReparseOrSymlink(info) || !info.IsDir() {
		return fmt.Errorf("%w: run entry is not a real directory", ErrUnsafePath)
	}
	return nil
}

func (s *WorkflowStore) checkSafeTarget(rel string) error {
	full := filepath.Join(s.workflowsPath, filepath.FromSlash(rel))
	if len(full) > MaxSafePathLen {
		return fmt.Errorf("%w: path exceeds %d characters", ErrUnsafePath, MaxSafePathLen)
	}
	return nil
}

func (s *WorkflowStore) writeAtomic(targetRel string, data []byte) error {
	if s.root == nil {
		return errors.New("workflow store is closed")
	}
	if err := s.checkSafeTarget(targetRel); err != nil {
		return err
	}
	if err := rejectNonRegularIfExists(s.root, targetRel); err != nil {
		return err
	}
	var rnd [6]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return fmt.Errorf("random temp name: %w", err)
	}
	dir := path.Dir(targetRel)
	tmpRel := path.Join(dir, ".tmp-"+hex.EncodeToString(rnd[:]))
	if runtime.GOOS == "windows" {
		fullTmp := filepath.Join(s.workflowsPath, filepath.FromSlash(tmpRel))
		if len(fullTmp) > maxWindowsTempPathLen {
			return fmt.Errorf("%w: staging path exceeds Windows MAX_PATH budget", ErrUnsafePath)
		}
	}

	f, err := s.root.OpenFile(tmpRel, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = f.Close()
			_ = s.root.Remove(tmpRel)
		}
	}()

	for written := 0; written < len(data); {
		n, wErr := f.Write(data[written:])
		if wErr != nil {
			return fmt.Errorf("write temp file: %w", wErr)
		}
		written += n
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	cleanup = false

	if err := s.root.Rename(tmpRel, targetRel); err != nil {
		_ = s.root.Remove(tmpRel)
		return fmt.Errorf("atomic rename: %w", err)
	}
	return nil
}

func splitAndValidateRelPath(rel string) ([]string, error) {
	if !utf8.ValidString(rel) || rel == "" || strings.Contains(rel, `\`) || strings.HasPrefix(rel, "/") {
		return nil, ErrUnsafePath
	}
	parts := strings.Split(rel, "/")
	for _, part := range parts {
		if err := validateSegment(part); err != nil {
			return nil, err
		}
	}
	return parts, nil
}

func validateSegment(seg string) error {
	if seg == "" || len(seg) > 64 || !utf8.ValidString(seg) {
		return ErrUnsafePath
	}
	if seg == "." || seg == ".." {
		return ErrUnsafePath
	}
	if strings.TrimRight(seg, " .") != seg {
		return ErrUnsafePath
	}
	if strings.ContainsAny(seg, `/\:*?"<>|`) {
		return ErrUnsafePath
	}
	if strings.ContainsFunc(seg, func(r rune) bool { return r < 32 }) {
		return ErrUnsafePath
	}
	stem, _, _ := strings.Cut(seg, ".")
	if isWindowsReservedDevice(stem) || isWindowsReservedDevice(seg) {
		return ErrUnsafePath
	}
	return nil
}

func isWindowsReservedDevice(name string) bool {
	upper := strings.ToUpper(strings.TrimRight(name, " "))
	switch upper {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(upper) == 4 && (strings.HasPrefix(upper, "COM") || strings.HasPrefix(upper, "LPT")) {
		return upper[3] >= '1' && upper[3] <= '9'
	}
	return false
}

func rejectNonRegularIfExists(root *os.Root, rel string) error {
	info, err := root.Lstat(rel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("%w: %v", ErrUnsafePath, err)
	}
	if !info.Mode().IsRegular() || isReparseOrSymlink(info) {
		return fmt.Errorf("%w: non-regular or symlinked target %s", ErrUnsafePath, rel)
	}
	return nil
}
