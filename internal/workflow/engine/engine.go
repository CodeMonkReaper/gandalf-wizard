// Package engine coordinates declarative workflows using text-only provider ports.
package engine

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/gates"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/store"
)

type MemoryPort interface {
	Query(ctx context.Context, projectID, callerID, query string) (MemoryResult, error)
}

type RouterPort interface {
	Execute(ctx context.Context, req RouterRequest) (RouterResult, error)
}

// RouterPreflight is implemented by router ports that can check concrete local
// selections without invoking a provider.
type RouterPreflight interface {
	CheckEligibility(ctx context.Context, steps []schema.StepSpec) error
}

type MemoryResult struct {
	Text string
	JSON bool
}
type RouterResult struct{ Text string }

type RouterRequest struct {
	Prompt, Complexity, Risk, Profile string
}

type Dependencies struct {
	Store                    *store.WorkflowStore
	Memory                   MemoryPort
	Router                   RouterPort
	ProjectID, CallerID      string
	SecretValues             []string
	MemoryReady, RouterReady bool
}

type ValidateReport struct {
	Workflow schema.ValidatedWorkflow
}

type RunSummary struct {
	State *store.RunState
}

func Validate(ctx context.Context, defPath string, deps Dependencies) (ValidateReport, error) {
	if err := ctx.Err(); err != nil {
		return ValidateReport{}, err
	}
	def, err := schema.LoadAndValidate(defPath, schema.ValidateOptions{
		CheckProviders: true, MemoryReady: deps.MemoryReady, RouterReady: deps.RouterReady,
	})
	var invalid *schema.ValidationErrors
	if errors.As(err, &invalid) {
		for i, entry := range invalid.Errors {
			entry.Field = gates.Redact(entry.Field, deps.SecretValues)
			entry.Message = gates.Redact(entry.Message, deps.SecretValues)
			invalid.Errors[i] = entry
		}
	}
	if err != nil {
		return ValidateReport{Workflow: def}, err
	}
	// Preflight must reject exactly what Run would reject, so validate and run
	// never disagree about a definition that embeds a credential.
	if err := checkMetadata(def, deps.SecretValues); err != nil {
		return ValidateReport{Workflow: def}, err
	}
	return ValidateReport{Workflow: def}, checkRouterEligibility(ctx, def.Definition.Steps, deps.Router)
}

func checkRouterEligibility(ctx context.Context, steps []schema.StepSpec, router RouterPort) error {
	preflight, ok := router.(RouterPreflight)
	if !ok {
		return nil
	}
	var selected []schema.StepSpec
	for _, step := range steps {
		if step.Kind == schema.StepKindRouterTask {
			selected = append(selected, step)
		}
	}
	if len(selected) > 0 {
		return preflight.CheckEligibility(ctx, selected)
	}
	return nil
}

func Run(ctx context.Context, defPath string, deps Dependencies) (RunSummary, error) {
	absolutePath, err := filepath.Abs(defPath)
	if err != nil {
		return RunSummary{}, errors.New("cannot resolve workflow definition path")
	}
	report, err := Validate(ctx, absolutePath, deps)
	if err != nil {
		return RunSummary{}, err
	}
	def := report.Workflow
	if err := checkProviders(def, deps); err != nil {
		return RunSummary{}, err
	}
	// The store redacts on write; it needs the explicit values before the first
	// checkpoint so operator-supplied text is covered too.
	deps.Store.SetSecretValues(deps.SecretValues)
	runID, _, err := deps.Store.CreateRun(def)
	if err != nil {
		return RunSummary{}, err
	}
	return startCreatedRun(ctx, def, deps, runID)
}

func startCreatedRun(ctx context.Context, def schema.ValidatedWorkflow, deps Dependencies, runID string) (RunSummary, error) {
	lock, err := acquireRun(deps.Store, runID)
	if err != nil {
		return RunSummary{}, err
	}
	defer lock.Close()
	// A different process can cancel or resume after CreateRun publishes the
	// checkpoint. Only the current state under this lock may authorize a start.
	state, err := deps.Store.LoadRun(runID)
	if err != nil {
		return RunSummary{}, err
	}
	if state.Status != "created" {
		return RunSummary{State: state}, ErrConflict
	}
	return newExecution(def, deps, state).run(ctx)
}

func checkProviders(def schema.ValidatedWorkflow, deps Dependencies) error {
	if def.RequiresMemory && (!deps.MemoryReady || deps.Memory == nil) {
		return &schema.ValidationErrors{Errors: []schema.ValidationError{
			{Code: "provider_unavailable", Field: "providers.romero", Message: "ROMERO is unavailable"},
		}}
	}
	if def.RequiresRouter && (!deps.RouterReady || deps.Router == nil) {
		return &schema.ValidationErrors{Errors: []schema.ValidationError{
			{Code: "provider_unavailable", Field: "providers.router", Message: "Router is unavailable"},
		}}
	}
	return nil
}

// Check metadata used as keys and paths before any part of it can reach disk.
func checkMetadata(def schema.ValidatedWorkflow, secrets []string) error {
	fields := []string{def.DefinitionPath, def.Definition.ID, def.Definition.Version,
		def.Definition.Objective.ID, def.Definition.Objective.Version}
	for _, step := range def.Definition.Steps {
		fields = append(fields, step.ID)
		fields = append(fields, step.Outputs...)
		if step.Router != nil {
			fields = append(fields, step.Router.Profile)
		}
	}
	for _, criterion := range def.Definition.Objective.SuccessCriteria {
		fields = append(fields, criterion.ID)
	}
	for _, field := range fields {
		if gates.HasSecrets(field, secrets) {
			return &schema.ValidationErrors{Errors: []schema.ValidationError{
				{Code: "secret_in_metadata", Field: "definition", Message: "workflow identifiers and paths must not contain credentials"},
			}}
		}
	}
	return nil
}
