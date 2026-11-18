package romero

import (
	"context"
	"errors"
	"testing"
)

func TestOrchestrator_EmptyDegradedIncomplete(t *testing.T) {
	t.Run("EmptyNoHits", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{}
		orch := NewMemoryQueryOrchestrator(adapter)
		outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "nada")
		if outcome.Status != StatusEmpty {
			t.Errorf("status: got %q, want %q", outcome.Status, StatusEmpty)
		}
		if len(outcome.Entries) != 0 {
			t.Errorf("entries: got %d, want 0", len(outcome.Entries))
		}
		if outcome.Degraded {
			t.Errorf("degraded: got true, want false")
		}
	})

	t.Run("EmptyDegradedPreservesNotice", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{
			Degraded:          true,
			DegradationNotice: "modo degradado activo",
		}
		orch := NewMemoryQueryOrchestrator(adapter)
		outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "nada")
		if outcome.Status != StatusEmpty {
			t.Errorf("status: got %q, want %q", outcome.Status, StatusEmpty)
		}
		if !outcome.Degraded {
			t.Errorf("degraded: got false, want true")
		}
		if outcome.DegradationNotice != "modo degradado activo" {
			t.Errorf("notice: got %q", outcome.DegradationNotice)
		}
	})

	t.Run("OKWithHitsPreservesDegradationNotice", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{
			Degraded:          true,
			DegradationNotice: "busqueda parcial",
			Records: []FakeMemoryRecord{
				{MemoryID: "M1", ProjectID: "Faro", Summary: "s", SourceRef: "F1", MatchQueries: []string{"x"}},
			},
		}
		orch := NewMemoryQueryOrchestrator(adapter)
		outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "x")
		if outcome.Status != StatusOK {
			t.Errorf("status: got %q, want %q", outcome.Status, StatusOK)
		}
		if !outcome.Degraded || outcome.DegradationNotice != "busqueda parcial" {
			t.Errorf("degradation not preserved: degraded=%v notice=%q", outcome.Degraded, outcome.DegradationNotice)
		}
	})

	t.Run("SearchInterruptedIsIncomplete", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{InterruptSearch: true}
		orch := NewMemoryQueryOrchestrator(adapter)
		outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "x")
		if outcome.Status != StatusIncomplete {
			t.Errorf("status: got %q, want %q", outcome.Status, StatusIncomplete)
		}
		if len(outcome.Entries) != 0 {
			t.Errorf("entries: got %d, want 0", len(outcome.Entries))
		}
	})

	t.Run("AllDetailsInterruptedIsIncomplete", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{
			InterruptDetailIDs: map[string]bool{"M1": true, "M2": true},
			Records: []FakeMemoryRecord{
				{MemoryID: "M1", ProjectID: "Faro", Summary: "s1", SourceRef: "F1", MatchQueries: []string{"x"}},
				{MemoryID: "M2", ProjectID: "Faro", Summary: "s2", SourceRef: "F2", MatchQueries: []string{"x"}},
			},
		}
		orch := NewMemoryQueryOrchestrator(adapter)
		outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "x")
		if outcome.Status != StatusIncomplete {
			t.Errorf("status: got %q, want %q (0 confirmed)", outcome.Status, StatusIncomplete)
		}
		if len(outcome.Entries) != 0 {
			t.Errorf("entries: got %d, want 0", len(outcome.Entries))
		}
	})
}

func TestOrchestrator_PartialInterruption(t *testing.T) {
	adapter := &RomeroFakeAdapter{
		InterruptDetailIDs: map[string]bool{"M2": true},
		Records: []FakeMemoryRecord{
			{MemoryID: "M1", ProjectID: "Faro", Summary: "confirmado", SourceRef: "F1", MatchQueries: []string{"x"}},
			{MemoryID: "M2", ProjectID: "Faro", Summary: "interrumpido", SourceRef: "F2", MatchQueries: []string{"x"}},
			{MemoryID: "M3", ProjectID: "Faro", Summary: "pendiente", SourceRef: "F3", MatchQueries: []string{"x"}},
		},
	}
	orch := NewMemoryQueryOrchestrator(adapter)
	outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "x")

	if outcome.Status != StatusPartial {
		t.Fatalf("status: got %q, want %q", outcome.Status, StatusPartial)
	}
	if len(outcome.Entries) != 1 {
		t.Fatalf("entries: got %d, want 1 (only M1 confirmed before interruption)", len(outcome.Entries))
	}
	if outcome.Entries[0].MemoryID != "M1" {
		t.Errorf("entry: got %q, want M1", outcome.Entries[0].MemoryID)
	}
	// Must not leak M2 or M3 data/IDs
	for _, e := range outcome.Entries {
		if e.MemoryID == "M2" || e.MemoryID == "M3" {
			t.Errorf("leaked interrupted/pending entry: %q", e.MemoryID)
		}
	}
}

func TestOrchestrator_SupersededDecision(t *testing.T) {
	adapter := &RomeroFakeAdapter{
		Records: []FakeMemoryRecord{
			{MemoryID: "M1", ProjectID: "Faro", Summary: "decision vigente", EpistemicState: "observed", SourceRef: "F1", VigenciaState: VigenciaActive, MatchQueries: []string{"decision"}},
			{MemoryID: "M2", ProjectID: "Faro", Summary: "decision sustituida", EpistemicState: "observed", SourceRef: "F2", VigenciaState: VigenciaSuperseded, ReplacedByMemoryID: "M1", MatchQueries: []string{"decision"}},
		},
	}
	orch := NewMemoryQueryOrchestrator(adapter)
	outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "decision arquitectonica")

	if outcome.Status != StatusOK {
		t.Fatalf("status: got %q, want %q", outcome.Status, StatusOK)
	}
	if len(outcome.Entries) != 2 {
		t.Fatalf("entries: got %d, want 2 (both active and superseded shown)", len(outcome.Entries))
	}
	byID := map[string]AttributedMemoryEntry{}
	for _, e := range outcome.Entries {
		byID[e.MemoryID] = e
	}
	if byID["M1"].VigenciaStatus != VigenciaActive {
		t.Errorf("M1 vigencia: got %q, want %q", byID["M1"].VigenciaStatus, VigenciaActive)
	}
	if byID["M2"].VigenciaStatus != VigenciaSuperseded {
		t.Errorf("M2 vigencia: got %q, want %q (from explicit ROMERO metadata)", byID["M2"].VigenciaStatus, VigenciaSuperseded)
	}
}

func TestOrchestrator_SupersededOnlyFromExplicitMetadata(t *testing.T) {
	// A hit with empty VigenciaState defaults to Active, never to Superseded.
	adapter := &RomeroFakeAdapter{
		Records: []FakeMemoryRecord{
			{MemoryID: "M1", ProjectID: "Faro", Summary: "sin metadata", SourceRef: "F1", VigenciaState: "", MatchQueries: []string{"x"}},
		},
	}
	orch := NewMemoryQueryOrchestrator(adapter)
	outcome := orch.QueryProjectMemory(context.Background(), newFaroWorkspace(), "x")
	if len(outcome.Entries) != 1 {
		t.Fatalf("entries: %d", len(outcome.Entries))
	}
	if outcome.Entries[0].VigenciaStatus == VigenciaSuperseded {
		t.Errorf("vigencia must not default to superseded; got %q", outcome.Entries[0].VigenciaStatus)
	}
	if !errors.Is(nil, nil) {
		// no-op to keep errors import used if needed
	}
}
