// Package doctor executes read-only environment and ROMERO connectivity diagnostics.
package doctor

import (
	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/harness"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/romero"
)

// CheckStatus classifies the outcome of an individual diagnostic check.
type CheckStatus string

const (
	StatusOK   CheckStatus = "OK"
	StatusWarn CheckStatus = "WARN"
	StatusFail CheckStatus = "FAIL"
)

// Ordered diagnostic check names required by the cli-gandalf specification.
const (
	CheckConfig     = "configuración"
	CheckProject    = "proyecto"
	CheckRomeroURL  = "romero.url"
	CheckCredential = "credencial"
	CheckRomeroConn = "romero.conexión"
	CheckHarness    = "harness"
)

// CheckResult holds the single-check status, detail line, and optional remediation hint.
type CheckResult struct {
	Name        string      `json:"name"`
	Status      CheckStatus `json:"status"`
	Detail      string      `json:"detail"`
	Remediation string      `json:"remediation,omitempty"`
}

// Report aggregates all six diagnostic results and counts failures and warnings.
type Report struct {
	Checks   []CheckResult `json:"checks"`
	Failures int           `json:"failures"`
	Warnings int           `json:"warnings"`
}

// Input provides pre-loaded configuration, ports, and harness state to doctor.Run.
type Input struct {
	ConfigLoadErr error
	Effective     config.EffectiveConfig
	APIKeyPresent bool
	StatusPort    romero.RomeroStatusPort
	HarnessReport harness.DetectionReport
}
