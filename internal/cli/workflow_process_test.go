package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
)

var (
	binaryOnce sync.Once
	binaryPath string
	binaryErr  error
)

// gandalfBinary builds the real CLI once per test run.
func gandalfBinary(t *testing.T) string {
	t.Helper()
	binaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "gandalf-acceptance")
		if err != nil {
			binaryErr = err
			return
		}
		name := "gandalf"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		binaryPath = filepath.Join(dir, name)
		build := exec.Command("go", "build", "-o", binaryPath, "./cmd/gandalf")
		build.Dir = filepath.Join("..", "..")
		if out, err := build.CombinedOutput(); err != nil {
			binaryErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if binaryErr != nil {
		t.Fatalf("no se pudo compilar el binario: %v", binaryErr)
	}
	return binaryPath
}

// fakeRomero serves the public ROMERO contract and can hold the first search.
type fakeRomero struct {
	server   *httptest.Server
	sentinel string
	started  chan struct{}
	release  chan struct{}
	blocking atomic.Bool
	once     sync.Once
}

func newFakeRomero(t *testing.T, sentinel string) *fakeRomero {
	t.Helper()
	f := &fakeRomero{sentinel: sentinel, started: make(chan struct{}, 16), release: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/memories/search", func(w http.ResponseWriter, r *http.Request) {
		select {
		case f.started <- struct{}{}:
		default:
		}
		if f.blocking.Load() {
			select {
			case <-f.release:
			case <-r.Context().Done():
				return
			}
		}
		// The first step keeps the deliverable clean; the second one carries the
		// sentinel so the sweep proves redaction without tripping the objective.
		var body struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		summary := "contexto de la propuesta"
		if strings.Contains(body.Query, "segunda") {
			summary = "contexto " + f.sentinel
		}
		writeJSONResponse(w, map[string]any{
			"candidates": []map[string]string{{
				"id": "m1", "summary": summary, "status": "active", "epistemic_status": "observed",
			}},
			"total_found":   1,
			"degraded_mode": true,
		})
	})
	mux.HandleFunc("/api/v1/memories/", func(w http.ResponseWriter, r *http.Request) {
		writeJSONResponse(w, map[string]any{
			"memory": map[string]any{
				"ID": "m1", "ProjectID": "demo", "Status": "active",
				"EpistemicStatus": "observed", "PrimarySourceID": "",
			},
		})
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func writeJSONResponse(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

// unblock releases every blocked search exactly once.
func (f *fakeRomero) unblock() {
	f.once.Do(func() { close(f.release) })
}

// waitForSearch blocks until the CLI asked ROMERO at least once.
func (f *fakeRomero) waitForSearch(t *testing.T) {
	t.Helper()
	select {
	case <-f.started:
	case <-time.After(10 * time.Second):
		t.Fatal("el proceso no consultó ROMERO")
	}
}

// gandalfProcess runs the real binary in its own process group.
type gandalfProcess struct {
	cmd    *exec.Cmd
	stdout *bytes.Buffer
	stderr *bytes.Buffer
	done   chan error
}

func startGandalf(t *testing.T, dir, romeroURL, sentinel string, args ...string) *gandalfProcess {
	t.Helper()
	p := &gandalfProcess{stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}, done: make(chan error, 1)}
	p.cmd = exec.Command(gandalfBinary(t), args...)
	p.cmd.Dir = dir
	p.cmd.Env = append(os.Environ(),
		"GANDALF_PROJECT=demo",
		"GANDALF_ROMERO_BASE_URL="+romeroURL,
		"ROMERO_API_KEY="+sentinel,
	)
	p.cmd.Stdout, p.cmd.Stderr = p.stdout, p.stderr
	configureProcessGroup(p.cmd)
	if err := p.cmd.Start(); err != nil {
		t.Fatalf("no se pudo arrancar el CLI: %v", err)
	}
	go func() { p.done <- p.cmd.Wait() }()
	return p
}

// wait returns the exit code of the process.
func (p *gandalfProcess) wait(t *testing.T) int {
	t.Helper()
	select {
	case err := <-p.done:
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		if err != nil {
			t.Fatalf("proceso: %v", err)
		}
		return 0
	case <-time.After(30 * time.Second):
		_ = p.cmd.Process.Kill()
		t.Fatal("el proceso no terminó")
		return -1
	}
}

// processDefinition is a memory-only workflow the real binary can run offline.
func processDefinition() schema.WorkflowDefinition {
	return schema.WorkflowDefinition{
		SchemaVersion: "1", ID: "proceso-aceptacion", Version: "1.0.0", ConcurrencyLimit: 1,
		Objective: schema.ObjectiveSpec{
			ID: "OBJ-PROC-001", Version: "1.0.0", Statement: "Correr dos pasos de memoria",
			Deliverables: []schema.DeliverableRef{{Name: "contexto", Source: "primero.texto"}},
			SuccessCriteria: []schema.CriterionSpec{
				{ID: "C1", Deliverable: "contexto", Rules: []schema.GateRuleSpec{{Kind: schema.RuleNonEmpty}}},
			},
		},
		Steps: []schema.StepSpec{
			{ID: "primero", Kind: schema.StepKindMemoryQuery, Outputs: []string{"texto"},
				Memory: &schema.MemoryStepSpec{Query: "primera"}},
			{ID: "segundo", Kind: schema.StepKindMemoryQuery, DependsOn: []string{"primero"}, Outputs: []string{"texto"},
				Memory: &schema.MemoryStepSpec{Query: "segunda"}},
		},
	}
}

// processProject prepares a project directory with the memory-only definition.
func processProject(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "proceso.json")
	data, err := json.MarshalIndent(processDefinition(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir, file
}

// parseProcessJSON decodes the CLI payload of a subprocess.
func parseProcessJSON(t *testing.T, out string) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("salida no json: %v\n%s", err, out)
	}
	return payload
}

// testWorkflowProcessInterrupt proves a real console interrupt pauses the run
// (12), resume finishes it (0) and the secret never lands on disk.
func testWorkflowProcessInterrupt(t *testing.T) {
	t.Helper()
	sentinel := "process-secret-sentinel"
	romero := newFakeRomero(t, sentinel)
	romero.blocking.Store(true)
	dir, file := processProject(t)
	before := projectHashes(t, dir)

	run := startGandalf(t, dir, romero.server.URL, sentinel, "workflow", "run", "--json", file)
	romero.waitForSearch(t)
	if err := sendInterrupt(run.cmd); err != nil {
		t.Fatalf("interrupción: %v", err)
	}
	if code := run.wait(t); code != ExitPaused {
		t.Fatalf("run interrumpido = %d\nstdout: %s\nstderr: %s", code, run.stdout, run.stderr)
	}
	payload := parseProcessJSON(t, run.stdout.String())
	runID, _ := payload["run_id"].(string)
	if runID == "" || payload["status"] != "paused" {
		t.Fatalf("payload interrumpido inesperado: %v", payload)
	}

	romero.unblock()
	resume := startGandalf(t, dir, romero.server.URL, sentinel, "workflow", "resume", "--json", runID)
	if code := resume.wait(t); code != ExitOK {
		t.Fatalf("resume = %d\nstdout: %s\nstderr: %s", code, resume.stdout, resume.stderr)
	}
	resumed := parseProcessJSON(t, resume.stdout.String())
	if resumed["status"] != "succeeded" {
		t.Errorf("estado tras resume: %v", resumed["status"])
	}
	steps := resumed["steps"].([]any)
	firstStep := steps[0].(map[string]any)
	artifacts := firstStep["artifacts"].([]any)
	relPath := artifacts[0].(map[string]any)["rel_path"].(string)
	content, err := os.ReadFile(filepath.Join(dir, relPath))
	if err != nil {
		t.Fatal(err)
	}
	evidence := parseProcessJSON(t, string(content))
	entries, _ := evidence["entries"].([]any)
	if evidence["status"] != "ok" || evidence["degraded"] != true ||
		evidence["degradation_notice"] != "Búsqueda en modo degradado" || len(entries) != 1 ||
		entries[0].(map[string]any)["memory_ref"] != "m1" {
		t.Errorf("evidencia HTTP de ROMERO incompleta: %v", evidence)
	}
	for name, out := range map[string]string{"run.stdout": run.stdout.String(), "run.stderr": run.stderr.String(),
		"resume.stdout": resume.stdout.String(), "resume.stderr": resume.stderr.String()} {
		if strings.Contains(out, sentinel) {
			t.Errorf("%s expuso el centinela", name)
		}
	}
	if hits := scanForSentinel(t, dir, sentinel); len(hits) != 0 {
		t.Errorf("el centinela quedó en disco: %v", hits)
	}
	if after := projectHashes(t, dir); !reflect.DeepEqual(before, after) {
		t.Errorf("el contenido del proyecto cambió: %v -> %v", before, after)
	}
}

// testWorkflowProcessContention proves two live processes never share a run.
func testWorkflowProcessContention(t *testing.T) {
	t.Helper()
	romero := newFakeRomero(t, "contencion")
	romero.blocking.Store(true)
	dir, file := processProject(t)

	run := startGandalf(t, dir, romero.server.URL, "contencion", "workflow", "run", "--json", file)
	romero.waitForSearch(t)
	runID := onlyRunID(t, dir)
	if runID == "" {
		t.Fatal("sin run_id en el proyecto")
	}
	contender := startGandalf(t, dir, romero.server.URL, "contencion", "workflow", "resume", "--json", runID)
	if code := contender.wait(t); code != ExitConflict {
		t.Fatalf("competidor = %d\nstdout: %s\nstderr: %s", code, contender.stdout, contender.stderr)
	}
	romero.unblock()
	if code := run.wait(t); code != ExitOK {
		t.Fatalf("run = %d\nstdout: %s\nstderr: %s", code, run.stdout, run.stderr)
	}
}

// testWorkflowProcessCancel proves a cooperative cancel from another process
// closes the run as cancelled (10) and reports the request.
func testWorkflowProcessCancel(t *testing.T) {
	t.Helper()
	romero := newFakeRomero(t, "cancelacion")
	romero.blocking.Store(true)
	dir, file := processProject(t)

	run := startGandalf(t, dir, romero.server.URL, "cancelacion", "workflow", "run", "--json", file)
	romero.waitForSearch(t)
	runID := onlyRunID(t, dir)
	if runID == "" {
		t.Fatal("sin run_id para cancelar")
	}
	cancel := startGandalf(t, dir, romero.server.URL, "cancelacion", "workflow", "cancel", "--json", runID)
	waitForCancelRequest(t, dir, runID)
	romero.unblock()
	if code := cancel.wait(t); code != ExitCanceled {
		t.Fatalf("cancel = %d\nstdout: %s\nstderr: %s", code, cancel.stdout, cancel.stderr)
	}
	payload := parseProcessJSON(t, cancel.stdout.String())
	if payload["requested"] != true || payload["run_status"] != "cancelled" {
		t.Errorf("cancelación inesperada: %v", payload)
	}
	if code := run.wait(t); code != ExitCanceled {
		t.Errorf("la corrida no cerró cancelada: %d", code)
	}
}

// waitForCancelRequest blocks until the cancelling process leaves its request,
// so the run reacts to the signal instead of to a fixed delay.
func waitForCancelRequest(t *testing.T, dir, runID string) {
	t.Helper()
	request := filepath.Join(dir, ".gandalf", "workflows", "runs", runID, "cancel.req")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(request); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("la cancelación no dejó %s", request)
}

// onlyRunID returns the identifier of the only run in the project.
func onlyRunID(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, ".gandalf", "workflows", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("corridas encontradas = %d", len(entries))
	}
	return entries[0].Name()
}

// TestMain removes the temporary installation of the CLI built for the tests.
func TestMain(m *testing.M) {
	code := m.Run()
	if binaryPath != "" {
		_ = os.RemoveAll(filepath.Dir(binaryPath))
	}
	os.Exit(code)
}
