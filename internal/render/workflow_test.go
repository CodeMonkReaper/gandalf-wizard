package render

import (
	"bytes"
	"encoding/json"
	"maps"
	"strings"
	"testing"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/engine"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/gates"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/store"
)

func TestWorkflowRender(t *testing.T) {
	secret := "only-fixture-known-credential"
	fixedSecret := "sk-" + strings.Repeat("a", 30)
	secrets := []string{secret}
	wf := engine.ValidateReport{Workflow: schema.ValidatedWorkflow{
		DefinitionPath: "private/path/to/definition.json",
		ExecutionOrder: []string{"background", "synthesis", "quality"},
		RequiresMemory: true,
		Definition: schema.WorkflowDefinition{
			ID: "proposal", Version: "1.0.0", ConcurrencyLimit: 4,
			Objective: schema.ObjectiveSpec{
				ID: "objective", Version: "1.0.0", Statement: "Revisar " + secret,
				Constraints:  []string{"Sin cambios", "No mostrar " + fixedSecret},
				Deliverables: []schema.DeliverableRef{{Name: "report", Source: "synthesis.report"}},
				SuccessCriteria: []schema.CriterionSpec{{
					ID: "report-ready", Deliverable: "report", Rules: []schema.GateRuleSpec{
						{Kind: schema.RuleContains, Value: "private criteria literal"},
					},
				}},
			},
			Steps: []schema.StepSpec{
				{ID: "synthesis", Kind: schema.StepKindRouterTask, DependsOn: []string{"background"},
					Router:     &schema.RouterStepSpec{Prompt: "private prompt"},
					Correction: &schema.CorrectionSpec{GateStepID: "quality", MaxIterations: 2}},
				{ID: "background", Kind: schema.StepKindMemoryQuery},
				{ID: "quality", Kind: schema.StepKindQualityGate, DependsOn: []string{"synthesis"},
					Gate: &schema.GateSpec{Target: "synthesis.report", Rules: []schema.GateRuleSpec{{Kind: schema.RuleNonEmpty}}}},
			},
		},
	}}
	ref := store.ArtifactRef{Name: "report", RelPath: "artifacts/synthesis-2-report.txt", SHA256: "digest", SizeBytes: 42,
		Summary: "Avance | " + secret + "\n" + fixedSecret + "\x1b[31m"}
	oldRef := store.ArtifactRef{Name: "report", RelPath: "artifacts/synthesis-1-report.txt",
		SHA256: "earlier-digest", Summary: "Avance previo"}
	state := &store.RunState{
		SchemaVersion: "future", RunID: "run-1", WorkflowID: "proposal", WorkflowVersion: "1.0.0",
		Status: "awaiting_approval", CheckpointDigest: "checkpoint", StateRevision: 5,
		ObjectiveID: "objective", ObjectiveVersion: "1.0.0",
		DefinitionPath: wf.Workflow.DefinitionPath, PendingGates: []string{"approval"},
		Steps: map[string]store.StepRecord{
			"synthesis": {ID: "synthesis", Kind: "router_task", Status: "passed", Iteration: 2,
				Artifacts: []store.ArtifactRef{ref}, Attempts: []store.AttemptRecord{
					{Iteration: 1, Status: "failed", Error: "intento " + secret, Artifacts: []store.ArtifactRef{oldRef}},
					{Iteration: 2, Status: "passed", Artifacts: []store.ArtifactRef{ref}},
				}},
			"approval": {ID: "approval", Kind: "human_approval", Status: "awaiting_approval"},
		},
		Gates: map[string]store.GateRecord{
			"quality": {StepID: "quality", Status: "pass", Warnings: []string{"aviso " + secret}},
		},
		Approvals:        []store.ApprovalRecord{{GateID: "earlier", Decision: "approve", Reason: "razón " + secret}},
		ObjectiveVerdict: &store.ObjectiveVerdictRecord{Status: "succeeded", MetCriteria: []string{"report-ready"}},
	}

	assertOutput := func(t *testing.T, command, status string, want []string, render func(*bytes.Buffer, bool) error) map[string]any {
		t.Helper()
		decoded := make(map[string]any)
		for _, asJSON := range []bool{false, true} {
			var b bytes.Buffer
			if err := render(&b, asJSON); err != nil {
				t.Errorf("render %s: %v", command, err)
				continue
			}
			for _, forbidden := range []string{secret, fixedSecret, "private prompt", "private criteria literal", wf.Workflow.DefinitionPath, "\x1b", `\u001b`, "outside-artifact.txt"} {
				if strings.Contains(b.String(), forbidden) {
					t.Errorf("forbidden material disclosed by %s (JSON=%t)", command, asJSON)
				}
			}
			for _, token := range want {
				if !strings.Contains(b.String(), token) {
					t.Errorf("%s (JSON=%t) missing %q", command, asJSON, token)
				}
			}
			if asJSON {
				if err := json.Unmarshal(b.Bytes(), &decoded); err != nil {
					t.Errorf("decode %s: %v", command, err)
				}
				if decoded["schema_version"] != "1" || decoded["command"] != command || decoded["status"] != status {
					t.Errorf("wrong JSON envelope for %s", command)
				}
			}
		}
		return decoded
	}

	t.Run("validate projects the objective and sorts graph topologically", func(t *testing.T) {
		decoded := assertOutput(t, "workflow validate", "validated",
			[]string{"objective", "report-ready", "background", "synthesis", gates.RedactedPlaceholder},
			func(w *bytes.Buffer, asJSON bool) error { return RenderWorkflowValidate(w, wf, asJSON, secrets) })
		steps, _ := decoded["steps"].([]any)
		if len(steps) != 3 || steps[0].(map[string]any)["id"] != "background" ||
			steps[1].(map[string]any)["id"] != "synthesis" || steps[2].(map[string]any)["id"] != "quality" {
			t.Fatal("wrong topological order")
		}
		correction, _ := steps[1].(map[string]any)["correction"].(map[string]any)
		gate, _ := steps[2].(map[string]any)["gate"].(map[string]any)
		if correction["gate_step_id"] != "quality" || correction["max_iterations"] != float64(2) || gate["target"] != "synthesis.report" {
			t.Error("preflight did not project the bounded correction loop")
		}
		var text bytes.Buffer
		if err := RenderWorkflowValidate(&text, wf, false, secrets); err != nil || !strings.Contains(text.String(), "Máximo 2 iteraciones") {
			t.Error("preflight table did not present the correction limit")
		}
	})

	t.Run("run shows checkpoint and safe artifact metadata", func(t *testing.T) {
		before, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"run-1", "checkpoint", "awaiting_approval", "approval", "report-ready", gates.RedactedPlaceholder,
			".gandalf/workflows/runs/run-1/artifacts/synthesis-2-report.txt",
			".gandalf/workflows/runs/run-1/artifacts/synthesis-1-report.txt"}
		decoded := assertOutput(t, "workflow run", "awaiting_approval", want,
			func(w *bytes.Buffer, asJSON bool) error {
				return RenderWorkflowRun(w, engine.RunSummary{State: state}, asJSON, secrets)
			})
		if decoded["run_status"] != "awaiting_approval" || decoded["state_revision"] != float64(5) {
			t.Error("run status or revision missing")
		}
		after, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Error("render mutated the run state")
		}
	})

	t.Run("status keeps the linked objective, prior attempts, and only confined paths", func(t *testing.T) {
		snapshot := *state
		snapshot.Steps = maps.Clone(state.Steps)
		step := snapshot.Steps["synthesis"]
		step.Artifacts = []store.ArtifactRef{
			ref,
			{Name: "invalid", RelPath: "artifacts/../outside-artifact.txt"},
			{Name: "absolute", RelPath: `C:\outside-artifact.txt`},
		}
		snapshot.Steps["synthesis"] = step
		decoded := assertOutput(t, "workflow status", "ok",
			[]string{"objective", "checkpoint", "awaiting_approval", "report-ready", gates.RedactedPlaceholder},
			func(w *bytes.Buffer, asJSON bool) error {
				return RenderWorkflowStatus(w, engine.StatusView{State: &snapshot}, asJSON, secrets)
			})
		steps, _ := decoded["steps"].([]any)
		if len(steps) != 2 {
			t.Fatal("missing projected steps")
		}
		synthesis := steps[1].(map[string]any)
		attempts, _ := synthesis["attempts"].([]any)
		if synthesis["id"] != "synthesis" || synthesis["iteration"] != float64(2) || len(attempts) != 2 {
			t.Error("missing attempt history")
		}
		artifacts, _ := synthesis["artifacts"].([]any)
		withheld := 0
		for _, entry := range artifacts {
			artifact, _ := entry.(map[string]any)
			if artifact["name"] != "invalid" && artifact["name"] != "absolute" {
				continue
			}
			if artifact["rel_path_withheld"] != true {
				t.Errorf("artifact %v did not report a withheld path", artifact["name"])
			}
			if _, ok := artifact["rel_path"]; ok {
				t.Errorf("artifact %v still exposes a rejected rel_path", artifact["name"])
			}
			withheld++
		}
		if withheld != 2 {
			t.Errorf("expected 2 artifacts with a withheld path, got %d", withheld)
		}
		var text bytes.Buffer
		if err := RenderWorkflowStatus(&text, engine.StatusView{State: &snapshot}, false, secrets); err != nil ||
			!strings.Contains(text.String(), "ruta fuera del espacio de corridas") {
			t.Error("the table hid the rejected artifact path instead of naming it")
		}
	})

	t.Run("approve, resume and cancel keep their command and run status", func(t *testing.T) {
		blocked := *state
		blocked.Status = "blocked"
		decoded := assertOutput(t, "workflow approve", "ok", []string{"blocked", "earlier", gates.RedactedPlaceholder},
			func(w *bytes.Buffer, asJSON bool) error {
				return RenderWorkflowApprove(w, engine.ApproveResult{State: &blocked}, asJSON, secrets)
			})
		if decoded["run_status"] != "blocked" {
			t.Error("missing blocked run status after rejection")
		}
		assertOutput(t, "workflow resume", "succeeded", []string{"run-1", "succeeded"},
			func(w *bytes.Buffer, asJSON bool) error {
				resumed := *state
				resumed.Status = "succeeded"
				return RenderWorkflowResume(w, engine.RunSummary{State: &resumed}, asJSON, secrets)
			})
		decoded = assertOutput(t, "workflow cancel", "ok", []string{"run-1", "running"},
			func(w *bytes.Buffer, asJSON bool) error {
				running := *state
				running.Status = "running"
				return RenderWorkflowCancel(w, engine.CancelResult{State: &running, Requested: true}, asJSON, secrets)
			})
		if decoded["requested"] != true || decoded["run_status"] != "running" {
			t.Error("cooperative cancellation was not reported with its run status")
		}
		closed := *state
		closed.Status = "cancelled"
		decoded = assertOutput(t, "workflow cancel", "ok", []string{"run-1", "cancelled"},
			func(w *bytes.Buffer, asJSON bool) error {
				return RenderWorkflowCancel(w, engine.CancelResult{State: &closed}, asJSON, secrets)
			})
		if value, ok := decoded["requested"]; !ok || value != false {
			t.Errorf("a direct cancellation must report requested=false, got %v (present=%t)", value, ok)
		}
		var text bytes.Buffer
		if err := RenderWorkflowCancel(&text, engine.CancelResult{State: &closed}, false, secrets); err != nil ||
			!strings.Contains(text.String(), "Cancelación directa") {
			t.Error("the table did not distinguish a direct cancellation")
		}
	})
}
