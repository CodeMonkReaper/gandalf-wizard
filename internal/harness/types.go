// Package harness detects installed AI harnesses and RSC metadata in read-only mode.
package harness

// Known harness names detected on PATH.
const (
	HarnessCodex       = "codex"
	HarnessVibe        = "vibe"
	HarnessAntigravity = "antigravity"
)

// HarnessStatus reports whether a single harness executable was detected.
type HarnessStatus struct {
	Name     string `json:"name"`
	Detected bool   `json:"detected"`
	Path     string `json:"path,omitempty"`
}

// DetectionReport summarizes read-only harness and RSC detection results.
type DetectionReport struct {
	Harnesses     []HarnessStatus `json:"harnesses"`
	RSCPresent    bool            `json:"rsc_present"`
	RSCConfigPath string          `json:"rsc_config_path,omitempty"`
}
