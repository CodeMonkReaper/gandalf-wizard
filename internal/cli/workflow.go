package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/render"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/romero"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/engine"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/gates"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/store"
)

// workflowFlags holds the parsed flags of one workflow subcommand.
type workflowFlags struct {
	jsonOut    bool
	gate       string
	checkpoint string
	decision   string
	reason     string
	project    string
	caller     string
	romeroURL  string
	timeout    string
	args       []string
}

// commandFlags adapts the workflow flags to the shared config loader.
func (p *workflowFlags) commandFlags() *commandFlags {
	return &commandFlags{
		project:   p.project,
		caller:    p.caller,
		romeroURL: p.romeroURL,
		timeout:   p.timeout,
		jsonOut:   p.jsonOut,
		args:      p.args,
	}
}

// parseWorkflowFlags parses the workflow flags; approval adds the gate flags and
// reason adds the optional operator motive shared by approve and cancel.
func parseWorkflowFlags(name string, args []string, out io.Writer, approval, reason bool) (*workflowFlags, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(out)
	p := &workflowFlags{}
	fs.BoolVar(&p.jsonOut, "json", false, "salida JSON estable")
	if approval {
		fs.StringVar(&p.gate, "gate", "", "puerta pendiente")
		fs.StringVar(&p.checkpoint, "checkpoint", "", "huella del checkpoint vigente")
		fs.StringVar(&p.decision, "decision", "", "approve o reject")
	}
	if reason {
		fs.StringVar(&p.reason, "reason", "", "motivo de la decisión o de la cancelación")
	}
	fs.StringVar(&p.project, "project", "", "proyecto activo")
	fs.StringVar(&p.caller, "caller", "", "identidad del solicitante")
	fs.StringVar(&p.romeroURL, "romero-url", "", "dirección de ROMERO")
	fs.StringVar(&p.timeout, "timeout", "", "tiempo límite de ROMERO")
	flagArgs, positional := splitWorkflowArgs(args)
	if err := fs.Parse(flagArgs); err != nil {
		return nil, err
	}
	p.args = positional
	return p, nil
}

// splitWorkflowArgs separates flags from positional arguments so the documented
// order (`<run-id> --gate ...`) parses the same as flags first.
func splitWorkflowArgs(args []string) (flags, positional []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		name, _, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if hasValue || name == "json" {
			continue
		}
		if i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return flags, positional
}

// workflowRuntime carries the resolved store and engine dependencies.
type workflowRuntime struct {
	store   *store.WorkflowStore
	engine  engine.Dependencies
	secrets []string
	timeout time.Duration
}

// withTimeout applies the configured ROMERO deadline to the invocation.
func (r *workflowRuntime) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.timeout > 0 {
		return context.WithTimeout(ctx, r.timeout)
	}
	return ctx, func() {}
}

func (r *workflowRuntime) close() {
	if r.store != nil {
		_ = r.store.Close()
	}
}

// workflowConfigError marks a failure that comes from configuration or flags,
// so the caller maps it to the configuration exit code.
type workflowConfigError struct{ err error }

func (e workflowConfigError) Error() string { return e.err.Error() }
func (e workflowConfigError) Unwrap() error { return e.err }

// workflowDeps resolves configuration, provider ports and the confined store.
// readOnly skips the store so validate never creates .gandalf (AC-01).
func (deps Dependencies) workflowDeps(ctx context.Context, p *workflowFlags, readOnly bool) (*workflowRuntime, error) {
	cfg, err := loadConfig(deps, p.commandFlags())
	if err != nil {
		return nil, workflowConfigError{err: err}
	}
	runtime := &workflowRuntime{secrets: deps.workflowSecrets(cfg), timeout: cfg.RomeroTimeout.Value}
	memory := deps.workflowMemoryPort(cfg)
	routing := deps.workflowRouterPort(cfg)
	runtime.engine = engine.Dependencies{
		Memory:       memory,
		Router:       routing,
		ProjectID:    cfg.Project.Value,
		CallerID:     cfg.Caller.Value,
		SecretValues: runtime.secrets,
	}
	// S6 checks the presence of the ports. The engine checks concrete router
	// selections only for definitions that use Router.
	runtime.engine.MemoryReady = deps.WorkflowMemory != nil ||
		(memory != nil && workflowMemoryReady(cfg))
	runtime.engine.RouterReady = routing != nil
	if readOnly {
		return runtime, nil
	}
	st, err := store.OpenRoot(deps.WorkDir)
	if err != nil {
		return nil, err
	}
	// The store redacts on write; approve and cancel do not receive the engine
	// dependencies, so the credential values are set as soon as it opens (R5-1).
	st.SetSecretValues(runtime.secrets)
	runtime.store = st
	runtime.engine.Store = st
	return runtime, nil
}

// workflowSecrets lists the credential values the store must redact.
func (deps Dependencies) workflowSecrets(cfg config.EffectiveConfig) []string {
	var secrets []string
	if value := deps.getenv(config.EnvKeyRomeroAPIKey); value != "" {
		secrets = append(secrets, value)
	}
	for _, executor := range cfg.Router.Executors {
		if executor.CredentialEnv == "" {
			continue
		}
		if value := deps.getenv(executor.CredentialEnv); value != "" {
			secrets = append(secrets, value)
		}
	}
	return secrets
}

// workflowMemoryPort returns the injected port, the ROMERO adapter when the
// configuration declares a base URL, or nil so preflight rejects the step.
func (deps Dependencies) workflowMemoryPort(cfg config.EffectiveConfig) engine.MemoryPort {
	if deps.WorkflowMemory != nil {
		return deps.WorkflowMemory
	}
	if strings.TrimSpace(cfg.RomeroBaseURL.Value) == "" {
		return nil
	}
	return memoryPort{orchestrator: romero.NewMemoryQueryOrchestrator(deps.readPort(cfg))}
}

// workflowRouterPort returns the injected port, the router adapter when the
// configuration declares executors and profiles, or nil for conditional S6.
func (deps Dependencies) workflowRouterPort(cfg config.EffectiveConfig) engine.RouterPort {
	if deps.WorkflowRouter != nil {
		return deps.WorkflowRouter
	}
	if len(cfg.Router.Executors) == 0 || len(cfg.Router.Profiles) == 0 {
		return nil
	}
	return routerPort{policy: cfg.Router, registry: deps.routerRegistryFor(cfg.Router), getenv: deps.getenv}
}

// workflowMemoryReady reports whether ROMERO was actually configured: the
// default URL alone is not an available provider.
func workflowMemoryReady(cfg config.EffectiveConfig) bool {
	switch cfg.RomeroBaseURL.Origin {
	case config.OriginFlag, config.OriginEnv, config.OriginProject, config.OriginGlobal:
		return true
	}
	return cfg.APIKeyPresent
}

// memoryPort adapts the ROMERO orchestrator to the engine memory port.
type memoryPort struct {
	orchestrator *romero.MemoryQueryOrchestrator
}

func (p memoryPort) Query(ctx context.Context, projectID, callerID, query string) (engine.MemoryResult, error) {
	outcome := p.orchestrator.QueryProjectMemory(ctx, romero.WorkspaceContext{
		ActiveProjectID: projectID, CallerIdentity: callerID,
	}, query)
	switch outcome.Status {
	case romero.StatusOK, romero.StatusEmpty, romero.StatusPartial:
	default:
		return engine.MemoryResult{}, fmt.Errorf("ROMERO no devolvió memoria (%s)", outcome.Status)
	}
	var evidence bytes.Buffer
	err := render.RenderMemoryQuery(&evidence, render.MemoryQueryView{
		Project: projectID, Outcome: outcome, View: romero.Present(outcome),
	}, true)
	if err != nil {
		return engine.MemoryResult{}, err
	}
	return engine.MemoryResult{Text: evidence.String(), JSON: true}, nil
}

// routerPort adapts select/run to the engine router port.
type routerPort struct {
	policy   config.RouterPolicy
	registry *router.Registry
	getenv   func(string) string
}

func (p routerPort) CheckEligibility(ctx context.Context, steps []schema.StepSpec) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	catalog := router.Build(ctx, p.policy, p.registry, router.DiscoveryLocalOnly)
	if err := ctx.Err(); err != nil {
		return err
	}
	var failures []schema.ValidationError
	for _, step := range steps {
		if step.Kind != schema.StepKindRouterTask || step.Router == nil {
			continue
		}
		req := engine.RouterRequest{Complexity: step.Router.Complexity, Risk: step.Router.Risk, Profile: step.Router.Profile}
		if len(selectWorkflowRouter(catalog, req).Ordered) == 0 {
			failures = append(failures, schema.ValidationError{
				Code: "provider_unavailable", Field: "steps." + step.ID + ".router",
				Message: "Router has no locally eligible profile for this step",
			})
		}
	}
	if len(failures) > 0 {
		return &schema.ValidationErrors{Errors: failures}
	}
	return nil
}

func selectWorkflowRouter(catalog router.Catalog, req engine.RouterRequest) router.Decision {
	return router.Select(catalog, router.Features{
		Complexity: router.Complexity(req.Complexity), Risk: router.Risk(req.Risk),
	}, req.Profile)
}

func (p routerPort) Execute(ctx context.Context, req engine.RouterRequest) (engine.RouterResult, error) {
	catalog := router.Build(ctx, p.policy, p.registry, router.DiscoveryLocalOnly)
	decision := selectWorkflowRouter(catalog, req)
	result := router.Run(ctx, p.policy, decision, req.Prompt, p.registry, p.getenv)
	if result.Status != router.StatusOK {
		return engine.RouterResult{}, fmt.Errorf("el router no ejecutó el paso (%s)", result.Status)
	}
	return engine.RouterResult{Text: result.Output}, nil
}

// runWorkflow dispatches `gandalf workflow <validate|run|status|approve|resume|cancel>`.
func runWorkflow(ctx context.Context, args []string, deps Dependencies) int {
	if len(args) == 0 {
		fmt.Fprintf(deps.Stderr, "error: workflow requiere un subcomando (validate, run, status, approve, resume, cancel)\n\n%s", usageText)
		return ExitUsageError
	}
	switch args[0] {
	case "validate":
		return runWorkflowValidate(ctx, args[1:], deps)
	case "run":
		return runWorkflowRun(ctx, args[1:], deps)
	case "status":
		return runWorkflowStatus(ctx, args[1:], deps)
	case "approve":
		return runWorkflowApprove(ctx, args[1:], deps)
	case "resume":
		return runWorkflowResume(ctx, args[1:], deps)
	case "cancel":
		return runWorkflowCancel(ctx, args[1:], deps)
	default:
		fmt.Fprintf(deps.Stderr, "error: subcomando de workflow desconocido %q\n\n%s", args[0], usageText)
		return ExitUsageError
	}
}

func runWorkflowValidate(ctx context.Context, args []string, deps Dependencies) int {
	p, err := parseWorkflowFlags("gandalf workflow validate", args, deps.Stderr, false, false)
	if err != nil {
		return ExitUsageError
	}
	if len(p.args) != 1 {
		fmt.Fprintln(deps.Stderr, "error: workflow validate requiere una ruta de definición")
		return ExitUsageError
	}
	runtime, err := deps.workflowDeps(ctx, p, true)
	if err != nil {
		return workflowSetupExit(deps, p.jsonOut, "workflow validate", err)
	}
	ctx, stop := runtime.withTimeout(ctx)
	defer stop()
	report, err := engine.Validate(ctx, p.args[0], runtime.engine)
	if err != nil {
		return workflowFailure(deps, p.jsonOut, runtime.secrets, "workflow validate", err)
	}
	if err := render.RenderWorkflowValidate(deps.Stdout, report, p.jsonOut, runtime.secrets); err != nil {
		fmt.Fprintln(deps.Stderr, "error interno:", err)
		return ExitInternalError
	}
	return ExitOK
}

func runWorkflowRun(ctx context.Context, args []string, deps Dependencies) int {
	p, err := parseWorkflowFlags("gandalf workflow run", args, deps.Stderr, false, false)
	if err != nil {
		return ExitUsageError
	}
	if len(p.args) != 1 {
		fmt.Fprintln(deps.Stderr, "error: workflow run requiere una ruta de definición")
		return ExitUsageError
	}
	runtime, err := deps.workflowDeps(ctx, p, false)
	if err != nil {
		return workflowSetupExit(deps, p.jsonOut, "workflow run", err)
	}
	defer runtime.close()
	ctx, stop := runtime.withTimeout(ctx)
	defer stop()
	summary, runErr := engine.Run(ctx, p.args[0], runtime.engine)
	return workflowRunExit(ctx, deps, p.jsonOut, runtime.secrets, "workflow run", summary, runErr)
}

func runWorkflowResume(ctx context.Context, args []string, deps Dependencies) int {
	p, err := parseWorkflowFlags("gandalf workflow resume", args, deps.Stderr, false, false)
	if err != nil {
		return ExitUsageError
	}
	if len(p.args) != 1 {
		fmt.Fprintln(deps.Stderr, "error: workflow resume requiere un run-id")
		return ExitUsageError
	}
	runtime, err := deps.workflowDeps(ctx, p, false)
	if err != nil {
		return workflowSetupExit(deps, p.jsonOut, "workflow resume", err)
	}
	defer runtime.close()
	ctx, stop := runtime.withTimeout(ctx)
	defer stop()
	summary, runErr := engine.Resume(ctx, p.args[0], runtime.engine)
	return workflowRunExit(ctx, deps, p.jsonOut, runtime.secrets, "workflow resume", summary, runErr)
}

// workflowRunExit renders the run summary when the engine persisted a state and
// maps the outcome to its reserved exit code.
func workflowRunExit(ctx context.Context, deps Dependencies, asJSON bool, secrets []string, command string, summary engine.RunSummary, runErr error) int {
	if summary.State != nil {
		if err := renderSummary(command, deps.Stdout, summary, asJSON, secrets); err != nil {
			fmt.Fprintln(deps.Stderr, "error interno:", err)
			return ExitInternalError
		}
		if runErr != nil && !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, context.DeadlineExceeded) {
			fmt.Fprintf(deps.Stderr, "error: %s\n", gates.Redact(runErr.Error(), secrets))
		}
		// A refused resume (terminal run, pending gate, changed definition or
		// busy lock) is a conflict even when the persisted state is terminal.
		if errors.Is(runErr, store.ErrRunBusy) || errors.Is(runErr, engine.ErrTerminalRun) ||
			errors.Is(runErr, engine.ErrPendingApproval) || errors.Is(runErr, engine.ErrConflict) {
			return ExitConflict
		}
		if runErr != nil && !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, context.DeadlineExceeded) {
			if code := workflowStateExit(summary.State.Status); code != ExitOK {
				return code
			}
			return workflowFailureExit(runErr)
		}
		return workflowStateExit(summary.State.Status)
	}
	if runErr == nil {
		return ExitOK
	}
	return workflowFailure(deps, asJSON, secrets, command, runErr)
}

func renderSummary(command string, out io.Writer, summary engine.RunSummary, asJSON bool, secrets []string) error {
	switch command {
	case "workflow run":
		return render.RenderWorkflowRun(out, summary, asJSON, secrets)
	case "workflow resume":
		return render.RenderWorkflowResume(out, summary, asJSON, secrets)
	default:
		return nil
	}
}

func runWorkflowStatus(ctx context.Context, args []string, deps Dependencies) int {
	p, err := parseWorkflowFlags("gandalf workflow status", args, deps.Stderr, false, false)
	if err != nil {
		return ExitUsageError
	}
	if len(p.args) != 1 {
		fmt.Fprintln(deps.Stderr, "error: workflow status requiere un run-id")
		return ExitUsageError
	}
	runtime, err := deps.workflowDeps(ctx, p, true)
	if err != nil {
		return workflowSetupExit(deps, p.jsonOut, "workflow status", err)
	}
	st, err := store.OpenExistingRoot(deps.WorkDir)
	if err != nil {
		return workflowFailure(deps, p.jsonOut, runtime.secrets, "workflow status", err)
	}
	defer st.Close()
	view, statusErr := engine.Status(p.args[0], st)
	if statusErr != nil {
		return workflowFailure(deps, p.jsonOut, runtime.secrets, "workflow status", statusErr)
	}
	if err := render.RenderWorkflowStatus(deps.Stdout, view, p.jsonOut, runtime.secrets); err != nil {
		fmt.Fprintln(deps.Stderr, "error interno:", err)
		return ExitInternalError
	}
	return ExitOK
}

func runWorkflowApprove(ctx context.Context, args []string, deps Dependencies) int {
	p, err := parseWorkflowFlags("gandalf workflow approve", args, deps.Stderr, true, true)
	if err != nil {
		return ExitUsageError
	}
	if len(p.args) != 1 || p.gate == "" || p.checkpoint == "" || p.decision == "" {
		fmt.Fprintln(deps.Stderr, "error: workflow approve requiere <run-id>, --gate, --checkpoint y --decision")
		return ExitUsageError
	}
	if p.decision != "approve" && p.decision != "reject" {
		fmt.Fprintln(deps.Stderr, "error: --decision debe ser approve o reject")
		return ExitUsageError
	}
	if p.decision == "reject" && strings.TrimSpace(p.reason) == "" {
		fmt.Fprintln(deps.Stderr, "error: --decision reject requiere --reason")
		return ExitUsageError
	}
	runtime, err := deps.workflowDeps(ctx, p, false)
	if err != nil {
		return workflowSetupExit(deps, p.jsonOut, "workflow approve", err)
	}
	defer runtime.close()
	result, approveErr := engine.Approve(p.args[0], p.gate, p.checkpoint, p.decision, p.reason, runtime.store)
	if approveErr != nil {
		return workflowFailure(deps, p.jsonOut, runtime.secrets, "workflow approve", approveErr)
	}
	if err := render.RenderWorkflowApprove(deps.Stdout, result, p.jsonOut, runtime.secrets); err != nil {
		fmt.Fprintln(deps.Stderr, "error interno:", err)
		return ExitInternalError
	}
	return ExitOK
}

func runWorkflowCancel(ctx context.Context, args []string, deps Dependencies) int {
	p, err := parseWorkflowFlags("gandalf workflow cancel", args, deps.Stderr, false, true)
	if err != nil {
		return ExitUsageError
	}
	if len(p.args) != 1 {
		fmt.Fprintln(deps.Stderr, "error: workflow cancel requiere un run-id")
		return ExitUsageError
	}
	runtime, err := deps.workflowDeps(ctx, p, false)
	if err != nil {
		return workflowSetupExit(deps, p.jsonOut, "workflow cancel", err)
	}
	defer runtime.close()
	result, cancelErr := engine.Cancel(ctx, p.args[0], p.reason, runtime.store)
	// A cooperative request waits for the coordinator to confirm; when the
	// executor is still draining, the request is already written and the run
	// stays running, so the command reports the accepted request.
	pendingRequest := workflowCancelPending(result, cancelErr)
	if cancelErr != nil && !pendingRequest {
		return workflowFailure(deps, p.jsonOut, runtime.secrets, "workflow cancel", cancelErr)
	}
	if err := render.RenderWorkflowCancel(deps.Stdout, result, p.jsonOut, runtime.secrets); err != nil {
		fmt.Fprintln(deps.Stderr, "error interno:", err)
		return ExitInternalError
	}
	if pendingRequest {
		return ExitOK
	}
	return workflowCancelExit(result, cancelErr)
}

// workflowCancelPending reports a cooperative request whose coordinator has not
// confirmed the shutdown yet: the request is written and the run keeps running.
func workflowCancelPending(result engine.CancelResult, err error) bool {
	return result.Requested && errors.Is(err, context.DeadlineExceeded)
}

// workflowCancelExit maps a finished cancellation: a run that already closed as
// cancelled keeps the reserved cancellation code.
func workflowCancelExit(result engine.CancelResult, err error) int {
	if err != nil {
		return workflowFailureExit(err)
	}
	if result.State == nil || result.State.Status == "cancelled" {
		return ExitCanceled
	}
	return ExitOK
}

// workflowSetupExit reports a failure that happened before touching the engine.
func workflowSetupExit(deps Dependencies, asJSON bool, command string, err error) int {
	var configErr workflowConfigError
	if errors.As(err, &configErr) {
		workflowRenderFailure(deps, asJSON, nil, command, "config_error", err.Error(), nil)
		return ExitConfigError
	}
	workflowRenderFailure(deps, asJSON, nil, command, "error", err.Error(), nil)
	return ExitInternalError
}

// workflowFailure reports a lifecycle or preflight error with its exit code.
func workflowFailure(deps Dependencies, asJSON bool, secrets []string, command string, err error) int {
	workflowRenderFailure(deps, asJSON, secrets, command, workflowFailureStatus(err), err.Error(), workflowValidationFailures(err))
	return workflowFailureExit(err)
}

// workflowRenderFailure writes the failure envelope and reports render errors.
func workflowRenderFailure(deps Dependencies, asJSON bool, secrets []string, command, status, message string, failures []schema.ValidationError) {
	if err := render.RenderWorkflowFailure(deps.Stdout, command, status, message, failures, asJSON, secrets); err != nil {
		fmt.Fprintln(deps.Stderr, "error interno:", err)
	}
}

// workflowFailureStatus names the envelope status of a failure.
func workflowFailureStatus(err error) string {
	var invalid *schema.ValidationErrors
	switch {
	case errors.As(err, &invalid):
		return "invalid"
	case errors.Is(err, store.ErrRunNotFound):
		return "not_found"
	case errors.Is(err, engine.ErrInvalidApproval):
		return "rejected"
	case errors.Is(err, engine.ErrConflict), errors.Is(err, engine.ErrTerminalRun),
		errors.Is(err, engine.ErrPendingApproval), errors.Is(err, store.ErrRunBusy):
		return "conflict"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "cancelled"
	default:
		return "error"
	}
}

// workflowValidationFailures exposes the redacted schema failures, if any.
func workflowValidationFailures(err error) []schema.ValidationError {
	var invalid *schema.ValidationErrors
	if errors.As(err, &invalid) {
		return invalid.Errors
	}
	return nil
}

// workflowStateExit maps a run state to the reserved workflow exit codes.
func workflowStateExit(status string) int {
	switch status {
	case "cancelled":
		return ExitCanceled
	case "awaiting_approval":
		return ExitAwaitingApproval
	case "paused":
		return ExitPaused
	case "partial":
		return ExitPartial
	case "blocked":
		return ExitBlocked
	case "failed":
		return ExitFailed
	default:
		return ExitOK
	}
}

// workflowFailureExit maps a lifecycle error to its reserved exit code.
func workflowFailureExit(err error) int {
	var invalid *schema.ValidationErrors
	switch {
	case err == nil:
		return ExitOK
	case errors.As(err, &invalid):
		return ExitConfigError
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ExitCanceled
	case errors.Is(err, store.ErrRunNotFound):
		return ExitConfigError
	case errors.Is(err, engine.ErrInvalidApproval):
		return ExitUsageError
	case errors.Is(err, engine.ErrConflict), errors.Is(err, engine.ErrTerminalRun),
		errors.Is(err, engine.ErrPendingApproval), errors.Is(err, store.ErrRunBusy):
		return ExitConflict
	default:
		return ExitInternalError
	}
}
