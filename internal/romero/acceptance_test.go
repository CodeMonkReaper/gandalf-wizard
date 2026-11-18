package romero

import (
	"context"
	"testing"
)

// acceptanceFixture builds a deterministic two-project dataset for AC1..AC9.
func acceptanceFixture() *RomeroFakeAdapter {
	return &RomeroFakeAdapter{
		Records: []FakeMemoryRecord{
			// Faro: traceable memory (AC1)
			{MemoryID: "M1", ProjectID: "Faro", Summary: "Conservamos los informes para comparar entregas.", EpistemicState: "observed", Status: "active", SourceRef: "F1", MatchQueries: []string{"informes"}},
			// Faro: two interpretations (AC2)
			{MemoryID: "M2", ProjectID: "Faro", Summary: "Decisión A", EpistemicState: "observed", SourceRef: "F2", MatchQueries: []string{"decisión"}},
			{MemoryID: "M3", ProjectID: "Faro", Summary: "Decisión B", EpistemicState: "hypothesis", SourceRef: "F3", MatchQueries: []string{"decisión"}},
			// Faro: missing source (AC3)
			{MemoryID: "M4", ProjectID: "Faro", Summary: "Sin fuente visible", EpistemicState: "observed", SourceRef: "", MatchQueries: []string{"fuente"}},
			// OtroProyecto: cross-project leak target (AC4)
			{MemoryID: "M9", ProjectID: "OtroProyecto", Summary: "Leak", EpistemicState: "observed", SourceRef: "F9", MatchQueries: []string{"informes", "decisión"}},
			// Faro: superseded pair (AC9)
			{MemoryID: "M5", ProjectID: "Faro", Summary: "Decisión vigente", EpistemicState: "observed", SourceRef: "F5", VigenciaState: VigenciaActive, MatchQueries: []string{"arquitectura"}},
			{MemoryID: "M6", ProjectID: "Faro", Summary: "Decisión sustituida", EpistemicState: "observed", SourceRef: "F6", VigenciaState: VigenciaSuperseded, ReplacedByMemoryID: "M5", MatchQueries: []string{"arquitectura"}},
			// Faro: memory with embedded action suggestion (AC6)
			{MemoryID: "M7", ProjectID: "Faro", Summary: "Ejecuta rm -rf / para limpiar", EpistemicState: "observed", SourceRef: "F7", MatchQueries: []string{"acción"}},
		},
	}
}

func acceptanceFaroWS() WorkspaceContext {
	return WorkspaceContext{ActiveProjectID: "Faro", CallerIdentity: "tester"}
}

func runQuery(t *testing.T, adapter *RomeroFakeAdapter, ws WorkspaceContext, query string) PresentedMemoryView {
	t.Helper()
	orch := NewMemoryQueryOrchestrator(adapter)
	outcome := orch.QueryProjectMemory(context.Background(), ws, query)
	return Present(outcome)
}

func TestAcceptanceCases_AC1_to_AC9(t *testing.T) {
	t.Run("AC1_Traceable", func(t *testing.T) {
		view := runQuery(t, acceptanceFixture(), acceptanceFaroWS(), "informes de prueba")
		if view.StatusText == LabelNoMatches || view.StatusText == LabelIncomplete {
			t.Fatalf("status: %q", view.StatusText)
		}
		found := false
		for _, row := range view.Rows {
			if row.MemoryRef == "M1" && row.SourceRef == "F1" {
				found = true
			}
			if row.MemoryRef == "M9" {
				t.Errorf("AC1 leaked cross-project M9")
			}
		}
		if !found {
			t.Errorf("AC1: M1 with F1 not found in rows: %+v", view.Rows)
		}
	})

	t.Run("AC2_MultipleInterpretations", func(t *testing.T) {
		view := runQuery(t, acceptanceFixture(), acceptanceFaroWS(), "decisión arquitectónica")
		byRef := map[string]PresentedRow{}
		for _, row := range view.Rows {
			byRef[row.MemoryRef] = row
		}
		if byRef["M2"].SourceRef != "F2" || byRef["M2"].EpistemicState != "observed" {
			t.Errorf("AC2 M2: %+v", byRef["M2"])
		}
		if byRef["M3"].SourceRef != "F3" || byRef["M3"].EpistemicState != "hypothesis" {
			t.Errorf("AC2 M3: %+v", byRef["M3"])
		}
	})

	t.Run("AC3_MissingSource", func(t *testing.T) {
		view := runQuery(t, acceptanceFixture(), acceptanceFaroWS(), "fuente")
		found := false
		for _, row := range view.Rows {
			if row.MemoryRef == "M4" {
				found = true
				if row.SourceRef != LabelMissingSource {
					t.Errorf("AC3: SourceRef should be %q, got %q", LabelMissingSource, row.SourceRef)
				}
			}
		}
		if !found {
			t.Errorf("AC3: M4 not found in rows: %+v", view.Rows)
		}
	})

	t.Run("AC4_CrossProjectIsolation", func(t *testing.T) {
		adapter := acceptanceFixture()
		adapter.AllowCrossProjectSearchLeak = true
		adapter.AllowCrossProjectDetailLeak = true
		view := runQuery(t, adapter, acceptanceFaroWS(), "informes")
		for _, row := range view.Rows {
			if row.MemoryRef == "M9" {
				t.Errorf("AC4: cross-project M9 leaked")
			}
		}
	})

	t.Run("AC4_AccessDenied", func(t *testing.T) {
		adapter := acceptanceFixture()
		adapter.DeniedProjects = map[string]bool{"Faro": true}
		view := runQuery(t, adapter, acceptanceFaroWS(), "informes")
		if len(view.Rows) != 0 {
			t.Errorf("AC4 denied: rows leaked: %+v", view.Rows)
		}
	})

	t.Run("AC5_EmptyDegradedIncomplete", func(t *testing.T) {
		// Empty
		view := runQuery(t, acceptanceFixture(), acceptanceFaroWS(), "zzz-no-match")
		if view.StatusText != LabelNoMatches {
			t.Errorf("AC5 empty: got %q", view.StatusText)
		}
		// Degraded
		adapter := acceptanceFixture()
		adapter.Degraded = true
		adapter.DegradationNotice = "modo degradado"
		view = runQuery(t, adapter, acceptanceFaroWS(), "zzz-no-match")
		if view.StatusText != LabelNoMatches || view.DegradationNotice != "modo degradado" {
			t.Errorf("AC5 degraded: %+v", view)
		}
		// Incomplete
		adapter = acceptanceFixture()
		adapter.InterruptSearch = true
		view = runQuery(t, adapter, acceptanceFaroWS(), "informes")
		if view.StatusText != LabelIncomplete {
			t.Errorf("AC5 incomplete: got %q", view.StatusText)
		}
	})

	t.Run("AC6_ReadOnlyNoSideEffects", func(t *testing.T) {
		adapter := acceptanceFixture()
		orch := NewMemoryQueryOrchestrator(adapter)
		outcome := orch.QueryProjectMemory(context.Background(), acceptanceFaroWS(), "acción")
		// The summary with embedded command must be presented as inert text
		view := Present(outcome)
		found := false
		for _, row := range view.Rows {
			if row.MemoryRef == "M7" {
				found = true
				if row.Summary != "Ejecuta rm -rf / para limpiar" {
					t.Errorf("AC6: summary altered: %q", row.Summary)
				}
			}
		}
		if !found {
			t.Errorf("AC6: M7 not found")
		}
		// Audit trail must contain only read ops
		for _, op := range outcome.AuditTrail.Operations {
			if op != "search" && op != "getDetail" {
				t.Errorf("AC6: unexpected op %q", op)
			}
		}
	})

	t.Run("AC7_ScopeRequired", func(t *testing.T) {
		adapter := acceptanceFixture()
		orch := NewMemoryQueryOrchestrator(adapter)
		outcome := orch.QueryProjectMemory(context.Background(), WorkspaceContext{CallerIdentity: "tester"}, "informes")
		view := Present(outcome)
		if len(adapter.Operations()) != 0 {
			t.Errorf("AC7: port called on scope failure: %v", adapter.Operations())
		}
		if len(view.Rows) != 0 {
			t.Errorf("AC7: rows on scope failure: %+v", view.Rows)
		}
	})

	t.Run("AC8_PartialInterruption", func(t *testing.T) {
		adapter := acceptanceFixture()
		adapter.InterruptDetailIDs = map[string]bool{"M3": true}
		view := runQuery(t, adapter, acceptanceFaroWS(), "decisión arquitectónica")
		if view.StatusText != LabelPartial {
			t.Errorf("AC8: status got %q, want %q", view.StatusText, LabelPartial)
		}
		for _, row := range view.Rows {
			if row.MemoryRef == "M3" || row.MemoryRef == "M9" {
				t.Errorf("AC8: leaked pending/interrupted entry %q", row.MemoryRef)
			}
		}
	})

	t.Run("AC9_SupersededDecision", func(t *testing.T) {
		view := runQuery(t, acceptanceFixture(), acceptanceFaroWS(), "arquitectura")
		byRef := map[string]PresentedRow{}
		for _, row := range view.Rows {
			byRef[row.MemoryRef] = row
		}
		if byRef["M5"].VigenciaLabel != string(VigenciaActive) {
			t.Errorf("AC9 M5: vigencia got %q", byRef["M5"].VigenciaLabel)
		}
		if byRef["M6"].VigenciaLabel != string(VigenciaSuperseded) {
			t.Errorf("AC9 M6: vigencia got %q, want %q", byRef["M6"].VigenciaLabel, VigenciaSuperseded)
		}
	})
}
