package romero

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// FakeMemoryRecord models an in-memory ROMERO record for deterministic testing.
type FakeMemoryRecord struct {
	MemoryID           string
	ProjectID          string
	Summary            string
	EpistemicState     string
	Status             string
	VigenciaState      VigenciaState
	ReplacedByMemoryID string
	SourceRef          string
	MatchQueries       []string
}

// RomeroFakeAdapter is an in-memory deterministic double implementing RomeroReadPort.
type RomeroFakeAdapter struct {
	mu sync.Mutex

	Records                     []FakeMemoryRecord
	DeniedProjects              map[string]bool
	DeniedMemoryIDs             map[string]bool
	Degraded                    bool
	DegradationNotice           string
	InterruptSearch             bool
	InterruptDetailIDs          map[string]bool
	OmitProjectIDInSearch       bool
	AllowCrossProjectSearchLeak bool
	AllowCrossProjectDetailLeak bool

	operations []string
}

var _ RomeroReadPort = (*RomeroFakeAdapter)(nil)

// Search returns matching compact candidates from the in-memory dataset.
func (a *RomeroFakeAdapter) Search(ctx context.Context, scope ResolvedScope, query string) (SearchResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.operations = append(a.operations, "search")

	if err := ctx.Err(); err != nil {
		return SearchResult{}, fmt.Errorf("%w: %w", ErrServiceInterrupted, err)
	}
	if a.DeniedProjects[scope.ProjectID] {
		return SearchResult{}, ErrAccessDenied
	}
	if a.InterruptSearch {
		return SearchResult{}, ErrServiceInterrupted
	}

	hits := make([]MemorySearchHit, 0, len(a.Records))
	for _, rec := range a.Records {
		if !a.AllowCrossProjectSearchLeak && rec.ProjectID != scope.ProjectID {
			continue
		}
		if !matchesQuery(rec.MatchQueries, query) {
			continue
		}
		projectID := rec.ProjectID
		if a.OmitProjectIDInSearch {
			projectID = ""
		}
		vigencia := rec.VigenciaState
		if vigencia == "" {
			vigencia = VigenciaActive
		}
		hits = append(hits, MemorySearchHit{
			MemoryID:           rec.MemoryID,
			ProjectID:          projectID,
			Summary:            rec.Summary,
			EpistemicState:     rec.EpistemicState,
			Status:             rec.Status,
			VigenciaState:      vigencia,
			ReplacedByMemoryID: rec.ReplacedByMemoryID,
		})
	}

	return SearchResult{
		Hits:              hits,
		Degraded:          a.Degraded,
		DegradationNotice: a.DegradationNotice,
	}, nil
}

// GetDetail returns the detail payload for a single memory ID.
func (a *RomeroFakeAdapter) GetDetail(ctx context.Context, scope ResolvedScope, memoryID string) (MemoryDetail, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.operations = append(a.operations, "getDetail")

	if err := ctx.Err(); err != nil {
		return MemoryDetail{}, fmt.Errorf("%w: %w", ErrDetailInterrupted, err)
	}
	if a.DeniedProjects[scope.ProjectID] || a.DeniedMemoryIDs[memoryID] {
		return MemoryDetail{}, ErrAccessDenied
	}
	if a.InterruptDetailIDs[memoryID] {
		return MemoryDetail{}, ErrDetailInterrupted
	}

	for _, rec := range a.Records {
		if rec.MemoryID != memoryID {
			continue
		}
		if !a.AllowCrossProjectDetailLeak && rec.ProjectID != scope.ProjectID {
			continue
		}
		return MemoryDetail{
			MemoryID:       rec.MemoryID,
			ProjectID:      rec.ProjectID,
			SourceRef:      rec.SourceRef,
			Status:         rec.Status,
			EpistemicState: rec.EpistemicState,
			SupersededBy:   rec.ReplacedByMemoryID,
		}, nil
	}

	return MemoryDetail{}, ErrAccessDenied
}

// Operations returns a defensive copy of recorded operation names ("search", "getDetail").
func (a *RomeroFakeAdapter) Operations() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.operations)
}

// ResetOperations clears the recorded operation trail.
func (a *RomeroFakeAdapter) ResetOperations() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.operations = nil
}

func matchesQuery(allowed []string, query string) bool {
	if len(allowed) == 0 {
		return true
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return false
	}
	for _, pattern := range allowed {
		p := strings.ToLower(strings.TrimSpace(pattern))
		if p != "" && (q == p || strings.Contains(q, p)) {
			return true
		}
	}
	return false
}
