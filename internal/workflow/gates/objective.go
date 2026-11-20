package gates

import (
	"sort"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
)

const (
	RunStatusSucceeded = "succeeded"
	RunStatusPartial   = "partial"
	RunStatusFailed    = "failed"
	RunStatusBlocked   = "blocked"
)

// StepOutcome records the execution and gate status of one workflow step.
type StepOutcome struct {
	ID         string   `json:"id"`
	Optional   bool     `json:"optional"`
	Status     string   `json:"status"`
	GateStatus string   `json:"gate_status,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

// ObjectiveVerdict summarizes final workflow evaluation against ObjectiveSpec (Q2 / AC-09).
type ObjectiveVerdict struct {
	Status         string   `json:"status"`
	MetCriteria    []string `json:"met_criteria,omitempty"`
	FailedCriteria []string `json:"failed_criteria,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

// EvaluateObjective decides whether a workflow succeeded, partially succeeded, failed, or was blocked.
func EvaluateObjective(obj schema.ObjectiveSpec, steps map[string]StepOutcome, artifacts map[string]string) ObjectiveVerdict {
	return EvaluateObjectiveWithSecrets(obj, steps, artifacts, nil)
}

// EvaluateObjectiveWithSecrets evaluates an objective while guarding deliverables against secret leaks.
func EvaluateObjectiveWithSecrets(obj schema.ObjectiveSpec, steps map[string]StepOutcome, artifacts map[string]string, secretValues []string) ObjectiveVerdict {
	verdict := ObjectiveVerdict{Status: RunStatusSucceeded}

	stepKeys := make([]string, 0, len(steps))
	for key := range steps {
		stepKeys = append(stepKeys, key)
	}
	sort.Strings(stepKeys)

	anyBlocked := false
	anyMandatoryFailed := false
	anyOptionalDegraded := false

	for _, key := range stepKeys {
		step := steps[key]
		for _, w := range step.Warnings {
			verdict.Warnings = append(verdict.Warnings, Redact(w, secretValues))
		}
		if step.Status == GateStatusBlocked || step.GateStatus == GateStatusBlocked {
			anyBlocked = true
		}
		if !step.Optional && step.Status != "passed" {
			anyMandatoryFailed = true
		}
		if step.Optional && step.Status != "passed" {
			anyOptionalDegraded = true
		}
	}

	missingDeliverable := false
	resolved := make(map[string]string, len(obj.Deliverables))
	for _, d := range obj.Deliverables {
		content, ok := artifacts[d.Name]
		if !ok {
			content, ok = artifacts[d.Source]
		}
		if !ok {
			missingDeliverable = true
			continue
		}
		resolved[d.Name] = content
		if HasSecrets(content, secretValues) {
			anyBlocked = true
		}
	}

	for _, c := range obj.SuccessCriteria {
		content, ok := resolved[c.Deliverable]
		if !ok {
			verdict.FailedCriteria = append(verdict.FailedCriteria, c.ID)
			continue
		}
		res := EvaluateGate(schema.GateSpec{
			Target:      c.Deliverable,
			Informative: false,
			Rules:       c.Rules,
		}, content, secretValues)
		switch res.Status {
		case GateStatusBlocked:
			anyBlocked = true
			verdict.FailedCriteria = append(verdict.FailedCriteria, c.ID)
		case GateStatusPass:
			verdict.MetCriteria = append(verdict.MetCriteria, c.ID)
		default:
			verdict.FailedCriteria = append(verdict.FailedCriteria, c.ID)
		}
	}

	switch {
	case anyBlocked:
		verdict.Status = RunStatusBlocked
	case anyMandatoryFailed || missingDeliverable || len(verdict.FailedCriteria) > 0:
		verdict.Status = RunStatusFailed
	case anyOptionalDegraded:
		verdict.Status = RunStatusPartial
	default:
		verdict.Status = RunStatusSucceeded
	}

	return verdict
}
