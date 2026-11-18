package doctor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/harness"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/romero"
)

func validInput() Input {
	return Input{
		Effective: config.EffectiveConfig{
			Project:           config.TrackedValue[string]{Value: "faro", Origin: config.OriginProject},
			RomeroBaseURL:     config.TrackedValue[string]{Value: "http://127.0.0.1:8420", Origin: config.OriginDefault},
			ProjectConfigPath: `C:\x\.gandalf.json`,
		},
		APIKeyPresent: true,
		StatusPort: &romero.RomeroFakeStatusAdapter{
			Status: romero.SystemStatus{Status: "ok", Version: "1.2.3", ActiveProfile: "lite"},
		},
		HarnessReport: harness.DetectionReport{
			Harnesses: []harness.HarnessStatus{
				{Name: harness.HarnessCodex, Detected: true, Path: "/bin/codex"},
				{Name: harness.HarnessVibe, Detected: true, Path: "/bin/vibe"},
				{Name: harness.HarnessAntigravity},
			},
			RSCPresent:    true,
			RSCConfigPath: `C:\x\.rsc.json`,
		},
	}
}

func checkNames(report Report) []string {
	names := make([]string, 0, len(report.Checks))
	for _, c := range report.Checks {
		names = append(names, c.Name)
	}
	return names
}

func TestDoctorChecks_AllOK(t *testing.T) {
	report := Run(context.Background(), validInput())

	want := []string{CheckConfig, CheckProject, CheckRomeroURL, CheckCredential, CheckRomeroConn, CheckHarness}
	if got := checkNames(report); !equalStrings(got, want) {
		t.Fatalf("orden de checks = %v, want %v", got, want)
	}
	for _, c := range report.Checks {
		if c.Status != StatusOK {
			t.Errorf("check %s = %s, want OK (detalle: %s)", c.Name, c.Status, c.Detail)
		}
	}
	if report.Failures != 0 || report.Warnings != 0 {
		t.Errorf("failures/warnings = %d/%d, want 0/0", report.Failures, report.Warnings)
	}
	harnessCheck := report.Checks[5]
	wantDetail := "codex: detectado · vibe: detectado · antigravity: no detectado · rsc: presente"
	if harnessCheck.Detail != wantDetail {
		t.Errorf("harness detail = %q, want %q", harnessCheck.Detail, wantDetail)
	}
}

func TestDoctorChecks_NoShortCircuitOnFailures(t *testing.T) {
	input := validInput()
	input.ConfigLoadErr = &config.ConfigError{Path: `C:\x\.gandalf.json`, Reason: "JSON inválido", Err: config.ErrInvalidConfig}
	input.Effective = config.EffectiveConfig{}
	input.APIKeyPresent = false
	input.StatusPort = &romero.RomeroFakeStatusAdapter{Err: romero.ErrAccessDenied}
	input.HarnessReport = harness.DetectionReport{}

	report := Run(context.Background(), input)

	want := []string{CheckConfig, CheckProject, CheckRomeroURL, CheckCredential, CheckRomeroConn, CheckHarness}
	if got := checkNames(report); !equalStrings(got, want) {
		t.Fatalf("con fallos múltiples el orden sigue siendo %v, got %v", want, got)
	}
	if len(report.Checks) != 6 {
		t.Fatalf("checks = %d, want 6 (sin cortocircuito)", len(report.Checks))
	}
	for _, c := range report.Checks[:5] {
		if c.Status != StatusFail {
			t.Errorf("check %s = %s, want FAIL", c.Name, c.Status)
		}
		if c.Remediation == "" {
			t.Errorf("check %s FAIL sin línea de remedio", c.Name)
		}
	}
	if report.Checks[5].Status != StatusOK {
		t.Errorf("harness = %s, want OK (detección informativa, referencia 4 de la spec)", report.Checks[5].Status)
	}
	if report.Failures != 5 {
		t.Errorf("failures = %d, want 5", report.Failures)
	}
}

func TestDoctorChecks_CredentialAbsentMatchesReference(t *testing.T) {
	input := validInput()
	input.APIKeyPresent = false
	input.StatusPort = &romero.RomeroFakeStatusAdapter{Err: romero.ErrAccessDenied}

	report := Run(context.Background(), input)

	cred := report.Checks[3]
	if cred.Status != StatusFail || cred.Detail != "ROMERO_API_KEY no está definida" {
		t.Errorf("credencial = %+v", cred)
	}
	if cred.Remediation != "Define ROMERO_API_KEY en el entorno de la sesión." {
		t.Errorf("remedio credencial = %q", cred.Remediation)
	}
	conn := report.Checks[4]
	if conn.Status != StatusFail || conn.Detail != "credencial rechazada o ausente (401)" {
		t.Errorf("romero.conexión = %+v", conn)
	}
	if conn.Remediation != "Revisa ROMERO_API_KEY." {
		t.Errorf("remedio conexión = %q", conn.Remediation)
	}
	if report.Failures != 2 || report.Warnings != 0 {
		t.Errorf("failures/warnings = %d/%d, want 2/0", report.Failures, report.Warnings)
	}
}

func TestDoctorChecks_ServiceInterrupted(t *testing.T) {
	input := validInput()
	input.StatusPort = &romero.RomeroFakeStatusAdapter{Err: fmt.Errorf("%w: boom", romero.ErrServiceInterrupted)}

	report := Run(context.Background(), input)

	conn := report.Checks[4]
	if conn.Status != StatusFail || !strings.Contains(conn.Detail, "ROMERO no responde") {
		t.Errorf("romero.conexión = %+v, want FAIL «ROMERO no responde»", conn)
	}
	if report.Failures != 1 {
		t.Errorf("failures = %d, want 1", report.Failures)
	}
}

func TestDoctorChecks_NilStatusPortFailsGracefully(t *testing.T) {
	input := validInput()
	input.StatusPort = nil

	report := Run(context.Background(), input)

	if len(report.Checks) != 6 {
		t.Fatalf("checks = %d, want 6", len(report.Checks))
	}
	conn := report.Checks[4]
	if conn.Status != StatusFail {
		t.Errorf("romero.conexión con puerto nil = %s, want FAIL", conn.Status)
	}
}

func TestDoctorChecks_ConfigErrorDetailIsSanitized(t *testing.T) {
	input := validInput()
	input.ConfigLoadErr = &config.ConfigError{
		Path:   `C:\x\.gandalf.json`,
		Reason: "credencial no permitida en archivo de configuración",
		Err:    config.ErrForbiddenCredentialKey,
	}

	report := Run(context.Background(), input)

	cfgCheck := report.Checks[0]
	if cfgCheck.Status != StatusFail {
		t.Fatalf("configuración = %s, want FAIL", cfgCheck.Status)
	}
	if !strings.Contains(cfgCheck.Detail, "credencial no permitida") || !strings.Contains(cfgCheck.Detail, `.gandalf.json`) {
		t.Errorf("detalle configuración = %q, debe citar archivo y motivo", cfgCheck.Detail)
	}
	if strings.Contains(cfgCheck.Detail, "supersecret") {
		t.Errorf("detalle configuración filtra valores: %q", cfgCheck.Detail)
	}
	if !errors.Is(input.ConfigLoadErr, config.ErrForbiddenCredentialKey) {
		t.Error("el error de configuración debe conservar la cadena de envoltura")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
