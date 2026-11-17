// Package romero defines domain types, ports, and read-only orchestration for querying ROMERO memory.
package romero

// Canonical labels required by the consulta-memoria-romero specification.
const (
	LabelNoMatches     = "Sin coincidencias visibles"
	LabelIncomplete    = "Consulta incompleta"
	LabelPartial       = "Consulta parcial"
	LabelMissingSource = "Fuente de origen no disponible"
)

// VigenciaState represents whether a memory decision is currently active or superseded by another.
type VigenciaState string

const (
	VigenciaActive     VigenciaState = "vigente"
	VigenciaSuperseded VigenciaState = "sustituida"
)

// OutcomeStatus classifies the overall result of a ROMERO project memory query.
type OutcomeStatus string

const (
	StatusOK            OutcomeStatus = "ok"
	StatusEmpty         OutcomeStatus = "empty"
	StatusPartial       OutcomeStatus = "partial"
	StatusIncomplete    OutcomeStatus = "incomplete"
	StatusAccessDenied  OutcomeStatus = "access_denied"
	StatusScopeRequired OutcomeStatus = "scope_required"
)

// WorkspaceContext carries the caller identity and project context for scope validation.
type WorkspaceContext struct {
	ActiveProjectID     string
	CandidateProjectIDs []string
	CallerIdentity      string
}

// ResolvedScope represents a validated single-project query boundary.
type ResolvedScope struct {
	ProjectID      string
	CallerIdentity string
}

// MemorySearchHit models a compact candidate returned by ROMERO's search operation.
type MemorySearchHit struct {
	MemoryID           string
	ProjectID          string
	Summary            string
	EpistemicState     string
	Status             string
	VigenciaState      VigenciaState
	ReplacedByMemoryID string
}

// SearchResult models the compact search response including degradation state.
type SearchResult struct {
	Hits              []MemorySearchHit
	Degraded          bool
	DegradationNotice string
}

// MemoryDetail models the detailed read of a single ROMERO memory node.
type MemoryDetail struct {
	MemoryID       string
	ProjectID      string
	SourceRef      string
	Status         string
	EpistemicState string
	SupersededBy   string
}

// AttributedMemoryEntry models a confirmed memory ready for attributed presentation.
type AttributedMemoryEntry struct {
	Summary          string
	EpistemicState   string
	MemoryID         string
	SourceRefDisplay string
	VigenciaStatus   VigenciaState
}

// AuditTrail records non-sensitive read operation names requested during a query.
type AuditTrail struct {
	Operations []string
}

// MemoryQueryOutcome holds the complete outcome of querying project memory in ROMERO.
type MemoryQueryOutcome struct {
	Status            OutcomeStatus
	Degraded          bool
	DegradationNotice string
	Entries           []AttributedMemoryEntry
	AuditTrail        AuditTrail
}
