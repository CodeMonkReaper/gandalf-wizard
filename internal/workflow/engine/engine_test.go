package engine

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/store"
)

type routerFunc func(context.Context, RouterRequest) (RouterResult, error)

func (f routerFunc) Execute(ctx context.Context, req RouterRequest) (RouterResult, error) {
	return f(ctx, req)
}

type memoryFunc func(context.Context, string, string, string) (MemoryResult, error)

func (f memoryFunc) Query(ctx context.Context, project, caller, query string) (MemoryResult, error) {
	return f(ctx, project, caller, query)
}

func prepareDefinition(t *testing.T, limit int, source string, steps ...schema.StepSpec) (string, string) {
	t.Helper()
	root, err := os.MkdirTemp("", "wf")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	file := filepath.Join(root, "workflow.json")
	def := schema.WorkflowDefinition{
		SchemaVersion: "1", ID: "workflow-test", Version: "1.0.0", ConcurrencyLimit: limit,
		Objective: schema.ObjectiveSpec{
			ID: "objective-test", Version: "1.0.0", Statement: "Produce a proposal",
			Deliverables: []schema.DeliverableRef{{Name: "proposal", Source: source}},
			SuccessCriteria: []schema.CriterionSpec{
				{ID: "nonempty", Deliverable: "proposal", Rules: []schema.GateRuleSpec{{Kind: schema.RuleNonEmpty}}},
			},
		},
		Steps: steps,
	}
	data, err := json.Marshal(def)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return root, file
}

func openStore(t *testing.T, root string) *store.WorkflowStore {
	t.Helper()
	s, err := store.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func routerStep(id string, dependencies ...string) schema.StepSpec {
	return schema.StepSpec{
		ID: id, Kind: schema.StepKindRouterTask, DependsOn: dependencies,
		Outputs: []string{"text"}, Router: &schema.RouterStepSpec{Prompt: id},
	}
}

func contextForTest(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func hasWorkflowCode(err error, code string) bool {
	var invalid *schema.ValidationErrors
	if !errors.As(err, &invalid) {
		return false
	}
	for _, entry := range invalid.Errors {
		if entry.Code == code {
			return true
		}
	}
	return false
}

func TestWorkflowEngineExecution(t *testing.T) {
	t.Run("read-only preflight", func(t *testing.T) {
		root, file := prepareDefinition(t, 1, "proposal.text", routerStep("proposal"))
		var calls atomic.Int32
		r, err := Validate(t.Context(), file, Dependencies{RouterReady: true, Router: routerFunc(func(context.Context, RouterRequest) (RouterResult, error) {
			calls.Add(1)
			return RouterResult{}, nil
		})})
		if err != nil || len(r.Workflow.ExecutionOrder) != 1 || calls.Load() != 0 {
			t.Fatalf("preflight = %+v, %v; provider calls = %d", r, err, calls.Load())
		}
		if _, err := os.Stat(filepath.Join(root, ".gandalf")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("preflight changed the project: %v", err)
		}
	})

	t.Run("parallel dependencies and limit", func(t *testing.T) {
		ctx := contextForTest(t)
		memory := schema.StepSpec{ID: "context", Kind: schema.StepKindMemoryQuery,
			Outputs: []string{"text"}, Memory: &schema.MemoryStepSpec{Query: "background"}}
		join := routerStep("proposal", "context", "risks", "support")
		join.Router.Prompt = "proposal: {{background}}"
		join.Inputs = map[string]string{"background": "context.text"}
		root, file := prepareDefinition(t, 2, "proposal.text", memory, routerStep("risks"), routerStep("support"), join)
		started := make(chan string, 4)
		release := make(chan struct{})
		wait := func(name string) error {
			started <- name
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		var active, peak atomic.Int32
		enter := func() func() {
			n := active.Add(1)
			for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
			}
			return func() { active.Add(-1) }
		}
		memoryPort := memoryFunc(func(_ context.Context, project, caller, query string) (MemoryResult, error) {
			defer enter()()
			if project != "project" || caller != "caller" || query != "background" {
				return MemoryResult{}, errors.New("wrong memory arguments")
			}
			return MemoryResult{Text: "context evidence"}, wait("context")
		})
		var joined atomic.Bool
		routerPort := routerFunc(func(_ context.Context, req RouterRequest) (RouterResult, error) {
			defer enter()()
			if strings.HasPrefix(req.Prompt, "proposal: ") {
				if req.Prompt != "proposal: context evidence" || active.Load() != 1 {
					return RouterResult{}, errors.New("join bypassed dependency barrier")
				}
				joined.Store(true)
				return RouterResult{Text: "final proposal"}, nil
			}
			return RouterResult{Text: "independent evidence"}, wait(req.Prompt)
		})
		done := make(chan error, 1)
		runID := make(chan string, 1)
		s := openStore(t, root)
		go func() {
			r, err := Run(ctx, file, Dependencies{Store: s, Router: routerPort, RouterReady: true, Memory: memoryPort, MemoryReady: true, ProjectID: "project", CallerID: "caller"})
			if err == nil && (r.State == nil || r.State.Status != "succeeded") {
				err = errors.New("workflow did not succeed")
			}
			if r.State != nil {
				runID <- r.State.RunID
			}
			done <- err
		}()
		for range 2 {
			select {
			case <-started:
			case err := <-done:
				t.Fatalf("run finished before two providers overlapped: %v", err)
			case <-ctx.Done():
				t.Fatal("two providers did not overlap")
			}
		}
		close(release)
		if err := <-done; err != nil || !joined.Load() || peak.Load() != 2 {
			t.Fatalf("parallel run: %v; joined=%t peak=%d", err, joined.Load(), peak.Load())
		}
		events, err := s.ReadEvents(<-runID)
		if err != nil {
			t.Fatal(err)
		}
		starts := 0
		for _, event := range events {
			if event.EventType == "step_finished" {
				break
			}
			if event.EventType == "step_started" {
				starts++
			}
		}
		if starts != 2 {
			t.Fatalf("started %d steps before the first finished; limit = 2", starts)
		}
	})

	for _, tc := range []struct {
		name, want string
		recover    bool
	}{
		{"correction succeeds on retry", "succeeded", true},
		{"correction stops at limit", "failed", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			draft := routerStep("draft")
			draft.Correction = &schema.CorrectionSpec{GateStepID: "review", MaxIterations: 2}
			review := schema.StepSpec{ID: "review", Kind: schema.StepKindQualityGate, DependsOn: []string{"draft"},
				Gate: &schema.GateSpec{Target: "draft.text", Rules: []schema.GateRuleSpec{{Kind: schema.RuleContains, Value: "complete"}}}}
			publish := routerStep("proposal", "review")
			root, file := prepareDefinition(t, 1, "proposal.text", draft, review, publish)
			attempts := 0
			r, err := Run(t.Context(), file, Dependencies{Store: openStore(t, root), RouterReady: true,
				Router: routerFunc(func(_ context.Context, req RouterRequest) (RouterResult, error) {
					if req.Prompt == "proposal" {
						return RouterResult{Text: "final proposal"}, nil
					}
					attempts++
					if tc.recover && strings.Contains(req.Prompt, "contains") {
						return RouterResult{Text: "complete"}, nil
					}
					return RouterResult{Text: "draft"}, nil
				}),
			})
			if err != nil || r.State == nil || r.State.Status != tc.want || attempts != 2 {
				t.Fatalf("run = %+v, %v; attempts = %d", r, err, attempts)
			}
			if len(r.State.Steps["draft"].Attempts) != 2 || r.State.ObjectiveVerdict == nil {
				t.Fatal("missing attempt records or objective verdict")
			}
			if !tc.recover && r.State.Steps["proposal"].Status != "skipped" {
				t.Fatal("dependent ran despite failed prerequisite")
			}
			if !tc.recover && r.State.Steps["review"].Status != "failed" {
				t.Fatal("exhausted correction did not publish the failed gate")
			}
		})
	}

	t.Run("hard gate wins over correction", func(t *testing.T) {
		draft := routerStep("draft")
		draft.Correction = &schema.CorrectionSpec{GateStepID: "format", MaxIterations: 2}
		format := schema.StepSpec{ID: "format", Kind: schema.StepKindQualityGate, DependsOn: []string{"draft"},
			Gate: &schema.GateSpec{Target: "draft.text", Rules: []schema.GateRuleSpec{{Kind: schema.RuleContains, Value: "complete"}}}}
		safety := schema.StepSpec{ID: "privacy", Kind: schema.StepKindQualityGate, DependsOn: []string{"draft"},
			Gate: &schema.GateSpec{Target: "draft.text", Rules: []schema.GateRuleSpec{{Kind: schema.RuleNoSecrets}}}}
		root, file := prepareDefinition(t, 1, "draft.text", draft, format, safety)
		secret := "synthetic-private-text"
		attempts := 0
		r, err := Run(t.Context(), file, Dependencies{Store: openStore(t, root), RouterReady: true, SecretValues: []string{secret},
			Router: routerFunc(func(_ context.Context, _ RouterRequest) (RouterResult, error) {
				attempts++
				if attempts == 1 {
					return RouterResult{Text: secret}, nil
				}
				return RouterResult{Text: "complete"}, nil
			}),
		})
		if err != nil || r.State.Status != "blocked" || attempts != 1 {
			t.Fatalf("hard gate run = %+v, %v; attempts = %d", r, err, attempts)
		}
	})

	t.Run("redacts rule diagnostics", func(t *testing.T) {
		secret := "synthetic-private-field-name"
		review := schema.StepSpec{ID: "review", Kind: schema.StepKindQualityGate, DependsOn: []string{"draft"},
			Gate: &schema.GateSpec{Target: "draft.text", Informative: true, Rules: []schema.GateRuleSpec{{Kind: schema.RuleJSONRequiredKeys, Keys: []string{secret}}}}}
		root, file := prepareDefinition(t, 1, "draft.text", routerStep("draft"), review)
		s := openStore(t, root)
		r, err := Run(t.Context(), file, Dependencies{Store: s, RouterReady: true, SecretValues: []string{secret},
			Router: routerFunc(func(context.Context, RouterRequest) (RouterResult, error) { return RouterResult{Text: "{}"}, nil }),
		})
		if err != nil {
			t.Fatal(err)
		}
		persisted, err := s.LoadRun(r.State.RunID)
		if err != nil {
			t.Fatal(err)
		}
		for _, state := range []*store.RunState{r.State, persisted} {
			raw, err := json.Marshal(state)
			if err != nil || strings.Contains(string(raw), secret) || !strings.Contains(string(raw), "[redactado]") {
				t.Fatalf("expected redacted diagnostic, encoding error: %v", err)
			}
		}
	})

	t.Run("blocking gate failure skips dependents of the evaluated step", func(t *testing.T) {
		review := schema.StepSpec{ID: "review", Kind: schema.StepKindQualityGate, DependsOn: []string{"draft"},
			Gate: &schema.GateSpec{Target: "draft.text", Rules: []schema.GateRuleSpec{{Kind: schema.RuleContains, Value: "approved"}}}}
		notice := schema.StepSpec{ID: "notice", Kind: schema.StepKindQualityGate, DependsOn: []string{"draft"},
			Gate: &schema.GateSpec{Target: "draft.text", Informative: true, Rules: []schema.GateRuleSpec{{Kind: schema.RuleContains, Value: "reviewed"}}}}
		proposal := routerStep("proposal", "draft")
		root, file := prepareDefinition(t, 1, "proposal.text", routerStep("draft"), review, notice, proposal)
		st := openStore(t, root)
		var calls atomic.Int32
		r, err := Run(t.Context(), file, Dependencies{Store: st, RouterReady: true,
			Router: routerFunc(func(context.Context, RouterRequest) (RouterResult, error) {
				calls.Add(1)
				return RouterResult{Text: "incomplete draft"}, nil
			}),
		})
		if err != nil || r.State.Status != "failed" {
			t.Fatalf("status = %+v, %v", r.State, err)
		}
		if got := r.State.Steps["proposal"].Status; got != "skipped" {
			t.Fatalf("dependent of the gated branch = %q, want skipped", got)
		}
		if got := r.State.Steps["review"].Status; got != "failed" {
			t.Errorf("evaluated gate step = %q, want failed", got)
		}
		events, err := st.ReadEvents(r.State.RunID)
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		for _, event := range events {
			if event.EventType == "gate_evaluated" && event.Status == "fail" {
				found[event.StepID] = true
			}
		}
		if !found["review"] || !found["notice"] {
			t.Errorf("missing gate evaluations: %v", found)
		}
		if calls.Load() != 1 {
			t.Fatalf("provider calls = %d, want only the gated step", calls.Load())
		}
	})

	t.Run("a cancelled initial checkpoint cannot be started", func(t *testing.T) {
		root, file := prepareDefinition(t, 1, "draft.text", routerStep("draft"))
		st := openStore(t, root)
		var calls atomic.Int32
		deps := Dependencies{Store: st, RouterReady: true, Router: routerFunc(func(context.Context, RouterRequest) (RouterResult, error) {
			calls.Add(1)
			return RouterResult{Text: "draft"}, nil
		})}
		report, err := Validate(t.Context(), file, deps)
		if err != nil {
			t.Fatal(err)
		}
		runID, _, err := st.CreateRun(report.Workflow)
		if err != nil {
			t.Fatal(err)
		}
		cancelled, err := Cancel(t.Context(), runID, "stop before start", st)
		if err != nil || cancelled.State.Status != "cancelled" {
			t.Fatalf("cancel = %+v, %v", cancelled, err)
		}
		r, err := startCreatedRun(t.Context(), report.Workflow, deps, runID)
		if !errors.Is(err, ErrConflict) || r.State == nil || r.State.Status != "cancelled" || calls.Load() != 0 {
			t.Fatalf("start = %+v, %v; provider calls = %d", r, err, calls.Load())
		}
		state, err := st.LoadRun(runID)
		if err != nil || state.CheckpointDigest != cancelled.State.CheckpointDigest {
			t.Fatalf("cancelled checkpoint changed: %+v, %v", state, err)
		}
	})

	t.Run("preflight rejects artifact names the store cannot write", func(t *testing.T) {
		draft := routerStep("draft")
		draft.Outputs = []string{"CON"}
		root, file := prepareDefinition(t, 1, "draft.CON", draft)
		var calls atomic.Int32
		_, err := Run(t.Context(), file, Dependencies{Store: openStore(t, root), RouterReady: true,
			Router: routerFunc(func(context.Context, RouterRequest) (RouterResult, error) {
				calls.Add(1)
				return RouterResult{Text: "ok"}, nil
			}),
		})
		if err == nil || !hasWorkflowCode(err, "invalid_identifier") {
			t.Fatalf("run = %v, want invalid_identifier", err)
		}
		if calls.Load() != 0 {
			t.Fatalf("preflight reached the provider %d times", calls.Load())
		}
	})

	t.Run("optional quality gates are rejected before running", func(t *testing.T) {
		review := schema.StepSpec{ID: "review", Kind: schema.StepKindQualityGate, DependsOn: []string{"draft"}, Optional: true,
			Gate: &schema.GateSpec{Target: "draft.text", Rules: []schema.GateRuleSpec{{Kind: schema.RuleNonEmpty}}}}
		root, file := prepareDefinition(t, 1, "draft.text", routerStep("draft"), review)
		_, err := Run(t.Context(), file, Dependencies{Store: openStore(t, root), RouterReady: true,
			Router: routerFunc(func(context.Context, RouterRequest) (RouterResult, error) {
				return RouterResult{Text: "draft"}, nil
			}),
		})
		if err == nil || !hasWorkflowCode(err, "invalid_step") {
			t.Fatalf("run = %v, want invalid_step for an optional gate", err)
		}
	})

	t.Run("mid-flight store failure leaves a coherent failed checkpoint", func(t *testing.T) {
		root, file := prepareDefinition(t, 1, "draft.text", routerStep("draft"))
		s := openStore(t, root)
		report, err := Validate(t.Context(), file, Dependencies{Store: s, RouterReady: true})
		if err != nil {
			t.Fatal(err)
		}
		_, state, err := s.CreateRun(report.Workflow)
		if err != nil {
			t.Fatal(err)
		}
		artifacts := filepath.Join(root, ".gandalf", "workflows", "runs", state.RunID, "artifacts")
		if err := os.RemoveAll(artifacts); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(artifacts, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		work := newExecution(report.Workflow, Dependencies{Store: s, RouterReady: true,
			Router: routerFunc(func(context.Context, RouterRequest) (RouterResult, error) {
				return RouterResult{Text: "draft"}, nil
			})}, state)
		if _, runErr := work.run(t.Context()); runErr == nil {
			t.Fatal("expected the artifact write to fail")
		}
		persisted, err := s.LoadRun(state.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if persisted.Status != "failed" || persisted.Error == "" {
			t.Fatalf("persisted run = %q error %q, want failed with a reason", persisted.Status, persisted.Error)
		}
		step := persisted.Steps["draft"]
		if step.Status != "failed" || len(step.Attempts) != 1 || step.Attempts[0].Status != "failed" || step.Attempts[0].Error == "" {
			t.Fatalf("persisted step = %+v", step)
		}
		if state.Status != persisted.Status || state.Steps["draft"].Status != step.Status ||
			state.Steps["draft"].Attempts[0].Status != step.Attempts[0].Status {
			t.Fatalf("in-memory state diverged: %+v vs %+v", state.Steps["draft"], step)
		}
	})

	t.Run("deliverable with a credential explains the block", func(t *testing.T) {
		const secret = "synthetic-dc-explained-secret"
		root, file := prepareDefinition(t, 1, "draft.text", routerStep("draft"))
		r, err := Run(t.Context(), file, Dependencies{Store: openStore(t, root), RouterReady: true, SecretValues: []string{secret},
			Router: routerFunc(func(context.Context, RouterRequest) (RouterResult, error) {
				return RouterResult{Text: "proposal with " + secret}, nil
			}),
		})
		if err != nil || r.State.Status != "blocked" {
			t.Fatalf("status = %+v, %v", r.State, err)
		}
		verdict := r.State.ObjectiveVerdict
		if verdict == nil || len(verdict.Warnings) == 0 || len(verdict.FailedCriteria) != 0 {
			t.Fatalf("verdict = %+v, want a warning explaining the block", verdict)
		}
		if strings.Contains(verdict.Warnings[0], secret) {
			t.Fatalf("warning leaked the secret: %q", verdict.Warnings[0])
		}
	})

	t.Run("events record the attempt and gate verdict", func(t *testing.T) {
		draft := routerStep("draft")
		draft.Correction = &schema.CorrectionSpec{GateStepID: "review", MaxIterations: 2}
		review := schema.StepSpec{ID: "review", Kind: schema.StepKindQualityGate, DependsOn: []string{"draft"},
			Gate: &schema.GateSpec{Target: "draft.text", Rules: []schema.GateRuleSpec{{Kind: schema.RuleContains, Value: "complete"}}}}
		style := schema.StepSpec{ID: "style", Kind: schema.StepKindQualityGate, DependsOn: []string{"draft"},
			Gate: &schema.GateSpec{Target: "draft.text", Informative: true, Rules: []schema.GateRuleSpec{{Kind: schema.RuleContains, Value: "polished"}}}}
		root, file := prepareDefinition(t, 1, "draft.text", draft, review, style)
		s := openStore(t, root)
		attempts := 0
		r, err := Run(t.Context(), file, Dependencies{Store: s, RouterReady: true,
			Router: routerFunc(func(_ context.Context, req RouterRequest) (RouterResult, error) {
				if !strings.HasPrefix(req.Prompt, "draft") {
					return RouterResult{Text: "ok"}, nil
				}
				attempts++
				if attempts == 2 {
					return RouterResult{Text: "complete"}, nil
				}
				return RouterResult{Text: "draft"}, nil
			}),
		})
		if err != nil || r.State.Status != "succeeded" {
			t.Fatalf("run = %+v, %v", r.State, err)
		}
		events, err := s.ReadEvents(r.State.RunID)
		if err != nil {
			t.Fatal(err)
		}
		var retry, informative *store.RunEvent
		for i := range events {
			switch {
			case events[i].EventType == "step_finished" && events[i].StepID == "draft" && events[i].Iteration == 1:
				retry = &events[i]
			case events[i].EventType == "gate_evaluated" && events[i].StepID == "style":
				informative = &events[i]
			}
		}
		if retry == nil || retry.Status != "failed" || !strings.Contains(retry.Message, "contains") {
			t.Fatalf("retry event = %+v, want the failed attempt and its diagnosis", retry)
		}
		if informative == nil || informative.Status != "fail" || informative.Message == "" {
			t.Fatalf("informative gate event = %+v, want the fail verdict", informative)
		}
	})

	t.Run("validate rejects credentials embedded in metadata", func(t *testing.T) {
		// Built at runtime so the repository scanner never sees a key-shaped literal.
		credential := "sk-" + "proj-" + strings.Repeat("aB3d", 6)
		root, file := prepareDefinition(t, 1, credential+".text", routerStep(credential))
		_, err := Validate(t.Context(), file, Dependencies{Store: openStore(t, root), RouterReady: true})
		if err == nil || !hasWorkflowCode(err, "secret_in_metadata") {
			t.Fatalf("validate = %v, want secret_in_metadata", err)
		}
		if strings.Contains(err.Error(), credential) {
			t.Fatalf("error leaked the credential: %v", err)
		}
	})

	t.Run("quality gate can produce an objective artifact", func(t *testing.T) {
		review := schema.StepSpec{ID: "review", Kind: schema.StepKindQualityGate, DependsOn: []string{"draft"}, Outputs: []string{"result"},
			Gate: &schema.GateSpec{Target: "draft.text", Rules: []schema.GateRuleSpec{{Kind: schema.RuleNonEmpty}}}}
		root, file := prepareDefinition(t, 1, "review.result", routerStep("draft"), review)
		r, err := Run(t.Context(), file, Dependencies{Store: openStore(t, root), RouterReady: true,
			Router: routerFunc(func(context.Context, RouterRequest) (RouterResult, error) { return RouterResult{Text: "draft"}, nil }),
		})
		if err != nil || r.State.Status != "succeeded" || len(r.State.Steps["review"].Artifacts) != 1 {
			t.Fatalf("gate deliverable = %+v, %v", r, err)
		}
	})
}
