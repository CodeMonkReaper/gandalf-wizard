package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/engine"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/gates"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/store"
)

type workflowMemoryFunc func(context.Context, string, string, string) (engine.MemoryResult, error)

func (f workflowMemoryFunc) Query(ctx context.Context, project, caller, query string) (engine.MemoryResult, error) {
	return f(ctx, project, caller, query)
}

type workflowRouterFunc func(context.Context, engine.RouterRequest) (engine.RouterResult, error)

func (f workflowRouterFunc) Execute(ctx context.Context, req engine.RouterRequest) (engine.RouterResult, error) {
	return f(ctx, req)
}

func workflowRouterStep(id string, dependencies ...string) schema.StepSpec {
	return schema.StepSpec{
		ID: id, Kind: schema.StepKindRouterTask, DependsOn: dependencies,
		Outputs: []string{"text"}, Router: &schema.RouterStepSpec{Prompt: id},
	}
}

func workflowMemoryStep(id, query string, dependencies ...string) schema.StepSpec {
	return schema.StepSpec{
		ID: id, Kind: schema.StepKindMemoryQuery, DependsOn: dependencies,
		Outputs: []string{"text"}, Memory: &schema.MemoryStepSpec{Query: query},
	}
}

func workflowApprovalStep(id string, dependencies ...string) schema.StepSpec {
	return schema.StepSpec{
		ID: id, Kind: schema.StepKindHumanApproval, DependsOn: dependencies,
		Approval: &schema.ApprovalStepSpec{Prompt: "Review the proposal"},
	}
}

func workflowGateStep(id, source string, rules ...schema.GateRuleSpec) schema.StepSpec {
	return schema.StepSpec{
		ID: id, Kind: schema.StepKindQualityGate, DependsOn: []string{"proposal"},
		Gate: &schema.GateSpec{Target: source, Rules: rules},
	}
}

func workflowProposal(steps ...schema.StepSpec) schema.WorkflowDefinition {
	return schema.WorkflowDefinition{
		SchemaVersion: "1", ID: "workflow-cli", Version: "1.0.0", ConcurrencyLimit: 1,
		Objective: schema.ObjectiveSpec{
			ID: "objective-cli", Version: "1.0.0", Statement: "Produce a proposal",
			Deliverables: []schema.DeliverableRef{{Name: "proposal", Source: "proposal.text"}},
			SuccessCriteria: []schema.CriterionSpec{
				{ID: "nonempty", Deliverable: "proposal", Rules: []schema.GateRuleSpec{{Kind: schema.RuleNonEmpty}}},
			},
		},
		Steps: steps,
	}
}

func writeWorkflowDefinition(t *testing.T, dir string, def schema.WorkflowDefinition) string {
	t.Helper()
	data, err := json.Marshal(def)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "workflow.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// workflowRunID extracts the run identifier from a JSON payload.
func workflowRunID(t *testing.T, payload map[string]any) string {
	t.Helper()
	runID, _ := payload["run_id"].(string)
	if runID == "" {
		t.Fatalf("payload without run_id: %v", payload)
	}
	return runID
}

func TestWorkflowCLI(t *testing.T) {
	t.Run("flags y subcomandos inválidos usan el código de uso", func(t *testing.T) {
		c := newCaptured(t)
		for _, args := range [][]string{
			{"workflow"},
			{"workflow", "desconocido"},
			{"workflow", "validate"},
			{"workflow", "run"},
			{"workflow", "status"},
			{"workflow", "approve", "run-1"},
			{"workflow", "cancel"},
		} {
			if code := runCLI(t, c, args...); code != ExitUsageError {
				t.Errorf("%v = %d; want %d", args, code, ExitUsageError)
			}
		}
	})

	t.Run("validate es de solo lectura y respeta los proveedores condicionales", func(t *testing.T) {
		c := newCaptured(t)
		file := writeWorkflowDefinition(t, c.deps.WorkDir, workflowProposal(workflowRouterStep("proposal")))

		if code := runCLI(t, c, "workflow", "validate", "--json", file); code != ExitConfigError {
			t.Fatalf("validate sin Router = %d; want %d", code, ExitConfigError)
		}
		failure := parseJSON(t, c.stdout.String())
		if failure["status"] != "invalid" {
			t.Errorf("estado inesperado: %v", failure["status"])
		}
		failures, _ := failure["failures"].([]any)
		if len(failures) == 0 {
			t.Fatalf("sin fallos de preflight: %v", failure)
		}
		first, _ := failures[0].(map[string]any)
		if first["code"] != "provider_unavailable" {
			t.Errorf("código inesperado: %v", first["code"])
		}

		c.deps.WorkflowRouter = workflowRouterFunc(func(context.Context, engine.RouterRequest) (engine.RouterResult, error) {
			return engine.RouterResult{Text: "unused"}, nil
		})
		if code := runCLI(t, c, "workflow", "validate", "--json", file); code != ExitOK {
			t.Fatalf("validate con Router = %d: %s", code, c.stderr.String())
		}
		report := parseJSON(t, c.stdout.String())
		if report["status"] != "validated" || report["schema_version"] != "1" || report["command"] != "workflow validate" {
			t.Errorf("envelope inesperado: %v", report)
		}
		if _, err := os.Stat(filepath.Join(c.deps.WorkDir, ".gandalf")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("validate creó estado local: %v", err)
		}
	})

	t.Run("run ejecuta con los puertos inyectados y sella la evidencia", func(t *testing.T) {
		c := newCaptured(t)
		calls := 0
		c.deps.WorkflowRouter = workflowRouterFunc(func(context.Context, engine.RouterRequest) (engine.RouterResult, error) {
			calls++
			return engine.RouterResult{Text: "propuesta lista"}, nil
		})
		file := writeWorkflowDefinition(t, c.deps.WorkDir, workflowProposal(workflowRouterStep("proposal")))
		if code := runCLI(t, c, "workflow", "run", "--json", file); code != ExitOK {
			t.Fatalf("run = %d: %s", code, c.stderr.String())
		}
		if calls != 1 {
			t.Errorf("llamadas al Router = %d; want 1", calls)
		}
		payload := parseJSON(t, c.stdout.String())
		if payload["status"] != "succeeded" || payload["run_status"] != "succeeded" {
			t.Fatalf("estado inesperado: %v", payload)
		}
		runID := workflowRunID(t, payload)
		steps, _ := payload["steps"].([]any)
		if len(steps) != 1 {
			t.Fatalf("pasos inesperados: %v", steps)
		}
		step, _ := steps[0].(map[string]any)
		artifacts, _ := step["artifacts"].([]any)
		if len(artifacts) != 1 {
			t.Fatalf("artefactos inesperados: %v", step)
		}
		artifact, _ := artifacts[0].(map[string]any)
		location, _ := artifact["rel_path"].(string)
		if !strings.HasPrefix(location, ".gandalf/workflows/runs/"+runID+"/artifacts/") {
			t.Errorf("ubicación inesperada: %q", location)
		}
		if _, err := os.Stat(filepath.Join(c.deps.WorkDir, ".gandalf", "workflows", "runs", runID, "state.json")); err != nil {
			t.Errorf("checkpoint ausente: %v", err)
		}
	})

	t.Run("run ejecuta la memoria con el adaptador de ROMERO de la configuración", func(t *testing.T) {
		c := newCaptured(t)
		c.withProjectFile(t, "faro")
		memory := faroReadPort()
		memory.Degraded = true
		memory.DegradationNotice = "Consulta con cobertura reducida"
		secret := `"status"`
		memory.Records[0].Summary += " " + secret
		c.deps.Getenv = func(key string) string {
			if key == config.EnvKeyRomeroAPIKey {
				return secret
			}
			return ""
		}
		c.deps.ReadPort = memory
		c.deps.WorkflowRouter = workflowRouterFunc(func(context.Context, engine.RouterRequest) (engine.RouterResult, error) {
			return engine.RouterResult{Text: "propuesta lista"}, nil
		})
		file := writeWorkflowDefinition(t, c.deps.WorkDir, workflowProposal(
			workflowMemoryStep("context", "informes"),
			workflowRouterStep("proposal", "context"),
		))
		if code := runCLI(t, c, "workflow", "run", "--json", "--romero-url", "http://romero.test", file); code != ExitOK {
			t.Fatalf("run con ROMERO = %d: %s", code, c.stderr.String())
		}
		payload := parseJSON(t, c.stdout.String())
		if payload["status"] != "succeeded" {
			t.Fatalf("estado inesperado: %v", payload["status"])
		}
		steps, _ := payload["steps"].([]any)
		if len(steps) != 2 {
			t.Fatalf("pasos inesperados: %v", steps)
		}
		context, _ := steps[0].(map[string]any)
		if context["id"] != "context" || context["status"] != "passed" {
			t.Errorf("paso de memoria inesperado: %v", context)
		}
		artifacts := context["artifacts"].([]any)
		rel := artifacts[0].(map[string]any)["rel_path"].(string)
		body, err := os.ReadFile(filepath.Join(c.deps.WorkDir, rel))
		if err != nil {
			t.Fatal(err)
		}
		evidence := parseJSON(t, string(body))
		entries, _ := evidence["entries"].([]any)
		if evidence["status"] != "ok" || evidence["degraded"] != true ||
			evidence["degradation_notice"] != memory.DegradationNotice || len(entries) != 1 {
			t.Fatalf("evidencia de ROMERO incompleta: %v", evidence)
		}
		entry := entries[0].(map[string]any)
		if entry["memory_ref"] != "M1" || entry["source_ref"] == nil {
			t.Fatalf("atribución ausente: %v", entry)
		}
		summary, _ := entry["summary"].(string)
		if strings.Contains(summary, secret) || !strings.Contains(summary, gates.RedactedPlaceholder) {
			t.Fatal("la credencial escapada no se redactó en el artefacto de ROMERO")
		}
	})

	t.Run("run distingue blocked de failed", func(t *testing.T) {
		sentinel := "workflow-cli-sentinel-credential"
		c := newCaptured(t)
		c.deps.Getenv = func(key string) string {
			if key == config.EnvKeyRomeroAPIKey {
				return sentinel
			}
			return ""
		}
		c.deps.WorkflowRouter = workflowRouterFunc(func(context.Context, engine.RouterRequest) (engine.RouterResult, error) {
			return engine.RouterResult{Text: sentinel}, nil
		})
		blockedFile := writeWorkflowDefinition(t, c.deps.WorkDir, workflowProposal(
			workflowRouterStep("proposal"),
			workflowGateStep("privacy", "proposal.text", schema.GateRuleSpec{Kind: schema.RuleNoSecrets}),
		))
		if code := runCLI(t, c, "workflow", "run", "--json", blockedFile); code != ExitBlocked {
			t.Fatalf("run bloqueado = %d: %s", code, c.stderr.String())
		}
		if payload := parseJSON(t, c.stdout.String()); payload["status"] != "blocked" {
			t.Errorf("estado inesperado: %v", payload["status"])
		}
		if strings.Contains(c.stdout.String(), sentinel) {
			t.Error("la salida expuso el valor de la credencial")
		}

		c.deps.WorkflowRouter = workflowRouterFunc(func(context.Context, engine.RouterRequest) (engine.RouterResult, error) {
			return engine.RouterResult{}, errors.New("proveedor no disponible")
		})
		failedFile := writeWorkflowDefinition(t, c.deps.WorkDir, workflowProposal(workflowRouterStep("proposal")))
		if code := runCLI(t, c, "workflow", "run", "--json", failedFile); code != ExitFailed {
			t.Fatalf("run fallido = %d: %s", code, c.stderr.String())
		}
		if payload := parseJSON(t, c.stdout.String()); payload["status"] != "failed" {
			t.Errorf("estado inesperado: %v", payload["status"])
		}
	})

	t.Run("run se detiene en la puerta humana y se reanuda tras aprobar", func(t *testing.T) {
		c := newCaptured(t)
		c.deps.WorkflowRouter = workflowRouterFunc(func(context.Context, engine.RouterRequest) (engine.RouterResult, error) {
			return engine.RouterResult{Text: "propuesta lista"}, nil
		})
		file := writeWorkflowDefinition(t, c.deps.WorkDir, workflowProposal(
			workflowRouterStep("proposal"),
			workflowApprovalStep("approval", "proposal"),
		))
		if code := runCLI(t, c, "workflow", "run", "--json", file); code != ExitAwaitingApproval {
			t.Fatalf("run con puerta = %d: %s", code, c.stderr.String())
		}
		payload := parseJSON(t, c.stdout.String())
		runID := workflowRunID(t, payload)
		if payload["run_status"] != "awaiting_approval" {
			t.Errorf("run_status inesperado: %v", payload["run_status"])
		}
		checkpoint, _ := payload["checkpoint_digest"].(string)
		pending, _ := payload["pending_gates"].([]any)
		if checkpoint == "" || len(pending) != 1 || pending[0] != "approval" {
			t.Fatalf("puerta pendiente inesperada: %v", payload)
		}

		if code := runCLI(t, c, "workflow", "status", "--json", runID); code != ExitOK {
			t.Fatalf("status = %d: %s", code, c.stderr.String())
		}
		snapshot := parseJSON(t, c.stdout.String())
		if snapshot["status"] != "ok" || snapshot["run_status"] != "awaiting_approval" || snapshot["command"] != "workflow status" {
			t.Errorf("envelope de status inesperado: %v", snapshot)
		}

		if code := runCLI(t, c, "workflow", "approve", runID, "--gate", "approval", "--checkpoint", "huella-ajena", "--decision", "approve"); code != ExitConflict {
			t.Errorf("approve con huella ajena = %d; want %d", code, ExitConflict)
		}
		if code := runCLI(t, c, "workflow", "approve", runID, "--gate", "approval", "--checkpoint", checkpoint, "--decision", "reject"); code != ExitUsageError {
			t.Errorf("reject sin motivo = %d; want %d", code, ExitUsageError)
		}
		if code := runCLI(t, c, "workflow", "approve", "--json", runID, "--gate", "approval", "--checkpoint", checkpoint, "--decision", "approve"); code != ExitOK {
			t.Fatalf("approve = %d: %s", code, c.stderr.String())
		}
		decision := parseJSON(t, c.stdout.String())
		if decision["status"] != "ok" || decision["run_status"] != "awaiting_approval" {
			t.Errorf("envelope de approve inesperado: %v", decision)
		}
		if code := runCLI(t, c, "workflow", "resume", "--json", runID); code != ExitOK {
			t.Fatalf("resume = %d: %s", code, c.stderr.String())
		}
		if resumed := parseJSON(t, c.stdout.String()); resumed["status"] != "succeeded" {
			t.Errorf("resume no cerró la corrida: %v", resumed["status"])
		}
		if code := runCLI(t, c, "workflow", "resume", "--json", runID); code != ExitConflict {
			t.Errorf("resume terminal = %d; want %d", code, ExitConflict)
		}
		// The conflict payload keeps the terminal state the plan asks for and
		// the reason reaches stderr without breaking the JSON envelope.
		if terminal := parseJSON(t, c.stdout.String()); terminal["status"] != "succeeded" {
			t.Errorf("el payload debe informar el estado terminal: %v", terminal["status"])
		}
		if !strings.Contains(c.stderr.String(), "terminal") {
			t.Errorf("el rechazo de resume no dejó diagnóstico: %q", c.stderr.String())
		}
		// A rejected gate blocks the run; resuming a blocked run is a conflict
		// even though the persisted state is terminal.
		rejected := workflowRunID(t, func() map[string]any {
			file := writeWorkflowDefinition(t, c.deps.WorkDir, workflowProposal(
				workflowRouterStep("proposal"),
				workflowApprovalStep("approval", "proposal"),
			))
			if code := runCLI(t, c, "workflow", "run", "--json", file); code != ExitAwaitingApproval {
				t.Fatalf("run con puerta = %d: %s", code, c.stderr.String())
			}
			return parseJSON(t, c.stdout.String())
		}())
		if code := runCLI(t, c, "workflow", "approve", "--json", rejected, "--gate", "approval",
			"--checkpoint", checkpointOf(t, c, rejected), "--decision", "reject", "--reason", "sin evidencia"); code != ExitOK {
			t.Fatalf("reject = %d: %s", code, c.stderr.String())
		}
		if code := runCLI(t, c, "workflow", "resume", "--json", rejected); code != ExitConflict {
			t.Errorf("resume bloqueado = %d; want %d", code, ExitConflict)
		}
		if blocked := parseJSON(t, c.stdout.String()); blocked["status"] != "blocked" {
			t.Errorf("el payload debe informar el estado bloqueado: %v", blocked["status"])
		}
	})

	t.Run("resume continúa una corrida interrumpida", func(t *testing.T) {
		c := newCaptured(t)
		ctx, interrupt := context.WithCancel(context.Background())
		c.deps.WorkflowRouter = workflowRouterFunc(func(ctx context.Context, _ engine.RouterRequest) (engine.RouterResult, error) {
			interrupt()
			return engine.RouterResult{}, ctx.Err()
		})
		file := writeWorkflowDefinition(t, c.deps.WorkDir, workflowProposal(workflowRouterStep("proposal")))
		if code := Run(ctx, []string{"workflow", "run", "--json", file}, c.deps); code != ExitPaused {
			t.Fatalf("run interrumpido = %d: %s", code, c.stderr.String())
		}
		payload := parseJSON(t, c.stdout.String())
		runID := workflowRunID(t, payload)
		if payload["status"] != "paused" {
			t.Fatalf("estado inesperado: %v", payload["status"])
		}
		c.deps.WorkflowRouter = workflowRouterFunc(func(context.Context, engine.RouterRequest) (engine.RouterResult, error) {
			return engine.RouterResult{Text: "propuesta lista"}, nil
		})
		if code := runCLI(t, c, "workflow", "resume", "--json", runID); code != ExitOK {
			t.Fatalf("resume = %d: %s", code, c.stderr.String())
		}
		if resumed := parseJSON(t, c.stdout.String()); resumed["status"] != "succeeded" {
			t.Errorf("resume no terminó la corrida: %v", resumed["status"])
		}
	})

	t.Run("cancel distingue la cancelación directa del conflicto terminal", func(t *testing.T) {
		c := newCaptured(t)
		c.deps.WorkflowRouter = workflowRouterFunc(func(context.Context, engine.RouterRequest) (engine.RouterResult, error) {
			return engine.RouterResult{Text: "propuesta lista"}, nil
		})
		file := writeWorkflowDefinition(t, c.deps.WorkDir, workflowProposal(
			workflowRouterStep("proposal"),
			workflowApprovalStep("approval", "proposal"),
		))
		if code := runCLI(t, c, "workflow", "run", "--json", file); code != ExitAwaitingApproval {
			t.Fatalf("run con puerta = %d: %s", code, c.stderr.String())
		}
		runID := workflowRunID(t, parseJSON(t, c.stdout.String()))
		if code := runCLI(t, c, "workflow", "cancel", "--json", runID); code != ExitCanceled {
			t.Fatalf("cancel directo = %d: %s", code, c.stderr.String())
		}
		direct := parseJSON(t, c.stdout.String())
		if direct["status"] != "ok" || direct["run_status"] != "cancelled" || direct["requested"] != false {
			t.Errorf("cancelación directa inesperada: %v", direct)
		}
		// Repeating the cancel keeps the cancellation code: the run is already
		// in the requested state.
		if code := runCLI(t, c, "workflow", "cancel", "--json", runID); code != ExitCanceled {
			t.Errorf("cancel repetido = %d; want %d", code, ExitCanceled)
		}
		// A terminal run that is not cancelled is a conflict.
		done := writeWorkflowDefinition(t, c.deps.WorkDir, workflowProposal(workflowRouterStep("proposal")))
		if code := runCLI(t, c, "workflow", "run", "--json", done); code != ExitOK {
			t.Fatalf("run = %d: %s", code, c.stderr.String())
		}
		succeeded := workflowRunID(t, parseJSON(t, c.stdout.String()))
		if code := runCLI(t, c, "workflow", "cancel", "--json", succeeded); code != ExitConflict {
			t.Errorf("cancel terminal = %d; want %d", code, ExitConflict)
		}
	})

	t.Run("approve redacta el motivo con la credencial del entorno", func(t *testing.T) {
		sentinel := "workflow-cli-reason-sentinel"
		c := newCaptured(t)
		c.deps.Getenv = func(key string) string {
			if key == config.EnvKeyRomeroAPIKey {
				return sentinel
			}
			return ""
		}
		c.deps.WorkflowRouter = workflowRouterFunc(func(context.Context, engine.RouterRequest) (engine.RouterResult, error) {
			return engine.RouterResult{Text: "propuesta lista"}, nil
		})
		file := writeWorkflowDefinition(t, c.deps.WorkDir, workflowProposal(
			workflowRouterStep("proposal"),
			workflowApprovalStep("approval", "proposal"),
		))
		if code := runCLI(t, c, "workflow", "run", "--json", file); code != ExitAwaitingApproval {
			t.Fatalf("run con puerta = %d: %s", code, c.stderr.String())
		}
		payload := parseJSON(t, c.stdout.String())
		runID := workflowRunID(t, payload)
		checkpoint, _ := payload["checkpoint_digest"].(string)
		if code := runCLI(t, c, "workflow", "approve", "--json", runID, "--gate", "approval",
			"--checkpoint", checkpoint, "--decision", "reject", "--reason", "motivo "+sentinel); code != ExitOK {
			t.Fatalf("approve = %d: %s", code, c.stderr.String())
		}
		if strings.Contains(c.stdout.String(), sentinel) {
			t.Error("la salida expuso el motivo sin redactar")
		}
		statePath := filepath.Join(c.deps.WorkDir, ".gandalf", "workflows", "runs", runID, "state.json")
		data, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), sentinel) {
			t.Error("el checkpoint persistió el valor de la credencial")
		}
		if !strings.Contains(string(data), gates.RedactedPlaceholder) {
			t.Error("el checkpoint no marca la redacción")
		}
	})

	t.Run("status de una corrida inexistente usa configuración", func(t *testing.T) {
		c := newCaptured(t)
		if code := runCLI(t, c, "workflow", "status", "--json", "run-inexistente"); code != ExitConfigError {
			t.Fatalf("status = %d; want %d", code, ExitConfigError)
		}
		if failure := parseJSON(t, c.stdout.String()); failure["status"] != "not_found" {
			t.Errorf("estado inesperado: %v", failure["status"])
		}
		if _, err := os.Stat(filepath.Join(c.deps.WorkDir, ".gandalf")); !os.IsNotExist(err) {
			t.Errorf("status alteró el directorio del proyecto: %v", err)
		}
	})

	t.Run("preflight rechaza un perfil explícito ausente con otro perfil elegible", func(t *testing.T) {
		c := newCaptured(t)
		withRouterConfig(t, c, fiveExecutors, fiveProfiles)
		c.deps.RouterRegistry = standardRegistry()
		step := workflowRouterStep("proposal")
		step.Router.Profile = "ok1/perfil-ausente"
		file := writeWorkflowDefinition(t, c.deps.WorkDir, workflowProposal(step))
		for _, command := range []string{"validate", "run"} {
			if code := runCLI(t, c, "workflow", command, "--json", file); code != ExitConfigError {
				t.Errorf("%s con perfil inexistente: código %d, salida %s", command, code, c.stdout.String())
				continue
			}
			if failure := parseJSON(t, c.stdout.String()); failure["status"] != "invalid" {
				t.Errorf("%s: diagnóstico inesperado: %v", command, failure)
			}
		}
		runs, err := os.ReadDir(filepath.Join(c.deps.WorkDir, ".gandalf", "workflows", "runs"))
		if !os.IsNotExist(err) && (err != nil || len(runs) != 0) {
			t.Errorf("el preflight creó una corrida: %v (%v)", runs, err)
		}
	})

	t.Run("resume requiere perfiles solo para los pasos pendientes", func(t *testing.T) {
		c := newCaptured(t)
		withRouterConfig(t, c, fiveExecutors, fiveProfiles)
		first := &fakeRouterAdapter{probe: okProbe()}
		second := &fakeRouterAdapter{probe: okProbe()}
		c.deps.RouterRegistry = registryWith(first, second, &fakeRouterAdapter{}, &fakeRouterAdapter{})
		draft := workflowRouterStep("draft")
		draft.Router.Profile = "ok1/modelo-a"
		proposal := workflowRouterStep("proposal", "approval")
		proposal.Router.Profile = "ok2/modelo-b"
		file := writeWorkflowDefinition(t, c.deps.WorkDir, workflowProposal(draft, workflowApprovalStep("approval", "draft"), proposal))
		if code := runCLI(t, c, "workflow", "run", "--json", file); code != ExitAwaitingApproval {
			t.Fatalf("run = %d: %s %s", code, c.stdout.String(), c.stderr.String())
		}
		payload := parseJSON(t, c.stdout.String())
		runID := workflowRunID(t, payload)
		checkpoint := payload["checkpoint_digest"].(string)
		if code := runCLI(t, c, "workflow", "approve", runID, "--gate", "approval", "--checkpoint", checkpoint, "--decision", "approve"); code != ExitOK {
			t.Fatalf("approve = %d: %s", code, c.stderr.String())
		}
		unavailable := &fakeRouterAdapter{}
		c.deps.RouterRegistry = registryWith(unavailable, second, &fakeRouterAdapter{}, &fakeRouterAdapter{})
		if code := runCLI(t, c, "workflow", "resume", "--json", runID); code != ExitOK {
			t.Fatalf("resume = %d: %s %s", code, c.stdout.String(), c.stderr.String())
		}
		if first.count() != 1 || second.count() != 1 || unavailable.count() != 0 {
			t.Fatalf("llamadas al perfil completado = %d, al pendiente = %d", first.count(), second.count())
		}
	})

	t.Run("validate rechaza un router declarado pero no elegible", func(t *testing.T) {
		c := newCaptured(t)
		withRouterConfig(t, c, fiveExecutors, fiveProfiles)
		missing := func() *fakeRouterAdapter {
			return &fakeRouterAdapter{probe: router.Probe{Installed: false, Reason: "no instalado"}}
		}
		c.deps.RouterRegistry = registryWith(missing(), missing(), missing(), missing())
		file := writeWorkflowDefinition(t, c.deps.WorkDir, workflowProposal(workflowRouterStep("proposal")))
		if code := runCLI(t, c, "workflow", "validate", "--json", file); code != ExitConfigError {
			t.Fatalf("validate con router no elegible = %d: %s", code, c.stderr.String())
		}
		failure := parseJSON(t, c.stdout.String())
		failures, _ := failure["failures"].([]any)
		if len(failures) == 0 {
			t.Fatalf("sin diagnóstico de proveedor: %v", failure)
		}
		first, _ := failures[0].(map[string]any)
		if first["code"] != "provider_unavailable" {
			t.Errorf("código inesperado: %v", first["code"])
		}
	})

	t.Run("los códigos reservados y los puertos conservan su contrato", func(t *testing.T) {
		if ExitOK != 0 || ExitInternalError != 1 || ExitUsageError != 2 || ExitConfigError != 3 ||
			ExitAccessDenied != 4 || ExitPartialQuery != 5 || ExitIncompleteQuery != 6 || ExitDoctorFailed != 7 ||
			ExitNoEligible != 8 || ExitExhausted != 9 || ExitCanceled != 10 ||
			ExitAwaitingApproval != 11 || ExitPaused != 12 || ExitPartial != 13 ||
			ExitBlocked != 14 || ExitFailed != 15 || ExitConflict != 16 {
			t.Fatal("los códigos reservados cambiaron de valor")
		}
		deps := Dependencies{Getenv: func(key string) string {
			if key == "TEST_CREDENTIAL" {
				return "credential-value"
			}
			return ""
		}}
		// The default ROMERO URL is not a configured provider (S6).
		if workflowMemoryReady(config.EffectiveConfig{}) {
			t.Error("la URL predeterminada no debe contar como ROMERO disponible")
		}
		flagged := config.EffectiveConfig{}
		flagged.RomeroBaseURL = config.TrackedValue[string]{Value: "http://romero.test", Origin: config.OriginFlag}
		if !workflowMemoryReady(flagged) {
			t.Error("una URL declarada por flag debe contar como ROMERO disponible")
		}
		if !workflowMemoryReady(config.EffectiveConfig{APIKeyPresent: true}) {
			t.Error("una credencial presente debe contar como ROMERO disponible")
		}
		if port := deps.workflowMemoryPort(config.EffectiveConfig{}); port != nil {
			t.Error("sin URL no debe existir puerto de memoria")
		}
		if port := deps.workflowMemoryPort(flagged); port == nil {
			t.Error("con URL debe existir puerto de memoria")
		}
		if port := deps.workflowRouterPort(config.EffectiveConfig{}); port != nil {
			t.Error("sin ejecutores no debe existir puerto de router")
		}
		policy := config.RouterPolicy{
			Executors: []config.RouterExecutor{{ID: "ok1", Kind: "cli", Authorized: true, CredentialEnv: "TEST_CREDENTIAL"}},
			Profiles:  []config.RouterProfile{{ID: "ok1/modelo", ExecutorID: "ok1", Model: "modelo"}},
		}
		withRouter := config.EffectiveConfig{Router: policy}
		port := deps.workflowRouterPort(withRouter)
		adapter, ok := port.(routerPort)
		if !ok || adapter.getenv("TEST_CREDENTIAL") != "credential-value" {
			t.Error("el adaptador de producción no conserva deps.getenv")
		}
		if secrets := deps.workflowSecrets(withRouter); len(secrets) != 1 || secrets[0] != "credential-value" {
			t.Errorf("secretos = %v", secrets)
		}
		if err := adapter.CheckEligibility(context.Background(), []schema.StepSpec{workflowRouterStep("proposal")}); err == nil {
			t.Error("un adaptador no instalado no debe contar como elegible")
		}
		c := newCaptured(t)
		runtime, err := c.deps.workflowDeps(context.Background(), &workflowFlags{timeout: "5s"}, true)
		if err != nil {
			t.Fatalf("workflowDeps: %v", err)
		}
		if runtime.timeout != 5*time.Second {
			t.Errorf("timeout = %v; want 5s", runtime.timeout)
		}
		if _, stop := runtime.withTimeout(context.Background()); stop == nil {
			t.Error("withTimeout no devolvió cancelación")
		}
	})
}

// checkpointOf reads the current checkpoint of a run through the CLI.
func checkpointOf(t *testing.T, c *captured, runID string) string {
	t.Helper()
	if code := runCLI(t, c, "workflow", "status", "--json", runID); code != ExitOK {
		t.Fatalf("status = %d: %s", code, c.stderr.String())
	}
	checkpoint, _ := parseJSON(t, c.stdout.String())["checkpoint_digest"].(string)
	if checkpoint == "" {
		t.Fatal("status sin checkpoint")
	}
	return checkpoint
}

// TestWorkflowCancelMapping pins the cooperative request mapping.
func TestWorkflowCancelMapping(t *testing.T) {
	cases := []struct {
		name   string
		result engine.CancelResult
		err    error
		want   int
	}{
		{"directa confirmada", engine.CancelResult{State: &store.RunState{Status: "cancelled"}}, nil, ExitCanceled},
		{"solicitud confirmada", engine.CancelResult{State: &store.RunState{Status: "cancelled"}, Requested: true}, nil, ExitCanceled},
		{"terminal", engine.CancelResult{State: &store.RunState{Status: "succeeded"}}, engine.ErrTerminalRun, ExitConflict},
		{"run inexistente", engine.CancelResult{}, store.ErrRunNotFound, ExitConfigError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := workflowCancelExit(tc.result, tc.err); got != tc.want {
				t.Errorf("workflowCancelExit = %d; want %d", got, tc.want)
			}
		})
	}
	running := engine.CancelResult{State: &store.RunState{Status: "running"}, Requested: true}
	if !workflowCancelPending(running, context.DeadlineExceeded) {
		t.Error("una solicitud sin confirmar debe aceptarse como pendiente")
	}
	if workflowCancelPending(running, nil) || workflowCancelPending(engine.CancelResult{}, context.DeadlineExceeded) {
		t.Error("solo una solicitud escrita y expirada es pendiente")
	}
}

// TestWorkflowExitCodes keeps the reserved workflow codes collision-free.
func TestWorkflowExitCodes(t *testing.T) {
	reserved := map[int]string{
		ExitOK: "ok", ExitInternalError: "internal", ExitUsageError: "usage", ExitConfigError: "config",
		ExitAccessDenied: "access", ExitPartialQuery: "partial query", ExitIncompleteQuery: "incomplete query",
		ExitDoctorFailed: "doctor", ExitNoEligible: "no eligible", ExitExhausted: "exhausted", ExitCanceled: "canceled",
		ExitAwaitingApproval: "awaiting approval", ExitPaused: "paused", ExitPartial: "partial",
		ExitBlocked: "blocked", ExitFailed: "failed", ExitConflict: "conflict",
	}
	seen := map[string]int{}
	for code, name := range reserved {
		if other, ok := seen[name]; ok && other != code {
			t.Errorf("el código %q se repite en %d y %d", name, other, code)
		}
		seen[name] = code
	}
	if len(reserved) != 17 {
		t.Errorf("códigos reservados = %d; want 17", len(reserved))
	}
}
