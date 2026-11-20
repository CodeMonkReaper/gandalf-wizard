package store

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
)

func sampleValidatedWorkflow() schema.ValidatedWorkflow {
	return schema.ValidatedWorkflow{
		DefinitionPath:   "workflows/analisis-propuesta.json",
		DefinitionSHA256: strings.Repeat("ab", 32),
		ExecutionOrder:   []string{"contexto_romero", "sintesis"},
		RequiresMemory:   true,
		RequiresRouter:   true,
		Definition: schema.WorkflowDefinition{
			SchemaVersion:    schema.SchemaVersion,
			ID:               "analisis-propuesta",
			Version:          "1.0.0",
			ConcurrencyLimit: 4,
			Objective: schema.ObjectiveSpec{
				ID:        "OBJ-PROP-001",
				Version:   "1.0.0",
				Statement: "Analizar propuesta tecnica",
				Deliverables: []schema.DeliverableRef{
					{Name: "reporte", Source: "sintesis.report"},
				},
				SuccessCriteria: []schema.CriterionSpec{
					{
						ID:          "CRIT-1",
						Deliverable: "reporte",
						Rules:       []schema.GateRuleSpec{{Kind: schema.RuleNonEmpty}},
					},
				},
			},
			Steps: []schema.StepSpec{
				{
					ID:      "contexto_romero",
					Kind:    schema.StepKindMemoryQuery,
					Outputs: []string{"summary"},
					Memory:  &schema.MemoryStepSpec{Query: "arquitectura"},
				},
				{
					ID:        "sintesis",
					Kind:      schema.StepKindRouterTask,
					DependsOn: []string{"contexto_romero"},
					Optional:  false,
					Outputs:   []string{"report"},
					Router:    &schema.RouterStepSpec{Prompt: "sintetiza"},
				},
			},
		},
	}
}

func createJunctionOrSymlink(t *testing.T, linkPath, targetDir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		out, err := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", linkPath, targetDir).CombinedOutput()
		if err != nil {
			t.Fatalf("mklink /J %s -> %s: %v (%s)", linkPath, targetDir, err, out)
		}
		return
	}
	if err := os.Symlink(targetDir, linkPath); err != nil {
		t.Fatalf("os.Symlink %s -> %s: %v", linkPath, targetDir, err)
	}
}

func TestWorkflowStoreLockHelper(t *testing.T) {
	if os.Getenv("GANDALF_WORKFLOW_LOCK_HELPER") != "1" {
		return
	}
	projectRoot := os.Getenv("GANDALF_HELPER_ROOT")
	runID := os.Getenv("GANDALF_HELPER_RUN")
	holdSeconds := 10 * time.Second

	st, err := OpenRoot(projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "OpenRoot error: %v\n", err)
		os.Exit(2)
	}
	defer func() { _ = st.Close() }()

	lock, err := st.AcquireExclusive(runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "AcquireExclusive error: %v\n", err)
		os.Exit(3)
	}
	defer func() { _ = lock.Close() }()

	fmt.Println("LOCKED")
	time.Sleep(holdSeconds)
}

func TestWorkflowStore(t *testing.T) {
	t.Run("confinamiento_y_rechazo_de_junctions_escapes_y_max_path", func(t *testing.T) {
		outside := t.TempDir()
		missing := filepath.Join(outside, "does-not-exist")
		if _, err := OpenRoot(missing); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("OpenRoot(missing) = %v, want ErrUnsafePath", err)
		}

		fileRoot := filepath.Join(outside, "plain.txt")
		if err := os.WriteFile(fileRoot, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenRoot(fileRoot); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("OpenRoot(file) = %v, want ErrUnsafePath", err)
		}

		junctionRoot := filepath.Join(outside, "root-junction")
		targetDir := filepath.Join(outside, "target")
		if err := os.Mkdir(targetDir, 0o700); err != nil {
			t.Fatal(err)
		}
		createJunctionOrSymlink(t, junctionRoot, targetDir)
		if _, err := OpenRoot(junctionRoot); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("OpenRoot(junction) = %v, want ErrUnsafePath", err)
		}

		// Junction nested inside .gandalf must also be rejected.
		projWithNestedLink := t.TempDir()
		if err := os.Mkdir(filepath.Join(projWithNestedLink, ".gandalf"), 0o700); err != nil {
			t.Fatal(err)
		}
		createJunctionOrSymlink(t, filepath.Join(projWithNestedLink, ".gandalf", "workflows"), targetDir)
		if _, err := OpenRoot(projWithNestedLink); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("OpenRoot(nested junction) = %v, want ErrUnsafePath", err)
		}

		// Valid root checks path traversal and long names.
		projectRoot := t.TempDir()
		st, err := OpenRoot(projectRoot)
		if err != nil {
			t.Fatalf("OpenRoot(valid): %v", err)
		}
		defer func() { _ = st.Close() }()

		runID, _, err := st.CreateRun(sampleValidatedWorkflow())
		if err != nil {
			t.Fatalf("CreateRun: %v", err)
		}

		unsafeIDs := []string{
			"",
			"..",
			"../escape",
			"a/b",
			`a\b`,
			"NUL",
			"run:alt",
			strings.Repeat("x", 65),
		}
		for _, bad := range unsafeIDs {
			if _, err := st.LoadRun(bad); !errors.Is(err, ErrUnsafePath) {
				t.Errorf("LoadRun(%q) = %v, want ErrUnsafePath", bad, err)
			}
			if _, err := st.AcquireExclusive(bad); !errors.Is(err, ErrUnsafePath) {
				t.Errorf("AcquireExclusive(%q) = %v, want ErrUnsafePath", bad, err)
			}
			if _, err := st.SaveArtifact(runID, "sintesis", 1, bad, "ok"); !errors.Is(err, ErrUnsafePath) {
				t.Errorf("SaveArtifact(name=%q) = %v, want ErrUnsafePath", bad, err)
			}
			if _, err := st.SaveArtifact(runID, bad, 1, "report", "ok"); !errors.Is(err, ErrUnsafePath) {
				t.Errorf("SaveArtifact(step=%q) = %v, want ErrUnsafePath", bad, err)
			}
		}

		// Traversal inside ReadArtifact.
		if _, err := st.ReadArtifact(runID, "../../state.json"); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("ReadArtifact traversal = %v, want ErrUnsafePath", err)
		}

		// Junction placed as a run directory must be rejected.
		escapedRun := "escaped-run"
		createJunctionOrSymlink(t, filepath.Join(projectRoot, ".gandalf", "workflows", "runs", escapedRun), targetDir)
		if _, err := st.LoadRun(escapedRun); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("LoadRun(run junction) = %v, want ErrUnsafePath", err)
		}

		// MAX_PATH protection: build a deep projectRoot (bounded <= 185 chars so CreateRun succeeds) where artifact path > MaxSafePathLen.
		baseDeep := t.TempDir()
		deepRoot := baseDeep
		const maxDeepRoot = 185
		for len(deepRoot) < 175 && len(deepRoot)+4 < maxDeepRoot {
			padLen := min(36, maxDeepRoot-len(deepRoot)-1)
			if padLen < 1 {
				break
			}
			deepRoot = filepath.Join(deepRoot, strings.Repeat("d", padLen))
		}
		if err := os.MkdirAll(deepRoot, 0o700); err != nil {
			t.Fatalf("mkdir deepRoot: %v", err)
		}
		deepStore, err := OpenRoot(deepRoot)
		if err != nil {
			t.Fatalf("OpenRoot(deepRoot): %v", err)
		}
		defer func() { _ = deepStore.Close() }()
		deepRunID, _, err := deepStore.CreateRun(sampleValidatedWorkflow())
		if err != nil {
			t.Fatalf("deep CreateRun: %v", err)
		}
		if _, err := deepStore.SaveArtifact(deepRunID, "sintesis", 1, strings.Repeat("a", 40), "payload"); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("SaveArtifact exceeding MAX_PATH got %v, want ErrUnsafePath", err)
		}
	})

	t.Run("apertura_concurrente_openroot", func(t *testing.T) {
		for round := 0; round < 15; round++ {
			freshRoot := t.TempDir()
			const workers = 12
			start := make(chan struct{})
			errs := make(chan error, workers)
			var wg sync.WaitGroup
			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					st, err := OpenRoot(freshRoot)
					if err != nil {
						errs <- err
						return
					}
					if closeErr := st.Close(); closeErr != nil {
						errs <- closeErr
					}
				}()
			}
			close(start)
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Fatalf("round %d concurrent OpenRoot failed: %v", round, err)
			}
		}
	})

	t.Run("creacion_checkpoint_artefactos_eventos_y_corrupcion", func(t *testing.T) {
		projectRoot := t.TempDir()
		st, err := OpenRoot(projectRoot)
		if err != nil {
			t.Fatalf("OpenRoot: %v", err)
		}
		defer func() { _ = st.Close() }()

		dynSecret := "romero-secret-" + "998877"
		mistralToken := "mstr" + "l_" + "StoreLeakSentinel99"
		st.SetSecretValues([]string{dynSecret})

		if _, err := st.LoadRun("missing-run-001"); !errors.Is(err, ErrRunNotFound) {
			t.Fatalf("LoadRun(missing) = %v, want ErrRunNotFound", err)
		}

		runID, initialState, err := st.CreateRun(sampleValidatedWorkflow())
		if err != nil {
			t.Fatalf("CreateRun: %v", err)
		}
		if len(runID) == 0 || len(runID) > 32 {
			t.Fatalf("runID %q must be compact (1..32 chars)", runID)
		}
		if initialState.StateRevision != 1 || len(initialState.CheckpointDigest) != 64 {
			t.Fatalf("unexpected initial state revision=%d digest=%q", initialState.StateRevision, initialState.CheckpointDigest)
		}
		if initialState.Steps["sintesis"].Status != "pending" {
			t.Fatalf("initial step status = %q, want pending", initialState.Steps["sintesis"].Status)
		}

		// Save artifact with secret material: store must enforce redaction and checksum.
		rawArtifact := "Informe final con " + dynSecret + " y " + mistralToken
		ref, err := st.SaveArtifact(runID, "sintesis", 2, "report", rawArtifact)
		if err != nil {
			t.Fatalf("SaveArtifact: %v", err)
		}
		gotContent, err := st.ReadArtifact(runID, ref.RelPath)
		if err != nil {
			t.Fatalf("ReadArtifact: %v", err)
		}
		if strings.Contains(gotContent, dynSecret) || strings.Contains(gotContent, mistralToken) {
			t.Fatalf("artifact persisted unredacted secret: %q", gotContent)
		}
		if strings.Contains(ref.Summary, dynSecret) || strings.Contains(ref.Summary, mistralToken) {
			t.Fatalf("artifact summary leaked secret: %q", ref.Summary)
		}
		expectedSum := sha256.Sum256([]byte(gotContent))
		if ref.SHA256 != hex.EncodeToString(expectedSum[:]) || ref.SizeBytes != int64(len(gotContent)) {
			t.Fatalf("artifact ref mismatch: %+v (len=%d)", ref, len(gotContent))
		}

		// Verify canonical digest determinism regardless of map key insertion order.
		cloneA := *initialState
		cloneA.Steps = map[string]StepRecord{
			"contexto_romero": initialState.Steps["contexto_romero"],
			"sintesis":        initialState.Steps["sintesis"],
		}
		cloneB := *initialState
		cloneB.Steps = map[string]StepRecord{
			"sintesis":        initialState.Steps["sintesis"],
			"contexto_romero": initialState.Steps["contexto_romero"],
		}
		if ComputeCheckpointDigest(&cloneA) != ComputeCheckpointDigest(&cloneB) {
			t.Fatalf("ComputeCheckpointDigest is not deterministic across map order")
		}

		// Update state via WriteCheckpoint and confirm revision, redaction, and digest advance.
		initialState.Status = "awaiting_approval"
		initialState.Error = "fallo temporal con " + dynSecret
		initialState.PendingGates = []string{"aprobacion_revision"}
		stepRec := initialState.Steps["sintesis"]
		stepRec.Status = "passed"
		stepRec.Iteration = 2
		stepRec.Artifacts = []ArtifactRef{ref}
		initialState.Steps["sintesis"] = stepRec

		digest2, err := st.WriteCheckpoint(runID, initialState)
		if err != nil {
			t.Fatalf("WriteCheckpoint: %v", err)
		}
		if initialState.StateRevision != 2 || len(digest2) != 64 {
			t.Fatalf("unexpected revision=%d digest=%q after WriteCheckpoint", initialState.StateRevision, digest2)
		}
		loaded, err := st.LoadRun(runID)
		if err != nil {
			t.Fatalf("LoadRun after WriteCheckpoint: %v", err)
		}
		if loaded.CheckpointDigest != digest2 || loaded.Status != "awaiting_approval" {
			t.Fatalf("loaded mismatch: status=%q digest=%q want %q", loaded.Status, loaded.CheckpointDigest, digest2)
		}
		if strings.Contains(loaded.Error, dynSecret) {
			t.Fatalf("loaded RunState.Error leaked secret: %q", loaded.Error)
		}

		// No residual .tmp files in run dir or artifacts dir.
		runDir := filepath.Join(projectRoot, ".gandalf", "workflows", "runs", runID)
		for _, dir := range []string{runDir, filepath.Join(runDir, "artifacts")} {
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("ReadDir %s: %v", dir, err)
			}
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), ".tmp") {
					t.Fatalf("residual temp file left behind: %s/%s", dir, e.Name())
				}
			}
		}

		// AppendEvent + ReadEvents with redaction.
		if err := st.AppendEvent(runID, RunEvent{
			Seq:       1,
			EventType: "step_failed",
			StepID:    "sintesis",
			Iteration: 1,
			Status:    "failed",
			Message:   "error usando " + mistralToken,
		}); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
		events, err := st.ReadEvents(runID)
		if err != nil {
			t.Fatalf("ReadEvents: %v", err)
		}
		if len(events) != 1 || strings.Contains(events[0].Message, mistralToken) {
			t.Fatalf("events mismatch or secret leaked: %+v", events)
		}

		// Tampering state.json digest/content must yield ErrCorruptState.
		statePath := filepath.Join(runDir, "state.json")
		rawState, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatal(err)
		}
		tampered := strings.Replace(string(rawState), `"awaiting_approval"`, `"succeeded"`, 1)
		if err := os.WriteFile(statePath, []byte(tampered), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := st.LoadRun(runID); !errors.Is(err, ErrCorruptState) {
			t.Fatalf("LoadRun(tampered) = %v, want ErrCorruptState", err)
		}

		if err := os.WriteFile(statePath, []byte(`{broken`), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := st.LoadRun(runID); !errors.Is(err, ErrCorruptState) {
			t.Fatalf("LoadRun(invalid json) = %v, want ErrCorruptState", err)
		}
	})

	t.Run("bloqueo_exclusivo_multiproceso_y_cancel_cooperativo", func(t *testing.T) {
		projectRoot := t.TempDir()
		st, err := OpenRoot(projectRoot)
		if err != nil {
			t.Fatalf("OpenRoot: %v", err)
		}
		defer func() { _ = st.Close() }()

		dynSecret := "cancel-secret-" + "554433"
		st.SetSecretValues([]string{dynSecret})

		if _, err := st.AcquireExclusive("missing-run-002"); !errors.Is(err, ErrRunNotFound) {
			t.Fatalf("AcquireExclusive(missing) = %v, want ErrRunNotFound", err)
		}

		runID, _, err := st.CreateRun(sampleValidatedWorkflow())
		if err != nil {
			t.Fatalf("CreateRun: %v", err)
		}

		// In-process contention between two handles on the same run.
		lock1, err := st.AcquireExclusive(runID)
		if err != nil {
			t.Fatalf("AcquireExclusive first: %v", err)
		}
		if _, err := st.AcquireExclusive(runID); !errors.Is(err, ErrRunBusy) {
			t.Fatalf("AcquireExclusive second in-process = %v, want ErrRunBusy", err)
		}
		if err := lock1.Close(); err != nil {
			t.Fatalf("lock1.Close: %v", err)
		}
		// Closing twice is safe.
		if err := lock1.Close(); err != nil {
			t.Fatalf("lock1.Close idempotent: %v", err)
		}

		// Real cross-process lock contention using this test binary as helper process.
		exe, err := os.Executable()
		if err != nil {
			t.Fatalf("os.Executable: %v", err)
		}
		cmd := exec.Command(exe, "-test.run=^TestWorkflowStoreLockHelper$")
		cmd.Env = append(os.Environ(),
			"GANDALF_WORKFLOW_LOCK_HELPER=1",
			"GANDALF_HELPER_ROOT="+projectRoot,
			"GANDALF_HELPER_RUN="+runID,
		)
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatalf("start helper: %v", err)
		}
		helperAlive := true
		defer func() {
			if helperAlive && cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		}()

		lineCh := make(chan string, 1)
		errCh := make(chan error, 1)
		go func() {
			scanner := bufio.NewScanner(stdoutPipe)
			for scanner.Scan() {
				txt := strings.TrimSpace(scanner.Text())
				if txt == "LOCKED" {
					lineCh <- txt
					return
				}
			}
			if scanErr := scanner.Err(); scanErr != nil {
				errCh <- scanErr
			} else {
				errCh <- errors.New("helper exited before printing LOCKED")
			}
		}()

		select {
		case <-lineCh:
		case helperErr := <-errCh:
			t.Fatalf("helper failed to acquire lock: %v", helperErr)
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out waiting for lock helper")
		}

		// Parent must observe ErrRunBusy while child holds the OS file lock.
		if _, err := st.AcquireExclusive(runID); !errors.Is(err, ErrRunBusy) {
			t.Fatalf("AcquireExclusive while child holds lock = %v, want ErrRunBusy", err)
		}

		// Cooperative cancel.req must work WITHOUT acquiring RunLock while child holds it.
		hasCancel, _, err := st.HasCancelRequest(runID)
		if err != nil || hasCancel {
			t.Fatalf("HasCancelRequest before cancel: got (%v, %v), want (false, nil)", hasCancel, err)
		}
		if err := st.WriteCancelRequest(runID, "abortar con "+dynSecret); err != nil {
			t.Fatalf("WriteCancelRequest while locked: %v", err)
		}
		hasCancel, reason, err := st.HasCancelRequest(runID)
		if err != nil || !hasCancel {
			t.Fatalf("HasCancelRequest after cancel: got (%v, %q, %v)", hasCancel, reason, err)
		}
		if strings.Contains(reason, dynSecret) {
			t.Fatalf("cancel reason leaked secret: %q", reason)
		}

		// Kill helper abruptly and confirm kernel releases run.lock so parent can acquire it.
		if err := cmd.Process.Kill(); err != nil {
			t.Fatalf("kill helper: %v", err)
		}
		_ = cmd.Wait()
		helperAlive = false

		lockAfterKill, err := st.AcquireExclusive(runID)
		if err != nil {
			t.Fatalf("AcquireExclusive after helper death = %v, want nil", err)
		}
		if err := lockAfterKill.Close(); err != nil {
			t.Fatalf("lockAfterKill.Close: %v", err)
		}
	})
}
