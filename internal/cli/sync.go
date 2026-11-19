package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/projectsync"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/synccatalog"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/syncfs"
)

const syncSchemaVersion = "1.0"

// syncOutput is the shared JSON envelope for catalog and sync commands. It only
// carries metadata: never raw file contents, never secrets.
type syncOutput struct {
	SchemaVersion string                      `json:"schema_version"`
	Status        string                      `json:"status"`
	Reason        string                      `json:"reason,omitempty"`
	Project       string                      `json:"project"`
	Targets       []string                    `json:"targets,omitempty"`
	Components    []componentSummary          `json:"components,omitempty"`
	ViewID        string                      `json:"view_id,omitempty"`
	OperationID   string                      `json:"operation_id,omitempty"`
	Changes       []projectsync.ChangeSummary `json:"changes,omitempty"`
	Diagnostics   []projectsync.Diagnostic    `json:"diagnostics,omitempty"`
}

type componentSummary struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Version string `json:"version"`
	Origin  string `json:"origin"`
	SHA256  string `json:"sha256"`
}

func summarizeEntry(entry synccatalog.Entry) componentSummary {
	return componentSummary{
		Kind:    string(entry.Kind),
		ID:      entry.ID,
		Version: entry.Version,
		Origin:  entry.Origin,
		SHA256:  entry.SHA256,
	}
}

func summarizeEntries(entries []synccatalog.Entry) []componentSummary {
	if len(entries) == 0 {
		return nil
	}
	out := make([]componentSummary, 0, len(entries))
	for _, entry := range entries {
		out = append(out, summarizeEntry(entry))
	}
	return out
}

// resolveProjectDir picks the project root: explicit flag, then the process
// working directory, then the current directory.
func resolveProjectDir(flagDir, workDir string) string {
	if flagDir != "" {
		return flagDir
	}
	if workDir != "" {
		return workDir
	}
	return "."
}

// emit writes the response as indented JSON on stdout when jsonMode is set, or
// as concise human-readable text otherwise. Errors go to stderr in text mode.
func emit(deps Dependencies, jsonMode bool, out syncOutput, text string) {
	out.SchemaVersion = syncSchemaVersion
	if jsonMode {
		encoder := json.NewEncoder(deps.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(out); err != nil {
			fmt.Fprintf(deps.Stderr, "error: encode json output: %v\n", err)
		}
		return
	}
	if text == "" {
		text = out.Status
	}
	fmt.Fprintln(deps.Stdout, text)
}

// fail emits a failure response and returns the exit code.
func fail(deps Dependencies, jsonMode bool, code int, status, reason string, out syncOutput) int {
	out.Status = status
	out.Reason = reason
	if jsonMode {
		emit(deps, true, out, "")
		return code
	}
	fmt.Fprintf(deps.Stderr, "error: %s\n", reason)
	return code
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

// parseComponentKind maps the CLI spelling of a component kind.
func parseComponentKind(raw string) (synccatalog.ComponentKind, bool) {
	switch synccatalog.ComponentKind(strings.TrimSpace(raw)) {
	case synccatalog.KindSkill:
		return synccatalog.KindSkill, true
	case synccatalog.KindRule:
		return synccatalog.KindRule, true
	case synccatalog.KindPattern:
		return synccatalog.KindPattern, true
	default:
		return "", false
	}
}

// parseDependency parses "<kind>:<id>@<version>".
func parseDependency(raw string) (synccatalog.Dependency, error) {
	trimmed := strings.TrimSpace(raw)
	at := strings.LastIndex(trimmed, "@")
	if at <= 0 || at == len(trimmed)-1 {
		return synccatalog.Dependency{}, fmt.Errorf("invalid --dep %q: expected <kind>:<id>@<version>", raw)
	}
	left, version := trimmed[:at], trimmed[at+1:]
	colon := strings.Index(left, ":")
	if colon <= 0 || colon == len(left)-1 {
		return synccatalog.Dependency{}, fmt.Errorf("invalid --dep %q: expected <kind>:<id>@<version>", raw)
	}
	kind, ok := parseComponentKind(left[:colon])
	if !ok {
		return synccatalog.Dependency{}, fmt.Errorf("invalid --dep %q: unknown kind %q", raw, left[:colon])
	}
	return synccatalog.Dependency{Kind: kind, ID: left[colon+1:], Version: version}, nil
}

// stringList collects repeatable flag values.
type stringList []string

func (l *stringList) String() string { return strings.Join(*l, ",") }

func (l *stringList) Set(value string) error {
	*l = append(*l, value)
	return nil
}

func runCatalog(args []string, deps Dependencies) int {
	if len(args) == 0 {
		return fail(deps, false, ExitUsageError, "rejected", "missing catalog subcommand (register|list)", syncOutput{})
	}
	switch args[0] {
	case "register":
		return runCatalogRegister(args[1:], deps)
	case "list":
		return runCatalogList(args[1:], deps)
	default:
		return fail(deps, false, ExitUsageError, "rejected", fmt.Sprintf("unknown catalog subcommand %q", args[0]), syncOutput{})
	}
}

func runCatalogRegister(args []string, deps Dependencies) int {
	fs := newFlagSet("catalog register", deps.Stderr)
	jsonMode := fs.Bool("json", false, "emit JSON output")
	projectDir := fs.String("project-dir", "", "project root")
	kind := fs.String("kind", "", "component kind (skill|rule|pattern)")
	id := fs.String("id", "", "component id")
	version := fs.String("version", "", "component version")
	source := fs.String("source", "", "component source path")
	origin := fs.String("origin", "", "component origin")
	var deps_ stringList
	fs.Var(&deps_, "dep", "dependency as <kind>:<id>@<version> (repeatable)")
	if err := fs.Parse(args); err != nil {
		return fail(deps, *jsonMode, ExitUsageError, "rejected", fmt.Sprintf("invalid flags: %v", err), syncOutput{})
	}

	project := resolveProjectDir(*projectDir, deps.WorkDir)
	out := syncOutput{Project: project}

	missing := make([]string, 0, 4)
	if *kind == "" {
		missing = append(missing, "--kind")
	}
	if *id == "" {
		missing = append(missing, "--id")
	}
	if *version == "" {
		missing = append(missing, "--version")
	}
	if *source == "" {
		missing = append(missing, "--source")
	}
	if len(missing) > 0 {
		return fail(deps, *jsonMode, ExitUsageError, "rejected", "missing required flags: "+strings.Join(missing, ", "), out)
	}

	componentKind, ok := parseComponentKind(*kind)
	if !ok {
		return fail(deps, *jsonMode, ExitUsageError, "rejected", fmt.Sprintf("invalid --kind %q (expected skill|rule|pattern)", *kind), out)
	}

	dependencies := make([]synccatalog.Dependency, 0, len(deps_))
	for _, raw := range deps_ {
		dependency, err := parseDependency(raw)
		if err != nil {
			return fail(deps, *jsonMode, ExitUsageError, "rejected", err.Error(), out)
		}
		dependencies = append(dependencies, dependency)
	}

	proj, err := syncfs.OpenProject(project)
	if err != nil {
		return failOpenProject(deps, *jsonMode, out, err)
	}
	defer proj.Close()

	catalog := synccatalog.New(proj)
	entry, err := catalog.Register(componentKind, *id, *version, *source, synccatalog.RegisterOptions{
		Origin:       *origin,
		Dependencies: dependencies,
	})
	if err != nil {
		return failCatalog(deps, *jsonMode, out, err)
	}

	out.Status = "registered"
	out.Components = []componentSummary{summarizeEntry(entry)}
	text := fmt.Sprintf("registered %s %s@%s (%s)", entry.Kind, entry.ID, entry.Version, entry.SHA256)
	emit(deps, *jsonMode, out, text)
	return ExitOK
}

func runCatalogList(args []string, deps Dependencies) int {
	fs := newFlagSet("catalog list", deps.Stderr)
	jsonMode := fs.Bool("json", false, "emit JSON output")
	projectDir := fs.String("project-dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return fail(deps, *jsonMode, ExitUsageError, "rejected", fmt.Sprintf("invalid flags: %v", err), syncOutput{})
	}

	project := resolveProjectDir(*projectDir, deps.WorkDir)
	out := syncOutput{Project: project}

	proj, err := syncfs.OpenProject(project)
	if err != nil {
		return failOpenProject(deps, *jsonMode, out, err)
	}
	defer proj.Close()

	catalog := synccatalog.New(proj)
	entries, err := catalog.List()
	if err != nil {
		return failCatalog(deps, *jsonMode, out, err)
	}

	out.Status = "ok"
	out.Components = summarizeEntries(entries)
	text := fmt.Sprintf("%d component(s) in catalog", len(entries))
	for _, entry := range entries {
		text += fmt.Sprintf("\n  %s %s@%s %s", entry.Kind, entry.ID, entry.Version, entry.SHA256)
	}
	emit(deps, *jsonMode, out, text)
	return ExitOK
}

func runSync(args []string, deps Dependencies) int {
	if len(args) == 0 {
		return fail(deps, false, ExitUsageError, "rejected", "missing sync subcommand (preview|apply|restore)", syncOutput{})
	}
	switch args[0] {
	case "preview":
		return runSyncPreview(args[1:], deps)
	case "apply":
		return runSyncApply(args[1:], deps)
	case "restore":
		return runSyncRestore(args[1:], deps)
	default:
		return fail(deps, false, ExitUsageError, "rejected", fmt.Sprintf("unknown sync subcommand %q", args[0]), syncOutput{})
	}
}

func runSyncPreview(args []string, deps Dependencies) int {
	fs := newFlagSet("sync preview", deps.Stderr)
	jsonMode := fs.Bool("json", false, "emit JSON output")
	projectDir := fs.String("project-dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return fail(deps, *jsonMode, ExitUsageError, "rejected", fmt.Sprintf("invalid flags: %v", err), syncOutput{})
	}

	project := resolveProjectDir(*projectDir, deps.WorkDir)
	out := syncOutput{Project: project}

	proj, err := syncfs.OpenProject(project)
	if err != nil {
		return failOpenProject(deps, *jsonMode, out, err)
	}
	defer proj.Close()

	selection, err := synccatalog.LoadSelection(proj)
	if err != nil {
		return failCatalog(deps, *jsonMode, out, err)
	}
	receipts, err := projectsync.LoadReceipts(proj)
	if err != nil {
		return failSync(deps, *jsonMode, out, err)
	}
	view, err := projectsync.Preview(proj, synccatalog.New(proj), selection, projectsync.DefaultVerifiedProfiles(), receipts)
	if err != nil {
		return failSync(deps, *jsonMode, out, err)
	}

	out.Targets = view.Targets
	out.Components = summarizeEntries(view.Components)
	out.Diagnostics = view.Diagnostics
	if !view.Applicable() {
		out.Status = "blocked"
		out.Reason = "preview has blocking diagnostics"
		out.ViewID = view.ID
		if *jsonMode {
			emit(deps, true, out, "")
			return ExitConfigError
		}
		fmt.Fprintf(deps.Stderr, "error: %s\n", out.Reason)
		for _, diagnostic := range view.Diagnostics {
			fmt.Fprintf(deps.Stderr, "  %s %s %s\n", diagnostic.Code, diagnostic.Path, diagnostic.Message)
		}
		return ExitConfigError
	}

	out.Status = "ok"
	out.ViewID = view.ID
	out.Changes = summarizeChanges(view.Changes)
	text := fmt.Sprintf("preview %s: %d change(s) across %s", view.ID, len(out.Changes), strings.Join(view.Targets, ", "))
	for _, change := range out.Changes {
		text += fmt.Sprintf("\n  %s %s", change.Action, change.Path)
	}
	emit(deps, *jsonMode, out, text)
	return ExitOK
}

func runSyncApply(args []string, deps Dependencies) int {
	fs := newFlagSet("sync apply", deps.Stderr)
	jsonMode := fs.Bool("json", false, "emit JSON output")
	projectDir := fs.String("project-dir", "", "project root")
	viewID := fs.String("view-id", "", "authorized preview view id")
	if err := fs.Parse(args); err != nil {
		return fail(deps, *jsonMode, ExitUsageError, "rejected", fmt.Sprintf("invalid flags: %v", err), syncOutput{})
	}

	project := resolveProjectDir(*projectDir, deps.WorkDir)
	out := syncOutput{Project: project}

	if *viewID == "" {
		return fail(deps, *jsonMode, ExitUsageError, "rejected", "missing --view-id authorization", out)
	}

	proj, err := syncfs.OpenProject(project)
	if err != nil {
		return failOpenProject(deps, *jsonMode, out, err)
	}
	defer proj.Close()

	selection, err := synccatalog.LoadSelection(proj)
	if err != nil {
		return failCatalog(deps, *jsonMode, out, err)
	}
	result, err := projectsync.Apply(proj, synccatalog.New(proj), selection, projectsync.DefaultVerifiedProfiles(), *viewID, projectsync.ApplyOptions{})
	out.ViewID = result.ViewID
	out.OperationID = result.OperationID
	out.Changes = result.Changes
	out.Diagnostics = result.Diagnostics
	if err != nil {
		return failSync(deps, *jsonMode, out, err)
	}

	out.Status = result.Status
	out.Reason = result.Reason
	text := fmt.Sprintf("%s: %d change(s) (operation %s)", result.Status, len(result.Changes), result.OperationID)
	for _, change := range result.Changes {
		text += fmt.Sprintf("\n  %s %s", change.Action, change.Path)
	}
	emit(deps, *jsonMode, out, text)
	return ExitOK
}

func runSyncRestore(args []string, deps Dependencies) int {
	fs := newFlagSet("sync restore", deps.Stderr)
	jsonMode := fs.Bool("json", false, "emit JSON output")
	projectDir := fs.String("project-dir", "", "project root")
	operationID := fs.String("operation-id", "", "operation id to restore")
	if err := fs.Parse(args); err != nil {
		return fail(deps, *jsonMode, ExitUsageError, "rejected", fmt.Sprintf("invalid flags: %v", err), syncOutput{})
	}

	project := resolveProjectDir(*projectDir, deps.WorkDir)
	out := syncOutput{Project: project}

	if *operationID == "" {
		return fail(deps, *jsonMode, ExitUsageError, "rejected", "missing --operation-id", out)
	}

	proj, err := syncfs.OpenProject(project)
	if err != nil {
		return failOpenProject(deps, *jsonMode, out, err)
	}
	defer proj.Close()

	result, err := projectsync.Restore(proj, *operationID, projectsync.RestoreOptions{})
	out.OperationID = result.OperationID
	out.Changes = result.Changes
	out.Diagnostics = result.Diagnostics
	if err != nil {
		return failSync(deps, *jsonMode, out, err)
	}

	out.Status = result.Status
	out.Reason = result.Reason
	text := fmt.Sprintf("%s: %d change(s) (operation %s)", result.Status, len(result.Changes), result.OperationID)
	for _, change := range result.Changes {
		text += fmt.Sprintf("\n  %s %s", change.Action, change.Path)
	}
	emit(deps, *jsonMode, out, text)
	return ExitOK
}

// summarizeChanges maps preview changes to the metadata-only summary shape so
// file contents never reach the output.
func summarizeChanges(changes []projectsync.Change) []projectsync.ChangeSummary {
	if len(changes) == 0 {
		return nil
	}
	out := make([]projectsync.ChangeSummary, 0, len(changes))
	for _, change := range changes {
		out = append(out, projectsync.ChangeSummary{
			Path:         change.Path,
			Action:       change.Action,
			Targets:      change.Targets,
			BeforeSHA256: change.Before.SHA256,
			AfterSHA256:  change.After.SHA256,
		})
	}
	return out
}

func failOpenProject(deps Dependencies, jsonMode bool, out syncOutput, err error) int {
	if errors.Is(err, syncfs.ErrUnsafePath) {
		return fail(deps, jsonMode, ExitAccessDenied, "rejected", "unsafe project path", out)
	}
	return fail(deps, jsonMode, ExitConfigError, "rejected", fmt.Sprintf("open project: %v", err), out)
}

func failCatalog(deps Dependencies, jsonMode bool, out syncOutput, err error) int {
	switch {
	case errors.Is(err, synccatalog.ErrInvalidInput):
		return fail(deps, jsonMode, ExitUsageError, "rejected", err.Error(), out)
	case errors.Is(err, synccatalog.ErrConflict):
		return fail(deps, jsonMode, ExitConfigError, "rejected", err.Error(), out)
	case errors.Is(err, synccatalog.ErrNotFound):
		return fail(deps, jsonMode, ExitConfigError, "rejected", err.Error(), out)
	case errors.Is(err, synccatalog.ErrIntegrity):
		return fail(deps, jsonMode, ExitConfigError, "rejected", err.Error(), out)
	case errors.Is(err, syncfs.ErrUnsafePath):
		return fail(deps, jsonMode, ExitAccessDenied, "rejected", "unsafe path", out)
	case errors.Is(err, syncfs.ErrBusy), errors.Is(err, syncfs.ErrConflict):
		return fail(deps, jsonMode, ExitConfigError, "rejected", err.Error(), out)
	default:
		return fail(deps, jsonMode, ExitInternalError, "error", err.Error(), out)
	}
}

func failSync(deps Dependencies, jsonMode bool, out syncOutput, err error) int {
	switch {
	case errors.Is(err, projectsync.ErrUnauthorized):
		return fail(deps, jsonMode, ExitUsageError, "rejected", err.Error(), out)
	case errors.Is(err, projectsync.ErrStaleView),
		errors.Is(err, projectsync.ErrRecoveryPending),
		errors.Is(err, projectsync.ErrRestoreConflict),
		errors.Is(err, syncfs.ErrBusy),
		errors.Is(err, syncfs.ErrConflict),
		errors.Is(err, os.ErrNotExist):
		return fail(deps, jsonMode, ExitConfigError, "rejected", err.Error(), out)
	case errors.Is(err, syncfs.ErrUnsafePath):
		return fail(deps, jsonMode, ExitAccessDenied, "rejected", "unsafe path", out)
	case errors.Is(err, synccatalog.ErrInvalidInput):
		return fail(deps, jsonMode, ExitUsageError, "rejected", err.Error(), out)
	default:
		return fail(deps, jsonMode, ExitInternalError, "error", err.Error(), out)
	}
}
