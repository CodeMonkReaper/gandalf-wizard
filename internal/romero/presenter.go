package romero

import "strings"

const (
	defaultDegradedNotice = "Búsqueda en modo degradado"
	labelAccessDenied     = "Acceso denegado a la memoria solicitada"
	labelScopeRequired    = "Identifica el proyecto activo antes de consultar ROMERO"
)

// PresentedRow represents a single attributed ROMERO memory entry prepared for display.
type PresentedRow struct {
	Summary        string
	EpistemicState string
	MemoryRef      string
	SourceRef      string
	VigenciaLabel  string
}

// PresentedMemoryView holds the user-facing attributed output for a ROMERO query outcome.
type PresentedMemoryView struct {
	StatusText        string
	DegradationNotice string
	Rows              []PresentedRow
}

// ProvenancePresenter formats verified MemoryQueryOutcome values into attributed views.
type ProvenancePresenter struct{}

// Present transforms a MemoryQueryOutcome into an attributed PresentedMemoryView.
func (ProvenancePresenter) Present(outcome MemoryQueryOutcome) PresentedMemoryView {
	return Present(outcome)
}

// Present transforms a MemoryQueryOutcome into an attributed PresentedMemoryView.
func Present(outcome MemoryQueryOutcome) PresentedMemoryView {
	switch outcome.Status {
	case StatusAccessDenied:
		return PresentedMemoryView{StatusText: labelAccessDenied}
	case StatusScopeRequired:
		return PresentedMemoryView{StatusText: labelScopeRequired}
	case StatusIncomplete:
		return PresentedMemoryView{
			StatusText:        LabelIncomplete,
			DegradationNotice: resolveDegradationNotice(outcome),
		}
	case StatusPartial:
		if len(outcome.Entries) == 0 {
			return PresentedMemoryView{
				StatusText:        LabelIncomplete,
				DegradationNotice: resolveDegradationNotice(outcome),
			}
		}
		return PresentedMemoryView{
			StatusText:        LabelPartial,
			DegradationNotice: resolveDegradationNotice(outcome),
			Rows:              mapEntries(outcome.Entries),
		}
	case StatusEmpty:
		return PresentedMemoryView{
			StatusText:        LabelNoMatches,
			DegradationNotice: resolveDegradationNotice(outcome),
		}
	default:
		if len(outcome.Entries) == 0 {
			return PresentedMemoryView{
				StatusText:        LabelNoMatches,
				DegradationNotice: resolveDegradationNotice(outcome),
			}
		}
		return PresentedMemoryView{
			DegradationNotice: resolveDegradationNotice(outcome),
			Rows:              mapEntries(outcome.Entries),
		}
	}
}

func resolveDegradationNotice(outcome MemoryQueryOutcome) string {
	if !outcome.Degraded {
		return ""
	}
	notice := strings.TrimSpace(outcome.DegradationNotice)
	if notice != "" {
		return notice
	}
	return defaultDegradedNotice
}

func mapEntries(entries []AttributedMemoryEntry) []PresentedRow {
	rows := make([]PresentedRow, 0, len(entries))
	for _, entry := range entries {
		sourceRef := strings.TrimSpace(entry.SourceRefDisplay)
		if sourceRef == "" {
			sourceRef = LabelMissingSource
		}
		vigencia := string(VigenciaActive)
		if entry.VigenciaStatus == VigenciaSuperseded {
			vigencia = string(VigenciaSuperseded)
		}
		rows = append(rows, PresentedRow{
			Summary:        entry.Summary,
			EpistemicState: entry.EpistemicState,
			MemoryRef:      entry.MemoryID,
			SourceRef:      sourceRef,
			VigenciaLabel:  vigencia,
		})
	}
	return rows
}
