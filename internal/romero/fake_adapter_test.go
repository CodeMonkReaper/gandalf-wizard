package romero

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestRomeroFakeAdapter(t *testing.T) {
	ctx := context.Background()
	faroScope := ResolvedScope{ProjectID: "Faro", CallerIdentity: "dev-user"}

	t.Run("AC1_AC2_AC3 traceable multiple and missing source records with operation trail", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{
			Records: []FakeMemoryRecord{
				{
					MemoryID:       "M1",
					ProjectID:      "Faro",
					Summary:        "Conservamos los informes para comparar entregas.",
					EpistemicState: "observed",
					Status:         "active",
					VigenciaState:  VigenciaActive,
					SourceRef:      "F1",
				},
				{
					MemoryID:       "M2",
					ProjectID:      "Faro",
					Summary:        "Hipotesis alternativa sobre informes.",
					EpistemicState: "hypothesis",
					Status:         "active",
					VigenciaState:  VigenciaActive,
					SourceRef:      "F2",
				},
				{
					MemoryID:       "M3",
					ProjectID:      "Faro",
					Summary:        "Recuerdo sin fuente visible.",
					EpistemicState: "observed",
					Status:         "active",
					VigenciaState:  VigenciaActive,
					SourceRef:      "",
				},
			},
		}

		res, err := adapter.Search(ctx, faroScope, "informes de prueba")
		if err != nil {
			t.Fatalf("Search unexpected error: %v", err)
		}
		if len(res.Hits) != 3 {
			t.Fatalf("len(Hits) = %d, want 3", len(res.Hits))
		}
		wantHit0 := MemorySearchHit{
			MemoryID:       "M1",
			ProjectID:      "Faro",
			Summary:        "Conservamos los informes para comparar entregas.",
			EpistemicState: "observed",
			Status:         "active",
			VigenciaState:  VigenciaActive,
		}
		if res.Hits[0] != wantHit0 {
			t.Fatalf("hit 0 = %+v, want %+v", res.Hits[0], wantHit0)
		}
		wantHit1 := MemorySearchHit{
			MemoryID:       "M2",
			ProjectID:      "Faro",
			Summary:        "Hipotesis alternativa sobre informes.",
			EpistemicState: "hypothesis",
			Status:         "active",
			VigenciaState:  VigenciaActive,
		}
		if res.Hits[1] != wantHit1 {
			t.Fatalf("hit 1 = %+v, want %+v", res.Hits[1], wantHit1)
		}

		d1, err := adapter.GetDetail(ctx, faroScope, "M1")
		if err != nil {
			t.Fatalf("GetDetail(M1) unexpected error: %v", err)
		}
		wantD1 := MemoryDetail{
			MemoryID:       "M1",
			ProjectID:      "Faro",
			SourceRef:      "F1",
			Status:         "active",
			EpistemicState: "observed",
		}
		if d1 != wantD1 {
			t.Fatalf("GetDetail(M1) = %+v, want %+v", d1, wantD1)
		}
		d3, err := adapter.GetDetail(ctx, faroScope, "M3")
		if err != nil {
			t.Fatalf("GetDetail(M3) unexpected error: %v", err)
		}
		wantD3 := MemoryDetail{
			MemoryID:       "M3",
			ProjectID:      "Faro",
			SourceRef:      "",
			Status:         "active",
			EpistemicState: "observed",
		}
		if d3 != wantD3 {
			t.Fatalf("GetDetail(M3) = %+v, want %+v", d3, wantD3)
		}

		wantOps := []string{"search", "getDetail", "getDetail"}
		if gotOps := adapter.Operations(); !slices.Equal(gotOps, wantOps) {
			t.Fatalf("Operations = %v, want %v", gotOps, wantOps)
		}
	})

	t.Run("AC4 multi-project isolation and project access denial", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{
			Records: []FakeMemoryRecord{
				{MemoryID: "M1", ProjectID: "Faro", Summary: "Dato de Faro", EpistemicState: "observed", Status: "active", VigenciaState: VigenciaActive, SourceRef: "F1"},
				{MemoryID: "M9", ProjectID: "OtroProyecto", Summary: "Dato secreto de otro proyecto", EpistemicState: "observed", Status: "active", VigenciaState: VigenciaActive, SourceRef: "F9"},
			},
		}

		res, err := adapter.Search(ctx, faroScope, "dato")
		if err != nil {
			t.Fatalf("Search unexpected error: %v", err)
		}
		if len(res.Hits) != 1 || res.Hits[0].MemoryID != "M1" {
			t.Fatalf("got hits %+v, want only M1 from Faro", res.Hits)
		}

		adapter.DeniedProjects = map[string]bool{"Faro": true}
		_, err = adapter.Search(ctx, faroScope, "dato")
		if !errors.Is(err, ErrAccessDenied) {
			t.Fatalf("Search denied err = %v, want ErrAccessDenied", err)
		}
		_, err = adapter.GetDetail(ctx, faroScope, "M1")
		if !errors.Is(err, ErrAccessDenied) {
			t.Fatalf("GetDetail denied err = %v, want ErrAccessDenied", err)
		}
	})

	t.Run("AC5 empty degraded and interrupted search", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{
			Degraded:          true,
			DegradationNotice: "Búsqueda en modo degradado",
		}
		res, err := adapter.Search(ctx, faroScope, "inexistente")
		if err != nil {
			t.Fatalf("Search unexpected error: %v", err)
		}
		if len(res.Hits) != 0 || !res.Degraded || res.DegradationNotice != "Búsqueda en modo degradado" {
			t.Fatalf("got %+v, want empty degraded result", res)
		}

		adapter.InterruptSearch = true
		_, err = adapter.Search(ctx, faroScope, "cualquiera")
		if !errors.Is(err, ErrServiceInterrupted) {
			t.Fatalf("Search interrupted err = %v, want ErrServiceInterrupted", err)
		}
	})

	t.Run("AC8 partial interruption on GetDetail for M2", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{
			Records: []FakeMemoryRecord{
				{MemoryID: "M1", ProjectID: "Faro", Summary: "Primero", EpistemicState: "observed", Status: "active", VigenciaState: VigenciaActive, SourceRef: "F1"},
				{MemoryID: "M2", ProjectID: "Faro", Summary: "Segundo", EpistemicState: "observed", Status: "active", VigenciaState: VigenciaActive, SourceRef: "F2"},
			},
			InterruptDetailIDs: map[string]bool{"M2": true},
		}

		d1, err := adapter.GetDetail(ctx, faroScope, "M1")
		if err != nil || d1.SourceRef != "F1" {
			t.Fatalf("GetDetail(M1) = %+v, err=%v; want F1", d1, err)
		}
		_, err = adapter.GetDetail(ctx, faroScope, "M2")
		if !errors.Is(err, ErrDetailInterrupted) {
			t.Fatalf("GetDetail(M2) err = %v, want ErrDetailInterrupted", err)
		}
	})

	t.Run("AC9 active and superseded decision pair in Faro", func(t *testing.T) {
		adapter := &RomeroFakeAdapter{
			Records: []FakeMemoryRecord{
				{
					MemoryID:           "M10",
					ProjectID:          "Faro",
					Summary:            "Decisión anterior sustituida.",
					EpistemicState:     "observed",
					Status:             "superseded",
					VigenciaState:      VigenciaSuperseded,
					ReplacedByMemoryID: "M11",
					SourceRef:          "F10",
				},
				{
					MemoryID:       "M11",
					ProjectID:      "Faro",
					Summary:        "Decisión vigente.",
					EpistemicState: "observed",
					Status:         "active",
					VigenciaState:  VigenciaActive,
					SourceRef:      "F11",
				},
			},
		}

		res, err := adapter.Search(ctx, faroScope, "decisión")
		if err != nil {
			t.Fatalf("Search unexpected error: %v", err)
		}
		if len(res.Hits) != 2 {
			t.Fatalf("len(Hits) = %d, want 2", len(res.Hits))
		}
		wantSupHit := MemorySearchHit{
			MemoryID:           "M10",
			ProjectID:          "Faro",
			Summary:            "Decisión anterior sustituida.",
			EpistemicState:     "observed",
			Status:             "superseded",
			VigenciaState:      VigenciaSuperseded,
			ReplacedByMemoryID: "M11",
		}
		if res.Hits[0] != wantSupHit {
			t.Fatalf("hit 0 = %+v, want %+v", res.Hits[0], wantSupHit)
		}
		d10, err := adapter.GetDetail(ctx, faroScope, "M10")
		if err != nil {
			t.Fatalf("GetDetail(M10) unexpected error: %v", err)
		}
		wantD10 := MemoryDetail{
			MemoryID:       "M10",
			ProjectID:      "Faro",
			SourceRef:      "F10",
			Status:         "superseded",
			EpistemicState: "observed",
			SupersededBy:   "M11",
		}
		if d10 != wantD10 {
			t.Fatalf("GetDetail(M10) = %+v, want %+v", d10, wantD10)
		}
	})

	t.Run("triangulation: context cancellation, opaque errors, MatchQueries, and defensive operations copy", func(t *testing.T) {
		t.Run("cancelled context wraps interrupted sentinels", func(t *testing.T) {
			adapter := &RomeroFakeAdapter{
				Records: []FakeMemoryRecord{
					{MemoryID: "M1", ProjectID: "Faro", Summary: "Algo", EpistemicState: "observed"},
				},
			}
			cancelledCtx, cancel := context.WithCancel(ctx)
			cancel()

			_, err := adapter.Search(cancelledCtx, faroScope, "algo")
			if !errors.Is(err, ErrServiceInterrupted) || !errors.Is(err, context.Canceled) {
				t.Fatalf("Search cancelled err = %v, want ErrServiceInterrupted and context.Canceled", err)
			}

			_, err = adapter.GetDetail(cancelledCtx, faroScope, "M1")
			if !errors.Is(err, ErrDetailInterrupted) || !errors.Is(err, context.Canceled) {
				t.Fatalf("GetDetail cancelled err = %v, want ErrDetailInterrupted and context.Canceled", err)
			}
		})

		t.Run("missing or cross-project GetDetail returns opaque ErrAccessDenied", func(t *testing.T) {
			adapter := &RomeroFakeAdapter{
				Records: []FakeMemoryRecord{
					{MemoryID: "SECRET-99", ProjectID: "OtroProyecto", Summary: "Privado", EpistemicState: "observed"},
				},
			}

			for _, id := range []string{"SECRET-99", "NONEXISTENT-42"} {
				_, err := adapter.GetDetail(ctx, faroScope, id)
				if !errors.Is(err, ErrAccessDenied) {
					t.Fatalf("GetDetail(%s) err = %v, want ErrAccessDenied", id, err)
				}
				if err.Error() != ErrAccessDenied.Error() {
					t.Fatalf("err.Error() = %q leaked identifier or extra detail", err.Error())
				}
			}
		})

		t.Run("MatchQueries, OmitProjectIDInSearch, and Operations copy/reset", func(t *testing.T) {
			adapter := &RomeroFakeAdapter{
				Records: []FakeMemoryRecord{
					{MemoryID: "M1", ProjectID: "Faro", Summary: "Informes de prueba", EpistemicState: "observed", MatchQueries: []string{"informes"}},
					{MemoryID: "M2", ProjectID: "Faro", Summary: "Despliegue", EpistemicState: "observed", MatchQueries: []string{"deploy"}},
				},
				OmitProjectIDInSearch: true,
			}

			res, err := adapter.Search(ctx, faroScope, "¿Para qué conservamos los informes?")
			if err != nil {
				t.Fatalf("Search unexpected error: %v", err)
			}
			if len(res.Hits) != 1 || res.Hits[0].MemoryID != "M1" || res.Hits[0].ProjectID != "" {
				t.Fatalf("unexpected filtered/omitted hits: %+v", res.Hits)
			}
			if res.Hits[0].VigenciaState != VigenciaActive {
				t.Fatalf("default VigenciaState = %q, want %q", res.Hits[0].VigenciaState, VigenciaActive)
			}

			ops := adapter.Operations()
			if !slices.Equal(ops, []string{"search"}) {
				t.Fatalf("Operations = %v, want [search]", ops)
			}
			ops[0] = "mutated"
			if got := adapter.Operations(); !slices.Equal(got, []string{"search"}) {
				t.Fatalf("Operations slice was mutated externally: %v", got)
			}

			adapter.ResetOperations()
			if len(adapter.Operations()) != 0 {
				t.Fatalf("expected empty Operations after reset, got %v", adapter.Operations())
			}
		})

		t.Run("DeniedMemoryIDs, separated cross-project leak flags, and colliding MemoryID across projects", func(t *testing.T) {
			t.Run("DeniedMemoryIDs returns opaque ErrAccessDenied", func(t *testing.T) {
				adapter := &RomeroFakeAdapter{
					Records: []FakeMemoryRecord{
						{MemoryID: "M1", ProjectID: "Faro", Summary: "Protegido", EpistemicState: "observed"},
					},
					DeniedMemoryIDs: map[string]bool{"M1": true},
				}
				_, err := adapter.GetDetail(ctx, faroScope, "M1")
				if !errors.Is(err, ErrAccessDenied) || err.Error() != ErrAccessDenied.Error() {
					t.Fatalf("GetDetail denied err = %v, want opaque ErrAccessDenied", err)
				}
			})

			t.Run("AllowCrossProjectSearchLeak leaks Search only while GetDetail remains isolated", func(t *testing.T) {
				adapter := &RomeroFakeAdapter{
					Records: []FakeMemoryRecord{
						{MemoryID: "M9", ProjectID: "OtroProyecto", Summary: "Ajeno", EpistemicState: "observed", SourceRef: "F9"},
					},
					AllowCrossProjectSearchLeak: true,
				}

				res, err := adapter.Search(ctx, faroScope, "ajeno")
				if err != nil {
					t.Fatalf("Search unexpected error: %v", err)
				}
				if len(res.Hits) != 1 || res.Hits[0].MemoryID != "M9" || res.Hits[0].ProjectID != "OtroProyecto" {
					t.Fatalf("expected leaked hit from OtroProyecto in Search, got %+v", res.Hits)
				}

				_, err = adapter.GetDetail(ctx, faroScope, "M9")
				if !errors.Is(err, ErrAccessDenied) {
					t.Fatalf("GetDetail with AllowCrossProjectSearchLeak only = %v, want ErrAccessDenied", err)
				}

				adapter.AllowCrossProjectDetailLeak = true
				d9, err := adapter.GetDetail(ctx, faroScope, "M9")
				if err != nil || d9.ProjectID != "OtroProyecto" || d9.SourceRef != "F9" {
					t.Fatalf("GetDetail with AllowCrossProjectDetailLeak = %+v, err=%v", d9, err)
				}
			})

			t.Run("colliding MemoryID across projects resolves active project record", func(t *testing.T) {
				adapter := &RomeroFakeAdapter{
					Records: []FakeMemoryRecord{
						{MemoryID: "M1", ProjectID: "OtroProyecto", Summary: "Otro", EpistemicState: "observed", SourceRef: "F-OTRO"},
						{MemoryID: "M1", ProjectID: "Faro", Summary: "Faro", EpistemicState: "observed", Status: "active", SourceRef: "F-FARO"},
					},
				}
				d1, err := adapter.GetDetail(ctx, faroScope, "M1")
				if err != nil {
					t.Fatalf("GetDetail(M1) unexpected error: %v", err)
				}
				if d1.ProjectID != "Faro" || d1.SourceRef != "F-FARO" {
					t.Fatalf("got detail %+v, want Faro's M1 record", d1)
				}
			})
		})
	})
}
