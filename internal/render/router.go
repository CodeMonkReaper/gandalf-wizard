package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/router"
)

// ModelsListProfile is one profile row in `models list` output.
type ModelsListProfile struct {
	ID           string   `json:"id"`
	Executor     string   `json:"executor"`
	Model        string   `json:"model"`
	Efforts      []string `json:"efforts"`
	Authorized   bool     `json:"authorized"`
	Availability string   `json:"availability"`
	Reason       string   `json:"reason,omitempty"`
	Origin       string   `json:"origin"`
}

// ModelsListJSON is the stable JSON payload for `gandalf models list --json`.
type ModelsListJSON struct {
	SchemaVersion string              `json:"schema_version"`
	Command       string              `json:"command"`
	Profiles      []ModelsListProfile `json:"profiles"`
}

// RouteAlternative is one considered profile in `route` output.
type RouteAlternative struct {
	Profile string `json:"profile"`
	Status  string `json:"status"`
	Detail  string `json:"detail"`
}

// RouteJSON is the stable JSON payload for `gandalf route --json`.
type RouteJSON struct {
	SchemaVersion string             `json:"schema_version"`
	Command       string             `json:"command"`
	Status        string             `json:"status"`
	Profile       string             `json:"profile"`
	Effort        string             `json:"effort"`
	Criteria      string             `json:"criteria"`
	Alternatives  []RouteAlternative `json:"alternatives"`
	Fallback      string             `json:"fallback"`
}

// RunAttempt is one attempt row in `run` output.
type RunAttempt struct {
	Profile string `json:"profile"`
	Effort  string `json:"effort"`
	Outcome string `json:"outcome"`
	Reason  string `json:"reason"`
}

// RunJSON is the stable JSON payload for `gandalf run --json`.
type RunJSON struct {
	SchemaVersion string       `json:"schema_version"`
	Command       string       `json:"command"`
	Status        string       `json:"status"`
	Profile       string       `json:"profile"`
	Effort        string       `json:"effort"`
	Attempts      []RunAttempt `json:"attempts"`
	Explanation   string       `json:"explanation"`
	Output        string       `json:"output"`
}

// executorDisplay names an executor for tables: CLI executors by ID, API
// executors as kind:protocol.
func executorDisplay(kind, id, protocol string) string {
	if kind == "api" && protocol != "" {
		return "api:" + protocol
	}
	return id
}

// effortsDisplay lists the declared efforts sorted, or the em dash when
// the profile declares none.
func effortsDisplay(efforts map[string]string) []string {
	keys := make([]string, 0, len(efforts))
	for k := range efforts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if keys == nil {
		keys = []string{}
	}
	return keys
}

// availabilityDisplay renders the availability state with its reason.
func availabilityDisplay(authorized bool, availability, reason string) string {
	if !authorized {
		return "no autorizado"
	}
	if availability == string(router.AvailabilityAvailable) {
		return "disponible"
	}
	if reason != "" {
		return availability + ": " + reason
	}
	return availability
}

// RenderModelsList writes the `gandalf models list` output as a Markdown
// table or stable JSON. The executor views carry the kind and protocol that
// the profile rows cannot show on their own.
func RenderModelsList(w io.Writer, profiles []router.ProfileView, executors []router.ProbeView, asJSON bool) error {
	byID := make(map[string]router.ProbeView, len(executors))
	for _, ev := range executors {
		byID[ev.Executor.ID] = ev
	}
	profileRow := func(pv router.ProfileView) ModelsListProfile {
		executor := pv.Profile.ExecutorID
		if ev, ok := byID[pv.Profile.ExecutorID]; ok {
			executor = executorDisplay(ev.Executor.Kind, ev.Executor.ID, ev.Executor.Protocol)
		}
		return ModelsListProfile{
			ID:           pv.Profile.ID,
			Executor:     executor,
			Model:        pv.Profile.Model,
			Efforts:      effortsDisplay(pv.Profile.Efforts),
			Authorized:   pv.Authorized,
			Availability: string(pv.Availability),
			Reason:       pv.Reason,
			Origin:       string(pv.Origin),
		}
	}
	if asJSON {
		rows := make([]ModelsListProfile, 0, len(profiles))
		for _, pv := range profiles {
			rows = append(rows, profileRow(pv))
		}
		return writeJSON(w, ModelsListJSON{
			SchemaVersion: SchemaVersion,
			Command:       "models list",
			Profiles:      rows,
		})
	}
	var b strings.Builder
	b.WriteString("| Perfil | Ejecutor | Modelo | Esfuerzos | Disponibilidad | Origen |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, pv := range profiles {
		row := profileRow(pv)
		effortsText := "—"
		if len(row.Efforts) > 0 {
			effortsText = strings.Join(row.Efforts, ", ")
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n",
			row.ID,
			row.Executor,
			row.Model,
			effortsText,
			availabilityDisplay(row.Authorized, row.Availability, row.Reason),
			row.Origin,
		)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// RenderRoute writes the `gandalf route` output as explained text or stable
// JSON.
func RenderRoute(w io.Writer, view RouteJSON, asJSON bool) error {
	if asJSON {
		return writeJSON(w, view)
	}
	var b strings.Builder
	profile := view.Profile
	if profile == "" {
		profile = "ninguno"
	}
	effort := view.Effort
	if effort == "" {
		effort = "sin esfuerzo"
	}
	fmt.Fprintf(&b, "Perfil elegido: %s · esfuerzo: %s\n", profile, effort)
	fmt.Fprintf(&b, "Criterios: %s\n", view.Criteria)
	b.WriteString("Alternativas:\n")
	for _, alt := range view.Alternatives {
		fmt.Fprintf(&b, "- %s: %s — %s\n", alt.Profile, alt.Status, alt.Detail)
	}
	fallback := view.Fallback
	if fallback == "" {
		fallback = "ninguno"
	}
	fmt.Fprintf(&b, "Fallback previsto: %s\n", fallback)
	_, err := io.WriteString(w, b.String())
	return err
}

// RenderRun writes the `gandalf run` output as a summary or stable JSON.
func RenderRun(w io.Writer, view RunJSON, asJSON bool) error {
	if asJSON {
		return writeJSON(w, view)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Estado: %s\n", view.Status)
	if view.Profile != "" {
		effort := view.Effort
		if effort == "" {
			effort = "sin esfuerzo"
		}
		fmt.Fprintf(&b, "Perfil: %s · esfuerzo: %s\n", view.Profile, effort)
	}
	if len(view.Attempts) > 0 {
		b.WriteString("Intentos:\n")
		for _, a := range view.Attempts {
			reason := a.Reason
			if reason != "" {
				reason = " (" + reason + ")"
			}
			fmt.Fprintf(&b, "- %s: %s%s\n", a.Profile, a.Outcome, reason)
		}
	}
	if view.Explanation != "" {
		fmt.Fprintf(&b, "Explicación: %s\n", view.Explanation)
	}
	if view.Output != "" {
		fmt.Fprintf(&b, "Salida:\n%s\n", view.Output)
	}
	_, err := io.WriteString(w, b.String())
	return err
}
