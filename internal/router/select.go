package router

import (
	"strconv"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
)

// Complexity declares the task complexity; the CLI translates flags (T008).
type Complexity string

const (
	ComplexityLow    Complexity = "baja"
	ComplexityMedium Complexity = "media"
	ComplexityHigh   Complexity = "alta"
)

// Risk declares the task risk; the CLI translates flags (T008).
type Risk string

const (
	RiskLow    Risk = "bajo"
	RiskMedium Risk = "medio"
	RiskHigh   Risk = "alto"
)

// Features declares the task characteristics that drive selection.
type Features struct {
	Complexity Complexity
	Risk       Risk
}

// Choice is one eligible profile in selection order with its effort handling.
type Choice struct {
	Profile     config.RouterProfile
	Effort      string // empty means no effort is sent
	Explanation string
}

// Assessment records why one catalog profile was or was not eligible.
type Assessment struct {
	ProfileID string
	Eligible  bool
	Reason    string
}

// Decision is the ordered, explained selection result.
type Decision struct {
	Ordered    []Choice
	Considered []Assessment
	Manual     bool
}

// Select computes a deterministic, explained selection. It is pure: no
// environment, clock, randomness, or task inference. The declared order of
// the catalog rules; effort never reorders candidates.
func Select(c Catalog, f Features, manualID string) Decision {
	if f.Complexity == "" {
		f.Complexity = ComplexityMedium
	}
	if f.Risk == "" {
		f.Risk = RiskMedium
	}
	requested := requestedEffort(f)

	decision := Decision{Ordered: []Choice{}, Considered: []Assessment{}, Manual: manualID != ""}

	pool := c.Profiles
	if manualID != "" {
		found := false
		for _, pv := range c.Profiles {
			if pv.Profile.ID == manualID {
				pool = []ProfileView{pv}
				found = true
				break
			}
		}
		if !found {
			decision.Considered = append(decision.Considered, Assessment{
				ProfileID: manualID,
				Eligible:  false,
				Reason:    manualUnknownReason(c, manualID),
			})
			return decision
		}
	}

	position := 0
	for _, pv := range pool {
		eligible, reason := assess(pv, f)
		decision.Considered = append(decision.Considered, Assessment{
			ProfileID: pv.Profile.ID,
			Eligible:  eligible,
			Reason:    reason,
		})
		if !eligible {
			continue
		}
		position++
		decision.Ordered = append(decision.Ordered, choiceFor(pv, requested, position, manualID))
	}

	if manualID == "" {
		for _, iv := range c.Inventory {
			decision.Considered = append(decision.Considered, Assessment{
				ProfileID: iv.Profile.ID,
				Eligible:  false,
				Reason:    "candidato de inventario, sin posición declarada",
			})
		}
	}
	return decision
}

// requestedEffort maps complexity to the effort key (baja→low, media→medium, alta→high).
func requestedEffort(f Features) string {
	switch f.Complexity {
	case ComplexityLow:
		return config.RouterEffortLow
	case ComplexityHigh:
		return config.RouterEffortHigh
	default:
		return config.RouterEffortMedium
	}
}

// assess applies the eligibility rules to one profile view.
func assess(pv ProfileView, f Features) (bool, string) {
	switch {
	case !pv.Authorized:
		return false, "no autorizado"
	case pv.Availability != AvailabilityAvailable:
		if pv.Reason != "" {
			return false, "no disponible (" + pv.Reason + ")"
		}
		return false, "no disponible"
	case f.Risk == RiskHigh && pv.Verification != VerificationVerified:
		return false, "no verificado"
	default:
		return true, ""
	}
}

// choiceFor builds the ordered choice with its effort handling and explanation.
func choiceFor(pv ProfileView, requested string, position int, manualID string) Choice {
	ch := Choice{Profile: pv.Profile}
	_, hasEffort := pv.Profile.Efforts[requested]
	if hasEffort {
		ch.Effort = requested
	}
	origin := "posición " + strconv.Itoa(position) + " del orden declarado"
	if manualID != "" {
		origin = "perfil manual"
	}
	if hasEffort {
		ch.Explanation = origin + "; admite esfuerzo " + requested
	} else {
		ch.Explanation = origin + "; no admite esfuerzo " + requested + ", se ejecuta sin esfuerzo"
	}
	return ch
}

// manualUnknownReason explains a manual ID that is not a declared profile.
func manualUnknownReason(c Catalog, manualID string) string {
	for _, iv := range c.Inventory {
		if iv.Profile.ID == manualID {
			return "candidato de inventario, sin posición declarada"
		}
	}
	return "perfil no declarado"
}
