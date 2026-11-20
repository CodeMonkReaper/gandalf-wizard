// Package schema parses and validates declarative Workflow Runtime 0.3 definitions.
package schema

import (
	"fmt"
	"strings"
)

const (
	// SchemaVersion is the supported workflow definition JSON schema version.
	SchemaVersion = "1"
	// DefaultConcurrencyLimit is applied when concurrency_limit is omitted.
	DefaultConcurrencyLimit = 4
	// MaxConcurrencyLimit bounds concurrent step execution to a safe ceiling.
	MaxConcurrencyLimit = 32
	// MaxIdentifierLen bounds step, workflow, objective, and deliverable IDs.
	MaxIdentifierLen = 64
	// MaxArtifactNameLen bounds output and deliverable artifact names for Windows paths.
	MaxArtifactNameLen = 64
)

// StepKind identifies the supported step primitives in Workflow Runtime 0.3.
type StepKind string

const (
	StepKindMemoryQuery   StepKind = "memory_query"
	StepKindRouterTask    StepKind = "router_task"
	StepKindQualityGate   StepKind = "quality_gate"
	StepKindHumanApproval StepKind = "human_approval"
)

// GateRuleKind identifies the closed deterministic catalog of quality gate rules.
type GateRuleKind string

const (
	RuleNonEmpty         GateRuleKind = "non_empty"
	RuleMinLength        GateRuleKind = "min_length"
	RuleMaxLength        GateRuleKind = "max_length"
	RuleContains         GateRuleKind = "contains"
	RuleNotContains      GateRuleKind = "not_contains"
	RuleRegexMatch       GateRuleKind = "regex_match"
	RuleRegexForbid      GateRuleKind = "regex_forbid"
	RuleJSONValid        GateRuleKind = "json_valid"
	RuleJSONRequiredKeys GateRuleKind = "json_required_keys"
	RuleNoSecrets        GateRuleKind = "no_secrets"
)

// WorkflowDefinition is the canonical strict-JSON declarative workflow model.
type WorkflowDefinition struct {
	SchemaVersion    string        `json:"schema_version"`
	ID               string        `json:"id"`
	Version          string        `json:"version"`
	ConcurrencyLimit int           `json:"concurrency_limit,omitempty"`
	Objective        ObjectiveSpec `json:"objective"`
	Steps            []StepSpec    `json:"steps"`
}

// ObjectiveSpec binds the workflow to an ODD objective with expected deliverables and criteria.
type ObjectiveSpec struct {
	ID              string           `json:"id"`
	Version         string           `json:"version"`
	Statement       string           `json:"statement"`
	Constraints     []string         `json:"constraints,omitempty"`
	Deliverables    []DeliverableRef `json:"deliverables"`
	SuccessCriteria []CriterionSpec  `json:"success_criteria"`
}

// DeliverableRef maps an objective deliverable name to a step output (<step_id>.<output>).
type DeliverableRef struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// CriterionSpec evaluates an objective deliverable using deterministic gate rules at closing.
type CriterionSpec struct {
	ID          string         `json:"id"`
	Deliverable string         `json:"deliverable"`
	Informative bool           `json:"informative,omitempty"`
	Rules       []GateRuleSpec `json:"rules"`
}

// StepSpec describes one node in the workflow DAG.
type StepSpec struct {
	ID         string            `json:"id"`
	Kind       StepKind          `json:"kind"`
	DependsOn  []string          `json:"depends_on,omitempty"`
	Optional   bool              `json:"optional,omitempty"`
	Inputs     map[string]string `json:"inputs,omitempty"`
	Outputs    []string          `json:"outputs,omitempty"`
	Memory     *MemoryStepSpec   `json:"memory,omitempty"`
	Router     *RouterStepSpec   `json:"router,omitempty"`
	Gate       *GateSpec         `json:"gate,omitempty"`
	Correction *CorrectionSpec   `json:"correction,omitempty"`
	Approval   *ApprovalStepSpec `json:"approval,omitempty"`
}

// MemoryStepSpec configures a read-only ROMERO memory query step.
type MemoryStepSpec struct {
	Query string `json:"query"`
}

// RouterStepSpec configures a text-only Router 0.2 execution step.
type RouterStepSpec struct {
	Prompt     string `json:"prompt"`
	Complexity string `json:"complexity,omitempty"`
	Risk       string `json:"risk,omitempty"`
	Profile    string `json:"profile,omitempty"`
}

// GateSpec configures a deterministic local quality gate over a step output (<step_id>.<output>).
type GateSpec struct {
	Target      string         `json:"target"`
	Informative bool           `json:"informative,omitempty"`
	Rules       []GateRuleSpec `json:"rules"`
}

// GateRuleSpec defines one deterministic check in the closed quality gate catalog.
type GateRuleSpec struct {
	Kind  GateRuleKind `json:"kind"`
	Value string       `json:"value,omitempty"`
	Limit int          `json:"limit,omitempty"`
	Keys  []string     `json:"keys,omitempty"`
}

// CorrectionSpec binds a router_task to a dependent blocking quality_gate with bounded retries.
type CorrectionSpec struct {
	GateStepID    string `json:"gate_step_id"`
	MaxIterations int    `json:"max_iterations"`
}

// ApprovalStepSpec configures a human approval gate prompt.
type ApprovalStepSpec struct {
	Prompt string `json:"prompt"`
}

// ValidateOptions controls read-only conditional provider checks during preflight (S6).
type ValidateOptions struct {
	CheckProviders bool
	MemoryReady    bool
	RouterReady    bool
}

// ValidatedWorkflow is the validated intermediate representation produced by preflight.
type ValidatedWorkflow struct {
	Definition       WorkflowDefinition
	DefinitionPath   string
	DefinitionSHA256 string
	ExecutionOrder   []string
	RequiresMemory   bool
	RequiresRouter   bool
}

// ValidationError reports one typed preflight or schema violation.
type ValidationError struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error implements error for a single validation error.
func (e ValidationError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s (%s): %s", e.Code, e.Field, e.Message)
}

// ValidationErrors aggregates typed preflight validation errors.
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

// Error implements error by listing all recorded validation failures.
func (e *ValidationErrors) Error() string {
	if e == nil || len(e.Errors) == 0 {
		return "validation failed"
	}
	parts := make([]string, 0, len(e.Errors))
	for _, item := range e.Errors {
		parts = append(parts, item.Error())
	}
	return strings.Join(parts, "; ")
}
