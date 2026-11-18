package router

import (
	"reflect"
	"testing"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
)

// view builds a ProfileView for selection fixtures.
func view(id, executorID, model string, authorized bool, av Availability, reason string, verification Verification) ProfileView {
	return ProfileView{
		Profile:      config.RouterProfile{ID: id, ExecutorID: executorID, Model: model},
		Authorized:   authorized,
		Availability: av,
		Reason:       reason,
		Origin:       OriginDeclared,
		Verification: verification,
	}
}

func withEfforts(v ProfileView, efforts map[string]string) ProfileView {
	v.Profile.Efforts = efforts
	return v
}

var highEfforts = map[string]string{"low": "low", "medium": "medium", "high": "high"}

// TestSelection covers the deterministic, explained selector (AC3-AC5, AC9,
// AC10, AC12, AC19).
func TestSelection(t *testing.T) {
	cases := []struct {
		name     string
		catalog  Catalog
		features Features
		manualID string
		check    func(t *testing.T, d Decision, c Catalog)
	}{
		{
			name: "seleccion automatica elige el primero elegible y fija el fallback",
			catalog: Catalog{
				Profiles: []ProfileView{
					withEfforts(view("agy/m1", "antigravity", "m1", true, AvailabilityAvailable, "", VerificationUnverified), highEfforts),
					view("vibe/m2", "vibe", "m2", true, AvailabilityAvailable, "", VerificationUnverified),
					view("api/m3", "api-x", "m3", false, AvailabilityUnauthorized, "ejecutor no autorizado", VerificationUnverified),
				},
			},
			features: Features{Complexity: ComplexityHigh, Risk: RiskMedium},
			check: func(t *testing.T, d Decision, c Catalog) {
				if d.Manual {
					t.Fatal("decision manual inesperada")
				}
				if len(d.Ordered) != 2 {
					t.Fatalf("esperaba 2 elegibles, got %d", len(d.Ordered))
				}
				if d.Ordered[0].Profile.ID != "agy/m1" || d.Ordered[0].Effort != "high" {
					t.Fatalf("elegido inesperado: %+v", d.Ordered[0])
				}
				if d.Ordered[1].Profile.ID != "vibe/m2" || d.Ordered[1].Effort != "" {
					t.Fatalf("fallback inesperado: %+v", d.Ordered[1])
				}
				if len(d.Considered) != 3 {
					t.Fatalf("esperaba 3 evaluaciones, got %d", len(d.Considered))
				}
				if d.Considered[0].ProfileID != "agy/m1" || !d.Considered[0].Eligible {
					t.Fatalf("evaluacion 0 inesperada: %+v", d.Considered[0])
				}
				if d.Considered[2].ProfileID != "api/m3" || d.Considered[2].Eligible || d.Considered[2].Reason != "no autorizado" {
					t.Fatalf("evaluacion no autorizada inesperada: %+v", d.Considered[2])
				}
			},
		},
		{
			name: "perfil sin esfuerzo solicitado se mantiene primero",
			catalog: Catalog{
				Profiles: []ProfileView{
					view("vibe/m2", "vibe", "m2", true, AvailabilityAvailable, "", VerificationUnverified),
					withEfforts(view("agy/m1", "antigravity", "m1", true, AvailabilityAvailable, "", VerificationUnverified), highEfforts),
				},
			},
			features: Features{Complexity: ComplexityHigh, Risk: RiskMedium},
			check: func(t *testing.T, d Decision, c Catalog) {
				if d.Ordered[0].Profile.ID != "vibe/m2" || d.Ordered[0].Effort != "" {
					t.Fatalf("el perfil sin esfuerzo no se mantuvo primero: %+v", d.Ordered[0])
				}
				if d.Ordered[1].Profile.ID != "agy/m1" || d.Ordered[1].Effort != "high" {
					t.Fatalf("segundo elegible inesperado: %+v", d.Ordered[1])
				}
			},
		},
		{
			name: "misma entrada produce decision identica",
			catalog: Catalog{
				Profiles: []ProfileView{
					withEfforts(view("agy/m1", "antigravity", "m1", true, AvailabilityAvailable, "", VerificationUnverified), highEfforts),
					view("api/m3", "api-x", "m3", false, AvailabilityUnauthorized, "ejecutor no autorizado", VerificationUnverified),
				},
				Inventory: []ProfileView{view("agy/m9", "antigravity", "m9", true, AvailabilityAvailable, "", VerificationVerified)},
			},
			features: Features{Complexity: ComplexityHigh, Risk: RiskMedium},
			check: func(t *testing.T, d Decision, c Catalog) {
				d2 := Select(c, Features{Complexity: ComplexityHigh, Risk: RiskMedium}, "")
				if !reflect.DeepEqual(d, d2) {
					t.Fatalf("decisiones distintas:\n%+v\n%+v", d, d2)
				}
			},
		},
		{
			name: "riesgo alto descarta no verificados",
			catalog: Catalog{
				Profiles: []ProfileView{
					view("agy/m1", "antigravity", "m1", true, AvailabilityAvailable, "", VerificationUnverified),
					view("agy/m2", "antigravity", "m2", true, AvailabilityAvailable, "", VerificationVerified),
				},
			},
			features: Features{Complexity: ComplexityMedium, Risk: RiskHigh},
			check: func(t *testing.T, d Decision, c Catalog) {
				if len(d.Ordered) != 1 || d.Ordered[0].Profile.ID != "agy/m2" {
					t.Fatalf("elegibles con riesgo alto inesperados: %+v", d.Ordered)
				}
				a := d.Considered[0]
				if a.ProfileID != "agy/m1" || a.Eligible || a.Reason != "no verificado" {
					t.Fatalf("evaluacion de no verificado inesperada: %+v", a)
				}
			},
		},
		{
			name: "riesgo medio mantiene no verificados elegibles",
			catalog: Catalog{
				Profiles: []ProfileView{
					view("agy/m1", "antigravity", "m1", true, AvailabilityAvailable, "", VerificationUnverified),
				},
			},
			features: Features{Complexity: ComplexityMedium, Risk: RiskMedium},
			check: func(t *testing.T, d Decision, c Catalog) {
				if len(d.Ordered) != 1 || d.Ordered[0].Profile.ID != "agy/m1" {
					t.Fatalf("no verificado deberia ser elegible con riesgo medio: %+v", d.Ordered)
				}
			},
		},
		{
			name: "manual con perfil elegible",
			catalog: Catalog{
				Profiles: []ProfileView{
					view("agy/m1", "antigravity", "m1", true, AvailabilityAvailable, "", VerificationUnverified),
					withEfforts(view("vibe/m2", "vibe", "m2", true, AvailabilityAvailable, "", VerificationUnverified), highEfforts),
				},
			},
			features: Features{Complexity: ComplexityHigh, Risk: RiskMedium},
			manualID: "vibe/m2",
			check: func(t *testing.T, d Decision, c Catalog) {
				if !d.Manual {
					t.Fatal("esperaba decision manual")
				}
				if len(d.Ordered) != 1 || d.Ordered[0].Profile.ID != "vibe/m2" || d.Ordered[0].Effort != "high" {
					t.Fatalf("eleccion manual inesperada: %+v", d.Ordered)
				}
				if len(d.Considered) != 1 || d.Considered[0].ProfileID != "vibe/m2" {
					t.Fatalf("evaluacion manual inesperada: %+v", d.Considered)
				}
			},
		},
		{
			name: "manual con perfil inelegible no anade otro",
			catalog: Catalog{
				Profiles: []ProfileView{
					withEfforts(view("agy/m1", "antigravity", "m1", true, AvailabilityAvailable, "", VerificationUnverified), highEfforts),
					view("api/m3", "api-x", "m3", false, AvailabilityUnauthorized, "ejecutor no autorizado", VerificationUnverified),
				},
			},
			features: Features{Complexity: ComplexityHigh, Risk: RiskMedium},
			manualID: "api/m3",
			check: func(t *testing.T, d Decision, c Catalog) {
				if !d.Manual {
					t.Fatal("esperaba decision manual")
				}
				if len(d.Ordered) != 0 {
					t.Fatalf("modo manual inelegible no debe ordenar: %+v", d.Ordered)
				}
				if len(d.Considered) != 1 || d.Considered[0].Eligible || d.Considered[0].Reason != "no autorizado" {
					t.Fatalf("evaluacion manual inelegible inesperada: %+v", d.Considered)
				}
			},
		},
		{
			name: "manual con id desconocido",
			catalog: Catalog{
				Profiles: []ProfileView{
					view("agy/m1", "antigravity", "m1", true, AvailabilityAvailable, "", VerificationUnverified),
				},
			},
			features: Features{Complexity: ComplexityMedium, Risk: RiskMedium},
			manualID: "no-existe",
			check: func(t *testing.T, d Decision, c Catalog) {
				if len(d.Ordered) != 0 || len(d.Considered) != 1 {
					t.Fatalf("decision con id desconocido inesperada: %+v", d)
				}
				if d.Considered[0].ProfileID != "no-existe" || d.Considered[0].Reason != "perfil no declarado" {
					t.Fatalf("evaluacion de id desconocido inesperada: %+v", d.Considered[0])
				}
			},
		},
		{
			name: "manual con id de inventario explica la falta de posicion",
			catalog: Catalog{
				Profiles:  []ProfileView{view("agy/m1", "antigravity", "m1", true, AvailabilityAvailable, "", VerificationUnverified)},
				Inventory: []ProfileView{view("agy/m9", "antigravity", "m9", true, AvailabilityAvailable, "", VerificationVerified)},
			},
			features: Features{Complexity: ComplexityMedium, Risk: RiskMedium},
			manualID: "agy/m9",
			check: func(t *testing.T, d Decision, c Catalog) {
				if len(d.Ordered) != 0 {
					t.Fatalf("inventario no debe ser elegible: %+v", d.Ordered)
				}
				if d.Considered[0].Reason != "candidato de inventario, sin posición declarada" {
					t.Fatalf("razon de inventario inesperada: %+v", d.Considered[0])
				}
			},
		},
		{
			name: "sin elegibles devuelve evaluaciones con motivos",
			catalog: Catalog{
				Profiles: []ProfileView{
					view("api/m3", "api-x", "m3", true, AvailabilityUnavailable, "falta la credencial", VerificationUnverified),
					view("api/m4", "api-y", "m4", false, AvailabilityUnauthorized, "ejecutor no autorizado", VerificationUnverified),
				},
			},
			features: Features{Complexity: ComplexityMedium, Risk: RiskMedium},
			check: func(t *testing.T, d Decision, c Catalog) {
				if len(d.Ordered) != 0 {
					t.Fatalf("sin elegibles no debe ordenar: %+v", d.Ordered)
				}
				if len(d.Considered) != 2 {
					t.Fatalf("esperaba 2 evaluaciones, got %d", len(d.Considered))
				}
				if d.Considered[0].Reason != "no disponible (falta la credencial)" {
					t.Fatalf("razon de no disponible inesperada: %+v", d.Considered[0])
				}
			},
		},
		{
			name: "inventario queda descartado sin posicion declarada",
			catalog: Catalog{
				Profiles:  []ProfileView{view("agy/m1", "antigravity", "m1", true, AvailabilityAvailable, "", VerificationUnverified)},
				Inventory: []ProfileView{view("agy/m9", "antigravity", "m9", true, AvailabilityAvailable, "", VerificationVerified)},
			},
			features: Features{Complexity: ComplexityMedium, Risk: RiskMedium},
			check: func(t *testing.T, d Decision, c Catalog) {
				if len(d.Ordered) != 1 || d.Ordered[0].Profile.ID != "agy/m1" {
					t.Fatalf("elegibles inesperados: %+v", d.Ordered)
				}
				last := d.Considered[len(d.Considered)-1]
				if last.ProfileID != "agy/m9" || last.Reason != "candidato de inventario, sin posición declarada" {
					t.Fatalf("descarte de inventario inesperado: %+v", last)
				}
			},
		},
		{
			name: "features vacias usan complejidad media y esfuerzo medium",
			catalog: Catalog{
				Profiles: []ProfileView{
					withEfforts(view("agy/m1", "antigravity", "m1", true, AvailabilityAvailable, "", VerificationUnverified), highEfforts),
				},
			},
			features: Features{},
			check: func(t *testing.T, d Decision, c Catalog) {
				if d.Ordered[0].Effort != "medium" {
					t.Fatalf("esfuerzo por defecto inesperado: %+v", d.Ordered[0])
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Select(tc.catalog, tc.features, tc.manualID)
			if tc.check == nil {
				t.Fatal("caso sin comprobacion")
			}
			tc.check(t, d, tc.catalog)
		})
	}
}
