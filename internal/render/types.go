// Package render formats deterministic human-readable and JSON CLI outputs.
package render

import (
	"github.com/CodeMonkReaper/gandalf-wizard/internal/doctor"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/romero"
)

// SchemaVersion identifies the stable JSON output contract version.
const SchemaVersion = "1"

// VersionView carries data for rendering `gandalf version`.
type VersionView struct {
	Version string
}

// VersionJSON is the stable JSON payload for `gandalf version --json`.
type VersionJSON struct {
	SchemaVersion string `json:"schema_version"`
	Command       string `json:"command"`
	Version       string `json:"version"`
}

// ConfigEntry represents one resolved configuration row for `config show`.
type ConfigEntry struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Origin string `json:"origin"`
}

// ConfigShowView carries data for rendering `gandalf config show`.
type ConfigShowView struct {
	Entries  []ConfigEntry
	Warnings []string
}

// ConfigShowJSON is the stable JSON payload for `gandalf config show --json`.
type ConfigShowJSON struct {
	SchemaVersion string        `json:"schema_version"`
	Command       string        `json:"command"`
	Entries       []ConfigEntry `json:"entries"`
	Warnings      []string      `json:"warnings"`
}

// DoctorJSON is the stable JSON payload for `gandalf doctor --json`.
type DoctorJSON struct {
	SchemaVersion string                     `json:"schema_version"`
	Command       string                     `json:"command"`
	Checks        []doctor.CheckResult       `json:"checks"`
	Failures      int                        `json:"failures"`
	Warnings      int                        `json:"warnings"`
	Executors     []doctor.ExecutorDiagnosis `json:"executors,omitempty"`
}

// MemoryQueryView carries data for rendering `gandalf memory query`.
type MemoryQueryView struct {
	Project string
	Outcome romero.MemoryQueryOutcome
	View    romero.PresentedMemoryView
}

// MemoryQueryJSONEntry represents one attributed memory item in JSON output.
type MemoryQueryJSONEntry struct {
	Summary        string  `json:"summary"`
	EpistemicState string  `json:"epistemic_state"`
	Vigencia       string  `json:"vigencia"`
	MemoryRef      string  `json:"memory_ref"`
	SourceRef      *string `json:"source_ref"`
}

// MemoryQueryJSON is the stable JSON payload for `gandalf memory query --json`.
type MemoryQueryJSON struct {
	SchemaVersion     string                 `json:"schema_version"`
	Command           string                 `json:"command"`
	Project           string                 `json:"project"`
	Status            string                 `json:"status"`
	StatusText        string                 `json:"status_text"`
	Degraded          bool                   `json:"degraded"`
	DegradationNotice string                 `json:"degradation_notice"`
	Entries           []MemoryQueryJSONEntry `json:"entries"`
}
