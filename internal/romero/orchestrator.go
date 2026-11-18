package romero

import (
	"context"
	"errors"
)

// MemoryQueryOrchestrator coordinates read-only two-step queries against a RomeroReadPort.
type MemoryQueryOrchestrator struct {
	port RomeroReadPort
}

// NewMemoryQueryOrchestrator builds an orchestrator over the given read-only port.
func NewMemoryQueryOrchestrator(port RomeroReadPort) *MemoryQueryOrchestrator {
	return &MemoryQueryOrchestrator{port: port}
}

// QueryProjectMemory executes a two-step read (search then detail) scoped to a single project.
// It guarantees zero port calls on scope failure, filters cross-project leaks at both
// search-hit and detail level without leaking data or IDs, and hides everything on access denied.
func (o *MemoryQueryOrchestrator) QueryProjectMemory(ctx context.Context, ws WorkspaceContext, query string) MemoryQueryOutcome {
	scope, err := ResolveScope(ws)
	if err != nil {
		return MemoryQueryOutcome{Status: StatusScopeRequired}
	}

	searchResult, err := o.port.Search(ctx, scope, query)
	if err != nil {
		if errors.Is(err, ErrAccessDenied) {
			return MemoryQueryOutcome{Status: StatusAccessDenied}
		}
		return MemoryQueryOutcome{Status: StatusIncomplete}
	}

	ops := []string{"search"}

	entries := make([]AttributedMemoryEntry, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		if hit.ProjectID != "" && hit.ProjectID != scope.ProjectID {
			continue
		}

		detail, derr := o.port.GetDetail(ctx, scope, hit.MemoryID)
		ops = append(ops, "getDetail")
		if derr != nil {
			if errors.Is(derr, ErrAccessDenied) {
				return MemoryQueryOutcome{
					Status:     StatusAccessDenied,
					AuditTrail: AuditTrail{Operations: ops},
				}
			}
			continue
		}
		if detail.ProjectID != scope.ProjectID {
			continue
		}

		vigencia := hit.VigenciaState
		if vigencia == "" {
			vigencia = VigenciaActive
		}
		entries = append(entries, AttributedMemoryEntry{
			Summary:          hit.Summary,
			EpistemicState:   hit.EpistemicState,
			MemoryID:         hit.MemoryID,
			SourceRefDisplay: detail.SourceRef,
			VigenciaStatus:   vigencia,
		})
	}

	status := StatusOK
	if len(entries) == 0 {
		status = StatusEmpty
	}

	return MemoryQueryOutcome{
		Status:            status,
		Degraded:          searchResult.Degraded,
		DegradationNotice: searchResult.DegradationNotice,
		Entries:           entries,
		AuditTrail:        AuditTrail{Operations: ops},
	}
}
