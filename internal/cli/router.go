package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/render"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router/antigravity"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router/direct"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router/vibe"
)

// routerFlags holds the parsed flags of one router command.
type routerFlags struct {
	jsonOut    bool
	complexity string
	risk       string
	profile    string
	project    string
	caller     string
	romeroURL  string
	timeout    string
	args       []string
}

// commandFlags adapts the router flags to the shared config loader.
func (p *routerFlags) commandFlags() *commandFlags {
	return &commandFlags{
		project:   p.project,
		caller:    p.caller,
		romeroURL: p.romeroURL,
		timeout:   p.timeout,
		jsonOut:   p.jsonOut,
		args:      p.args,
	}
}

// parseRouterFlags parses the router command flags; withProfile enables the
// manual profile flag reserved for `run`.
func parseRouterFlags(name string, args []string, out io.Writer, withProfile bool) (*routerFlags, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(out)
	p := &routerFlags{}
	fs.BoolVar(&p.jsonOut, "json", false, "salida JSON estable")
	fs.StringVar(&p.complexity, "complexity", "", "complejidad de la tarea (baja, media, alta)")
	fs.StringVar(&p.risk, "risk", "", "riesgo de la tarea (bajo, medio, alto)")
	if withProfile {
		fs.StringVar(&p.profile, "profile", "", "perfil manual del router")
	}
	fs.StringVar(&p.project, "project", "", "proyecto activo")
	fs.StringVar(&p.caller, "caller", "", "identidad del solicitante")
	fs.StringVar(&p.romeroURL, "romero-url", "", "dirección de ROMERO")
	fs.StringVar(&p.timeout, "timeout", "", "tiempo límite de ROMERO")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	p.args = fs.Args()
	return p, nil
}

// featuresFrom validates the feature flags and applies the media/medio
// defaults of the specification.
func featuresFrom(p *routerFlags) (router.Features, error) {
	f := router.Features{Complexity: router.ComplexityMedium, Risk: router.RiskMedium}
	switch p.complexity {
	case "":
	case string(router.ComplexityLow), string(router.ComplexityMedium), string(router.ComplexityHigh):
		f.Complexity = router.Complexity(p.complexity)
	default:
		return f, fmt.Errorf("complejidad inválida %q (baja, media o alta)", p.complexity)
	}
	switch p.risk {
	case "":
	case string(router.RiskLow), string(router.RiskMedium), string(router.RiskHigh):
		f.Risk = router.Risk(p.risk)
	default:
		return f, fmt.Errorf("riesgo inválido %q (bajo, medio o alto)", p.risk)
	}
	return f, nil
}

// requestedEffortFor maps a complexity to the effort key it requests.
func requestedEffortFor(c router.Complexity) string {
	switch c {
	case router.ComplexityLow:
		return config.RouterEffortLow
	case router.ComplexityHigh:
		return config.RouterEffortHigh
	default:
		return config.RouterEffortMedium
	}
}

// routerRegistryFor resolves the adapter registry: the injected one wins;
// otherwise the production adapters are wired, with the direct API adapter
// serving every api executor of the policy.
func (deps Dependencies) routerRegistryFor(policy config.RouterPolicy) *router.Registry {
	if deps.RouterRegistry != nil {
		return deps.RouterRegistry
	}
	reg := router.NewRegistry()
	reg.Register("antigravity", antigravity.New())
	reg.Register("vibe", vibe.New())
	api := &direct.Adapter{Getenv: deps.getenv}
	for _, ex := range policy.Executors {
		if ex.Kind == config.RouterKindAPI {
			reg.Register(ex.ID, api)
		}
	}
	return reg
}

// runModels handles `gandalf models <list> [--json]`.
func runModels(ctx context.Context, args []string, deps Dependencies) int {
	if len(args) == 0 || args[0] != "list" {
		fmt.Fprintf(deps.Stderr, "error: subcomando de models desconocido (solo list)\n\n%s", usageText)
		return ExitUsageError
	}
	p, err := parseRouterFlags("gandalf models list", args[1:], deps.Stderr, false)
	if err != nil {
		return ExitUsageError
	}
	cfg, err := loadConfig(deps, p.commandFlags())
	if err != nil {
		fmt.Fprintln(deps.Stderr, err.Error())
		return ExitConfigError
	}
	policy := cfg.Router
	reg := deps.routerRegistryFor(policy)
	catalog := router.Build(ctx, policy, reg, router.DiscoveryLocalAndDocumentedFree)
	profiles := make([]router.ProfileView, 0, len(catalog.Profiles)+len(catalog.Inventory))
	profiles = append(profiles, catalog.Profiles...)
	profiles = append(profiles, catalog.Inventory...)
	if err := render.RenderModelsList(deps.Stdout, profiles, catalog.Executors, p.jsonOut); err != nil {
		fmt.Fprintln(deps.Stderr, "error interno:", err)
		return ExitInternalError
	}
	return ExitOK
}

// runRoute handles `gandalf route [--json] [--complexity] [--risk] <tarea>`.
// It never executes an executor: the catalog is local-only and selection
// is pure.
func runRoute(ctx context.Context, args []string, deps Dependencies) int {
	p, err := parseRouterFlags("gandalf route", args, deps.Stderr, false)
	if err != nil {
		return ExitUsageError
	}
	features, ferr := featuresFrom(p)
	if ferr != nil {
		fmt.Fprintln(deps.Stderr, "error:", ferr.Error())
		return ExitUsageError
	}
	task := strings.Join(p.args, " ")
	if strings.TrimSpace(task) == "" {
		fmt.Fprintln(deps.Stderr, "error: route requiere una tarea")
		return ExitUsageError
	}
	cfg, err := loadConfig(deps, p.commandFlags())
	if err != nil {
		fmt.Fprintln(deps.Stderr, err.Error())
		return ExitConfigError
	}
	policy := cfg.Router
	reg := deps.routerRegistryFor(policy)
	catalog := router.Build(ctx, policy, reg, router.DiscoveryLocalOnly)
	decision := router.Select(catalog, features, "")
	if err := render.RenderRoute(deps.Stdout, routeView(features, decision), p.jsonOut); err != nil {
		fmt.Fprintln(deps.Stderr, "error interno:", err)
		return ExitInternalError
	}
	if len(decision.Ordered) == 0 {
		return ExitNoEligible
	}
	return ExitOK
}

// routeView maps a selection decision to the route payload.
func routeView(f router.Features, d router.Decision) render.RouteJSON {
	view := render.RouteJSON{
		SchemaVersion: render.SchemaVersion,
		Command:       "route",
		Status:        "ok",
		Criteria:      fmt.Sprintf("complejidad %s → esfuerzo %s; riesgo %s; solo perfiles autorizados y disponibles", f.Complexity, requestedEffortFor(f.Complexity), f.Risk),
		Alternatives:  []render.RouteAlternative{},
	}
	if len(d.Ordered) > 0 {
		view.Profile = d.Ordered[0].Profile.ID
		view.Effort = d.Ordered[0].Effort
		if len(d.Ordered) > 1 {
			view.Fallback = d.Ordered[1].Profile.ID
		}
	} else {
		view.Status = "no-eligible"
	}
	for _, ch := range d.Ordered[min(1, len(d.Ordered)):] {
		view.Alternatives = append(view.Alternatives, render.RouteAlternative{
			Profile: ch.Profile.ID,
			Status:  "elegible",
			Detail:  ch.Explanation,
		})
	}
	for _, a := range d.Considered {
		if a.Eligible {
			continue
		}
		view.Alternatives = append(view.Alternatives, render.RouteAlternative{
			Profile: a.ProfileID,
			Status:  "descartado",
			Detail:  a.Reason,
		})
	}
	return view
}

// runRun handles `gandalf run [--json] [--complexity] [--risk] [--profile] <tarea>`.
func runRun(ctx context.Context, args []string, deps Dependencies) int {
	p, err := parseRouterFlags("gandalf run", args, deps.Stderr, true)
	if err != nil {
		return ExitUsageError
	}
	features, ferr := featuresFrom(p)
	if ferr != nil {
		fmt.Fprintln(deps.Stderr, "error:", ferr.Error())
		return ExitUsageError
	}
	task := strings.Join(p.args, " ")
	if strings.TrimSpace(task) == "" {
		fmt.Fprintln(deps.Stderr, "error: run requiere una tarea")
		return ExitUsageError
	}
	cfg, err := loadConfig(deps, p.commandFlags())
	if err != nil {
		fmt.Fprintln(deps.Stderr, err.Error())
		return ExitConfigError
	}
	policy := cfg.Router
	reg := deps.routerRegistryFor(policy)
	catalog := router.Build(ctx, policy, reg, router.DiscoveryLocalOnly)
	decision := router.Select(catalog, features, p.profile)
	result := router.Run(ctx, policy, decision, task, reg, deps.getenv)
	view := runView(result)
	if result.Status == router.StatusNoEligible {
		view.Explanation = noEligibleExplanation(decision)
	}
	if err := render.RenderRun(deps.Stdout, view, p.jsonOut); err != nil {
		fmt.Fprintln(deps.Stderr, "error interno:", err)
		return ExitInternalError
	}
	switch result.Status {
	case router.StatusOK:
		return ExitOK
	case router.StatusNoEligible:
		return ExitNoEligible
	case router.StatusExhausted:
		return ExitExhausted
	case router.StatusCanceled:
		return ExitCanceled
	default:
		return ExitInternalError
	}
}

// runView maps an orchestration result to the run payload.
func runView(r router.Result) render.RunJSON {
	view := render.RunJSON{
		SchemaVersion: render.SchemaVersion,
		Command:       "run",
		Status:        string(r.Status),
		Profile:       r.Profile.ID,
		Effort:        r.Effort,
		Attempts:      []render.RunAttempt{},
		Explanation:   r.Explanation,
		Output:        r.Output,
	}
	for _, a := range r.Attempts {
		view.Attempts = append(view.Attempts, render.RunAttempt{
			Profile: a.Profile.ID,
			Effort:  a.Effort,
			Outcome: string(a.Outcome),
			Reason:  a.Reason,
		})
	}
	return view
}

// noEligibleExplanation names the discard reason of every considered
// profile, in catalog order.
func noEligibleExplanation(d router.Decision) string {
	var reasons []string
	for _, a := range d.Considered {
		if !a.Eligible {
			reasons = append(reasons, fmt.Sprintf("%s: %s", a.ProfileID, a.Reason))
		}
	}
	if len(reasons) == 0 {
		return "sin perfiles elegibles"
	}
	return "sin perfiles elegibles — " + strings.Join(reasons, "; ")
}
