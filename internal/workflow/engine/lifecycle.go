package engine

import (
	"context"
	"errors"
	"fmt"
	"os/user"
	"slices"
	"strings"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/gates"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/store"
)

var (
	ErrConflict        = errors.New("workflow state conflict")
	ErrTerminalRun     = errors.New("workflow run is terminal")
	ErrInvalidApproval = errors.New("invalid workflow approval")
	ErrPendingApproval = errors.New("mandatory workflow approval pending")
)

const cancelPollInterval = 50 * time.Millisecond
const cancelWaitLimit = 3 * time.Second

type StatusView struct{ State *store.RunState }
type ApproveResult struct{ State *store.RunState }
type CancelResult struct {
	State     *store.RunState
	Requested bool
}

func Status(runID string, st *store.WorkflowStore) (StatusView, error) {
	state, err := st.LoadRun(runID)
	return StatusView{State: state}, err
}

func Approve(runID, gateID, checkpointDigest, decision, reason string, st *store.WorkflowStore) (ApproveResult, error) {
	if checkpointDigest == "" || (decision != "approve" && decision != "reject") ||
		(decision == "reject" && strings.TrimSpace(reason) == "") {
		return ApproveResult{}, ErrInvalidApproval
	}
	lock, err := acquireRun(st, runID)
	if err != nil {
		return ApproveResult{}, err
	}
	defer lock.Close()
	state, err := st.LoadRun(runID)
	if err != nil {
		return ApproveResult{}, err
	}
	result := ApproveResult{State: state}
	slot := slices.Index(state.PendingGates, gateID)
	step := state.Steps[gateID]
	if state.Status != "awaiting_approval" || slot < 0 || step.Status != "awaiting_approval" ||
		state.CheckpointDigest != checkpointDigest {
		return result, fmt.Errorf("%w: approval does not match an open gate and current checkpoint", ErrConflict)
	}
	def, err := unchangedDefinition(state)
	if err != nil {
		return result, err
	}
	pendingCancel, _, err := st.HasCancelRequest(runID)
	if err != nil {
		return result, err
	}
	if pendingCancel {
		return result, fmt.Errorf("%w: cancellation pending", ErrConflict)
	}

	record := state.Gates[gateID]
	record.StepID, record.CheckedAt = gateID, now()
	step.Status, record.Status = "passed", gates.GateStatusPass
	state.PendingGates = slices.Delete(state.PendingGates, slot, slot+1)
	if decision == "reject" {
		step.Status, record.Status = "skipped", gates.GateStatusBlocked
		if !step.Optional {
			step.Status, record.Status = "failed", gates.GateStatusBlocked
			state.Status = "blocked"
			markRemaining(state, "skipped")
			state.ObjectiveVerdict = &store.ObjectiveVerdictRecord{Status: "blocked"}
		}
	}
	if state.Gates == nil {
		state.Gates = make(map[string]store.GateRecord)
	}
	state.Steps[gateID], state.Gates[gateID] = step, record
	state.Approvals = append(state.Approvals, store.ApprovalRecord{
		GateID: gateID, Decision: decision, Reason: reason, CheckpointDigest: checkpointDigest,
		DecidedAt: record.CheckedAt, DecidedBy: localOperator(),
	})
	if decision == "approve" {
		e := newExecution(def, Dependencies{Store: st}, state)
		spec := e.steps[gateID]
		if len(spec.Outputs) > 0 {
			e.evaluateGates(gateID, decision)
			refs, err := e.saveOutputs(spec, step.Iteration, decision, false)
			if err != nil {
				result.State = persistedStateOr(st, runID, result.State)
				return result, err
			}
			step.Artifacts = refs
			state.Steps[gateID] = step
		}
	}
	if err := persistEvent(st, state, "approval_decided", gateID, "", "decision="+decision); err != nil {
		result.State = persistedStateOr(st, runID, result.State)
		return result, err
	}
	return result, nil
}

// persistedStateOr returns the checkpoint on disk, falling back to the in-memory
// state only when the store cannot be read: a failed operation must never hand
// back a decision that was not persisted.
func persistedStateOr(st *store.WorkflowStore, runID string, fallback *store.RunState) *store.RunState {
	if latest, err := st.LoadRun(runID); err == nil {
		return latest
	}
	return fallback
}

func Resume(ctx context.Context, runID string, deps Dependencies) (RunSummary, error) {
	// The store redacts on write; it needs the explicit values before the first
	// checkpoint so operator-supplied text is covered too.
	deps.Store.SetSecretValues(deps.SecretValues)
	lock, err := acquireRun(deps.Store, runID)
	if err != nil {
		return RunSummary{}, err
	}
	defer lock.Close()
	state, err := deps.Store.LoadRun(runID)
	if err != nil {
		return RunSummary{}, err
	}
	result := RunSummary{State: state}
	if terminal(state.Status) {
		return result, ErrTerminalRun
	}
	requested, reason, err := deps.Store.HasCancelRequest(runID)
	if err != nil {
		return result, err
	}
	if requested {
		_, err := cancelLocked(deps.Store, state, reason)
		return result, err
	}
	def, err := unchangedDefinition(state)
	if err != nil {
		return result, err
	}
	for _, id := range state.PendingGates {
		if !state.Steps[id].Optional {
			return result, ErrPendingApproval
		}
	}
	remaining := def
	remaining.RequiresMemory, remaining.RequiresRouter = false, false
	remaining.Definition.Steps = nil
	for _, spec := range def.Definition.Steps {
		status := state.Steps[spec.ID].Status
		if status == "passed" || status == "cancelled" {
			continue
		}
		remaining.RequiresMemory = remaining.RequiresMemory || spec.Kind == schema.StepKindMemoryQuery
		remaining.RequiresRouter = remaining.RequiresRouter || spec.Kind == schema.StepKindRouterTask
		remaining.Definition.Steps = append(remaining.Definition.Steps, spec)
	}
	if err := checkProviders(remaining, deps); err != nil {
		return result, err
	}
	if err := checkMetadata(def, deps.SecretValues); err != nil {
		return result, err
	}
	if err := checkRouterEligibility(ctx, remaining.Definition.Steps, deps.Router); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	for _, id := range state.PendingGates {
		step := state.Steps[id]
		step.Status = "skipped"
		state.Steps[id] = step
		record := state.Gates[id]
		record.Status = gates.GateStatusSkipped
		state.Gates[id] = record
	}
	state.PendingGates = nil
	for _, spec := range def.Definition.Steps {
		step := state.Steps[spec.ID]
		switch step.Status {
		case "running":
			if n := len(step.Attempts); n > 0 {
				attempt := &step.Attempts[n-1]
				attempt.Status, attempt.EndedAt = "cancelled", now()
			}
			step.Status = "pending"
		case "skipped":
			// Dependency failures can be recovered. A skipped human gate
			// records a branch decision and remains skipped.
			if gate := state.Gates[spec.ID]; spec.Approval == nil || gate.Status == "" {
				step.Status = "pending"
			}
		case "failed":
			// A failed quality gate is evidence, not a retryable step.
			if spec.Kind != schema.StepKindQualityGate {
				// A manual resume opens a fresh bounded correction cycle when the
				// previous one exhausted max_iterations, keeping every attempt.
				if spec.Correction != nil && correctionAttempts(step) >= spec.Correction.MaxIterations {
					step.CorrectionCycleStart = len(step.Attempts)
				}
				step.Status = "pending"
			}
		}
		state.Steps[spec.ID] = step
	}
	for _, spec := range def.Definition.Steps {
		if spec.Gate == nil {
			continue
		}
		source, _, _ := strings.Cut(spec.Gate.Target, ".")
		step := state.Steps[spec.ID]
		if state.Steps[source].Status == "pending" && step.Status == "failed" {
			step.Status = "pending"
			state.Steps[spec.ID] = step
			delete(state.Gates, spec.ID)
		}
	}
	state.ObjectiveVerdict, state.Error = nil, ""
	return newExecution(def, deps, state).run(ctx)
}

func Cancel(ctx context.Context, runID, reason string, st *store.WorkflowStore) (CancelResult, error) {
	if err := ctx.Err(); err != nil {
		return CancelResult{}, err
	}
	lock, err := acquireRun(st, runID)
	if err == nil {
		defer lock.Close()
		state, err := st.LoadRun(runID)
		if err != nil {
			return CancelResult{}, err
		}
		return cancelLocked(st, state, reason)
	}
	if !errors.Is(err, store.ErrRunBusy) {
		return CancelResult{}, err
	}
	state, err := st.LoadRun(runID)
	if err != nil {
		return CancelResult{}, err
	}
	if terminal(state.Status) {
		return terminalCancelResult(state)
	}
	if err := st.WriteCancelRequest(runID, reason); err != nil {
		return CancelResult{State: state}, err
	}
	return waitForCancellation(ctx, st, state, reason)
}

func waitForCancellation(ctx context.Context, st *store.WorkflowStore, state *store.RunState, reason string) (CancelResult, error) {
	ctx, cancel := context.WithTimeout(ctx, cancelWaitLimit)
	defer cancel()
	ticker := time.NewTicker(cancelPollInterval)
	defer ticker.Stop()
	runID := state.RunID
	result := CancelResult{State: state, Requested: true}
	for {
		select {
		case <-ctx.Done():
			return result, fmt.Errorf("cancellation requested, confirmation pending: %w", ctx.Err())
		case <-ticker.C:
			latest, err := st.LoadRun(runID)
			if err != nil {
				return result, err
			}
			result.State = latest
			if terminal(latest.Status) {
				finished, err := terminalCancelResult(latest)
				finished.Requested = true
				return finished, err
			}
			lock, err := acquireRun(st, runID)
			if errors.Is(err, store.ErrRunBusy) {
				continue
			}
			if err != nil {
				return result, err
			}
			defer lock.Close()
			latest, err = st.LoadRun(runID)
			if err != nil {
				return result, err
			}
			finished, err := cancelLocked(st, latest, reason)
			finished.Requested = true
			return finished, err
		}
	}
}

func acquireRun(st *store.WorkflowStore, runID string) (*store.RunLock, error) {
	lock, err := st.AcquireExclusive(runID)
	if errors.Is(err, store.ErrRunBusy) {
		return nil, fmt.Errorf("%w: %w", ErrConflict, err)
	}
	return lock, err
}

func unchangedDefinition(state *store.RunState) (schema.ValidatedWorkflow, error) {
	def, err := schema.LoadAndValidate(state.DefinitionPath, schema.ValidateOptions{})
	if err != nil || def.DefinitionSHA256 != state.DefinitionSHA256 {
		return schema.ValidatedWorkflow{}, fmt.Errorf("%w: workflow definition changed or is unavailable", ErrConflict)
	}
	return def, nil
}

func terminal(status string) bool {
	switch status {
	case "succeeded", "partial", "blocked", "cancelled":
		return true
	default:
		return false
	}
}

func terminalCancelResult(state *store.RunState) (CancelResult, error) {
	if state.Status == "cancelled" {
		return CancelResult{State: state}, nil
	}
	return CancelResult{State: state}, ErrTerminalRun
}

func cancelLocked(st *store.WorkflowStore, state *store.RunState, reason string) (CancelResult, error) {
	if terminal(state.Status) {
		return terminalCancelResult(state)
	}
	state.Status, state.Error = "cancelled", reason
	state.ObjectiveVerdict = nil
	markRemaining(state, "cancelled")
	if err := persist(st, state, "run_cancelled", ""); err != nil {
		return CancelResult{State: persistedStateOr(st, state.RunID, state)}, err
	}
	return CancelResult{State: state}, nil
}

func markRemaining(state *store.RunState, status string) {
	state.PendingGates = nil
	for id, step := range state.Steps {
		switch step.Status {
		case "running":
			if n := len(step.Attempts); n > 0 {
				step.Attempts[n-1].Status, step.Attempts[n-1].EndedAt = status, now()
			}
			fallthrough
		case "pending", "awaiting_approval":
			step.Status = status
			state.Steps[id] = step
		}
	}
}

func localOperator() string {
	operator, err := user.Current()
	if err != nil || operator.Username == "" {
		return "local"
	}
	return operator.Username
}
