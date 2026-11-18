package router

import (
	"context"
	"sort"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
)

// Availability states of a profile view.
type Availability string

const (
	AvailabilityUnauthorized Availability = "no autorizado"
	AvailabilityUnavailable  Availability = "no disponible"
	AvailabilityAvailable    Availability = "disponible"
)

// Verification states: whether the model's presence has evidence.
type Verification string

const (
	VerificationVerified   Verification = "verificado"
	VerificationUnverified Verification = "no verificado"
)

// Origin states: where the profile's model data came from.
type Origin string

const (
	OriginDeclared   Origin = "declarado"
	OriginDiscovered Origin = "descubierto"
)

// ProfileView is one catalog row for a profile.
type ProfileView struct {
	Profile      config.RouterProfile
	Authorized   bool
	Availability Availability
	Reason       string
	Origin       Origin
	Verification Verification
}

// ProbeView is one catalog row for an executor's local diagnosis.
type ProbeView struct {
	Executor config.RouterExecutor
	Probe    Probe
}

// Catalog is the per-invocation snapshot of profiles and executor states.
// Profiles holds declared profiles in declared order; Inventory holds
// discovered models without a declared profile, sorted by ID and excluded
// from selection.
type Catalog struct {
	Profiles  []ProfileView
	Inventory []ProfileView
	Executors []ProbeView
}

// execState holds the per-executor evidence gathered while building.
type execState struct {
	executor   config.RouterExecutor
	probe      Probe
	confirmed  map[string]bool
	discovered []Candidate
}

// Build composes the catalog snapshot for one command. It probes every
// executor locally and runs documented, quota-free discovery only for
// authorized executors when the mode allows it. No mode invokes Execute.
func Build(ctx context.Context, policy config.RouterPolicy, reg *Registry, mode DiscoveryMode) Catalog {
	catalog := Catalog{Profiles: []ProfileView{}, Inventory: []ProfileView{}, Executors: []ProbeView{}}

	states := map[string]*execState{}

	for _, ex := range policy.Executors {
		st := &execState{executor: ex}
		adapter, ok := reg.AdapterFor(ex.ID)
		if !ok {
			st.probe = Probe{Auth: AuthUnknown, TextOnly: TextOnlyUnproven, Reason: "adaptador no registrado"}
		} else {
			st.probe = adapter.Probe(ctx, ex)
		}
		catalog.Executors = append(catalog.Executors, ProbeView{Executor: ex, Probe: st.probe})

		if ok && ex.Authorized && mode == DiscoveryLocalAndDocumentedFree {
			candidates, err := adapter.Discover(ctx, ex)
			if err != nil {
				// A failed or unavailable discovery keeps declarations with
				// unknown evidence; it never removes a declared profile.
				states[ex.ID] = st
				continue
			}
			st.confirmed = map[string]bool{}
			for _, c := range candidates {
				st.confirmed[c.Model] = true
				st.discovered = append(st.discovered, c)
			}
		}
		states[ex.ID] = st
	}

	seenPairs := map[string]bool{}
	for _, pf := range policy.Profiles {
		pair := pf.ExecutorID + "\x00" + pf.Model
		if seenPairs[pair] {
			continue
		}
		seenPairs[pair] = true
		catalog.Profiles = append(catalog.Profiles, buildProfileView(pf, states[pf.ExecutorID]))
	}

	for _, st := range states {
		if st == nil || st.confirmed == nil {
			continue
		}
		seenModels := map[string]bool{}
		for _, c := range st.discovered {
			pair := st.executor.ID + "\x00" + c.Model
			if seenPairs[pair] || seenModels[c.Model] {
				continue
			}
			seenModels[c.Model] = true
			view := ProfileView{
				Profile: config.RouterProfile{
					ID:         st.executor.ID + "/" + c.Model,
					ExecutorID: st.executor.ID,
					Model:      c.Model,
					Efforts:    c.Efforts,
				},
				Origin:       OriginDiscovered,
				Verification: VerificationVerified,
			}
			view.Authorized, view.Availability, view.Reason = availabilityFor(st.executor, st.probe)
			catalog.Inventory = append(catalog.Inventory, view)
		}
	}
	sort.Slice(catalog.Inventory, func(i, j int) bool {
		return catalog.Inventory[i].Profile.ID < catalog.Inventory[j].Profile.ID
	})
	return catalog
}

// buildProfileView maps one declared profile to its catalog view.
func buildProfileView(pf config.RouterProfile, st *execState) ProfileView {
	view := ProfileView{Profile: pf}
	if st == nil {
		view.Authorized = false
		view.Availability = AvailabilityUnavailable
		view.Reason = "ejecutor no declarado"
		view.Origin = OriginDeclared
		view.Verification = VerificationUnverified
		return view
	}
	view.Authorized, view.Availability, view.Reason = availabilityFor(st.executor, st.probe)
	view.Origin = OriginDeclared
	view.Verification = VerificationUnverified
	if st.confirmed != nil && st.confirmed[pf.Model] {
		view.Origin = OriginDiscovered
		view.Verification = VerificationVerified
	}
	return view
}

// availabilityFor derives the availability state from authorization and the
// local probe. The probe's own reason wins over the default reason.
func availabilityFor(ex config.RouterExecutor, probe Probe) (bool, Availability, string) {
	if !ex.Authorized {
		return false, AvailabilityUnauthorized, "ejecutor no autorizado"
	}
	switch {
	case !probe.Installed:
		return true, AvailabilityUnavailable, probeReason(probe.Reason, "ejecutor no instalado")
	case probe.Auth == AuthMissing:
		return true, AvailabilityUnavailable, probeReason(probe.Reason, "falta la credencial")
	case probe.Auth == AuthUnknown:
		return true, AvailabilityUnavailable, probeReason(probe.Reason, "autenticación desconocida")
	case probe.TextOnly != TextOnlyEnforced:
		return true, AvailabilityUnavailable, probeReason(probe.Reason, "modo solo texto no demostrado")
	default:
		return true, AvailabilityAvailable, ""
	}
}

// probeReason prefers the adapter's sanitized reason over the default.
func probeReason(adapterReason, def string) string {
	if adapterReason != "" {
		return adapterReason
	}
	return def
}
