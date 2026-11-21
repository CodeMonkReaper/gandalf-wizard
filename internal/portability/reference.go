// Package portability fixes the shared reference case of phase 04: the single
// versioned definition of analisis-propuesta, the catalog, the resolved input
// and the fixed copy of the ROMERO answer. The Runtime 0.3 acceptance test and
// the portability test read the case from here, so both destinations start from
// the same committed revision.
package portability

import (
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
)

const (
	// DefinitionID and DefinitionVersion name the single versioned definition.
	DefinitionID      = "analisis-propuesta"
	DefinitionVersion = "1.0.0"
	// ObjectiveID and ObjectiveVersion name the objective of the definition.
	ObjectiveID      = "OBJ-PROP-001"
	ObjectiveVersion = "1.0.0"
	// Revision identifies the shared case itself. Any change to the definition,
	// the catalog, the input or the memory copy requires a new revision and
	// invalidates the comparison made with the previous one.
	Revision = "1"
	// ReferenceConcurrency is the concurrency of the reference graph.
	ReferenceConcurrency = 2
)

// Input is the resolved text of the proposal under analysis. Both destinations
// receive exactly this text.
const Input = "Propuesta: adoptar el motor de workflow 0.3 como base de la fase 04 y medir la portabilidad entre Codex y OpenCode."

// Destination names one executor of the phase 04 comparison.
type Destination string

const (
	// Codex is the first destination of the case.
	Codex Destination = "codex"
	// OpenCode is the second destination of the case.
	OpenCode Destination = "opencode"
)

// Destinations returns both destinations of the case, in a fixed order.
func Destinations() []Destination { return []Destination{Codex, OpenCode} }

// Selection is the provider selection that the case declares for one step.
type Selection struct {
	Step       string `json:"step"`
	Complexity string `json:"complexity"`
	Risk       string `json:"risk"`
}

// Catalog returns the fixed selection of every router step, in definition order.
// T002 contrasts it with the capabilities observed in each destination.
func Catalog() []Selection {
	def := Definition(ReferenceConcurrency)
	catalog := make([]Selection, 0, len(def.Steps))
	for _, step := range def.Steps {
		if step.Kind != schema.StepKindRouterTask || step.Router == nil {
			continue
		}
		catalog = append(catalog, Selection{
			Step:       step.ID,
			Complexity: step.Router.Complexity,
			Risk:       step.Router.Risk,
		})
	}
	return catalog
}

// Definition returns the reference workflow of the specification:
// analisis-propuesta@1.0.0 with objective OBJ-PROP-001@1.0.0.
func Definition(concurrency int) schema.WorkflowDefinition {
	return schema.WorkflowDefinition{
		SchemaVersion: "1", ID: DefinitionID, Version: DefinitionVersion, ConcurrencyLimit: concurrency,
		Objective: schema.ObjectiveSpec{
			ID: ObjectiveID, Version: ObjectiveVersion,
			Statement:   "Analizar propuesta tecnica con contexto historico y riesgos",
			Constraints: []string{"solo texto", "sin secretos"},
			Deliverables: []schema.DeliverableRef{
				{Name: "riesgos", Source: "borrador_riesgos.riesgos"},
				{Name: "recomendacion", Source: "sintesis.recomendacion"},
			},
			SuccessCriteria: []schema.CriterionSpec{
				{ID: "CRIT-RIESGOS", Deliverable: "riesgos", Rules: []schema.GateRuleSpec{{Kind: schema.RuleNonEmpty}}},
				{ID: "CRIT-REC", Deliverable: "recomendacion", Rules: []schema.GateRuleSpec{
					{Kind: schema.RuleNonEmpty}, {Kind: schema.RuleJSONValid},
					{Kind: schema.RuleJSONRequiredKeys, Keys: []string{"decision", "riesgos"}},
				}},
			},
		},
		Steps: []schema.StepSpec{
			{ID: "contexto_romero", Kind: schema.StepKindMemoryQuery, Outputs: []string{"contexto"},
				Memory: &schema.MemoryStepSpec{Query: MemoryQuery}},
			{ID: "borrador_riesgos", Kind: schema.StepKindRouterTask, Outputs: []string{"riesgos"},
				Router: &schema.RouterStepSpec{Prompt: "Lista riesgos tecnicos", Complexity: "media", Risk: "bajo"}},
			{ID: "sintesis", Kind: schema.StepKindRouterTask, DependsOn: []string{"contexto_romero", "borrador_riesgos"},
				Inputs:  map[string]string{"contexto": "contexto_romero.contexto", "riesgos": "borrador_riesgos.riesgos"},
				Outputs: []string{"recomendacion"},
				Router:  &schema.RouterStepSpec{Prompt: "Sintetiza en JSON con decision y riesgos"},
				Correction: &schema.CorrectionSpec{
					GateStepID: "gate_sintesis", MaxIterations: 2,
				}},
			{ID: "gate_sintesis", Kind: schema.StepKindQualityGate, DependsOn: []string{"sintesis"},
				Gate: &schema.GateSpec{Target: "sintesis.recomendacion", Rules: []schema.GateRuleSpec{
					{Kind: schema.RuleNonEmpty}, {Kind: schema.RuleJSONValid},
					{Kind: schema.RuleJSONRequiredKeys, Keys: []string{"decision", "riesgos"}},
					{Kind: schema.RuleNoSecrets},
				}}},
			{ID: "aprobacion_revision", Kind: schema.StepKindHumanApproval, DependsOn: []string{"gate_sintesis"},
				Approval: &schema.ApprovalStepSpec{Prompt: "Revisar recomendacion y aprobar cierre"}},
		},
	}
}
