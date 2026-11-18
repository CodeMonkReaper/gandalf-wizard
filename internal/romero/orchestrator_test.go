package romero

import (
	"context"
	"slices"
	"testing"
)

func newFaroWorkspace() WorkspaceContext {
	return WorkspaceContext{
		ActiveProjectID: "Faro",
		CallerIdentity:  "tester",
	}
}

func TestOrchestrator_Traceable(t *testing.T) {
	adapter := &RomeroFakeAdapter{
		Records: []FakeMemoryRecord{
			{MemoryID: "M1", ProjectID: "Faro", Summary: "Conservamos los informes.", EpistemicState: "observed", Status: "active", SourceRef: "F1", MatchQueries: []string{"informes"}},
		},
	}
	orch := NewMemoryQueryOrchestrator(adapter)
	outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "informes de prueba")
	if outcome.Status != StatusOK {
		t.Fatalf("status: got %q, want %q", outcome.Status, StatusOK)
	}
	if len(outcome.Entries) != 1 {
		t.Fatalf("entries: got %d, want 1", len(outcome.Entries))
	}
	e := outcome.Entries[0]
	if e.MemoryID != "M1" {
		t.Errorf("MemoryID: got %q", e.MemoryID)
	}
	if e.SourceRefDisplay != "F1" {
		t.Errorf("SourceRefDisplay: got %q", e.SourceRefDisplay)
	}
	if e.VigenciaStatus != VigenciaActive {
		t.Errorf("Vigencia: got %q", e.VigenciaStatus)
	}
}

func TestOrchestrator_Multiple(t *testing.T) {
	adapter := &RomeroFakeAdapter{
		Records: []FakeMemoryRecord{
			{MemoryID: "M1", ProjectID: "Faro", Summary: "A", EpistemicState: "observed", SourceRef: "F1", MatchQueries: []string{"decision"}},
			{MemoryID: "M2", ProjectID: "Faro", Summary: "B", EpistemicState: "hypothesis", SourceRef: "F2", MatchQueries: []string{"decision"}},
		},
	}
	orch := NewMemoryQueryOrchestrator(adapter)
	outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "decision arquitectonica")
	if outcome.Status != StatusOK {
		t.Fatalf("status: %q", outcome.Status)
	}
	if len(outcome.Entries) != 2 {
		t.Fatalf("entries: got %d, want 2", len(outcome.Entries))
	}
	byID := map[string]AttributedMemoryEntry{}
	for _, e := range outcome.Entries {
		byID[e.MemoryID] = e
	}
	if byID["M1"].SourceRefDisplay != "F1" {
		t.Errorf("M1 src: %q", byID["M1"].SourceRefDisplay)
	}
	if byID["M2"].SourceRefDisplay != "F2" {
		t.Errorf("M2 src: %q", byID["M2"].SourceRefDisplay)
	}
	if byID["M1"].EpistemicState != "observed" || byID["M2"].EpistemicState != "hypothesis" {
		t.Errorf("epistemic: M1=%q M2=%q", byID["M1"].EpistemicState, byID["M2"].EpistemicState)
	}
}

func TestOrchestrator_MissingSource(t *testing.T) {
	adapter := &RomeroFakeAdapter{
		Records: []FakeMemoryRecord{
			{MemoryID: "M3", ProjectID: "Faro", Summary: "Sin fuente", EpistemicState: "observed", SourceRef: "", MatchQueries: []string{"fuente"}},
		},
	}
	orch := NewMemoryQueryOrchestrator(adapter)
	outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "fuente")
	if outcome.Status != StatusOK {
		t.Fatalf("status: %q", outcome.Status)
	}
	if len(outcome.Entries) != 1 {
		t.Fatalf("entries: %d", len(outcome.Entries))
	}
	if outcome.Entries[0].SourceRefDisplay != "" {
		t.Errorf("src: got %q, want empty", outcome.Entries[0].SourceRefDisplay)
	}
}

func TestOrchestrator_ScopeAndPermissions(t *testing.T) {
	t.Run("ScopeMissingZeroPortCalls", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{}
		orch := NewMemoryQueryOrchestrator(adapter)
		outcome := orch.QueryProjectMemory(context.Background(), WorkspaceContext{CallerIdentity: "tester"}, "x")
		if outcome.Status != StatusScopeRequired {
			t.Errorf("status: %q", outcome.Status)
		}
		if len(adapter.Operations()) != 0 {
			t.Errorf("port calls: %v", adapter.Operations())
		}
		if len(outcome.Entries) != 0 {
			t.Errorf("entries: %d", len(outcome.Entries))
		}
	})
	t.Run("ScopeAmbiguousZeroPortCalls", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{}
		orch := NewMemoryQueryOrchestrator(adapter)
		ws := WorkspaceContext{CallerIdentity: "tester", CandidateProjectIDs: []string{"Faro", "OtroProyecto"}}
		outcome := orch.QueryProjectMemory(context.Background(), ws, "x")
		if outcome.Status != StatusScopeRequired {
			t.Errorf("status: %q", outcome.Status)
		}
		if len(adapter.Operations()) != 0 {
			t.Errorf("port calls: %v", adapter.Operations())
		}
	})
	t.Run("CrossProjectSearchLeakFiltered", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{
			AllowCrossProjectSearchLeak: true,
			Records: []FakeMemoryRecord{
				{MemoryID: "M1", ProjectID: "Faro", Summary: "ok", SourceRef: "F1", MatchQueries: []string{"x"}},
				{MemoryID: "M9", ProjectID: "OtroProyecto", Summary: "leak", SourceRef: "F9", MatchQueries: []string{"x"}},
			},
		}
		orch := NewMemoryQueryOrchestrator(adapter)
		outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "x")
		for _, e := range outcome.Entries {
			if e.MemoryID == "M9" {
				t.Errorf("leaked M9")
			}
		}
		if len(outcome.Entries) != 1 || outcome.Entries[0].MemoryID != "M1" {
			t.Errorf("entries: %v", outcome.Entries)
		}
	})
	t.Run("CrossProjectDetailLeakFiltered", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{
			AllowCrossProjectSearchLeak: true,
			AllowCrossProjectDetailLeak: true,
			Records: []FakeMemoryRecord{
				{MemoryID: "M1", ProjectID: "Faro", Summary: "ok", SourceRef: "F1", MatchQueries: []string{"x"}},
				{MemoryID: "M9", ProjectID: "OtroProyecto", Summary: "leak", SourceRef: "F9", MatchQueries: []string{"x"}},
			},
		}
		orch := NewMemoryQueryOrchestrator(adapter)
		outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "x")
		for _, e := range outcome.Entries {
			if e.MemoryID == "M9" {
				t.Errorf("leaked M9 detail")
			}
		}
	})
	t.Run("DetailLevelCrossProjectFilteredViaEmptyHitProjectID", func(t *testing.T) {
		// Hit arrives with empty ProjectID (passes the hit filter), but the detail
		// comes back from another project and must be filtered at detail level.
		adapter := &RomeroFakeAdapter{
			OmitProjectIDInSearch:       true,
			AllowCrossProjectDetailLeak: true,
			Records: []FakeMemoryRecord{
				{MemoryID: "M1", ProjectID: "Faro", Summary: "ok", SourceRef: "F1", MatchQueries: []string{"x"}},
				{MemoryID: "M9", ProjectID: "OtroProyecto", Summary: "leak detail", SourceRef: "F9", MatchQueries: []string{"x"}},
			},
		}
		orch := NewMemoryQueryOrchestrator(adapter)
		outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "x")
		for _, e := range outcome.Entries {
			if e.MemoryID == "M9" {
				t.Errorf("detail-level cross-project leak: M9 present")
			}
		}
		for _, e := range outcome.Entries {
			if e.MemoryID == "M1" {
				return // M1 confirmed, M9 filtered — pass
			}
		}
		t.Errorf("M1 not confirmed; entries: %v", outcome.Entries)
	})
	t.Run("DetailAccessDeniedHidesEverything", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{
			DeniedMemoryIDs: map[string]bool{"M1": true},
			Records: []FakeMemoryRecord{
				{MemoryID: "M1", ProjectID: "Faro", Summary: "ok", SourceRef: "F1", MatchQueries: []string{"x"}},
			},
		}
		orch := NewMemoryQueryOrchestrator(adapter)
		outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "x")
		if outcome.Status != StatusAccessDenied {
			t.Errorf("status: got %q, want %q", outcome.Status, StatusAccessDenied)
		}
		if len(outcome.Entries) != 0 {
			t.Errorf("entries leaked on detail denial: %d", len(outcome.Entries))
		}
		if len(outcome.AuditTrail.Operations) != 0 {
			t.Errorf("audit trail leaks hit count on denial: %v", outcome.AuditTrail.Operations)
		}
	})
	t.Run("AccessDeniedHidesEverything", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{DeniedProjects: map[string]bool{"Faro": true}}
		orch := NewMemoryQueryOrchestrator(adapter)
		outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "x")
		if outcome.Status != StatusAccessDenied {
			t.Errorf("status: %q", outcome.Status)
		}
		if len(outcome.Entries) != 0 {
			t.Errorf("entries leaked: %d", len(outcome.Entries))
		}
	})
	t.Run("EmptyProjectIDHitAllowedThrough", func(t *testing.T) {
		// A hit with empty ProjectID passes the hit filter (per T005 formula) and is
		// confirmed via detail when the detail's ProjectID matches the scope.
		adapter := &RomeroFakeAdapter{
			OmitProjectIDInSearch: true,
			Records: []FakeMemoryRecord{
				{MemoryID: "M1", ProjectID: "Faro", Summary: "ok", SourceRef: "F1", MatchQueries: []string{"x"}},
			},
		}
		orch := NewMemoryQueryOrchestrator(adapter)
		outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "x")
		if len(outcome.Entries) != 1 || outcome.Entries[0].MemoryID != "M1" {
			t.Errorf("empty-ProjectID hit should be confirmed via detail; entries: %v", outcome.Entries)
		}
	})
}

func TestOrchestrator_ReadOnlyNoSideEffects(t *testing.T) {
	adapter := &RomeroFakeAdapter{
		Records: []FakeMemoryRecord{
			{MemoryID: "M1", ProjectID: "Faro", Summary: "s", SourceRef: "F1", MatchQueries: []string{"q"}},
			{MemoryID: "M2", ProjectID: "Faro", Summary: "s2", SourceRef: "F2", MatchQueries: []string{"q"}},
		},
	}
	orch := NewMemoryQueryOrchestrator(adapter)
	outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "q")
	ops := outcome.AuditTrail.Operations
	if len(ops) == 0 {
		t.Fatal("audit empty")
	}
	for _, op := range ops {
		if op != "search" && op != "getDetail" {
			t.Errorf("unexpected op: %q", op)
		}
	}
	if len(ops) < 1+len(outcome.Entries) {
		t.Errorf("ops: got %d, want >= %d", len(ops), 1+len(outcome.Entries))
	}
	if !slices.Contains(ops, "search") {
		t.Errorf("missing search: %v", ops)
	}
}
