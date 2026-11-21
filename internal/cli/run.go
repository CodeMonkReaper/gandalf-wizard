package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/doctor"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/harness"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/render"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/romero"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router"
)

const usageText = `Uso: gandalf <subcomando> [flags]

Subcomandos:
  help                         Muestra esta ayuda
  version [--json]             Muestra la versión
  config show [--json]         Muestra la configuración efectiva y su origen
  config init --project <id>   Crea la configuración del proyecto (nunca sobrescribe)
  doctor [--json]              Diagnostica el entorno (solo lectura)
  memory query [--json] <pregunta>  Consulta la memoria del proyecto en ROMERO
  models list [--json]         Muestra el catálogo de perfiles del router
  route [--json] [--complexity <baja|media|alta>] [--risk <bajo|medio|alto>] <tarea>
                               Elige un perfil y explica por qué (no ejecuta)
  run [--json] [--complexity <baja|media|alta>] [--risk <bajo|medio|alto>] [--profile <perfil>] <tarea>
                               Ejecuta la tarea con el perfil elegido y fallback
  catalog register [--json] --kind <skill|rule|pattern> --id <id> --version <ver> --source <ruta>
                               Registra un componente inmutable en el catálogo local
  catalog list [--json]        Lista las entradas registradas en el catálogo local
  sync preview [--json]        Calcula la vista previa determinista de sincronización (solo lectura)
  sync apply [--json] --view-id <id>
                               Aplica una vista previa autorizada con respaldo privado
  sync restore [--json] --operation-id <id>
                               Restaura una operación aplicada o interrumpida
  workflow validate [--json] <definición>
                               Preflight de solo lectura de una definición de workflow
  workflow run [--json] <definición>
                               Ejecuta la definición y conserva checkpoints en .gandalf/
  workflow status [--json] <run-id>
                               Consulta el estado de una corrida
  workflow approve [--json] <run-id> --gate <id> --checkpoint <huella> --decision <approve|reject> [--reason <texto>]
                               Registra la decisión del operador sobre una puerta
  workflow resume [--json] <run-id>
                               Reanuda una corrida reanudable sin repetir pasos concluidos
  workflow cancel [--json] <run-id> [--reason <texto>]
                               Cancela una corrida no terminal

Flags de anulación (config show, doctor, memory query, models, route, run):
  --project <id>        Proyecto activo para esta ejecución
  --caller <id>         Identidad del solicitante
  --romero-url <url>    Dirección de ROMERO
  --timeout <duración>  Tiempo límite de ROMERO (p. ej. 10s)
  --json                Salida JSON estable
`

// Run dispatches a Gandalf CLI invocation and returns the mapped exit code.
func Run(ctx context.Context, args []string, deps Dependencies) int {
	if len(args) == 0 {
		fmt.Fprint(deps.Stderr, usageText)
		return ExitUsageError
	}

	switch args[0] {
	case "help", "--help", "-h":
		fmt.Fprint(deps.Stdout, usageText)
		return ExitOK
	case "version":
		return runVersion(args[1:], deps)
	case "config":
		return runConfig(args[1:], deps)
	case "doctor":
		return runDoctor(ctx, args[1:], deps)
	case "memory":
		return runMemory(ctx, args[1:], deps)
	case "models":
		return runModels(ctx, args[1:], deps)
	case "route":
		return runRoute(ctx, args[1:], deps)
	case "run":
		return runRun(ctx, args[1:], deps)
	case "catalog":
		return runCatalog(args[1:], deps)
	case "sync":
		return runSync(args[1:], deps)
	case "workflow":
		return runWorkflow(ctx, args[1:], deps)
	default:
		fmt.Fprintf(deps.Stderr, "error: subcomando desconocido %q\n\n%s", args[0], usageText)
		return ExitUsageError
	}
}

// commandFlags holds the parsed per-invocation flags.
type commandFlags struct {
	project   string
	caller    string
	romeroURL string
	timeout   string
	jsonOut   bool
	args      []string
}

// parseCommandFlags parses the shared flag set; allowOverrides enables the
// per-execution value overrides. Parse errors go to out (stderr) and return err.
func parseCommandFlags(name string, args []string, out io.Writer, allowOverrides bool) (*commandFlags, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(out)
	p := &commandFlags{}
	fs.BoolVar(&p.jsonOut, "json", false, "salida JSON estable")
	if allowOverrides {
		fs.StringVar(&p.project, "project", "", "proyecto activo")
		fs.StringVar(&p.caller, "caller", "", "identidad del solicitante")
		fs.StringVar(&p.romeroURL, "romero-url", "", "dirección de ROMERO")
		fs.StringVar(&p.timeout, "timeout", "", "tiempo límite de ROMERO")
	}
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	p.args = fs.Args()
	return p, nil
}

// getenv resolves the environment hook with an os.Getenv fallback.
func (deps Dependencies) getenv(key string) string {
	if deps.Getenv == nil {
		return os.Getenv(key)
	}
	return deps.Getenv(key)
}

// loadConfig resolves the effective configuration applying flag overrides.
func loadConfig(deps Dependencies, p *commandFlags) (config.EffectiveConfig, error) {
	return config.Load(config.LoadInput{
		WorkDir:       deps.WorkDir,
		UserConfigDir: deps.UserConfigDir,
		Getenv:        deps.Getenv,
		DefaultCaller: deps.DefaultCaller,
		Overrides: config.FlagOverrides{
			Project:       p.project,
			Caller:        p.caller,
			RomeroBaseURL: p.romeroURL,
			RomeroTimeout: p.timeout,
		},
	})
}

// authTokenProvider reads the ROMERO credential from the environment on each call.
func (deps Dependencies) authTokenProvider() func() string {
	return func() string {
		return deps.getenv(config.EnvKeyRomeroAPIKey)
	}
}

// readPort returns the injected read port or builds a REST adapter.
func (deps Dependencies) readPort(cfg config.EffectiveConfig) romero.RomeroReadPort {
	if deps.ReadPort != nil {
		return deps.ReadPort
	}
	return romero.NewRomeroPublicClientAdapter(cfg.RomeroBaseURL.Value, deps.HTTPClient, deps.authTokenProvider())
}

// statusPort returns the injected status port or builds a REST adapter.
func (deps Dependencies) statusPort(cfg config.EffectiveConfig) romero.RomeroStatusPort {
	if deps.StatusPort != nil {
		return deps.StatusPort
	}
	return romero.NewRomeroPublicClientAdapter(cfg.RomeroBaseURL.Value, deps.HTTPClient, deps.authTokenProvider())
}

// runVersion handles `gandalf version [--json]`.
func runVersion(args []string, deps Dependencies) int {
	p, err := parseCommandFlags("gandalf version", args, deps.Stderr, false)
	if err != nil {
		return ExitUsageError
	}
	if err := render.RenderVersion(deps.Stdout, render.VersionView{Version: deps.Version}, p.jsonOut); err != nil {
		fmt.Fprintln(deps.Stderr, "error interno:", err)
		return ExitInternalError
	}
	return ExitOK
}

// runConfig handles `gandalf config <show|init>`.
func runConfig(args []string, deps Dependencies) int {
	if len(args) == 0 {
		fmt.Fprintln(deps.Stderr, "error: falta el subcomando de config (show o init)")
		fmt.Fprint(deps.Stderr, usageText)
		return ExitUsageError
	}
	switch args[0] {
	case "show":
		return runConfigShow(args[1:], deps)
	case "init":
		return runConfigInit(args[1:], deps)
	default:
		fmt.Fprintf(deps.Stderr, "error: subcomando de config desconocido %q\n\n%s", args[0], usageText)
		return ExitUsageError
	}
}

// runConfigShow handles `gandalf config show [--json]`.
func runConfigShow(args []string, deps Dependencies) int {
	p, err := parseCommandFlags("gandalf config show", args, deps.Stderr, true)
	if err != nil {
		return ExitUsageError
	}
	cfg, err := loadConfig(deps, p)
	if err != nil {
		fmt.Fprintln(deps.Stderr, err.Error())
		return ExitConfigError
	}
	if err := render.RenderConfigShow(deps.Stdout, buildConfigShowView(cfg), p.jsonOut); err != nil {
		fmt.Fprintln(deps.Stderr, "error interno:", err)
		return ExitInternalError
	}
	return ExitOK
}

// buildConfigShowView maps the effective configuration to the show view.
func buildConfigShowView(cfg config.EffectiveConfig) render.ConfigShowView {
	credential := "ausente"
	if cfg.APIKeyPresent {
		credential = "presente"
	}
	return render.ConfigShowView{
		Entries: []render.ConfigEntry{
			{Key: "project", Value: cfg.Project.Value, Origin: string(cfg.Project.Origin)},
			{Key: "caller", Value: cfg.Caller.Value, Origin: string(cfg.Caller.Origin)},
			{Key: "romero.base_url", Value: cfg.RomeroBaseURL.Value, Origin: string(cfg.RomeroBaseURL.Origin)},
			{Key: "romero.timeout", Value: cfg.RomeroTimeout.Value.String(), Origin: string(cfg.RomeroTimeout.Origin)},
			{Key: "credencial (ROMERO_API_KEY)", Value: credential, Origin: string(cfg.APIKeyOrigin)},
		},
		Warnings: cfg.Warnings,
	}
}

// runConfigInit handles `gandalf config init --project <id>`.
func runConfigInit(args []string, deps Dependencies) int {
	fs := flag.NewFlagSet("gandalf config init", flag.ContinueOnError)
	fs.SetOutput(deps.Stderr)
	project := fs.String("project", "", "identificador del proyecto")
	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if *project == "" {
		fmt.Fprintln(deps.Stderr, "error: config init requiere --project <id>")
		return ExitUsageError
	}
	path, err := config.InitProjectConfig(deps.WorkDir, *project)
	if err != nil {
		fmt.Fprintln(deps.Stderr, err.Error())
		return ExitConfigError
	}
	fmt.Fprintf(deps.Stdout, "Configuración creada: %s\n", path)
	return ExitOK
}

// runDoctor handles `gandalf doctor [--json]`.
func runDoctor(ctx context.Context, args []string, deps Dependencies) int {
	p, err := parseCommandFlags("gandalf doctor", args, deps.Stderr, true)
	if err != nil {
		return ExitUsageError
	}

	cfg, cfgErr := loadConfig(deps, p)
	harnessReport := harness.Detect(deps.WorkDir, deps.LookPath)

	input := doctor.Input{
		ConfigLoadErr: cfgErr,
		Effective:     cfg,
		APIKeyPresent: deps.getenv(config.EnvKeyRomeroAPIKey) != "",
		StatusPort:    deps.statusPort(cfg),
		HarnessReport: harnessReport,
	}
	if cfgErr == nil {
		// The doctor only reports the router executors' local state; the
		// local-only catalog probes without spending quota.
		reg := deps.routerRegistryFor(cfg.Router)
		catalog := router.Build(ctx, cfg.Router, reg, router.DiscoveryLocalOnly)
		input.RouterExecutors = catalog.Executors
	}

	dctx := ctx
	if cfgErr == nil && cfg.RomeroTimeout.Value > 0 {
		var cancel context.CancelFunc
		dctx, cancel = context.WithTimeout(ctx, cfg.RomeroTimeout.Value)
		defer cancel()
	}

	report := doctor.Run(dctx, input)
	if err := render.RenderDoctor(deps.Stdout, report, p.jsonOut); err != nil {
		fmt.Fprintln(deps.Stderr, "error interno:", err)
		return ExitInternalError
	}
	if report.Failures > 0 {
		return ExitDoctorFailed
	}
	return ExitOK
}

// runMemory handles `gandalf memory query [--json] <pregunta>`.
func runMemory(ctx context.Context, args []string, deps Dependencies) int {
	if len(args) == 0 || args[0] != "query" {
		fmt.Fprintf(deps.Stderr, "error: subcomando de memory desconocido (solo query)\n\n%s", usageText)
		return ExitUsageError
	}
	p, err := parseCommandFlags("gandalf memory query", args[1:], deps.Stderr, true)
	if err != nil {
		return ExitUsageError
	}
	question := strings.Join(p.args, " ")
	if strings.TrimSpace(question) == "" {
		fmt.Fprintln(deps.Stderr, "error: memory query requiere una pregunta")
		return ExitUsageError
	}

	cfg, err := loadConfig(deps, p)
	if err != nil {
		fmt.Fprintln(deps.Stderr, err.Error())
		return ExitConfigError
	}
	if cfg.Project.Value == "" {
		fmt.Fprintln(deps.Stderr, "Identifica el proyecto activo antes de consultar ROMERO")
		return ExitConfigError
	}

	orchestrator := romero.NewMemoryQueryOrchestrator(deps.readPort(cfg))
	qctx := ctx
	if cfg.RomeroTimeout.Value > 0 {
		var cancel context.CancelFunc
		qctx, cancel = context.WithTimeout(ctx, cfg.RomeroTimeout.Value)
		defer cancel()
	}
	ws := romero.WorkspaceContext{
		ActiveProjectID: cfg.Project.Value,
		CallerIdentity:  cfg.Caller.Value,
	}
	outcome := orchestrator.QueryProjectMemory(qctx, ws, question)
	view := romero.Present(outcome)

	if err := render.RenderMemoryQuery(deps.Stdout, render.MemoryQueryView{
		Project: cfg.Project.Value,
		Outcome: outcome,
		View:    view,
	}, p.jsonOut); err != nil {
		fmt.Fprintln(deps.Stderr, "error interno:", err)
		return ExitInternalError
	}

	switch outcome.Status {
	case romero.StatusOK, romero.StatusEmpty:
		return ExitOK
	case romero.StatusAccessDenied:
		return ExitAccessDenied
	case romero.StatusPartial:
		return ExitPartialQuery
	case romero.StatusIncomplete:
		return ExitIncompleteQuery
	case romero.StatusScopeRequired:
		return ExitConfigError
	default:
		return ExitInternalError
	}
}
