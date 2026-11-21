package engine

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/gates"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/store"
)

type execution struct {
	def           schema.ValidatedWorkflow
	deps          Dependencies
	state         *store.RunState
	steps         map[string]schema.StepSpec
	gatesBySource map[string][]schema.StepSpec
}

type stepResult struct {
	id         string
	text       string
	structured bool
	err        error
}

func newExecution(def schema.ValidatedWorkflow, deps Dependencies, state *store.RunState) *execution {
	e := &execution{def: def, deps: deps, state: state,
		steps: make(map[string]schema.StepSpec), gatesBySource: make(map[string][]schema.StepSpec)}
	if state.Gates == nil {
		state.Gates = make(map[string]store.GateRecord)
	}
	for _, step := range def.Definition.Steps {
		e.steps[step.ID] = step
		if step.Gate != nil {
			source, _, _ := strings.Cut(step.Gate.Target, ".")
			e.gatesBySource[source] = append(e.gatesBySource[source], step)
		}
	}
	return e
}

func (e *execution) run(ctx context.Context) (RunSummary, error) {
	summary, err := e.drive(ctx)
	if err != nil {
		e.recordAbort(err)
	}
	return summary, err
}

// recordAbort leaves a coherent, resumable checkpoint when the coordinator stops
// on an internal error instead of a workflow outcome.
func (e *execution) recordAbort(cause error) {
	if terminal(e.state.Status) {
		return
	}
	e.state.Status = "failed"
	e.state.Error = gates.Redact(cause.Error(), e.deps.SecretValues)
	e.state.ObjectiveVerdict = nil
	markRemaining(e.state, "pending")
	_ = e.checkpoint("run_failed", "")
}

func (e *execution) drive(ctx context.Context) (RunSummary, error) {
	summary := RunSummary{State: e.state}
	e.state.Status = "running"
	startEvent := "run_started"
	if e.state.StateRevision > 1 {
		startEvent = "run_resumed"
	}
	if err := e.checkpoint(startEvent, ""); err != nil {
		return summary, err
	}
	limit := e.def.Definition.ConcurrencyLimit
	results := make(chan stepResult, limit)
	active := 0
	workCtx, stop := context.WithCancel(ctx)
	ticker := time.NewTicker(cancelPollInterval)
	defer ticker.Stop()
	// Drain all launched work before releasing the run lock, including on I/O failure.
	defer func() {
		stop()
		for ; active > 0; active-- {
			<-results
		}
	}()

	explicitCancel := false
	cancelReason := ""
	contextDone := ctx.Done()
	checkCancel := func() error {
		if explicitCancel {
			return nil
		}
		requested, reason, err := e.deps.Store.HasCancelRequest(e.state.RunID)
		if requested {
			explicitCancel, cancelReason = true, reason
			stop()
		}
		return err
	}

	for {
		if err := checkCancel(); err != nil {
			return summary, err
		}
		if !explicitCancel && ctx.Err() == nil && !e.hardBlocked() {
			if err := e.collectApprovals(); err != nil {
				return summary, err
			}
			if len(e.state.PendingGates) == 0 && active < limit {
				launched, progressed, err := e.scheduleOne(workCtx, results)
				if err != nil {
					return summary, err
				}
				if launched {
					active++
				}
				if progressed {
					continue
				}
			}
		}
		if active == 0 {
			break
		}
		select {
		case r := <-results:
			active--
			if err := e.finish(r, workCtx.Err() != nil); err != nil {
				return summary, err
			}
		case <-contextDone:
			contextDone = nil
		case <-ticker.C:
		}
	}

	if err := checkCancel(); err != nil {
		return summary, err
	}
	switch {
	case explicitCancel:
		_, err := cancelLocked(e.deps.Store, e.state, gates.Redact(cancelReason, e.deps.SecretValues))
		return summary, err
	case e.hardBlocked():
		for _, id := range e.def.ExecutionOrder {
			if e.steps[id].Gate == nil || e.state.Steps[id].Status != "pending" {
				continue
			}
			if record := e.state.Gates[id]; record.Status == gates.GateStatusBlocked {
				if err := e.runGate(e.steps[id]); err != nil {
					return summary, err
				}
			} else if record.Status != "" {
				if err := e.skipStep(id); err != nil {
					return summary, err
				}
			}
		}
		markRemaining(e.state, "skipped")
		if err := e.closeObjective(); err != nil {
			return summary, err
		}
	case len(e.state.PendingGates) > 0:
		e.state.Status = "awaiting_approval"
	case ctx.Err() != nil:
		e.state.Status = "paused"
	default:
		if err := e.closeObjective(); err != nil {
			return summary, err
		}
	}
	return summary, e.checkpoint("run_stopped", "")
}

// Collect all reachable approvals before allowing even one new step to start.
func (e *execution) collectApprovals() error {
	for _, id := range e.def.ExecutionOrder {
		step := e.steps[id]
		if step.Kind != schema.StepKindHumanApproval {
			continue
		}
		if ready, _ := e.readiness(step); !ready {
			continue
		}
		e.state.PendingGates = append(e.state.PendingGates, id)
		e.state.Gates[id] = store.GateRecord{
			StepID: id, Status: gates.GateStatusNeedsHumanReview, CheckedAt: now(),
			Prompt: gates.Redact(step.Approval.Prompt, e.deps.SecretValues),
		}
		if err := e.setStep(id, "awaiting_approval", "approval_pending"); err != nil {
			return err
		}
	}
	return nil
}

func (e *execution) scheduleOne(ctx context.Context, results chan<- stepResult) (launched, progressed bool, err error) {
	for _, id := range e.def.ExecutionOrder {
		if ctx.Err() != nil {
			break
		}
		step := e.steps[id]
		ready, skipped := e.readiness(step)
		if skipped {
			return false, true, e.skipStep(id)
		}
		if !ready {
			continue
		}
		if step.Kind == schema.StepKindQualityGate {
			return false, true, e.runGate(step)
		}
		// A reachable approval would already have been collected above.
		err := e.launch(ctx, step, results)
		return err == nil, true, err
	}
	return false, false, nil
}

func (e *execution) readiness(step schema.StepSpec) (ready, skipped bool) {
	if e.state.Steps[step.ID].Status != "pending" {
		return false, false
	}
	ready = true
	record := e.state.Gates[step.ID]
	source, _, _ := strings.Cut(record.Target, ".")
	for _, id := range step.DependsOn {
		switch e.state.Steps[id].Status {
		case "passed":
			// An evaluated gate must publish its result for its own source.
			// Other dependents still cannot consume rejected evidence.
			if id != source && e.blockingGateFailed(id) {
				return false, true
			}
		case "failed":
			if id != source || record.Status != gates.GateStatusFail || record.Informative {
				return false, true
			}
		case "skipped", "cancelled":
			return false, true
		default:
			ready = false
		}
	}
	return ready, false
}

// blockingGateFailed reports whether a mandatory gate rejected the output of the
// dependency. A failed gate fails its branch: no dependent may consume the
// rejected artifact even when the producing step itself is "passed".
func (e *execution) blockingGateFailed(source string) bool {
	for _, gateStep := range e.gatesBySource[source] {
		record, ok := e.state.Gates[gateStep.ID]
		if !ok || record.Informative {
			continue
		}
		if record.Status == gates.GateStatusFail || record.Status == gates.GateStatusBlocked {
			return true
		}
	}
	return false
}

func (e *execution) runGate(step schema.StepSpec) error {
	record, ok := e.state.Gates[step.ID]
	if !ok {
		return fmt.Errorf("%w: quality gate evidence is missing", store.ErrCorruptState)
	}
	status := "passed"
	if record.Status != gates.GateStatusPass && !record.Informative {
		status = "failed"
	}
	s := e.state.Steps[step.ID]
	s.Warnings = record.Warnings
	if status == "passed" && len(step.Outputs) > 0 {
		body, err := json.Marshal(record)
		if err != nil {
			return err
		}
		_, hadSecret, err := gates.RedactJSON(string(body), e.deps.SecretValues)
		if err != nil {
			return err
		}
		e.evaluateGatesWithSecretEvidence(step.ID, string(body), hadSecret)
		refs, err := e.saveOutputs(step, s.Iteration, string(body), true)
		if err != nil {
			return err
		}
		s.Artifacts = refs
	}
	e.state.Steps[step.ID] = s
	e.state.Warnings = append(e.state.Warnings, record.Warnings...)
	return e.setStepEvent(step.ID, status, "gate_evaluated", record.Status, gateMessage(record))
}

// A gate whose other dependencies failed can be skipped by the scheduler even
// though an available source already supplied an evaluation to audit.
func (e *execution) skipStep(id string) error {
	record := e.state.Gates[id]
	if e.steps[id].Gate != nil && record.Status != "" {
		return e.setStepEvent(id, "skipped", "gate_evaluated", record.Status, gateMessage(record))
	}
	return e.setStep(id, "skipped", "step_skipped")
}

func gateMessage(record store.GateRecord) string {
	if len(record.Violations) > 0 {
		return strings.Join(record.Violations, "\n")
	}
	return strings.Join(record.Warnings, "\n")
}

func (e *execution) launch(ctx context.Context, step schema.StepSpec, results chan<- stepResult) error {
	input, err := e.input(step)
	if err != nil {
		return err
	}
	record := e.state.Steps[step.ID]
	record.Iteration++
	if n := len(record.Attempts); n > 0 && record.Attempts[n-1].Diagnosis != "" {
		input += "\n\nQuality check feedback:\n" + record.Attempts[n-1].Diagnosis
	}
	input = gates.Redact(input, e.deps.SecretValues)
	record.Attempts = append(record.Attempts, store.AttemptRecord{
		Iteration: record.Iteration, Status: "running", StartedAt: now(),
	})
	e.state.Steps[step.ID] = record
	if err := e.setStep(step.ID, "running", "step_started"); err != nil {
		return err
	}
	go func() {
		r := stepResult{id: step.ID}
		if err := ctx.Err(); err != nil {
			r.err = err
		} else if step.Kind == schema.StepKindMemoryQuery {
			response, err := e.deps.Memory.Query(ctx, e.deps.ProjectID, e.deps.CallerID, input)
			r.text, r.structured, r.err = response.Text, response.JSON, err
		} else {
			response, err := e.deps.Router.Execute(ctx, RouterRequest{
				Prompt: input, Complexity: step.Router.Complexity, Risk: step.Router.Risk,
				Profile: gates.Redact(step.Router.Profile, e.deps.SecretValues),
			})
			r.text, r.err = response.Text, err
		}
		results <- r
	}()
	return nil
}

func (e *execution) input(step schema.StepSpec) (string, error) {
	var text string
	if step.Memory != nil {
		text = step.Memory.Query
	} else {
		text = step.Router.Prompt
	}
	keys := make([]string, 0, len(step.Inputs))
	for key := range step.Inputs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	replacements := make([]string, 0, 2*len(keys))
	for _, key := range keys {
		content, _, err := e.artifact(step.Inputs[key])
		if err != nil {
			return "", err
		}
		replacements = append(replacements, "{{"+key+"}}", content)
	}
	return strings.NewReplacer(replacements...).Replace(text), nil
}

func (e *execution) finish(r stepResult, interrupted bool) error {
	step := e.steps[r.id]
	record := e.state.Steps[r.id]
	attempt := &record.Attempts[len(record.Attempts)-1]
	attempt.EndedAt, attempt.Status = now(), "passed"
	next := "passed"
	if r.err != nil {
		attempt.Status = "failed"
		attempt.Error = gates.Redact(r.err.Error(), e.deps.SecretValues)
		record.Error, next = attempt.Error, "failed"
		if interrupted && (errors.Is(r.err, context.Canceled) || errors.Is(r.err, context.DeadlineExceeded)) {
			next = "pending"
		}
	} else {
		hadSecret := gates.HasSecrets(r.text, e.deps.SecretValues)
		if r.structured {
			_, detected, err := gates.RedactJSON(r.text, e.deps.SecretValues)
			if err != nil {
				return err
			}
			hadSecret = detected
		}
		blocked := e.evaluateGatesWithSecretEvidence(r.id, r.text, hadSecret)
		switch {
		case blocked:
			attempt.Status, next, attempt.GateStatus = "failed", "failed", gates.GateStatusBlocked
		case step.Correction != nil:
			g := e.state.Gates[step.Correction.GateStepID]
			attempt.GateStatus = g.Status
			if g.Status != gates.GateStatusPass {
				attempt.Diagnosis = strings.Join(g.Violations, "\n")
				attempt.Status, next = "failed", "failed"
				if g.Retryable && correctionAttempts(record) < step.Correction.MaxIterations {
					next = "pending"
				}
			}
		}
		refs, err := e.saveOutputs(step, record.Iteration, r.text, r.structured)
		if err != nil {
			// Keep the checkpoint honest: the attempt failed and the run must not
			// keep reporting a "passed" attempt that was never persisted.
			attempt.Status = "failed"
			attempt.Error = gates.Redact(err.Error(), e.deps.SecretValues)
			record.Error = attempt.Error
			e.state.Steps[r.id] = record
			if persistErr := e.setStepEvent(r.id, "failed", "step_failed", "failed", attempt.Error); persistErr != nil {
				return persistErr
			}
			return err
		}
		attempt.Artifacts = refs
		record.Artifacts, record.Error = attempt.Artifacts, ""
	}
	e.state.Steps[r.id] = record
	message := attempt.Diagnosis
	if message == "" {
		message = attempt.Error
	}
	return e.setStepEvent(r.id, next, "step_finished", attempt.Status, message)
}

func correctionAttempts(record store.StepRecord) int {
	start := record.CorrectionCycleStart
	if start < 0 || start > len(record.Attempts) {
		start = 0
	}
	count := 0
	for _, attempt := range record.Attempts[start:] {
		if attempt.GateStatus != "" {
			count++
		}
	}
	return count
}

// Persist only decisions from raw output, never the original output.
func (e *execution) evaluateGates(source, raw string) bool {
	return e.evaluateGatesWithSecretEvidence(source, raw, gates.HasSecrets(raw, e.deps.SecretValues))
}

func (e *execution) evaluateGatesWithSecretEvidence(source, raw string, hadSecret bool) bool {
	blocked := false
	for _, gateStep := range e.gatesBySource[source] {
		g := gates.EvaluateGateWithSecretEvidence(*gateStep.Gate, raw, hadSecret)
		for _, list := range [][]string{g.Violations, g.Warnings} {
			for i, message := range list {
				list[i] = gates.Redact(message, e.deps.SecretValues)
			}
		}
		e.state.Gates[gateStep.ID] = store.GateRecord{
			StepID: gateStep.ID, Target: gateStep.Gate.Target, Status: g.Status,
			Informative: g.Informative, Retryable: g.Retryable, Violations: g.Violations,
			Warnings: g.Warnings, CheckedAt: now(),
		}
		blocked = blocked || g.Status == gates.GateStatusBlocked
	}
	return blocked
}

func (e *execution) saveOutputs(step schema.StepSpec, iteration int, raw string, structured bool) ([]store.ArtifactRef, error) {
	save := e.deps.Store.SaveArtifact
	if structured {
		save = e.deps.Store.SaveJSONArtifact
	}
	var refs []store.ArtifactRef
	for _, name := range step.Outputs {
		ref, err := save(e.state.RunID, step.ID, iteration, name, raw)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func (e *execution) hardBlocked() bool {
	for id, gate := range e.state.Gates {
		source, _, _ := strings.Cut(gate.Target, ".")
		if gate.Status == gates.GateStatusBlocked && (e.state.Steps[id].Status == "failed" || e.state.Steps[source].Status == "passed") {
			return true
		}
	}
	for _, step := range e.state.Steps {
		if n := len(step.Attempts); n > 0 && step.Status == "failed" && step.Attempts[n-1].GateStatus == gates.GateStatusBlocked {
			return true
		}
	}
	return false
}

func (e *execution) closeObjective() error {
	outcomes := make(map[string]gates.StepOutcome, len(e.state.Steps))
	for id, step := range e.state.Steps {
		outcomes[id] = gates.StepOutcome{ID: id, Optional: step.Optional, Status: step.Status, Warnings: step.Warnings}
	}
	artifacts := make(map[string]string)
	secret := make(map[string]bool)
	for _, deliverable := range e.def.Definition.Objective.Deliverables {
		id, _, _ := strings.Cut(deliverable.Source, ".")
		if e.state.Steps[id].Status != "passed" {
			continue
		}
		text, hadSecret, err := e.artifact(deliverable.Source)
		if err != nil {
			return err
		}
		artifacts[deliverable.Source] = text
		secret[deliverable.Source] = hadSecret
	}
	verdict := gates.EvaluateObjectiveWithEvidence(e.def.Definition.Objective, outcomes, artifacts, secret, e.deps.SecretValues)
	if e.hardBlocked() {
		verdict.Status = gates.RunStatusBlocked
	}
	e.state.Status = verdict.Status
	e.state.ObjectiveVerdict = &store.ObjectiveVerdictRecord{
		Status: verdict.Status, MetCriteria: verdict.MetCriteria, FailedCriteria: verdict.FailedCriteria, Warnings: verdict.Warnings,
	}
	if e.state.Error == "" && (verdict.Status == gates.RunStatusFailed || verdict.Status == gates.RunStatusBlocked) {
		if gateID, ok := e.rejectedGate(); ok {
			e.state.Error = gates.Redact("blocking quality gate "+gateID+" rejected the step output", e.deps.SecretValues)
		}
	}
	return nil
}

// rejectedGate names the first mandatory gate that rejected an output, in
// deterministic order, so a failed run can explain itself.
func (e *execution) rejectedGate() (string, bool) {
	ids := make([]string, 0, len(e.state.Gates))
	for id := range e.state.Gates {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		record := e.state.Gates[id]
		if record.Informative {
			continue
		}
		if record.Status == gates.GateStatusFail || record.Status == gates.GateStatusBlocked {
			return id, true
		}
	}
	return "", false
}

func (e *execution) artifact(source string) (string, bool, error) {
	id, name, _ := strings.Cut(source, ".")
	for _, ref := range e.state.Steps[id].Artifacts {
		if ref.Name != name {
			continue
		}
		content, err := e.deps.Store.ReadArtifact(e.state.RunID, ref.RelPath)
		if err != nil {
			return "", false, err
		}
		if fmt.Sprintf("%x", sha256.Sum256([]byte(content))) != ref.SHA256 {
			return "", false, fmt.Errorf("%w: artifact digest mismatch", store.ErrCorruptState)
		}
		if ref.JSON {
			clean, detected, err := gates.RedactJSON(content, e.deps.SecretValues)
			return clean, detected || ref.SecretDetected, err
		}
		clean := gates.Redact(content, e.deps.SecretValues)
		return clean, ref.SecretDetected || gates.HasSecrets(content, e.deps.SecretValues), nil
	}
	return "", false, fmt.Errorf("%w: artifact is missing", store.ErrCorruptState)
}

func (e *execution) setStep(id, status, event string) error {
	return e.setStepEvent(id, status, event, "", "")
}

// setStepEvent records the transition with an optional explicit event verdict,
// so the append-only trail never contradicts the checkpoint it corroborates.
func (e *execution) setStepEvent(id, status, event, eventStatus, eventMessage string) error {
	record := e.state.Steps[id]
	record.Status = status
	e.state.Steps[id] = record
	return e.checkpointEvent(event, id, eventStatus, eventMessage)
}

func (e *execution) checkpoint(event, stepID string) error {
	return e.checkpointEvent(event, stepID, "", "")
}

func (e *execution) checkpointEvent(event, stepID, eventStatus, eventMessage string) error {
	return persistEvent(e.deps.Store, e.state, event, stepID, eventStatus, eventMessage)
}

func persist(s *store.WorkflowStore, state *store.RunState, event, stepID string) error {
	return persistEvent(s, state, event, stepID, "", "")
}

func persistEvent(s *store.WorkflowStore, state *store.RunState, event, stepID, eventStatus, eventMessage string) error {
	if _, err := s.WriteCheckpoint(state.RunID, state); err != nil {
		return err
	}
	entry := store.RunEvent{Seq: state.StateRevision, EventType: event, StepID: stepID, Status: state.Status}
	if stepID != "" {
		step := state.Steps[stepID]
		entry.Iteration, entry.Status, entry.Message = step.Iteration, step.Status, step.Error
	}
	if eventStatus != "" {
		entry.Status = eventStatus
	}
	if eventMessage != "" {
		entry.Message = eventMessage
	}
	return s.AppendEvent(state.RunID, entry)
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
