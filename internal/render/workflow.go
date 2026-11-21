package render

import (
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strings"
	"unicode"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/engine"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/gates"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/store"
)

// workflowOutput projects the CLI contract without disclosing the definition
// path, provider inputs, or the bodies of persisted artifacts.
type workflowOutput struct {
	SchemaVersion    string             `json:"schema_version"`
	Command          string             `json:"command"`
	Status           string             `json:"status"`
	WorkflowID       string             `json:"workflow_id,omitempty"`
	WorkflowVersion  string             `json:"workflow_version,omitempty"`
	Objective        *workflowObjective `json:"objective,omitempty"`
	ConcurrencyLimit int                `json:"concurrency_limit,omitempty"`
	RequiresMemory   *bool              `json:"requires_memory,omitempty"`
	RequiresRouter   *bool              `json:"requires_router,omitempty"`
	RunID            string             `json:"run_id,omitempty"`
	RunStatus        string             `json:"run_status,omitempty"`
	StateRevision    int64              `json:"state_revision,omitempty"`
	CheckpointDigest string             `json:"checkpoint_digest,omitempty"`
	// Requested is set only by workflow cancel; the pointer keeps a direct
	// cancellation (requested false) visible instead of omitting the field.
	Requested        *bool              `json:"requested,omitempty"`
	Steps            []workflowStep     `json:"steps"`
	PendingGates     []string           `json:"pending_gates,omitempty"`
	Gates            []workflowGate     `json:"gates,omitempty"`
	Approvals        []workflowApproval `json:"approvals,omitempty"`
	ObjectiveVerdict *workflowVerdict   `json:"objective_verdict,omitempty"`
	Warnings         []string           `json:"warnings,omitempty"`
	Error            string             `json:"error,omitempty"`
}

type workflowObjective struct {
	ID              string                  `json:"id"`
	Version         string                  `json:"version"`
	Statement       string                  `json:"statement,omitempty"`
	Constraints     []string                `json:"constraints,omitempty"`
	Deliverables    []schema.DeliverableRef `json:"deliverables,omitempty"`
	SuccessCriteria []workflowCriterion     `json:"success_criteria,omitempty"`
}

type workflowCriterion struct {
	ID          string   `json:"id"`
	Deliverable string   `json:"deliverable"`
	Rules       []string `json:"rules"`
}

type workflowStep struct {
	ID         string                 `json:"id"`
	Kind       string                 `json:"kind"`
	DependsOn  []string               `json:"depends_on,omitempty"`
	Optional   bool                   `json:"optional,omitempty"`
	Correction *schema.CorrectionSpec `json:"correction,omitempty"`
	Gate       *workflowGatePlan      `json:"gate,omitempty"`
	Status     string                 `json:"status,omitempty"`
	Iteration  int                    `json:"iteration,omitempty"`
	Attempts   []workflowAttempt      `json:"attempts,omitempty"`
	Artifacts  []workflowArtifact     `json:"artifacts,omitempty"`
	Warnings   []string               `json:"warnings,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

type workflowGatePlan struct {
	Target      string   `json:"target"`
	Rules       []string `json:"rules"`
	Informative bool     `json:"informative,omitempty"`
}

type workflowAttempt struct {
	Iteration  int                `json:"iteration"`
	Status     string             `json:"status"`
	GateStatus string             `json:"gate_status,omitempty"`
	Diagnosis  string             `json:"diagnosis,omitempty"`
	Error      string             `json:"error,omitempty"`
	Artifacts  []workflowArtifact `json:"artifacts,omitempty"`
}

type workflowArtifact struct {
	Name    string `json:"name"`
	RelPath string `json:"rel_path,omitempty"`
	// RelPathWithheld marks an artifact that records a path the run space does
	// not admit, so a reader can tell "no path" from "path rejected".
	RelPathWithheld bool   `json:"rel_path_withheld,omitempty"`
	SHA256          string `json:"sha256"`
	SizeBytes       int64  `json:"size_bytes"`
	Summary         string `json:"summary"`
}

type workflowGate struct {
	StepID      string   `json:"step_id"`
	Target      string   `json:"target,omitempty"`
	Status      string   `json:"status"`
	Informative bool     `json:"informative,omitempty"`
	Violations  []string `json:"violations,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

type workflowApproval struct {
	GateID           string `json:"gate_id"`
	Decision         string `json:"decision"`
	Reason           string `json:"reason,omitempty"`
	CheckpointDigest string `json:"checkpoint_digest"`
	DecidedAt        string `json:"decided_at"`
}

type workflowVerdict struct {
	Status         string   `json:"status"`
	MetCriteria    []string `json:"met_criteria,omitempty"`
	FailedCriteria []string `json:"failed_criteria,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

type workflowRedactor struct {
	secrets []string
}

func (r workflowRedactor) text(value string) string {
	clean := gates.Redact(value, r.secrets)
	// Control characters have no place in a terminal cell. Check the result
	// again because removing them can join portions of a credential.
	plain := strings.Map(func(ch rune) rune {
		switch {
		case unicode.IsControl(ch) && unicode.IsSpace(ch):
			return ' '
		case unicode.IsControl(ch):
			return -1
		default:
			return ch
		}
	}, clean)
	if plain != clean {
		return gates.Redact(plain, r.secrets)
	}
	return clean
}

func (r workflowRedactor) list(values []string) []string {
	clean := make([]string, len(values))
	for i, value := range values {
		clean[i] = r.text(value)
	}
	return clean
}

func (r workflowRedactor) ruleKinds(rules []schema.GateRuleSpec) []string {
	kinds := make([]string, len(rules))
	for i, rule := range rules {
		kinds[i] = r.text(string(rule.Kind))
	}
	return kinds
}

// RenderWorkflowValidate writes the read-only preflight summary and graph.
func RenderWorkflowValidate(w io.Writer, view engine.ValidateReport, asJSON bool, secretValues []string) error {
	r := workflowRedactor{secrets: secretValues}
	wf := view.Workflow
	def := wf.Definition
	obj := &workflowObjective{
		ID: r.text(def.Objective.ID), Version: r.text(def.Objective.Version),
		Statement: r.text(def.Objective.Statement), Constraints: r.list(def.Objective.Constraints),
	}
	for _, d := range def.Objective.Deliverables {
		obj.Deliverables = append(obj.Deliverables, schema.DeliverableRef{Name: r.text(d.Name), Source: r.text(d.Source)})
	}
	for _, c := range def.Objective.SuccessCriteria {
		obj.SuccessCriteria = append(obj.SuccessCriteria, workflowCriterion{
			ID: r.text(c.ID), Deliverable: r.text(c.Deliverable), Rules: r.ruleKinds(c.Rules),
		})
	}
	output := workflowOutput{
		SchemaVersion: SchemaVersion, Command: "workflow validate", Status: "validated",
		WorkflowID: r.text(def.ID), WorkflowVersion: r.text(def.Version), Objective: obj,
		ConcurrencyLimit: def.ConcurrencyLimit, RequiresMemory: &wf.RequiresMemory, RequiresRouter: &wf.RequiresRouter,
		Steps: make([]workflowStep, 0, len(def.Steps)),
	}
	steps := make(map[string]schema.StepSpec, len(def.Steps))
	for _, step := range def.Steps {
		steps[step.ID] = step
	}
	for _, id := range wf.ExecutionOrder {
		if step, ok := steps[id]; ok {
			view := workflowStep{
				ID: r.text(id), Kind: r.text(string(step.Kind)), DependsOn: r.list(step.DependsOn), Optional: step.Optional,
			}
			if c := step.Correction; c != nil {
				view.Correction = &schema.CorrectionSpec{GateStepID: r.text(c.GateStepID), MaxIterations: c.MaxIterations}
			}
			if g := step.Gate; g != nil {
				view.Gate = &workflowGatePlan{Target: r.text(g.Target), Informative: g.Informative, Rules: r.ruleKinds(g.Rules)}
			}
			output.Steps = append(output.Steps, view)
		}
	}
	return writeWorkflow(w, output, asJSON)
}

// RenderWorkflowRun writes the outcome of an execution.
func RenderWorkflowRun(w io.Writer, view engine.RunSummary, asJSON bool, secretValues []string) error {
	return writeWorkflow(w, workflowRunOutput("workflow run", view.State, secretValues), asJSON)
}

// RenderWorkflowStatus writes a read-only snapshot with a separately labeled run state.
func RenderWorkflowStatus(w io.Writer, view engine.StatusView, asJSON bool, secretValues []string) error {
	return writeWorkflowAcknowledgment(w, "workflow status", view.State, asJSON, secretValues)
}

// RenderWorkflowApprove writes the recorded decision and the resulting run state.
func RenderWorkflowApprove(w io.Writer, view engine.ApproveResult, asJSON bool, secretValues []string) error {
	return writeWorkflowAcknowledgment(w, "workflow approve", view.State, asJSON, secretValues)
}

// RenderWorkflowCancel reports whether the run was cancelled or a request was made.
func RenderWorkflowCancel(w io.Writer, view engine.CancelResult, asJSON bool, secretValues []string) error {
	output := workflowAcknowledgment("workflow cancel", view.State, secretValues)
	requested := view.Requested
	output.Requested = &requested
	return writeWorkflow(w, output, asJSON)
}

// RenderWorkflowResume writes the outcome of resuming an existing run.
func RenderWorkflowResume(w io.Writer, view engine.RunSummary, asJSON bool, secretValues []string) error {
	return writeWorkflow(w, workflowRunOutput("workflow resume", view.State, secretValues), asJSON)
}

func writeWorkflowAcknowledgment(w io.Writer, command string, state *store.RunState, asJSON bool, secretValues []string) error {
	return writeWorkflow(w, workflowAcknowledgment(command, state, secretValues), asJSON)
}

// workflowAcknowledgment keeps the run state in run_status so every command
// that only records an operator action reports status "ok" the same way.
func workflowAcknowledgment(command string, state *store.RunState, secrets []string) workflowOutput {
	output := workflowRunOutput(command, state, secrets)
	if state != nil {
		output.Status = "ok"
	}
	return output
}

func workflowRunOutput(command string, state *store.RunState, secrets []string) workflowOutput {
	output := workflowOutput{SchemaVersion: SchemaVersion, Command: command, Status: "error", Steps: []workflowStep{}}
	if state == nil {
		return output
	}
	r := workflowRedactor{secrets: secrets}
	output.Status = r.text(state.Status)
	output.RunStatus = output.Status
	output.RunID = r.text(state.RunID)
	output.WorkflowID = r.text(state.WorkflowID)
	output.WorkflowVersion = r.text(state.WorkflowVersion)
	if state.ObjectiveID != "" {
		output.Objective = &workflowObjective{ID: r.text(state.ObjectiveID), Version: r.text(state.ObjectiveVersion)}
	}
	output.StateRevision = state.StateRevision
	output.CheckpointDigest = r.text(state.CheckpointDigest)
	output.PendingGates = r.list(state.PendingGates)
	output.Warnings = r.list(state.Warnings)
	output.Error = r.text(state.Error)

	stepIDs := make([]string, 0, len(state.Steps))
	for id := range state.Steps {
		stepIDs = append(stepIDs, id)
	}
	slices.Sort(stepIDs)
	for _, id := range stepIDs {
		step := state.Steps[id]
		view := workflowStep{
			ID: r.text(id), Kind: r.text(step.Kind), Optional: step.Optional,
			Status: r.text(step.Status), Iteration: step.Iteration, Error: r.text(step.Error),
			Warnings: r.list(step.Warnings), Artifacts: r.artifacts(state.RunID, step.Artifacts),
		}
		for _, a := range step.Attempts {
			view.Attempts = append(view.Attempts, workflowAttempt{
				Iteration: a.Iteration, Status: r.text(a.Status), GateStatus: r.text(a.GateStatus),
				Diagnosis: r.text(a.Diagnosis), Error: r.text(a.Error), Artifacts: r.artifacts(state.RunID, a.Artifacts),
			})
		}
		output.Steps = append(output.Steps, view)
	}
	gateIDs := make([]string, 0, len(state.Gates))
	for id := range state.Gates {
		gateIDs = append(gateIDs, id)
	}
	slices.Sort(gateIDs)
	for _, id := range gateIDs {
		gate := state.Gates[id]
		output.Gates = append(output.Gates, workflowGate{
			StepID: r.text(id), Target: r.text(gate.Target), Status: r.text(gate.Status),
			Informative: gate.Informative, Violations: r.list(gate.Violations), Warnings: r.list(gate.Warnings),
		})
	}
	for _, a := range state.Approvals {
		output.Approvals = append(output.Approvals, workflowApproval{
			GateID: r.text(a.GateID), Decision: r.text(a.Decision), Reason: r.text(a.Reason),
			CheckpointDigest: r.text(a.CheckpointDigest), DecidedAt: r.text(a.DecidedAt),
		})
	}
	if verdict := state.ObjectiveVerdict; verdict != nil {
		output.ObjectiveVerdict = &workflowVerdict{
			Status: r.text(verdict.Status), MetCriteria: r.list(verdict.MetCriteria),
			FailedCriteria: r.list(verdict.FailedCriteria), Warnings: r.list(verdict.Warnings),
		}
	}
	return output
}

func (r workflowRedactor) artifacts(runID string, refs []store.ArtifactRef) []workflowArtifact {
	artifacts := make([]workflowArtifact, 0, len(refs))
	for _, ref := range refs {
		var location string
		folder, name, _ := strings.Cut(ref.RelPath, "/")
		if folder == "artifacts" && localWorkflowSegment(runID) && localWorkflowSegment(name) {
			location = r.text(".gandalf/workflows/runs/" + runID + "/" + ref.RelPath)
		}
		artifacts = append(artifacts, workflowArtifact{
			Name: r.text(ref.Name), RelPath: location, RelPathWithheld: ref.RelPath != "" && location == "",
			SHA256: r.text(ref.SHA256), SizeBytes: ref.SizeBytes, Summary: r.text(ref.Summary),
		})
	}
	return artifacts
}

func localWorkflowSegment(value string) bool {
	return value != "." && fs.ValidPath(value) && !strings.ContainsAny(value, "/\\:") &&
		strings.IndexFunc(value, unicode.IsControl) < 0
}

// artifactLocation never renders an empty cell: a reader must be able to tell
// an artifact without a path from one whose path the run space does not admit.
func artifactLocation(artifact workflowArtifact) string {
	switch {
	case artifact.RelPathWithheld:
		return "ruta fuera del espacio de corridas"
	case artifact.RelPath == "":
		return "sin ruta registrada"
	default:
		return artifact.RelPath
	}
}

var workflowCell = strings.NewReplacer("\\", "\\\\", "|", "\\|").Replace

// workflowFailureEntry mirrors one schema failure inside the stable envelope.
type workflowFailureEntry struct {
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type workflowFailureOutput struct {
	SchemaVersion string                 `json:"schema_version"`
	Command       string                 `json:"command"`
	Status        string                 `json:"status"`
	Error         string                 `json:"error,omitempty"`
	Failures      []workflowFailureEntry `json:"failures,omitempty"`
}

// RenderWorkflowFailure reports why a workflow command did not complete with
// the same versioned envelope and redaction as the successful outputs.
func RenderWorkflowFailure(w io.Writer, command, status, message string, failures []schema.ValidationError, asJSON bool, secretValues []string) error {
	r := workflowRedactor{secrets: secretValues}
	output := workflowFailureOutput{
		SchemaVersion: SchemaVersion, Command: r.text(command), Status: r.text(status), Error: r.text(message),
	}
	for _, item := range failures {
		output.Failures = append(output.Failures, workflowFailureEntry{
			Code: r.text(item.Code), Field: r.text(item.Field), Message: r.text(item.Message),
		})
	}
	if asJSON {
		return writeJSON(w, output)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s · %s\n", workflowCell(output.Command), workflowCell(output.Status))
	for _, item := range output.Failures {
		field := item.Field
		if field == "" {
			field = "definición"
		}
		fmt.Fprintf(&b, "- %s: %s (%s)\n", workflowCell(field), workflowCell(item.Message), workflowCell(item.Code))
	}
	if output.Error != "" && len(output.Failures) == 0 {
		fmt.Fprintf(&b, "Error %s\n", workflowCell(output.Error))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func writeWorkflow(w io.Writer, output workflowOutput, asJSON bool) error {
	if asJSON {
		return writeJSON(w, output)
	}
	cell := workflowCell
	var b strings.Builder
	fmt.Fprintf(&b, "%s · %s\n", output.Command, cell(output.Status))
	if output.WorkflowID != "" {
		fmt.Fprintf(&b, "Workflow %s@%s\n", cell(output.WorkflowID), cell(output.WorkflowVersion))
	}
	if output.RunID != "" {
		fmt.Fprintf(&b, "Corrida %s · %s · revisión %d\nCheckpoint %s\n",
			cell(output.RunID), cell(output.RunStatus), output.StateRevision, cell(output.CheckpointDigest))
	}
	if obj := output.Objective; obj != nil {
		fmt.Fprintf(&b, "Objetivo %s@%s\n", cell(obj.ID), cell(obj.Version))
		if obj.Statement != "" {
			fmt.Fprintln(&b, cell(obj.Statement))
		}
		for _, c := range obj.Constraints {
			fmt.Fprintf(&b, "- Restricción %s\n", cell(c))
		}
		for _, d := range obj.Deliverables {
			fmt.Fprintf(&b, "- Entregable %s → %s\n", cell(d.Name), cell(d.Source))
		}
		for _, c := range obj.SuccessCriteria {
			fmt.Fprintf(&b, "- Criterio %s sobre %s (%s)\n", cell(c.ID), cell(c.Deliverable), cell(strings.Join(c.Rules, ", ")))
		}
	}
	if output.RequiresMemory != nil && output.RequiresRouter != nil {
		fmt.Fprintf(&b, "Concurrencia %d · ROMERO requerido %t · Router requerido %t\n",
			output.ConcurrencyLimit, *output.RequiresMemory, *output.RequiresRouter)
		b.WriteString("\n| Paso | Tipo | Dependencias | Opcional |\n| --- | --- | --- | --- |\n")
		for _, step := range output.Steps {
			fmt.Fprintf(&b, "| %s | %s | %s | %t |\n", cell(step.ID), cell(step.Kind), cell(strings.Join(step.DependsOn, ", ")), step.Optional)
		}
	} else if len(output.Steps) != 0 {
		b.WriteString("\n| Paso | Tipo | Estado | Iteración |\n| --- | --- | --- | --- |\n")
		for _, step := range output.Steps {
			fmt.Fprintf(&b, "| %s | %s | %s | %d |\n", cell(step.ID), cell(step.Kind), cell(step.Status), step.Iteration)
		}
	}
	for _, step := range output.Steps {
		if c := step.Correction; c != nil {
			fmt.Fprintf(&b, "- %s · Máximo %d iteraciones · control %s\n", cell(step.ID), c.MaxIterations, cell(c.GateStepID))
		}
		if g := step.Gate; g != nil {
			fmt.Fprintf(&b, "- %s · %s · %s\n", cell(step.ID), cell(g.Target), cell(strings.Join(g.Rules, ", ")))
		}
		if step.Error != "" || len(step.Warnings) != 0 {
			fmt.Fprintf(&b, "- %s · %s\n", cell(step.ID), cell(workflowDetails(step.Error, step.Warnings)))
		}
		for _, a := range step.Attempts {
			fmt.Fprintf(&b, "- %s · intento %d · %s %s\n", cell(step.ID), a.Iteration, cell(a.Status),
				cell(workflowDetails(a.GateStatus, []string{a.Diagnosis, a.Error})))
		}
		for _, a := range stepArtifacts(step) {
			fmt.Fprintf(&b, "- Artefacto %s · %s · %d bytes · %s · %s\n",
				cell(a.Name), cell(artifactLocation(a)), a.SizeBytes, cell(a.SHA256), cell(a.Summary))
		}
	}
	for _, gateID := range output.PendingGates {
		fmt.Fprintf(&b, "- Puerta pendiente %s · --checkpoint %s\n", cell(gateID), cell(output.CheckpointDigest))
	}
	for _, g := range output.Gates {
		fmt.Fprintf(&b, "- Control %s · %s %s\n", cell(g.StepID), cell(g.Status),
			cell(workflowDetails(g.Target, append(slices.Clone(g.Violations), g.Warnings...))))
	}
	for _, a := range output.Approvals {
		fmt.Fprintf(&b, "- Decisión %s · %s · %s %s\n", cell(a.GateID), cell(a.Decision), cell(a.DecidedAt), cell(a.Reason))
	}
	if verdict := output.ObjectiveVerdict; verdict != nil {
		fmt.Fprintf(&b, "Veredicto %s\n", cell(verdict.Status))
		for _, id := range verdict.MetCriteria {
			fmt.Fprintf(&b, "- Criterio cumplido %s\n", cell(id))
		}
		for _, id := range verdict.FailedCriteria {
			fmt.Fprintf(&b, "- Criterio incumplido %s\n", cell(id))
		}
		for _, warning := range verdict.Warnings {
			fmt.Fprintf(&b, "- Aviso %s\n", cell(warning))
		}
	}
	if output.Requested != nil {
		if *output.Requested {
			b.WriteString("Cancelación solicitada\n")
		} else {
			b.WriteString("Cancelación directa\n")
		}
	}
	if output.Error != "" {
		fmt.Fprintf(&b, "Error %s\n", cell(output.Error))
	}
	for _, warning := range output.Warnings {
		fmt.Fprintf(&b, "- Aviso %s\n", cell(warning))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func stepArtifacts(step workflowStep) []workflowArtifact {
	var artifacts []workflowArtifact
	seen := make(map[workflowArtifact]bool)
	add := func(items []workflowArtifact) {
		for _, item := range items {
			if !seen[item] {
				seen[item] = true
				artifacts = append(artifacts, item)
			}
		}
	}
	add(step.Artifacts)
	for _, attempt := range step.Attempts {
		add(attempt.Artifacts)
	}
	return artifacts
}

func workflowDetails(first string, rest []string) string {
	parts := []string{first}
	parts = append(parts, rest...)
	return strings.Join(slices.DeleteFunc(parts, func(part string) bool { return part == "" }), " · ")
}
