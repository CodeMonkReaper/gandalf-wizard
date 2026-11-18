package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/doctor"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/romero"
)

// RenderVersion writes the `gandalf version` output as plain text or stable JSON.
func RenderVersion(w io.Writer, view VersionView, asJSON bool) error {
	if asJSON {
		return writeJSON(w, VersionJSON{
			SchemaVersion: SchemaVersion,
			Command:       "version",
			Version:       view.Version,
		})
	}
	_, err := fmt.Fprintf(w, "gandalf %s\n", view.Version)
	return err
}

// RenderConfigShow writes the `gandalf config show` output as a Markdown table or stable JSON.
func RenderConfigShow(w io.Writer, view ConfigShowView, asJSON bool) error {
	if asJSON {
		entries := nonNilEntries(view.Entries)
		warnings := nonNilStrings(view.Warnings)
		return writeJSON(w, ConfigShowJSON{
			SchemaVersion: SchemaVersion,
			Command:       "config show",
			Entries:       entries,
			Warnings:      warnings,
		})
	}

	var b strings.Builder
	b.WriteString("| Clave | Valor | Origen |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, e := range view.Entries {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", e.Key, e.Value, e.Origin)
	}
	for _, warning := range view.Warnings {
		fmt.Fprintf(&b, "Aviso: %s\n", warning)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// RenderDoctor writes the `gandalf doctor` output as labeled check lines or stable JSON.
// Router executors are informational: they never add to the failure count.
func RenderDoctor(w io.Writer, report doctor.Report, asJSON bool) error {
	if asJSON {
		checks := make([]doctor.CheckResult, 0, len(report.Checks))
		checks = append(checks, report.Checks...)
		return writeJSON(w, DoctorJSON{
			SchemaVersion: SchemaVersion,
			Command:       "doctor",
			Checks:        checks,
			Failures:      report.Failures,
			Warnings:      report.Warnings,
			Executors:     report.Executors,
		})
	}

	var b strings.Builder
	for _, c := range report.Checks {
		fmt.Fprintf(&b, "%-6s %-18s%s\n", "["+string(c.Status)+"]", c.Name, c.Detail)
		if c.Remediation != "" {
			fmt.Fprintf(&b, "       → %s\n", c.Remediation)
		}
	}
	fmt.Fprintf(&b, "Resultado: fallos=%d avisos=%d\n", report.Failures, report.Warnings)
	if len(report.Executors) > 0 {
		b.WriteString("\nEjecutores del router:\n")
		for _, e := range report.Executors {
			installed := "no instalado"
			if e.Installed {
				installed = "instalado"
			}
			authorized := "no autorizado"
			if e.Authorized {
				authorized = "autorizado"
			}
			line := fmt.Sprintf("- %s: %s, %s, autenticación %s, solo texto %s", e.ID, installed, authorized, e.Auth, e.TextOnly)
			if e.Reason != "" {
				line += fmt.Sprintf(" (%s)", e.Reason)
			}
			b.WriteString(line + "\n")
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// RenderMemoryQuery writes the `gandalf memory query` output as an attributed
// Markdown table or stable JSON. JSON entries are always a list and source_ref
// is null when the original source is unavailable.
func RenderMemoryQuery(w io.Writer, view MemoryQueryView, asJSON bool) error {
	if asJSON {
		entries := make([]MemoryQueryJSONEntry, 0, len(view.Outcome.Entries))
		for _, e := range view.Outcome.Entries {
			var sourceRef *string
			if trimmed := strings.TrimSpace(e.SourceRefDisplay); trimmed != "" {
				sourceRef = &trimmed
			}
			vigencia := e.VigenciaStatus
			if vigencia == "" {
				vigencia = romero.VigenciaActive
			}
			entries = append(entries, MemoryQueryJSONEntry{
				Summary:        e.Summary,
				EpistemicState: e.EpistemicState,
				Vigencia:       string(vigencia),
				MemoryRef:      e.MemoryID,
				SourceRef:      sourceRef,
			})
		}
		return writeJSON(w, MemoryQueryJSON{
			SchemaVersion:     SchemaVersion,
			Command:           "memory query",
			Project:           view.Project,
			Status:            string(view.Outcome.Status),
			StatusText:        view.View.StatusText,
			Degraded:          view.Outcome.Degraded,
			DegradationNotice: view.View.DegradationNotice,
			Entries:           entries,
		})
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Proyecto: %s\n", view.Project)
	if view.View.StatusText != "" {
		fmt.Fprintf(&b, "\n%s\n", view.View.StatusText)
	}
	if view.View.DegradationNotice != "" {
		fmt.Fprintf(&b, "\n%s\n", view.View.DegradationNotice)
	}
	if len(view.View.Rows) > 0 {
		b.WriteString("\n")
		b.WriteString("| Resumen atribuido a ROMERO | Estado declarado | Vigencia | Referencia del recuerdo | Referencia de la fuente original |\n")
		b.WriteString("| --- | --- | --- | --- | --- |\n")
		for _, row := range view.View.Rows {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", row.Summary, row.EpistemicState, row.VigenciaLabel, row.MemoryRef, row.SourceRef)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// writeJSON encodes the payload with stable two-space indentation and a trailing newline.
func writeJSON(w io.Writer, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// nonNilEntries guarantees a non-nil slice so JSON emits [] instead of null.
func nonNilEntries(entries []ConfigEntry) []ConfigEntry {
	if entries == nil {
		return []ConfigEntry{}
	}
	return entries
}

// nonNilStrings guarantees a non-nil slice so JSON emits [] instead of null.
func nonNilStrings(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}
