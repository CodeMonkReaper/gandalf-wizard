package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/portability"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/engine"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/gates"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/store"
)

// referenceDefinition reads the shared case of phase 04 from
// internal/portability, so the acceptance test and the portability test start
// from the same committed revision.
func referenceDefinition(concurrency int) schema.WorkflowDefinition {
	return portability.Definition(concurrency)
}

func writeDefinition(t *testing.T, dir string, def schema.WorkflowDefinition) string {
	t.Helper()
	data, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "workflow.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeCase writes the shared case exactly as internal/portability defines it,
// so the acceptance test runs the definition the case fingerprints.
func writeCase(t *testing.T, dir string) string {
	t.Helper()
	data, err := portability.DefinitionJSON()
	if err != nil {
		t.Fatalf("definición canónica del caso: %v", err)
	}
	path := filepath.Join(dir, "workflow.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("escribir el caso: %v", err)
	}
	return path
}

// workflowCalls records the provider calls and the peak concurrency.
type workflowCalls struct {
	mu     sync.Mutex
	calls  []string
	byName map[string]int
	active int
	peak   int
}

func (c *workflowCalls) enter(name string) func() {
	c.mu.Lock()
	if c.byName == nil {
		c.byName = map[string]int{}
	}
	c.calls = append(c.calls, name)
	c.byName[name]++
	c.active++
	if c.active > c.peak {
		c.peak = c.active
	}
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		c.active--
		c.mu.Unlock()
	}
}

func (c *workflowCalls) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.calls)
}

// countPrefix counts the calls whose recorded prompt starts with the prefix: a
// correction retry appends the quality-check feedback to the original prompt.
func (c *workflowCalls) countPrefix(prefix string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for name, calls := range c.byName {
		if strings.HasPrefix(name, prefix) {
			count += calls
		}
	}
	return count
}

func (c *workflowCalls) countOf(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.byName[name]
}

func (c *workflowCalls) peakValue() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.peak
}

// referencePorts wires deterministic providers for the reference workflow. The
// memory port answers from the shared case of phase 04.
func referencePorts(t *testing.T, calls *workflowCalls, recommendation func(attempt int) string) (engine.MemoryPort, engine.RouterPort) {
	t.Helper()
	fixed, err := portability.NewFixedMemory(portability.FixedMemory())
	if err != nil {
		t.Fatalf("copia de memoria compartida invalida: %v", err)
	}
	memory := workflowMemoryFunc(func(ctx context.Context, projectID, callerID, query string) (engine.MemoryResult, error) {
		defer calls.enter("memory")()
		return fixed.Query(ctx, projectID, callerID, query)
	})
	var mu sync.Mutex
	attempts := 0
	routing := workflowRouterFunc(func(_ context.Context, req engine.RouterRequest) (engine.RouterResult, error) {
		defer calls.enter(req.Prompt)()
		if strings.Contains(req.Prompt, "riesgos tecnicos") {
			return engine.RouterResult{Text: "riesgo: dependencia externa"}, nil
		}
		mu.Lock()
		attempts++
		current := attempts
		mu.Unlock()
		if recommendation != nil {
			return engine.RouterResult{Text: recommendation(current)}, nil
		}
		return engine.RouterResult{Text: `{"decision":"continuar","riesgos":["dependencia externa"]}`}, nil
	})
	return memory, routing
}

// referenceRunner prepares a project with the reference definition and ports.
func referenceRunner(t *testing.T, calls *workflowCalls, recommendation func(int) string) (*captured, string) {
	t.Helper()
	c := newCaptured(t)
	c.withProjectFile(t, "demo")
	c.deps.WorkflowMemory, c.deps.WorkflowRouter = referencePorts(t, calls, recommendation)
	return c, writeCase(t, c.deps.WorkDir)
}

// runReference executes the reference workflow through the CLI.
func runReference(t *testing.T, c *captured, file string) map[string]any {
	t.Helper()
	if code := runCLI(t, c, "workflow", "run", "--json", file); code != ExitAwaitingApproval {
		t.Fatalf("run de referencia = %d: stderr=%s stdout=%s", code, c.stderr.String(), c.stdout.String())
	}
	return parseJSON(t, c.stdout.String())
}

// projectFiles lists the files of a project, ignoring the runtime state.
func projectFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == ".gandalf" || strings.HasPrefix(rel, ".gandalf/") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// scanForSentinel reports every file under .gandalf containing the sentinel.
func scanForSentinel(t *testing.T, root, sentinel string) []string {
	t.Helper()
	var hits []string
	err := filepath.WalkDir(filepath.Join(root, ".gandalf"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), sentinel) {
			hits = append(hits, path)
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return hits
}

// projectHashes maps every project file, runtime state excluded, to its digest.
func projectHashes(t *testing.T, root string) map[string]string {
	t.Helper()
	hashes := map[string]string{}
	for _, rel := range projectFiles(t, root) {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		hashes[rel] = hex.EncodeToString(sum[:])
	}
	return hashes
}

// TestWorkflowAcceptance proves the reference scenario end to end (AC-01–AC-10)
// through cli.Run with deterministic ports, and delegates the real-process
// evidence to the helpers in workflow_process_test.go.
func TestWorkflowAcceptance(t *testing.T) {
	t.Run("AC-01 preflight read-only describes the graph without provider calls", func(t *testing.T) {
		calls := &workflowCalls{}
		c, file := referenceRunner(t, calls, nil)
		if code := runCLI(t, c, "workflow", "validate", "--json", file); code != ExitOK {
			t.Fatalf("validate = %d: %s", code, c.stderr.String())
		}
		payload := parseJSON(t, c.stdout.String())
		objective, _ := payload["objective"].(map[string]any)
		if payload["status"] != "validated" || objective["id"] != "OBJ-PROP-001" || objective["version"] != "1.0.0" {
			t.Fatalf("preflight inesperado: %v", payload)
		}
		steps, _ := payload["steps"].([]any)
		if len(steps) != 5 {
			t.Fatalf("pasos del grafo = %d", len(steps))
		}
		if calls.count() != 0 {
			t.Error("el preflight llamó a los proveedores")
		}
		if _, err := os.Stat(filepath.Join(c.deps.WorkDir, ".gandalf")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("el preflight creó estado local: %v", err)
		}
	})

	t.Run("AC-02 independent branches run in parallel and join at the gate", func(t *testing.T) {
		c, file := referenceRunner(t, &workflowCalls{}, nil)
		arrived := make(chan struct{}, 2)
		gate := make(chan struct{})
		go func() {
			<-arrived
			<-arrived
			close(gate)
		}()
		wait := func(ctx context.Context, name string) error {
			arrived <- struct{}{}
			select {
			case <-gate:
				return nil
			case <-time.After(2 * time.Second):
				return fmt.Errorf("la rama %s no se ejecutó en paralelo", name)
			}
		}
		c.deps.WorkflowMemory = workflowMemoryFunc(func(ctx context.Context, _, _, _ string) (engine.MemoryResult, error) {
			if err := wait(ctx, "memoria"); err != nil {
				return engine.MemoryResult{}, err
			}
			return engine.MemoryResult{Text: "decision previa"}, nil
		})
		c.deps.WorkflowRouter = workflowRouterFunc(func(ctx context.Context, req engine.RouterRequest) (engine.RouterResult, error) {
			if strings.Contains(req.Prompt, "riesgos tecnicos") {
				if err := wait(ctx, "borrador"); err != nil {
					return engine.RouterResult{}, err
				}
				return engine.RouterResult{Text: "riesgo: dependencia externa"}, nil
			}
			return engine.RouterResult{Text: `{"decision":"continuar","riesgos":["x"]}`}, nil
		})
		runReference(t, c, file)
	})

	t.Run("AC-03 a blocking gate rejects the producer and skips dependents", func(t *testing.T) {
		def := referenceDefinition(portability.ReferenceConcurrency)
		for i := range def.Steps {
			if def.Steps[i].ID == "sintesis" {
				// Without a correction the producer passes and the gate rejects it.
				def.Steps[i].Correction = nil
			}
		}
		c := newCaptured(t)
		c.withProjectFile(t, "demo")
		c.deps.WorkflowMemory, c.deps.WorkflowRouter = referencePorts(t, &workflowCalls{}, func(int) string { return "sin json" })
		file := writeDefinition(t, c.deps.WorkDir, def)
		if code := runCLI(t, c, "workflow", "run", "--json", file); code != ExitFailed {
			t.Fatalf("gate bloqueante = %d: stderr=%s stdout=%s", code, c.stderr.String(), c.stdout.String())
		}
		payload := parseJSON(t, c.stdout.String())
		byID := stepsByID(t, payload)
		if byID["sintesis"]["status"] != "passed" {
			t.Errorf("el productor no llegó a pasar: %v", byID["sintesis"]["status"])
		}
		// The gate is evaluated with its producer, so its verdict lives in the
		// gate evidence and the gate step itself never gets launched.
		var verdict map[string]any
		for _, raw := range payload["gates"].([]any) {
			if gate, _ := raw.(map[string]any); gate["step_id"] == "gate_sintesis" {
				verdict = gate
			}
		}
		if verdict["status"] != "fail" {
			t.Fatalf("el gate no rechazó la salida: %v", verdict)
		}
		if violations, _ := verdict["violations"].([]any); len(violations) == 0 {
			t.Error("el rechazo no dejó motivo")
		}
		if byID["aprobacion_revision"]["status"] != "skipped" {
			t.Errorf("el dependiente del gate no se omitió: %v", byID["aprobacion_revision"]["status"])
		}
		// The rejected evidence survives the failure: the producer artifact is
		// still on disk and readable.
		artifacts, _ := byID["sintesis"]["artifacts"].([]any)
		if len(artifacts) != 1 {
			t.Fatalf("artefactos del productor = %d; want 1", len(artifacts))
		}
		artifact, _ := artifacts[0].(map[string]any)
		rel, _ := artifact["rel_path"].(string)
		body, err := os.ReadFile(filepath.Join(c.deps.WorkDir, filepath.FromSlash(rel)))
		if err != nil || !strings.Contains(string(body), "sin json") {
			t.Errorf("la evidencia rechazada no se conservó en %q: %v", rel, err)
		}
	})

	t.Run("AC-03 an informative gate warns without blocking", func(t *testing.T) {
		def := referenceDefinition(portability.ReferenceConcurrency)
		for i := range def.Steps {
			switch def.Steps[i].ID {
			case "sintesis":
				// An informative gate cannot be the correction target.
				def.Steps[i].Correction = nil
			case "gate_sintesis":
				def.Steps[i].Gate.Informative = true
				def.Steps[i].Gate.Rules = []schema.GateRuleSpec{
					{Kind: schema.RuleNonEmpty}, {Kind: schema.RuleMinLength, Limit: 1000},
				}
			}
		}
		c := newCaptured(t)
		c.withProjectFile(t, "demo")
		c.deps.WorkflowMemory, c.deps.WorkflowRouter = referencePorts(t, &workflowCalls{}, nil)
		file := writeDefinition(t, c.deps.WorkDir, def)
		if code := runCLI(t, c, "workflow", "run", "--json", file); code != ExitAwaitingApproval {
			t.Fatalf("gate informativo = %d: stderr=%s stdout=%s", code, c.stderr.String(), c.stdout.String())
		}
		payload := parseJSON(t, c.stdout.String())
		byID := stepsByID(t, payload)
		if byID["gate_sintesis"]["status"] != "passed" {
			t.Errorf("el gate informativo debía seguir: %v", byID["gate_sintesis"]["status"])
		}
		warned := false
		for _, raw := range payload["gates"].([]any) {
			gate, _ := raw.(map[string]any)
			if gate["step_id"] != "gate_sintesis" {
				continue
			}
			warnings, _ := gate["warnings"].([]any)
			if len(warnings) != 0 {
				warned = true
			}
		}
		if !warned {
			t.Error("el aviso del gate informativo no quedó registrado")
		}
	})

	t.Run("AC-04 correction cycles are bounded and recorded", func(t *testing.T) {
		calls := &workflowCalls{}
		c, file := runnerWithRecommendation(t, calls, func(attempt int) string {
			if attempt == 1 {
				return "sin json"
			}
			return `{"decision":"continuar","riesgos":["dependencia externa"]}`
		})
		payload := runReference(t, c, file)
		step := stepsByID(t, payload)["sintesis"]
		attempts, _ := step["attempts"].([]any)
		if step["iteration"] != float64(2) || len(attempts) != 2 {
			t.Errorf("iteración/intentos = %v/%d; want 2/2", step["iteration"], len(attempts))
		}
		first, _ := attempts[0].(map[string]any)
		if first["gate_status"] != "fail" || first["diagnosis"] == "" {
			t.Errorf("el primer intento no dejó diagnóstico: %v", first)
		}
	})

	t.Run("AC-04 an exhausted correction cycle stops at max_iterations", func(t *testing.T) {
		calls := &workflowCalls{}
		c, file := runnerWithRecommendation(t, calls, func(int) string { return "sin json" })
		if code := runCLI(t, c, "workflow", "run", "--json", file); code != ExitFailed {
			t.Fatalf("ciclo agotado = %d: %s", code, c.stderr.String())
		}
		step := stepsByID(t, parseJSON(t, c.stdout.String()))["sintesis"]
		if attempts, _ := step["attempts"].([]any); len(attempts) != 2 {
			t.Errorf("intentos = %d; want 2 (max_iterations)", len(attempts))
		}
		if got := calls.countPrefix("Sintetiza en JSON con decision y riesgos"); got != 2 {
			t.Errorf("llamadas al productor = %d; want 2", got)
		}
	})

	t.Run("AC-05 the human gate stops every branch until it is decided", func(t *testing.T) {
		def := referenceDefinition(portability.ReferenceConcurrency)
		def.Steps = append(def.Steps, schema.StepSpec{
			ID: "post", Kind: schema.StepKindRouterTask, DependsOn: []string{"aprobacion_revision"},
			Outputs: []string{"texto"}, Router: &schema.RouterStepSpec{Prompt: "cierre posterior"},
		})
		calls := &workflowCalls{}
		c := newCaptured(t)
		c.withProjectFile(t, "demo")
		c.deps.WorkflowMemory, c.deps.WorkflowRouter = referencePorts(t, calls, nil)
		file := writeDefinition(t, c.deps.WorkDir, def)
		if code := runCLI(t, c, "workflow", "run", "--json", file); code != ExitAwaitingApproval {
			t.Fatalf("run con puerta = %d: %s", code, c.stderr.String())
		}
		payload := parseJSON(t, c.stdout.String())
		runID := workflowRunID(t, payload)
		checkpoint, _ := payload["checkpoint_digest"].(string)
		before := calls.count()
		if code := runCLI(t, c, "workflow", "status", "--json", runID); code != ExitOK {
			t.Fatalf("status = %d", code)
		}
		snapshot := parseJSON(t, c.stdout.String())
		if calls.count() != before {
			t.Error("la parada 4A lanzó pasos nuevos")
		}
		pending, _ := snapshot["pending_gates"].([]any)
		if snapshot["checkpoint_digest"] != checkpoint || len(pending) != 1 || pending[0] != "aprobacion_revision" {
			t.Errorf("status no expone checkpoint y puerta: %v", snapshot)
		}
		if code := runCLI(t, c, "workflow", "approve", "--json", runID, "--gate", "aprobacion_revision",
			"--checkpoint", checkpoint, "--decision", "approve"); code != ExitOK {
			t.Fatalf("approve = %d: %s", code, c.stderr.String())
		}
		if calls.count() != before {
			t.Error("approve lanzó pasos por sí solo")
		}
		if code := runCLI(t, c, "workflow", "resume", "--json", runID); code != ExitOK {
			t.Fatalf("resume = %d: %s", code, c.stderr.String())
		}
		if calls.countOf("cierre posterior") != 1 {
			t.Error("el paso posterior a la puerta no se ejecutó tras aprobar")
		}
	})

	t.Run("AC-05 the reference scenario closes succeeded with its full trace", func(t *testing.T) {
		calls := &workflowCalls{}
		c, file := referenceRunner(t, calls, nil)
		payload := runReference(t, c, file)
		runID := workflowRunID(t, payload)
		checkpoint, _ := payload["checkpoint_digest"].(string)
		if code := runCLI(t, c, "workflow", "approve", "--json", runID, "--gate", "aprobacion_revision",
			"--checkpoint", checkpoint, "--decision", "approve"); code != ExitOK {
			t.Fatalf("approve = %d: %s", code, c.stderr.String())
		}
		if code := runCLI(t, c, "workflow", "resume", "--json", runID); code != ExitOK {
			t.Fatalf("resume = %d: %s", code, c.stderr.String())
		}
		closed := parseJSON(t, c.stdout.String())
		if closed["status"] != "succeeded" || closed["run_status"] != "succeeded" {
			t.Fatalf("cierre inesperado: %v", closed)
		}
		approvals, _ := closed["approvals"].([]any)
		if len(approvals) != 1 {
			t.Fatalf("aprobaciones = %d", len(approvals))
		}
		if approval, _ := approvals[0].(map[string]any); approval["decision"] != "approve" || approval["gate_id"] != "aprobacion_revision" {
			t.Errorf("aprobación inesperada: %v", approval)
		}
		gateSeen := false
		for _, raw := range closed["gates"].([]any) {
			gate, _ := raw.(map[string]any)
			if gate["step_id"] == "gate_sintesis" && gate["status"] == "pass" {
				gateSeen = true
			}
		}
		if !gateSeen {
			t.Error("el gate no aparece aprobado en el cierre")
		}
		for id, step := range stepsByID(t, closed) {
			if step["status"] != "passed" {
				continue
			}
			if step["kind"] == string(schema.StepKindQualityGate) || step["kind"] == string(schema.StepKindHumanApproval) {
				// Gates and approvals decide without producing artifacts.
				continue
			}
			artifacts, _ := step["artifacts"].([]any)
			if len(artifacts) == 0 {
				t.Errorf("el paso %s cerró sin artefactos", id)
				continue
			}
			artifact, _ := artifacts[0].(map[string]any)
			if location, _ := artifact["rel_path"].(string); !strings.HasPrefix(location, ".gandalf/workflows/runs/"+runID+"/artifacts/") {
				t.Errorf("artefacto de %s fuera del espacio de corridas: %q", id, location)
			}
		}
		verdict, _ := closed["objective_verdict"].(map[string]any)
		if verdict["status"] != "succeeded" {
			t.Errorf("veredicto inesperado: %v", verdict)
		}
	})

	t.Run("AC-05 a rejection blocks the run and its resume", func(t *testing.T) {
		c, file := referenceRunner(t, &workflowCalls{}, nil)
		payload := runReference(t, c, file)
		runID := workflowRunID(t, payload)
		checkpoint, _ := payload["checkpoint_digest"].(string)
		if code := runCLI(t, c, "workflow", "approve", "--json", runID, "--gate", "aprobacion_revision",
			"--checkpoint", checkpoint, "--decision", "reject", "--reason", "sin evidencia"); code != ExitOK {
			t.Fatalf("reject = %d: %s", code, c.stderr.String())
		}
		if code := runCLI(t, c, "workflow", "status", "--json", runID); code != ExitOK {
			t.Fatalf("status = %d", code)
		}
		if snapshot := parseJSON(t, c.stdout.String()); snapshot["run_status"] != "blocked" {
			t.Errorf("estado tras rechazo = %v", snapshot["run_status"])
		}
		if code := runCLI(t, c, "workflow", "resume", "--json", runID); code != ExitConflict {
			t.Errorf("resume de una corrida bloqueada = %d", code)
		}
	})

	t.Run("AC-06 approvals require the current checkpoint and the unchanged definition", func(t *testing.T) {
		c, file := referenceRunner(t, &workflowCalls{}, nil)
		payload := runReference(t, c, file)
		runID := workflowRunID(t, payload)
		checkpoint, _ := payload["checkpoint_digest"].(string)
		foreign := strings.Repeat("a", 64)
		if foreign == checkpoint {
			t.Fatal("la huella ajena coincide con la vigente")
		}
		if code := runCLI(t, c, "workflow", "approve", "--json", runID, "--gate", "aprobacion_revision",
			"--checkpoint", foreign, "--decision", "approve"); code != ExitConflict {
			t.Errorf("huella ajena = %d; want %d", code, ExitConflict)
		}
		if code := runCLI(t, c, "workflow", "approve", "--json", runID, "--gate", "aprobacion_revision",
			"--checkpoint", checkpoint, "--decision", "approve"); code != ExitOK {
			t.Fatalf("approve = %d: %s", code, c.stderr.String())
		}
		altered := referenceDefinition(portability.ReferenceConcurrency)
		altered.Objective.Statement = "Definición alterada"
		writeDefinition(t, c.deps.WorkDir, altered)
		if code := runCLI(t, c, "workflow", "resume", "--json", runID); code != ExitConflict {
			t.Errorf("resume con definición alterada = %d", code)
		}
	})

	t.Run("AC-07 resume continues a paused run without repeating passed steps", func(t *testing.T) {
		calls := &workflowCalls{}
		c, file := referenceRunner(t, calls, nil)
		ctx, interrupt := context.WithCancel(context.Background())
		workDir := c.deps.WorkDir
		c.deps.WorkflowRouter = workflowRouterFunc(func(ctx context.Context, req engine.RouterRequest) (engine.RouterResult, error) {
			if !strings.Contains(req.Prompt, "riesgos tecnicos") {
				return engine.RouterResult{Text: `{"decision":"continuar","riesgos":["x"]}`}, nil
			}
			waitForStepStatus(workDir, "contexto_romero", "passed", 5*time.Second)
			interrupt()
			return engine.RouterResult{}, ctx.Err()
		})
		if code := Run(ctx, []string{"workflow", "run", "--json", file}, c.deps); code != ExitPaused {
			t.Fatalf("run interrumpido = %d: %s", code, c.stderr.String())
		}
		payload := parseJSON(t, c.stdout.String())
		runID := workflowRunID(t, payload)
		if step := stepsByID(t, payload)["contexto_romero"]; step["status"] != "passed" {
			t.Fatalf("el paso completado no quedó como passed: %v", step["status"])
		}
		memoryCalls := calls.countOf("memory")
		if memoryCalls == 0 {
			t.Fatal("la memoria no llegó a ejecutarse")
		}
		c.deps.WorkflowMemory, c.deps.WorkflowRouter = referencePorts(t, calls, nil)
		if code := runCLI(t, c, "workflow", "resume", "--json", runID); code != ExitAwaitingApproval {
			t.Fatalf("resume = %d: %s", code, c.stderr.String())
		}
		if got := calls.countOf("memory"); got != memoryCalls {
			t.Errorf("resume repitió el paso concluido: %d llamadas a memoria (antes %d)", got, memoryCalls)
		}
		if got := calls.countPrefix("Sintetiza en JSON con decision y riesgos"); got != 1 {
			t.Errorf("llamadas a la síntesis = %d; want 1", got)
		}
		if code := runCLI(t, c, "workflow", "resume", "--json", runID); code != ExitConflict {
			t.Errorf("resume en puerta = %d; want %d", code, ExitConflict)
		}
	})

	t.Run("AC-08 another handle holding the run lock yields a conflict", func(t *testing.T) {
		c, file := referenceRunner(t, &workflowCalls{}, nil)
		ctx, interrupt := context.WithCancel(context.Background())
		c.deps.WorkflowMemory = workflowMemoryFunc(func(ctx context.Context, _, _, _ string) (engine.MemoryResult, error) {
			interrupt()
			return engine.MemoryResult{}, ctx.Err()
		})
		if code := Run(ctx, []string{"workflow", "run", "--json", file}, c.deps); code != ExitPaused {
			t.Fatalf("run interrumpido = %d", code)
		}
		runID := workflowRunID(t, parseJSON(t, c.stdout.String()))
		holder, err := store.OpenRoot(c.deps.WorkDir)
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Close()
		lock, err := holder.AcquireExclusive(runID)
		if err != nil {
			t.Fatalf("no se pudo simular el coordinador vivo: %v", err)
		}
		c.deps.WorkflowMemory, c.deps.WorkflowRouter = referencePorts(t, &workflowCalls{}, nil)
		code := runCLI(t, c, "workflow", "resume", "--json", runID)
		lock.Close()
		if code != ExitConflict {
			t.Errorf("resume con lock ocupado = %d; want %d", code, ExitConflict)
		}
	})

	t.Run("AC-09 optional branch failure closes partial with the full trace", func(t *testing.T) {
		def := referenceDefinition(portability.ReferenceConcurrency)
		def.Steps = append(def.Steps,
			schema.StepSpec{ID: "extra", Kind: schema.StepKindRouterTask, Optional: true, Outputs: []string{"texto"},
				Router: &schema.RouterStepSpec{Prompt: "informe extra"}})
		c := newCaptured(t)
		c.withProjectFile(t, "demo")
		c.deps.WorkflowMemory, c.deps.WorkflowRouter = referencePorts(t, &workflowCalls{}, nil)
		c.deps.WorkflowRouter = workflowRouterFunc(func(_ context.Context, req engine.RouterRequest) (engine.RouterResult, error) {
			switch {
			case strings.Contains(req.Prompt, "informe extra"):
				return engine.RouterResult{}, errors.New("rama opcional caída")
			case strings.Contains(req.Prompt, "riesgos tecnicos"):
				return engine.RouterResult{Text: "riesgo: dependencia externa"}, nil
			default:
				return engine.RouterResult{Text: `{"decision":"continuar","riesgos":["x"]}`}, nil
			}
		})
		file := writeDefinition(t, c.deps.WorkDir, def)
		if code := runCLI(t, c, "workflow", "run", "--json", file); code != ExitAwaitingApproval {
			t.Fatalf("run con rama opcional = %d: stderr=%s stdout=%s", code, c.stderr.String(), c.stdout.String())
		}
		payload := parseJSON(t, c.stdout.String())
		runID := workflowRunID(t, payload)
		checkpoint, _ := payload["checkpoint_digest"].(string)
		if code := runCLI(t, c, "workflow", "approve", "--json", runID, "--gate", "aprobacion_revision",
			"--checkpoint", checkpoint, "--decision", "approve"); code != ExitOK {
			t.Fatalf("approve = %d: %s", code, c.stderr.String())
		}
		if code := runCLI(t, c, "workflow", "resume", "--json", runID); code != ExitPartial {
			t.Fatalf("cierre parcial = %d: stderr=%s stdout=%s", code, c.stderr.String(), c.stdout.String())
		}
		closed := parseJSON(t, c.stdout.String())
		if closed["status"] != "partial" {
			t.Errorf("estado inesperado: %v", closed["status"])
		}
		verdict, _ := closed["objective_verdict"].(map[string]any)
		if met, _ := verdict["met_criteria"].([]any); verdict["status"] != "partial" || len(met) != 2 {
			t.Errorf("veredicto inesperado: %v", verdict)
		}
		if approvals, _ := closed["approvals"].([]any); len(approvals) != 1 {
			t.Errorf("la traza de aprobación falta: %v", closed["approvals"])
		}
		if gate, _ := stepsByID(t, closed)["gate_sintesis"]; gate["status"] != "passed" {
			t.Errorf("gate de cierre inesperado: %v", gate["status"])
		}
	})

	t.Run("AC-09 a hard policy blocks the run", func(t *testing.T) {
		blocked := newCaptured(t)
		blocked.withProjectFile(t, "demo")
		sentinel := "acceptance-hidden-credential"
		blocked.deps.Getenv = func(key string) string {
			if key == config.EnvKeyRomeroAPIKey {
				return sentinel
			}
			return ""
		}
		blocked.deps.WorkflowRouter = workflowRouterFunc(func(context.Context, engine.RouterRequest) (engine.RouterResult, error) {
			return engine.RouterResult{Text: "riesgo " + sentinel}, nil
		})
		policyDef := workflowProposal(
			workflowRouterStep("proposal"),
			workflowGateStep("privacy", "proposal.text", schema.GateRuleSpec{Kind: schema.RuleNoSecrets}),
		)
		policyFile := writeDefinition(t, blocked.deps.WorkDir, policyDef)
		if code := runCLI(t, blocked, "workflow", "run", "--json", policyFile); code != ExitBlocked {
			t.Fatalf("política dura = %d: stderr=%s stdout=%s", code, blocked.stderr.String(), blocked.stdout.String())
		}
		if payload := parseJSON(t, blocked.stdout.String()); payload["status"] != "blocked" {
			t.Errorf("estado inesperado: %v", payload["status"])
		}
	})

	t.Run("AC-10 the sentinel never reaches output or disk and the project stays intact", func(t *testing.T) {
		sentinel := "acceptance-secret-sentinel"
		calls := &workflowCalls{}
		c, file := referenceRunner(t, calls, nil)
		c.deps.Getenv = func(key string) string {
			if key == config.EnvKeyRomeroAPIKey {
				return sentinel
			}
			return ""
		}
		c.deps.WorkflowMemory = workflowMemoryFunc(func(context.Context, string, string, string) (engine.MemoryResult, error) {
			return engine.MemoryResult{Text: "contexto " + sentinel}, nil
		})
		before := projectHashes(t, c.deps.WorkDir)
		payload := runReference(t, c, file)
		runID := workflowRunID(t, payload)
		if strings.Contains(c.stdout.String(), sentinel) || strings.Contains(c.stderr.String(), sentinel) {
			t.Error("la salida del comando expuso el centinela")
		}
		if code := runCLI(t, c, "workflow", "status", "--json", runID); code != ExitOK {
			t.Fatalf("status = %d", code)
		}
		if strings.Contains(c.stdout.String(), sentinel) {
			t.Error("el estado expuso el centinela")
		}
		if hits := scanForSentinel(t, c.deps.WorkDir, sentinel); len(hits) != 0 {
			t.Errorf("el centinela quedó en disco: %v", hits)
		}
		if !strings.Contains(readRunState(t, c.deps.WorkDir, runID), gates.RedactedPlaceholder) {
			t.Error("el checkpoint no marca la redacción")
		}
		if after := projectHashes(t, c.deps.WorkDir); !reflect.DeepEqual(before, after) {
			t.Errorf("el contenido del proyecto cambió: %v -> %v", before, after)
		}
	})

	t.Run("process interruption, resume, contention and cooperative cancel", func(t *testing.T) {
		testWorkflowProcessInterrupt(t)
		testWorkflowProcessContention(t)
		testWorkflowProcessCancel(t)
	})
}

// stepsByID indexes the projected steps of a workflow payload.
func stepsByID(t *testing.T, payload map[string]any) map[string]map[string]any {
	t.Helper()
	byID := map[string]map[string]any{}
	steps, _ := payload["steps"].([]any)
	for _, raw := range steps {
		step, _ := raw.(map[string]any)
		id, _ := step["id"].(string)
		byID[id] = step
	}
	return byID
}

// waitForStepStatus polls the only run of the project until the step reaches
// the expected status.
func waitForStepStatus(workDir, stepID, want string, limit time.Duration) bool {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(filepath.Join(workDir, ".gandalf", "workflows", "runs"))
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		for _, entry := range entries {
			data, err := os.ReadFile(filepath.Join(workDir, ".gandalf", "workflows", "runs", entry.Name(), "state.json"))
			if err != nil {
				continue
			}
			var state struct {
				Steps map[string]struct {
					Status string `json:"status"`
				} `json:"steps"`
			}
			if json.Unmarshal(data, &state) != nil {
				continue
			}
			if state.Steps[stepID].Status == want {
				return true
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// runnerWithRecommendation prepares the reference workflow with a fixed answer.
func runnerWithRecommendation(t *testing.T, calls *workflowCalls, recommendation func(int) string) (*captured, string) {
	t.Helper()
	c, file := referenceRunner(t, calls, recommendation)
	return c, file
}

// readRunState returns the raw checkpoint of a run.
func readRunState(t *testing.T, workDir, runID string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(workDir, ".gandalf", "workflows", "runs", runID, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
