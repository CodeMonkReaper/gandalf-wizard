package router

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
)

// fakeAdapter is a deterministic adapter double with invocation counters.
type fakeAdapter struct {
	probe         Probe
	candidates    []Candidate
	discoverErr   error
	probeCalls    int
	discoverCalls int
	executeCalls  int
}

func (f *fakeAdapter) Probe(ctx context.Context, ex config.RouterExecutor) Probe {
	f.probeCalls++
	return f.probe
}

func (f *fakeAdapter) Discover(ctx context.Context, ex config.RouterExecutor) ([]Candidate, error) {
	f.discoverCalls++
	if f.discoverErr != nil {
		return nil, f.discoverErr
	}
	return f.candidates, nil
}

func (f *fakeAdapter) Execute(ctx context.Context, exec Execution) (string, error) {
	f.executeCalls++
	return "texto", nil
}

var readyProbe = Probe{Installed: true, Auth: AuthReady, TextOnly: TextOnlyEnforced}

func cliExecutor(id string, authorized bool) config.RouterExecutor {
	return config.RouterExecutor{ID: id, Kind: config.RouterKindCLI, Authorized: authorized}
}

func profile(id, executorID, model string) config.RouterProfile {
	return config.RouterProfile{ID: id, ExecutorID: executorID, Model: model}
}

func policyWith(executors []config.RouterExecutor, profiles []config.RouterProfile) config.RouterPolicy {
	return config.RouterPolicy{Executors: executors, Profiles: profiles, AttemptTimeout: 60 * time.Second}
}

// TestCatalog covers catalog building: order, origin, dedup, inventory,
// discovery gating, and probe-based availability (AC1, AC2, AC5, AC15, AC19).
func TestCatalog(t *testing.T) {
	cases := []struct {
		name     string
		mode     DiscoveryMode
		policy   config.RouterPolicy
		adapters map[string]*fakeAdapter
		check    func(t *testing.T, c Catalog, adapters map[string]*fakeAdapter)
	}{
		{
			name: "perfiles declarados en orden, origen declarado y sin descubrimiento en modo local",
			mode: DiscoveryLocalOnly,
			policy: policyWith(
				[]config.RouterExecutor{cliExecutor("antigravity", true), cliExecutor("vibe", true)},
				[]config.RouterProfile{profile("antigravity/m1", "antigravity", "m1"), profile("vibe/m2", "vibe", "m2")},
			),
			adapters: map[string]*fakeAdapter{
				"antigravity": {probe: readyProbe, candidates: []Candidate{{Model: "m1"}}},
				"vibe":        {probe: readyProbe},
			},
			check: func(t *testing.T, c Catalog, adapters map[string]*fakeAdapter) {
				if len(c.Profiles) != 2 {
					t.Fatalf("esperaba 2 perfiles, got %d", len(c.Profiles))
				}
				if c.Profiles[0].Profile.ID != "antigravity/m1" || c.Profiles[1].Profile.ID != "vibe/m2" {
					t.Fatalf("orden de perfiles inesperado: %+v", c.Profiles)
				}
				for _, pv := range c.Profiles {
					if !pv.Authorized || pv.Availability != AvailabilityAvailable || pv.Reason != "" {
						t.Fatalf("perfil %q inesperado: %+v", pv.Profile.ID, pv)
					}
					if pv.Origin != OriginDeclared || pv.Verification != VerificationUnverified {
						t.Fatalf("origen/verificacion inesperados en %q: %+v", pv.Profile.ID, pv)
					}
				}
				if len(c.Executors) != 2 || c.Executors[0].Executor.ID != "antigravity" {
					t.Fatalf("vistas de ejecutor inesperadas: %+v", c.Executors)
				}
				for id, a := range adapters {
					if a.discoverCalls != 0 {
						t.Fatalf("modo local disparo descubrimiento en %s", id)
					}
					if a.executeCalls != 0 {
						t.Fatalf("Build invoco Execute en %s", id)
					}
				}
			},
		},
		{
			name: "descubrimiento confirmado marca verificado y descubierto",
			mode: DiscoveryLocalAndDocumentedFree,
			policy: policyWith(
				[]config.RouterExecutor{cliExecutor("antigravity", true)},
				[]config.RouterProfile{profile("antigravity/m1", "antigravity", "m1")},
			),
			adapters: map[string]*fakeAdapter{
				"antigravity": {probe: readyProbe, candidates: []Candidate{{Model: "m1"}}},
			},
			check: func(t *testing.T, c Catalog, adapters map[string]*fakeAdapter) {
				if adapters["antigravity"].discoverCalls != 1 {
					t.Fatalf("esperaba 1 descubrimiento, got %d", adapters["antigravity"].discoverCalls)
				}
				pv := c.Profiles[0]
				if pv.Verification != VerificationVerified || pv.Origin != OriginDiscovered {
					t.Fatalf("perfil confirmado inesperado: %+v", pv)
				}
				if adapters["antigravity"].executeCalls != 0 {
					t.Fatal("Build invoco Execute")
				}
			},
		},
		{
			name: "candidato sin perfil declarado queda en inventario ordenado y deduplicado",
			mode: DiscoveryLocalAndDocumentedFree,
			policy: policyWith(
				[]config.RouterExecutor{cliExecutor("antigravity", true)},
				[]config.RouterProfile{profile("antigravity/m1", "antigravity", "m1")},
			),
			adapters: map[string]*fakeAdapter{
				"antigravity": {probe: readyProbe, candidates: []Candidate{{Model: "m9"}, {Model: "m9"}, {Model: "m1"}}},
			},
			check: func(t *testing.T, c Catalog, adapters map[string]*fakeAdapter) {
				if len(c.Profiles) != 1 || c.Profiles[0].Profile.ID != "antigravity/m1" {
					t.Fatalf("perfiles inesperados: %+v", c.Profiles)
				}
				if len(c.Inventory) != 1 {
					t.Fatalf("esperaba 1 candidato de inventario, got %d", len(c.Inventory))
				}
				iv := c.Inventory[0]
				if iv.Profile.ID != "antigravity/m9" || iv.Profile.ExecutorID != "antigravity" || iv.Profile.Model != "m9" {
					t.Fatalf("candidato de inventario inesperado: %+v", iv)
				}
				if iv.Origin != OriginDiscovered || iv.Verification != VerificationVerified {
					t.Fatalf("origen/verificacion de inventario inesperados: %+v", iv)
				}
				if !iv.Authorized || iv.Availability != AvailabilityAvailable {
					t.Fatalf("disponibilidad de inventario inesperada: %+v", iv)
				}
			},
		},
		{
			name: "ejecutor no autorizado no dispara descubrimiento",
			mode: DiscoveryLocalAndDocumentedFree,
			policy: policyWith(
				[]config.RouterExecutor{cliExecutor("api-x", false)},
				[]config.RouterProfile{profile("api-x/m1", "api-x", "m1")},
			),
			adapters: map[string]*fakeAdapter{
				"api-x": {probe: readyProbe, candidates: []Candidate{{Model: "m1"}}},
			},
			check: func(t *testing.T, c Catalog, adapters map[string]*fakeAdapter) {
				if adapters["api-x"].discoverCalls != 0 {
					t.Fatal("ejecutor no autorizado disparo descubrimiento")
				}
				pv := c.Profiles[0]
				if pv.Authorized || pv.Availability != AvailabilityUnauthorized {
					t.Fatalf("perfil no autorizado inesperado: %+v", pv)
				}
				if pv.Reason != "ejecutor no autorizado" {
					t.Fatalf("razon inesperada: %q", pv.Reason)
				}
				if len(c.Inventory) != 0 {
					t.Fatalf("inventario inesperado: %+v", c.Inventory)
				}
			},
		},
		{
			name: "credencial ausente marca no disponible",
			mode: DiscoveryLocalOnly,
			policy: policyWith(
				[]config.RouterExecutor{cliExecutor("api-x", true)},
				[]config.RouterProfile{profile("api-x/m1", "api-x", "m1")},
			),
			adapters: map[string]*fakeAdapter{
				"api-x": {probe: Probe{Installed: true, Auth: AuthMissing, TextOnly: TextOnlyEnforced}},
			},
			check: func(t *testing.T, c Catalog, adapters map[string]*fakeAdapter) {
				pv := c.Profiles[0]
				if pv.Availability != AvailabilityUnavailable || pv.Reason != "falta la credencial" {
					t.Fatalf("perfil sin credencial inesperado: %+v", pv)
				}
			},
		},
		{
			name: "binario ausente marca no disponible",
			mode: DiscoveryLocalOnly,
			policy: policyWith(
				[]config.RouterExecutor{cliExecutor("antigravity", true)},
				[]config.RouterProfile{profile("antigravity/m1", "antigravity", "m1")},
			),
			adapters: map[string]*fakeAdapter{
				"antigravity": {probe: Probe{Installed: false, Auth: AuthUnknown, TextOnly: TextOnlyUnproven}},
			},
			check: func(t *testing.T, c Catalog, adapters map[string]*fakeAdapter) {
				pv := c.Profiles[0]
				if pv.Availability != AvailabilityUnavailable || pv.Reason != "ejecutor no instalado" {
					t.Fatalf("perfil sin binario inesperado: %+v", pv)
				}
			},
		},
		{
			name: "modo solo texto no demostrado marca no disponible",
			mode: DiscoveryLocalOnly,
			policy: policyWith(
				[]config.RouterExecutor{cliExecutor("antigravity", true)},
				[]config.RouterProfile{profile("antigravity/m1", "antigravity", "m1")},
			),
			adapters: map[string]*fakeAdapter{
				"antigravity": {probe: Probe{Installed: true, Auth: AuthReady, TextOnly: TextOnlyUnproven}},
			},
			check: func(t *testing.T, c Catalog, adapters map[string]*fakeAdapter) {
				pv := c.Profiles[0]
				if pv.Availability != AvailabilityUnavailable || pv.Reason != "modo solo texto no demostrado" {
					t.Fatalf("perfil con texto no demostrado inesperado: %+v", pv)
				}
			},
		},
		{
			name: "autenticacion desconocida marca no disponible",
			mode: DiscoveryLocalOnly,
			policy: policyWith(
				[]config.RouterExecutor{cliExecutor("antigravity", true)},
				[]config.RouterProfile{profile("antigravity/m1", "antigravity", "m1")},
			),
			adapters: map[string]*fakeAdapter{
				"antigravity": {probe: Probe{Installed: true, Auth: AuthUnknown, TextOnly: TextOnlyEnforced}},
			},
			check: func(t *testing.T, c Catalog, adapters map[string]*fakeAdapter) {
				pv := c.Profiles[0]
				if pv.Availability != AvailabilityUnavailable || pv.Reason != "autenticación desconocida" {
					t.Fatalf("perfil con autenticacion desconocida inesperado: %+v", pv)
				}
			},
		},
		{
			name: "razon del sondeo prevalece sobre el defecto",
			mode: DiscoveryLocalOnly,
			policy: policyWith(
				[]config.RouterExecutor{cliExecutor("antigravity", true)},
				[]config.RouterProfile{profile("antigravity/m1", "antigravity", "m1")},
			),
			adapters: map[string]*fakeAdapter{
				"antigravity": {probe: Probe{Installed: true, Auth: AuthMissing, TextOnly: TextOnlyEnforced, Reason: "sin sesión iniciada"}},
			},
			check: func(t *testing.T, c Catalog, adapters map[string]*fakeAdapter) {
				pv := c.Profiles[0]
				if pv.Reason != "sin sesión iniciada" {
					t.Fatalf("razon del sondeo ignorada: %q", pv.Reason)
				}
			},
		},
		{
			name: "deduplicacion de perfiles por ejecutor y modelo",
			mode: DiscoveryLocalOnly,
			policy: policyWith(
				[]config.RouterExecutor{cliExecutor("antigravity", true)},
				[]config.RouterProfile{profile("p1", "antigravity", "m1"), profile("p2", "antigravity", "m1")},
			),
			adapters: map[string]*fakeAdapter{
				"antigravity": {probe: readyProbe},
			},
			check: func(t *testing.T, c Catalog, adapters map[string]*fakeAdapter) {
				if len(c.Profiles) != 1 || c.Profiles[0].Profile.ID != "p1" {
					t.Fatalf("deduplicacion inesperada: %+v", c.Profiles)
				}
			},
		},
		{
			name: "fallo de descubrimiento conserva perfiles declarados",
			mode: DiscoveryLocalAndDocumentedFree,
			policy: policyWith(
				[]config.RouterExecutor{cliExecutor("antigravity", true)},
				[]config.RouterProfile{profile("antigravity/m1", "antigravity", "m1")},
			),
			adapters: map[string]*fakeAdapter{
				"antigravity": {probe: readyProbe, discoverErr: errors.New("boom")},
			},
			check: func(t *testing.T, c Catalog, adapters map[string]*fakeAdapter) {
				if adapters["antigravity"].discoverCalls != 1 {
					t.Fatal("esperaba intento de descubrimiento")
				}
				pv := c.Profiles[0]
				if pv.Availability != AvailabilityAvailable || pv.Verification != VerificationUnverified || pv.Origin != OriginDeclared {
					t.Fatalf("perfil tras fallo de descubrimiento inesperado: %+v", pv)
				}
			},
		},
		{
			name: "descubrimiento no disponible conserva perfiles declarados",
			mode: DiscoveryLocalAndDocumentedFree,
			policy: policyWith(
				[]config.RouterExecutor{cliExecutor("antigravity", true)},
				[]config.RouterProfile{profile("antigravity/m1", "antigravity", "m1")},
			),
			adapters: map[string]*fakeAdapter{
				"antigravity": {probe: readyProbe, discoverErr: ErrDiscoveryUnavailable},
			},
			check: func(t *testing.T, c Catalog, adapters map[string]*fakeAdapter) {
				pv := c.Profiles[0]
				if pv.Availability != AvailabilityAvailable || pv.Verification != VerificationUnverified || pv.Origin != OriginDeclared {
					t.Fatalf("perfil sin descubrimiento inesperado: %+v", pv)
				}
				if len(c.Inventory) != 0 {
					t.Fatalf("inventario inesperado: %+v", c.Inventory)
				}
			},
		},
		{
			name: "sin adaptador registrado marca no disponible",
			mode: DiscoveryLocalOnly,
			policy: policyWith(
				[]config.RouterExecutor{cliExecutor("antigravity", true)},
				[]config.RouterProfile{profile("antigravity/m1", "antigravity", "m1")},
			),
			adapters: map[string]*fakeAdapter{},
			check: func(t *testing.T, c Catalog, adapters map[string]*fakeAdapter) {
				pv := c.Profiles[0]
				if pv.Availability != AvailabilityUnavailable || pv.Reason != "adaptador no registrado" {
					t.Fatalf("perfil sin adaptador inesperado: %+v", pv)
				}
				if len(c.Executors) != 1 || c.Executors[0].Probe.Reason != "adaptador no registrado" {
					t.Fatalf("vista de ejecutor sin adaptador inesperada: %+v", c.Executors)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := NewRegistry()
			for id, a := range tc.adapters {
				reg.Register(id, a)
			}
			c := Build(context.Background(), tc.policy, reg, tc.mode)
			if tc.check == nil {
				t.Fatal("caso sin comprobacion")
			}
			tc.check(t, c, tc.adapters)
		})
	}
}
