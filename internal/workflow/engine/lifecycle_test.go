package engine

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/store"
)

func approvalStep(id string, optional bool, dependencies ...string) schema.StepSpec {
	return schema.StepSpec{
		ID: id, Kind: schema.StepKindHumanApproval, Optional: optional, DependsOn: dependencies,
		Approval: &schema.ApprovalStepSpec{Prompt: "Review the proposal"},
	}
}

func requireLifecycleState(t *testing.T, state *store.RunState, err error, want string) *store.RunState {
	t.Helper()
	if err != nil {
		t.Fatalf("want run status %q, got %v", want, err)
	}
	if state == nil || state.Status != want || state.RunID == "" || state.CheckpointDigest == "" {
		t.Fatalf("want run with ID, checkpoint and status %q, got %+v", want, state)
	}
	return state
}

func lifecycleStatus(t *testing.T, s *store.WorkflowStore, runID, want string) *store.RunState {
	t.Helper()
	view, err := Status(runID, s)
	return requireLifecycleState(t, view.State, err, want)
}

func requireCheckpointUnchanged(t *testing.T, s *store.WorkflowStore, initial *store.RunState) {
	t.Helper()
	state := lifecycleStatus(t, s, initial.RunID, initial.Status)
	if state.CheckpointDigest != initial.CheckpointDigest || state.StateRevision != initial.StateRevision {
		t.Fatal("read or rejected operation changed the checkpoint")
	}
}

type lifecycleCalls struct {
	mu    sync.Mutex
	calls []string
}

func (c *lifecycleCalls) add(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, name)
}

func (c *lifecycleCalls) require(t *testing.T, want ...string) {
	t.Helper()
	c.mu.Lock()
	got := slices.Clone(c.calls)
	c.mu.Unlock()
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("provider calls = %v; want %v", got, want)
	}
}

func finishLifecycleCall(ctx context.Context, ready <-chan struct{}) error {
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestWorkflowEngineLifecycle(t *testing.T) {
	t.Run("status reads the last checkpoint", testWorkflowReadOnlyStatus)
	t.Run("approval stops the parallel frontier and resumes once", testWorkflowApprovalBarrier)
	t.Run("approval is bound to the current run, gate and definition", testWorkflowApprovalBinding)
	t.Run("reject stops the required run or only the optional branch", testWorkflowRejection)
	t.Run("interrupt pauses and preserves completed steps", testWorkflowInterruption)
	t.Run("recoverable failure reruns only unfinished steps", testWorkflowFailedRecovery)
	t.Run("manual resume starts a fresh bounded correction cycle", testWorkflowCorrectionRecovery)
	t.Run("each manual resume grants exactly one more bounded cycle", testWorkflowCorrectionExhaustion)
	t.Run("explicit secrets never reach the persisted decision trail", testWorkflowSecretRedaction)
	t.Run("an undecided optional gate is skipped, never failed", testWorkflowOptionalGateSkip)
	t.Run("a run left running by a dead process is recoverable", testWorkflowCrashRecovery)
	t.Run("cancel a running workflow cooperatively", testWorkflowActiveCancellation)
	t.Run("cancel an idle workflow without replay", testWorkflowIdleCancellation)
	t.Run("a blocking gate rejection explains the failed run", testWorkflowFailureDiagnostic)
	t.Run("cancel a failed run without depending on lock contention", testWorkflowCancelFailedRun)
	t.Run("approve reports the persisted state when the decision artifact fails", testWorkflowApprovalPersistedState)
	t.Run("the approval decision reaches the event log", testWorkflowApprovalEvent)
	t.Run("a rejected approval artifact retains the gate evaluation", func(t *testing.T) {
		approval := approvalStep("approval", false)
		approval.Outputs = []string{"decision"}
		gate := schema.StepSpec{ID: "verify", Kind: schema.StepKindQualityGate, DependsOn: []string{"approval"},
			Gate: &schema.GateSpec{Target: "approval.decision", Rules: []schema.GateRuleSpec{{Kind: schema.RuleContains, Value: "different"}}}}
		root, file := prepareDefinition(t, 1, "approval.decision", approval, gate)
		st := openStore(t, root)
		deps := Dependencies{Store: st}
		r, err := Run(t.Context(), file, deps)
		initial := requireLifecycleState(t, r.State, err, "awaiting_approval")
		if _, err := Approve(initial.RunID, "approval", initial.CheckpointDigest, "approve", "", st); err != nil {
			t.Fatal(err)
		}
		r, err = Resume(t.Context(), initial.RunID, deps)
		final := requireLifecycleState(t, r.State, err, "failed")
		if final.Steps["verify"].Status != "failed" {
			t.Fatalf("gate state = %+v", final.Steps["verify"])
		}
		events, err := st.ReadEvents(initial.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.ContainsFunc(events, func(event store.RunEvent) bool {
			return event.StepID == "verify" && event.EventType == "gate_evaluated" && event.Status == "fail"
		}) {
			t.Fatal("gate evaluation missing from event log")
		}
	})
}

func testWorkflowReadOnlyStatus(t *testing.T) {
	root, definition := prepareDefinition(t, 1, "proposal.text", routerStep("proposal"))
	s := openStore(t, root)
	r, err := Run(t.Context(), definition, Dependencies{Store: s, RouterReady: true,
		Router: routerFunc(func(context.Context, RouterRequest) (RouterResult, error) {
			return RouterResult{Text: "proposal evidence"}, nil
		}),
	})
	state := requireLifecycleState(t, r.State, err, "succeeded")
	view := lifecycleStatus(t, s, state.RunID, "succeeded")
	if got := view.Steps["proposal"].Artifacts; len(got) != 1 || got[0].RelPath == "" ||
		got[0].Summary != "proposal evidence" {
		t.Fatalf("status lost artifact location or summary: %+v", got)
	}
	requireCheckpointUnchanged(t, s, state)
}

func testWorkflowApprovalBarrier(t *testing.T) {
	root, definition := prepareDefinition(t, 2, "proposal.text",
		routerStep("setup"), routerStep("inflight"), routerStep("early", "setup"),
		approvalStep("review-one", false, "setup"), approvalStep("review-two", false, "setup"),
		routerStep("proposal", "early", "inflight", "review-one", "review-two"))
	s := openStore(t, root)
	ctx := contextForTest(t)
	inFlight := make(chan struct{})
	setupReady := make(chan struct{})
	var calls lifecycleCalls
	deps := Dependencies{Store: s, RouterReady: true, Router: routerFunc(func(ctx context.Context, req RouterRequest) (RouterResult, error) {
		calls.add(req.Prompt)
		switch req.Prompt {
		case "setup":
			if err := finishLifecycleCall(ctx, inFlight); err != nil {
				return RouterResult{}, err
			}
			close(setupReady)
		case "inflight":
			close(inFlight)
			if err := finishLifecycleCall(ctx, setupReady); err != nil {
				return RouterResult{}, err
			}
		}
		return RouterResult{Text: "evidence for " + req.Prompt}, nil
	})}
	r, err := Run(ctx, definition, deps)
	waiting := requireLifecycleState(t, r.State, err, "awaiting_approval")
	if waiting.Steps["inflight"].Status != "passed" || waiting.Steps["early"].Status != "pending" ||
		len(waiting.PendingGates) != 2 || !slices.Contains(waiting.PendingGates, "review-one") ||
		!slices.Contains(waiting.PendingGates, "review-two") {
		t.Fatalf("approval did not drain only the active work and collect the pending gates: %+v", waiting)
	}
	calls.require(t, "inflight", "setup")
	requireCheckpointUnchanged(t, s, waiting)
	for _, gateID := range waiting.PendingGates {
		if waiting.Gates[gateID].Status != "needs_human_review" {
			t.Fatalf("pending gate %q is not marked needs_human_review: %+v", gateID, waiting.Gates[gateID])
		}
	}

	a, err := Approve(waiting.RunID, "review-one", waiting.CheckpointDigest, "approve", "", s)
	one := requireLifecycleState(t, a.State, err, "awaiting_approval")
	if one.CheckpointDigest == waiting.CheckpointDigest {
		t.Fatal("approval did not advance the checkpoint")
	}
	if one.Gates["review-one"].Status != "pass" || one.Steps["review-one"].Status != "passed" {
		t.Fatalf("approved gate was not recorded as passed: %+v", one.Gates["review-one"])
	}
	if _, err := Approve(waiting.RunID, "review-one", one.CheckpointDigest, "approve", "", s); err == nil {
		t.Fatal("closed gate accepted a second approval")
	}
	if _, err := Approve(waiting.RunID, "review-two", waiting.CheckpointDigest, "approve", "", s); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale approval error = %v; want conflict", err)
	}
	if _, err := Resume(ctx, waiting.RunID, deps); !errors.Is(err, ErrPendingApproval) {
		t.Fatalf("resume with a second required gate = %v; want pending approval", err)
	}
	calls.require(t, "inflight", "setup")
	requireCheckpointUnchanged(t, s, one)

	a, err = Approve(waiting.RunID, "review-two", one.CheckpointDigest, "approve", "evidence reviewed", s)
	if err != nil || a.State == nil || len(a.State.PendingGates) != 0 || len(a.State.Approvals) != 2 {
		t.Fatalf("second approval = %+v, %v", a, err)
	}
	calls.require(t, "inflight", "setup")
	resumed, err := Resume(ctx, waiting.RunID, deps)
	active := requireLifecycleState(t, resumed.State, err, "succeeded")
	if active.RunID != waiting.RunID || active.ObjectiveVerdict == nil || !slices.Contains(active.ObjectiveVerdict.MetCriteria, "nonempty") {
		t.Fatalf("resume lost run identity or objective evidence: %+v", active)
	}
	calls.require(t, "early", "inflight", "proposal", "setup")
	requireTerminalResume(t, deps, active)
	calls.require(t, "early", "inflight", "proposal", "setup")
}

func testWorkflowApprovalBinding(t *testing.T) {
	root, definition := prepareDefinition(t, 1, "proposal.text",
		approvalStep("review", false), routerStep("proposal", "review"))
	s := openStore(t, root)
	var calls lifecycleCalls
	deps := Dependencies{Store: s, RouterReady: true, Router: routerFunc(func(_ context.Context, req RouterRequest) (RouterResult, error) {
		calls.add(req.Prompt)
		return RouterResult{Text: "proposal"}, nil
	})}
	r, err := Run(t.Context(), definition, deps)
	first := requireLifecycleState(t, r.State, err, "awaiting_approval")
	r, err = Run(t.Context(), definition, deps)
	second := requireLifecycleState(t, r.State, err, "awaiting_approval")
	t.Run("competing writer", func(t *testing.T) {
		lock, err := s.AcquireExclusive(first.RunID)
		if err != nil {
			t.Fatalf("could not hold the run lock: %v", err)
		}
		defer lock.Close()
		if _, err := Approve(first.RunID, "review", first.CheckpointDigest, "approve", "", s); !errors.Is(err, ErrConflict) {
			t.Fatalf("concurrent approval error = %v; want lock conflict", err)
		}
		requireCheckpointUnchanged(t, s, first)
	})
	for _, tc := range []struct {
		name, gate, digest, decision, reason string
		want                                 error
	}{
		{"foreign checkpoint", "review", second.CheckpointDigest, "approve", "", ErrConflict},
		{"missing checkpoint", "review", "", "approve", "", ErrInvalidApproval},
		{"missing rejection reason", "review", first.CheckpointDigest, "reject", " \t ", ErrInvalidApproval},
		{"unknown gate", "not-a-gate", first.CheckpointDigest, "approve", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Approve(first.RunID, tc.gate, tc.digest, tc.decision, tc.reason, s); err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("approval error = %v; want rejection matching %v", err, tc.want)
			}
			requireCheckpointUnchanged(t, s, first)
		})
	}

	raw, err := os.ReadFile(definition)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(raw), "Produce a proposal", "Produce a changed proposal", 1)
	if changed == string(raw) {
		t.Fatal("fixture did not change")
	}
	if err := os.WriteFile(definition, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Approve(first.RunID, "review", first.CheckpointDigest, "approve", "", s); !errors.Is(err, ErrConflict) {
		t.Fatalf("approval for changed definition = %v; want conflict", err)
	}
	requireCheckpointUnchanged(t, s, first)
	if _, err := Resume(t.Context(), first.RunID, deps); !errors.Is(err, ErrConflict) {
		t.Fatalf("resume for changed definition = %v; want conflict", err)
	}
	requireCheckpointUnchanged(t, s, first)
	calls.require(t)
}

func testWorkflowRejection(t *testing.T) {
	for _, tc := range []struct {
		name     string
		optional bool
	}{
		{"required", false}, {"optional", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addendum := routerStep("addendum", "review")
			addendum.Optional = tc.optional
			root, definition := prepareDefinition(t, 1, "proposal.text",
				approvalStep("review", tc.optional), addendum, routerStep("proposal"))
			s := openStore(t, root)
			var calls lifecycleCalls
			deps := Dependencies{Store: s, RouterReady: true, Router: routerFunc(func(_ context.Context, req RouterRequest) (RouterResult, error) {
				calls.add(req.Prompt)
				return RouterResult{Text: "main proposal"}, nil
			})}
			r, err := Run(t.Context(), definition, deps)
			waiting := requireLifecycleState(t, r.State, err, "awaiting_approval")
			a, err := Approve(waiting.RunID, "review", waiting.CheckpointDigest, "reject", "Needs a revision", s)
			if err != nil || a.State == nil || len(a.State.Approvals) != 1 {
				t.Fatalf("reject = %+v, %v", a, err)
			}
			if got := a.State.Approvals[0]; got.Decision != "reject" || got.Reason != "Needs a revision" ||
				got.CheckpointDigest != waiting.CheckpointDigest || got.DecidedAt == "" {
				t.Fatalf("rejection lost audit information: %+v", got)
			}
			var audit struct {
				Actor string `json:"decided_by"`
			}
			raw, err := json.Marshal(a.State.Approvals[0])
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &audit); err != nil || audit.Actor == "" {
				t.Fatalf("rejection has no recorded operator: %v", err)
			}
			if got := a.State.Gates["review"].Status; got != "blocked" {
				t.Fatalf("rejected gate = %q; want blocked even when its branch is optional", got)
			}
			if tc.optional {
				r, err = Resume(t.Context(), waiting.RunID, deps)
				done := requireLifecycleState(t, r.State, err, "partial")
				if done.Steps["addendum"].Status != "skipped" || done.Steps["proposal"].Status != "passed" {
					t.Fatalf("optional rejection did not isolate its branch: %+v", done.Steps)
				}
				calls.require(t, "proposal")
				requireTerminalResume(t, deps, done)
			} else {
				done := lifecycleStatus(t, s, waiting.RunID, "blocked")
				requireTerminalResume(t, deps, done)
			}
			if slices.Contains(a.State.PendingGates, "review") {
				t.Fatal("rejected gate remains pending")
			}
		})
	}
}

func requireTerminalResume(t *testing.T, deps Dependencies, expected *store.RunState) {
	t.Helper()
	r, err := Resume(t.Context(), expected.RunID, deps)
	if !errors.Is(err, ErrTerminalRun) || r.State == nil || r.State.Status != expected.Status {
		t.Fatalf("terminal resume of %q = %+v, %v", expected.Status, r, err)
	}
	requireCheckpointUnchanged(t, deps.Store, expected)
}

func testWorkflowInterruption(t *testing.T) {
	root, definition := prepareDefinition(t, 1, "proposal.text",
		routerStep("draft"), routerStep("proposal", "draft"))
	s := openStore(t, root)
	ctx, interrupt := context.WithCancel(contextForTest(t))
	defer interrupt()
	var calls lifecycleCalls
	deps := Dependencies{Store: s, RouterReady: true, Router: routerFunc(func(ctx context.Context, req RouterRequest) (RouterResult, error) {
		calls.add(req.Prompt)
		if req.Prompt == "proposal" {
			interrupt()
			return RouterResult{}, ctx.Err()
		}
		return RouterResult{Text: "saved draft"}, nil
	})}
	r, err := Run(ctx, definition, deps)
	paused := requireLifecycleState(t, r.State, err, "paused")
	draft := lifecycleStatus(t, s, paused.RunID, "paused").Steps["draft"]
	if draft.Status != "passed" || len(draft.Artifacts) != 1 {
		t.Fatalf("interruption lost completed evidence: %+v", draft)
	}
	deps.Router = routerFunc(func(_ context.Context, req RouterRequest) (RouterResult, error) {
		calls.add(req.Prompt)
		return RouterResult{Text: "finished proposal"}, nil
	})
	r, err = Resume(t.Context(), paused.RunID, deps)
	resumed := requireLifecycleState(t, r.State, err, "succeeded")
	if resumed.RunID != paused.RunID || !slices.Equal(resumed.Steps["draft"].Artifacts, draft.Artifacts) {
		t.Fatal("resume changed completed artifact evidence or run identity")
	}
	calls.require(t, "draft", "proposal", "proposal")
}

func testWorkflowFailedRecovery(t *testing.T) {
	root, definition := prepareDefinition(t, 1, "publish.text",
		routerStep("draft"), routerStep("proposal", "draft"), routerStep("publish", "proposal"))
	s := openStore(t, root)
	var calls lifecycleCalls
	deps := Dependencies{Store: s, RouterReady: true, Router: routerFunc(func(_ context.Context, req RouterRequest) (RouterResult, error) {
		calls.add(req.Prompt)
		if req.Prompt == "proposal" {
			return RouterResult{}, errors.New("temporary provider failure")
		}
		return RouterResult{Text: "saved draft"}, nil
	})}
	r, err := Run(t.Context(), definition, deps)
	failed := requireLifecycleState(t, r.State, err, "failed")
	if failed.Steps["publish"].Status != "skipped" || failed.Steps["draft"].Status != "passed" {
		t.Fatalf("failed checkpoint = %+v", failed.Steps)
	}
	deps.Router = routerFunc(func(_ context.Context, req RouterRequest) (RouterResult, error) {
		calls.add(req.Prompt)
		return RouterResult{Text: "publication"}, nil
	})
	r, err = Resume(t.Context(), failed.RunID, deps)
	complete := requireLifecycleState(t, r.State, err, "succeeded")
	if complete.RunID != failed.RunID || len(complete.Steps["proposal"].Attempts) < 2 {
		t.Fatal("resume lost run identity or attempt audit history")
	}
	calls.require(t, "draft", "proposal", "proposal", "publish")
	events, err := s.ReadEvents(failed.RunID)
	if err != nil {
		t.Fatal(err)
	}
	started, resumed := 0, 0
	for _, event := range events {
		switch event.EventType {
		case "run_started":
			started++
		case "run_resumed":
			resumed++
		}
	}
	if started != 1 || resumed != 1 {
		t.Fatalf("run lifecycle events: started=%d resumed=%d, want 1 and 1", started, resumed)
	}
}

func testWorkflowCorrectionRecovery(t *testing.T) {
	draft := routerStep("draft", "source")
	draft.Correction = &schema.CorrectionSpec{GateStepID: "review", MaxIterations: 2}
	review := schema.StepSpec{
		ID: "review", Kind: schema.StepKindQualityGate, DependsOn: []string{"draft"},
		Gate: &schema.GateSpec{Target: "draft.text", Rules: []schema.GateRuleSpec{
			{Kind: schema.RuleContains, Value: "complete"},
		}},
	}
	root, definition := prepareDefinition(t, 1, "publish.text",
		routerStep("source"), draft, review, routerStep("publish", "review"))
	s := openStore(t, root)
	var calls lifecycleCalls
	makeRouter := func(draftOutput string) routerFunc {
		return func(_ context.Context, req RouterRequest) (RouterResult, error) {
			if strings.HasPrefix(req.Prompt, "draft") {
				calls.add("draft")
				return RouterResult{Text: draftOutput}, nil
			}
			calls.add(req.Prompt)
			return RouterResult{Text: "final evidence"}, nil
		}
	}
	deps := Dependencies{Store: s, RouterReady: true, Router: makeRouter("needs revision")}
	r, err := Run(t.Context(), definition, deps)
	exhausted := requireLifecycleState(t, r.State, err, "failed")
	calls.require(t, "source", "draft", "draft")

	deps.Router = makeRouter("complete")
	r, err = Resume(t.Context(), exhausted.RunID, deps)
	done := requireLifecycleState(t, r.State, err, "succeeded")
	attempts := done.Steps["draft"].Attempts
	if done.RunID != exhausted.RunID || len(attempts) != 3 || attempts[0].Status != "failed" ||
		attempts[1].Status != "failed" || attempts[2].Status != "passed" {
		t.Fatalf("resume lost prior correction attempts or run identity: %+v", attempts)
	}
	calls.require(t, "source", "draft", "draft", "draft", "publish")
}

func testWorkflowCorrectionExhaustion(t *testing.T) {
	draft := routerStep("draft", "source")
	draft.Correction = &schema.CorrectionSpec{GateStepID: "review", MaxIterations: 2}
	review := schema.StepSpec{
		ID: "review", Kind: schema.StepKindQualityGate, DependsOn: []string{"draft"},
		Gate: &schema.GateSpec{Target: "draft.text", Rules: []schema.GateRuleSpec{
			{Kind: schema.RuleContains, Value: "complete"},
		}},
	}
	root, definition := prepareDefinition(t, 1, "publish.text",
		routerStep("source"), draft, review, routerStep("publish", "review"))
	s := openStore(t, root)
	var calls lifecycleCalls
	output := "needs revision"
	deps := Dependencies{Store: s, RouterReady: true, Router: routerFunc(func(_ context.Context, req RouterRequest) (RouterResult, error) {
		if strings.HasPrefix(req.Prompt, "draft") {
			calls.add("draft")
			return RouterResult{Text: output}, nil
		}
		calls.add(req.Prompt)
		return RouterResult{Text: "final evidence"}, nil
	})}
	r, err := Run(t.Context(), definition, deps)
	first := requireLifecycleState(t, r.State, err, "failed")
	if got := len(first.Steps["draft"].Attempts); got != 2 {
		t.Fatalf("first cycle attempts = %d; want 2", got)
	}

	r, err = Resume(t.Context(), first.RunID, deps)
	second := requireLifecycleState(t, r.State, err, "failed")
	if got := len(second.Steps["draft"].Attempts); got != 4 {
		t.Fatalf("second cycle attempts = %d; want 4 (a fresh budget of 2)", got)
	}

	output = "complete"
	r, err = Resume(t.Context(), first.RunID, deps)
	done := requireLifecycleState(t, r.State, err, "succeeded")
	attempts := done.Steps["draft"].Attempts
	if len(attempts) != 5 || attempts[4].Status != "passed" {
		t.Fatalf("third cycle did not close on its first attempt: %+v", attempts)
	}
	calls.require(t, "source", "draft", "draft", "draft", "draft", "draft", "publish")
}

func testWorkflowSecretRedaction(t *testing.T) {
	const secret = "only-fixture-known-credential"
	approvalWorkflow := func(t *testing.T) (string, string) {
		t.Helper()
		return prepareDefinition(t, 1, "proposal.text",
			routerStep("draft"), approvalStep("review", false, "draft"), routerStep("proposal", "review"))
	}
	working := routerFunc(func(_ context.Context, req RouterRequest) (RouterResult, error) {
		return RouterResult{Text: "evidence for " + req.Prompt}, nil
	})

	t.Run("run configures the store before an approval reason is persisted", func(t *testing.T) {
		root, definition := approvalWorkflow(t)
		s := openStore(t, root)
		deps := Dependencies{Store: s, RouterReady: true, Router: working, SecretValues: []string{secret}}
		r, err := Run(t.Context(), definition, deps)
		waiting := requireLifecycleState(t, r.State, err, "awaiting_approval")
		if _, err := Approve(waiting.RunID, "review", waiting.CheckpointDigest, "reject", "motivo "+secret, s); err != nil {
			t.Fatalf("approve: %v", err)
		}
		persisted, err := s.LoadRun(waiting.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if got := persisted.Approvals[0].Reason; strings.Contains(got, secret) {
			t.Fatalf("approval reason persisted the explicit secret: %q", got)
		}
	})

	t.Run("resume configures the store before a cancellation reason is persisted", func(t *testing.T) {
		root, definition := approvalWorkflow(t)
		first := openStore(t, root)
		failing := Dependencies{Store: first, RouterReady: true, SecretValues: []string{secret},
			Router: routerFunc(func(context.Context, RouterRequest) (RouterResult, error) {
				return RouterResult{}, errors.New("temporary provider failure")
			})}
		r, err := Run(t.Context(), definition, failing)
		failed := requireLifecycleState(t, r.State, err, "failed")

		second := openStore(t, root)
		deps := Dependencies{Store: second, RouterReady: true, Router: working, SecretValues: []string{secret}}
		r, err = Resume(t.Context(), failed.RunID, deps)
		waiting := requireLifecycleState(t, r.State, err, "awaiting_approval")
		result, err := Cancel(t.Context(), waiting.RunID, "motivo "+secret, second)
		done := requireLifecycleState(t, result.State, err, "cancelled")
		if strings.Contains(done.Error, secret) {
			t.Fatalf("cancellation reason persisted the explicit secret: %q", done.Error)
		}
	})
}

func testWorkflowOptionalGateSkip(t *testing.T) {
	root, definition := prepareDefinition(t, 1, "proposal.text",
		routerStep("draft"), approvalStep("review", true, "draft"), routerStep("proposal", "draft"))
	s := openStore(t, root)
	deps := Dependencies{Store: s, RouterReady: true, Router: routerFunc(func(_ context.Context, req RouterRequest) (RouterResult, error) {
		return RouterResult{Text: "evidence for " + req.Prompt}, nil
	})}
	r, err := Run(t.Context(), definition, deps)
	waiting := requireLifecycleState(t, r.State, err, "awaiting_approval")
	if got := waiting.Gates["review"].Status; got != "needs_human_review" {
		t.Fatalf("pending optional gate = %q; want needs_human_review", got)
	}
	r, err = Resume(t.Context(), waiting.RunID, deps)
	done := requireLifecycleState(t, r.State, err, "partial")
	if got := done.Steps["review"].Status; got != "skipped" {
		t.Fatalf("undecided optional gate step = %q; want skipped", got)
	}
	if got := done.Gates["review"].Status; got != "skipped" {
		t.Fatalf("undecided optional gate recorded as %q; want skipped", got)
	}
}

func testWorkflowCrashRecovery(t *testing.T) {
	root, definition := prepareDefinition(t, 1, "proposal.text",
		routerStep("draft"), routerStep("proposal", "draft"))
	s := openStore(t, root)
	def, err := schema.LoadAndValidate(definition, schema.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	runID, state, err := s.CreateRun(def)
	if err != nil {
		t.Fatal(err)
	}
	// A process that died mid-step leaves the checkpoint in "running" while the
	// OS has already released run.lock, so nothing else can advance the run.
	state.Status = "running"
	draft := state.Steps["draft"]
	draft.Status, draft.Iteration = "running", 1
	draft.Attempts = []store.AttemptRecord{{Iteration: 1, Status: "running", StartedAt: now()}}
	state.Steps["draft"] = draft
	if _, err := s.WriteCheckpoint(runID, state); err != nil {
		t.Fatal(err)
	}

	var calls lifecycleCalls
	deps := Dependencies{Store: s, RouterReady: true, Router: routerFunc(func(_ context.Context, req RouterRequest) (RouterResult, error) {
		calls.add(req.Prompt)
		return RouterResult{Text: "evidence for " + req.Prompt}, nil
	})}
	r, err := Resume(t.Context(), runID, deps)
	done := requireLifecycleState(t, r.State, err, "succeeded")
	attempts := done.Steps["draft"].Attempts
	if done.RunID != runID || len(attempts) != 2 || attempts[0].Status != "cancelled" || attempts[1].Status != "passed" {
		t.Fatalf("crash recovery lost run identity or the abandoned attempt: %+v", attempts)
	}
	calls.require(t, "draft", "proposal")
}

func testWorkflowActiveCancellation(t *testing.T) {
	t.Run("wait for terminal checkpoint", func(t *testing.T) {
		runActiveCancellation(t, true)
	})
	t.Run("request survives an interrupted wait", func(t *testing.T) {
		runActiveCancellation(t, false)
	})
}

func runActiveCancellation(t *testing.T, waitForCheckpoint bool) {
	ctx, stop := context.WithCancel(contextForTest(t))
	defer stop()
	root, definition := prepareDefinition(t, 1, "publish.text",
		routerStep("draft"), routerStep("hold", "draft"), routerStep("publish", "hold"))
	s := openStore(t, root)
	started := make(chan struct{})
	executorStopped := make(chan struct{})
	drained := make(chan struct{})
	var closeDrain sync.Once
	allowDrain := func() { closeDrain.Do(func() { close(drained) }) }
	defer allowDrain()
	var calls lifecycleCalls
	deps := Dependencies{Store: s, RouterReady: true, Router: routerFunc(func(ctx context.Context, req RouterRequest) (RouterResult, error) {
		calls.add(req.Prompt)
		if req.Prompt == "hold" {
			close(started)
			<-ctx.Done()
			close(executorStopped)
			<-drained
			return RouterResult{}, ctx.Err()
		}
		return RouterResult{Text: "evidence"}, nil
	})}
	type runResult struct {
		state *store.RunState
		err   error
	}
	runs := make(chan runResult, 1)
	go func() {
		r, err := Run(ctx, definition, deps)
		runs <- runResult{state: r.State, err: err}
	}()
	select {
	case <-started:
	case r := <-runs:
		t.Fatalf("run ended before entering its in-flight step: %+v, %v", r.state, r.err)
	case <-ctx.Done():
		t.Fatal("run never entered its in-flight step")
	}
	dirs, err := os.ReadDir(filepath.Join(root, ".gandalf", "workflows", "runs"))
	if err != nil || len(dirs) != 1 {
		t.Fatalf("active run missing in the store: %v, %v", dirs, err)
	}
	id := dirs[0].Name()
	if _, err := Resume(ctx, id, deps); !errors.Is(err, ErrConflict) {
		t.Fatalf("concurrent resume = %v; want lock conflict", err)
	}
	lifecycleStatus(t, s, id, "running")
	type cancellationResult struct {
		result CancelResult
		err    error
	}
	cancelCtx, stopWaiting := context.WithCancel(ctx)
	defer stopWaiting()
	cancels := make(chan cancellationResult, 1)
	go func() {
		r, err := Cancel(cancelCtx, id, "operator stopped work", s)
		cancels <- cancellationResult{result: r, err: err}
	}()
	select {
	case <-executorStopped:
	case <-ctx.Done():
		t.Fatal("the cooperative request did not cancel the active provider context")
	}
	if !waitForCheckpoint {
		stopWaiting()
		select {
		case r := <-cancels:
			if !errors.Is(r.err, context.Canceled) || !r.result.Requested ||
				r.result.State == nil || r.result.State.Status != "running" {
				t.Fatalf("interrupted wait = %+v, %v; want acknowledged request without terminal confirmation", r.result, r.err)
			}
		case <-ctx.Done():
			t.Fatal("cancel did not stop waiting when its context ended")
		}
	}
	allowDrain()

	select {
	case r := <-runs:
		requireLifecycleState(t, r.state, r.err, "cancelled")
	case <-ctx.Done():
		t.Fatal("run did not stop after the cancellation request")
	}
	if waitForCheckpoint {
		select {
		case r := <-cancels:
			requireLifecycleState(t, r.result.State, r.err, "cancelled")
		case <-ctx.Done():
			t.Fatal("cancel did not confirm the terminal checkpoint")
		}
	}
	done := lifecycleStatus(t, s, id, "cancelled")
	if got := done.Steps["draft"]; got.Status != "passed" || len(got.Artifacts) != 1 {
		t.Fatalf("cancel lost completed evidence: %+v", got)
	}
	calls.require(t, "draft", "hold")
	requireTerminalResume(t, deps, done)
}

func testWorkflowIdleCancellation(t *testing.T) {
	for _, status := range []string{"awaiting_approval", "paused"} {
		t.Run(status, func(t *testing.T) {
			second := approvalStep("pause", false, "draft")
			if status == "paused" {
				second = routerStep("pause", "draft")
			}
			root, definition := prepareDefinition(t, 1, "proposal.text",
				routerStep("draft"), second, routerStep("proposal", "pause"))
			s := openStore(t, root)
			ctx, interrupt := context.WithCancel(contextForTest(t))
			defer interrupt()
			var calls lifecycleCalls
			deps := Dependencies{Store: s, RouterReady: true, Router: routerFunc(func(ctx context.Context, req RouterRequest) (RouterResult, error) {
				calls.add(req.Prompt)
				if req.Prompt == "pause" {
					interrupt()
					return RouterResult{}, ctx.Err()
				}
				return RouterResult{Text: "draft evidence"}, nil
			})}
			r, err := Run(ctx, definition, deps)
			waiting := requireLifecycleState(t, r.State, err, status)
			result, err := Cancel(t.Context(), waiting.RunID, "no longer needed", s)
			done := requireLifecycleState(t, result.State, err, "cancelled")
			persisted := lifecycleStatus(t, s, done.RunID, "cancelled")
			if done.RunID != waiting.RunID || done.CheckpointDigest != persisted.CheckpointDigest ||
				persisted.Steps["draft"].Status != "passed" || len(persisted.Steps["draft"].Artifacts) != 1 {
				t.Fatalf("cancellation changed run identity or lost evidence: %+v", persisted)
			}
			requireTerminalResume(t, deps, persisted)
			if status == "paused" {
				calls.require(t, "draft", "pause")
				return
			}
			calls.require(t, "draft")
		})
	}
}

func testWorkflowFailureDiagnostic(t *testing.T) {
	review := schema.StepSpec{ID: "review", Kind: schema.StepKindQualityGate, DependsOn: []string{"draft"},
		Gate: &schema.GateSpec{Target: "draft.text", Rules: []schema.GateRuleSpec{{Kind: schema.RuleContains, Value: "approved"}}}}
	root, definition := prepareDefinition(t, 1, "proposal.text",
		routerStep("draft"), review, routerStep("proposal", "draft"))
	s := openStore(t, root)
	var calls lifecycleCalls
	deps := Dependencies{Store: s, RouterReady: true, Router: routerFunc(func(context.Context, RouterRequest) (RouterResult, error) {
		calls.add("draft")
		return RouterResult{Text: "incomplete"}, nil
	})}
	r, err := Run(t.Context(), definition, deps)
	failed := requireLifecycleState(t, r.State, err, "failed")
	if !strings.Contains(failed.Error, "review") {
		t.Fatalf("failed run error = %q; want the blocking gate named", failed.Error)
	}
	calls.require(t, "draft")
	again, err := Resume(t.Context(), failed.RunID, deps)
	resumed := requireLifecycleState(t, again.State, err, "failed")
	if resumed.Error != failed.Error {
		t.Fatalf("resume changed the diagnostic: %q vs %q", resumed.Error, failed.Error)
	}
	calls.require(t, "draft")
}

func testWorkflowCancelFailedRun(t *testing.T) {
	root, definition := prepareDefinition(t, 1, "draft.text", routerStep("draft"))
	s := openStore(t, root)
	r, err := Run(t.Context(), definition, Dependencies{Store: s, RouterReady: true,
		Router: routerFunc(func(context.Context, RouterRequest) (RouterResult, error) {
			return RouterResult{}, errors.New("provider down")
		})})
	failed := requireLifecycleState(t, r.State, err, "failed")
	lock, err := s.AcquireExclusive(failed.RunID)
	if err != nil {
		t.Fatalf("test could not hold the run lock: %v", err)
	}
	type outcome struct {
		result CancelResult
		err    error
	}
	cancels := make(chan outcome, 1)
	go func() {
		result, err := Cancel(t.Context(), failed.RunID, "discard", s)
		cancels <- outcome{result: result, err: err}
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		requested, _, requestErr := s.HasCancelRequest(failed.RunID)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		if requested {
			break
		}
		select {
		case got := <-cancels:
			t.Fatalf("cancel gave up while the lock was held: %v", got.err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("cancel did not write a cooperative request")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	got := <-cancels
	done := requireLifecycleState(t, got.result.State, got.err, "cancelled")
	if done.RunID != failed.RunID || done.Steps["draft"].Status != "failed" {
		t.Fatalf("cancellation changed run identity or lost the failure evidence: %+v", done.Steps["draft"])
	}
}

func testWorkflowApprovalPersistedState(t *testing.T) {
	review := approvalStep("review", false, "draft")
	review.Outputs = []string{"decision"}
	root, definition := prepareDefinition(t, 1, "review.decision", routerStep("draft"), review)
	s := openStore(t, root)
	r, err := Run(t.Context(), definition, Dependencies{Store: s, RouterReady: true,
		Router: routerFunc(func(context.Context, RouterRequest) (RouterResult, error) {
			return RouterResult{Text: "draft"}, nil
		})})
	waiting := requireLifecycleState(t, r.State, err, "awaiting_approval")
	artifacts := filepath.Join(root, ".gandalf", "workflows", "runs", waiting.RunID, "artifacts")
	if err := os.RemoveAll(artifacts); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifacts, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Approve(waiting.RunID, "review", waiting.CheckpointDigest, "approve", "ok", s)
	if err == nil {
		t.Fatal("approve accepted an unwritable decision artifact")
	}
	if result.State == nil || result.State.Steps["review"].Status != "awaiting_approval" || len(result.State.PendingGates) != 1 {
		t.Fatalf("approve returned a decision that was never persisted: %+v", result.State)
	}
	requireCheckpointUnchanged(t, s, waiting)
}

func testWorkflowApprovalEvent(t *testing.T) {
	root, definition := prepareDefinition(t, 1, "draft.text",
		routerStep("draft"), approvalStep("review", false, "draft"))
	s := openStore(t, root)
	r, err := Run(t.Context(), definition, Dependencies{Store: s, RouterReady: true,
		Router: routerFunc(func(context.Context, RouterRequest) (RouterResult, error) {
			return RouterResult{Text: "draft"}, nil
		})})
	waiting := requireLifecycleState(t, r.State, err, "awaiting_approval")
	if _, err := Approve(waiting.RunID, "review", waiting.CheckpointDigest, "reject", "not ready", s); err != nil {
		t.Fatal(err)
	}
	events, err := s.ReadEvents(waiting.RunID)
	if err != nil {
		t.Fatal(err)
	}
	decided := false
	for _, event := range events {
		if event.EventType != "approval_decided" {
			continue
		}
		decided = true
		if event.Message != "decision=reject" {
			t.Fatalf("approval event message = %q; want decision=reject", event.Message)
		}
	}
	if !decided {
		t.Fatal("no approval decision event recorded")
	}
}
