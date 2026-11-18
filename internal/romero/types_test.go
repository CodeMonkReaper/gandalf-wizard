package romero

import "testing"

func TestDomainTypesAndLabels(t *testing.T) {
	t.Run("canonical labels", func(t *testing.T) {
		tests := []struct {
			name string
			got  string
			want string
		}{
			{name: "no matches", got: LabelNoMatches, want: "Sin coincidencias visibles"},
			{name: "incomplete", got: LabelIncomplete, want: "Consulta incompleta"},
			{name: "partial", got: LabelPartial, want: "Consulta parcial"},
			{name: "missing source", got: LabelMissingSource, want: "Fuente de origen no disponible"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if tt.got != tt.want {
					t.Fatalf("got %q, want %q", tt.got, tt.want)
				}
			})
		}
	})

	t.Run("vigencia and outcome enums", func(t *testing.T) {
		if VigenciaActive != "vigente" {
			t.Fatalf("VigenciaActive = %q, want %q", VigenciaActive, "vigente")
		}
		if VigenciaSuperseded != "sustituida" {
			t.Fatalf("VigenciaSuperseded = %q, want %q", VigenciaSuperseded, "sustituida")
		}

		statuses := []struct {
			name string
			got  OutcomeStatus
			want OutcomeStatus
		}{
			{name: "ok", got: StatusOK, want: "ok"},
			{name: "empty", got: StatusEmpty, want: "empty"},
			{name: "partial", got: StatusPartial, want: "partial"},
			{name: "incomplete", got: StatusIncomplete, want: "incomplete"},
			{name: "access denied", got: StatusAccessDenied, want: "access_denied"},
			{name: "scope required", got: StatusScopeRequired, want: "scope_required"},
		}

		for _, tt := range statuses {
			t.Run(tt.name, func(t *testing.T) {
				if tt.got != tt.want {
					t.Fatalf("got %q, want %q", tt.got, tt.want)
				}
			})
		}
	})

	t.Run("domain structs hold trace and scope fields", func(t *testing.T) {
		ws := WorkspaceContext{
			ActiveProjectID:     "Faro",
			CandidateProjectIDs: []string{"Faro"},
			CallerIdentity:      "dev-user",
		}
		scope := ResolvedScope{
			ProjectID:      ws.ActiveProjectID,
			CallerIdentity: ws.CallerIdentity,
		}
		hit := MemorySearchHit{
			MemoryID:           "M1",
			ProjectID:          scope.ProjectID,
			Summary:            "Conservamos los informes para comparar entregas.",
			EpistemicState:     "observed",
			Status:             "active",
			VigenciaState:      VigenciaActive,
			ReplacedByMemoryID: "",
		}
		searchRes := SearchResult{
			Hits:              []MemorySearchHit{hit},
			Degraded:          false,
			DegradationNotice: "",
		}
		detail := MemoryDetail{
			MemoryID:       hit.MemoryID,
			ProjectID:      scope.ProjectID,
			SourceRef:      "F1",
			Status:         "active",
			EpistemicState: "observed",
			SupersededBy:   "",
		}
		entry := AttributedMemoryEntry{
			Summary:          hit.Summary,
			EpistemicState:   detail.EpistemicState,
			MemoryID:         detail.MemoryID,
			SourceRefDisplay: detail.SourceRef,
			VigenciaStatus:   hit.VigenciaState,
		}
		outcome := MemoryQueryOutcome{
			Status:     StatusOK,
			Entries:    []AttributedMemoryEntry{entry},
			AuditTrail: AuditTrail{Operations: []string{"search", "getDetail"}},
		}

		if len(searchRes.Hits) != 1 || len(outcome.Entries) != 1 {
			t.Fatalf("unexpected hit/entry counts: hits=%d entries=%d", len(searchRes.Hits), len(outcome.Entries))
		}
		if outcome.Entries[0].MemoryID != "M1" || outcome.Entries[0].SourceRefDisplay != "F1" {
			t.Fatalf("unexpected entry references: %+v", outcome.Entries[0])
		}
		if len(outcome.AuditTrail.Operations) != 2 {
			t.Fatalf("unexpected audit trail length: %d", len(outcome.AuditTrail.Operations))
		}
	})

	t.Run("triangulation: enum uniqueness and zero-value outcome invariant", func(t *testing.T) {
		seenVigencia := map[VigenciaState]bool{}
		for _, v := range []VigenciaState{VigenciaActive, VigenciaSuperseded} {
			if v == "" || seenVigencia[v] {
				t.Fatalf("invalid or duplicate VigenciaState: %q", v)
			}
			seenVigencia[v] = true
		}

		seenStatus := map[OutcomeStatus]bool{}
		for _, s := range []OutcomeStatus{
			StatusOK,
			StatusEmpty,
			StatusPartial,
			StatusIncomplete,
			StatusAccessDenied,
			StatusScopeRequired,
		} {
			if s == "" || seenStatus[s] {
				t.Fatalf("invalid or duplicate OutcomeStatus: %q", s)
			}
			seenStatus[s] = true
		}

		var zeroOutcome MemoryQueryOutcome
		if zeroOutcome.Status != "" || zeroOutcome.Degraded || len(zeroOutcome.Entries) != 0 || len(zeroOutcome.AuditTrail.Operations) != 0 {
			t.Fatalf("unexpected non-zero default outcome: %+v", zeroOutcome)
		}
	})
}
