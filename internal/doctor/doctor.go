package doctor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/romero"
)

// Run executes the six diagnostics in the fixed order required by the spec:
// configuración, proyecto, romero.url, credencial, romero.conexión, harness.
// A failure in any check never skips the remaining checks.
func Run(ctx context.Context, input Input) Report {
	report := Report{Checks: make([]CheckResult, 0, 6)}
	report.Checks = append(report.Checks, checkConfig(input))
	report.Checks = append(report.Checks, checkProject(input))
	report.Checks = append(report.Checks, checkRomeroURL(input))
	report.Checks = append(report.Checks, checkCredential(input))
	report.Checks = append(report.Checks, checkRomeroConn(ctx, input))
	report.Checks = append(report.Checks, checkHarness(input))

	for _, c := range report.Checks {
		switch c.Status {
		case StatusFail:
			report.Failures++
		case StatusWarn:
			report.Warnings++
		}
	}

	if len(input.RouterExecutors) > 0 {
		report.Executors = make([]ExecutorDiagnosis, 0, len(input.RouterExecutors))
		for _, ev := range input.RouterExecutors {
			report.Executors = append(report.Executors, ExecutorDiagnosis{
				ID:         ev.Executor.ID,
				Kind:       string(ev.Executor.Kind),
				Installed:  ev.Probe.Installed,
				Authorized: ev.Executor.Authorized,
				Auth:       string(ev.Probe.Auth),
				TextOnly:   string(ev.Probe.TextOnly),
				Reason:     ev.Probe.Reason,
			})
		}
	}
	return report
}

// checkConfig reports whether the configuration files loaded cleanly.
func checkConfig(input Input) CheckResult {
	if input.ConfigLoadErr != nil {
		return CheckResult{
			Name:        CheckConfig,
			Status:      StatusFail,
			Detail:      input.ConfigLoadErr.Error(),
			Remediation: "Corrige el archivo de configuración citado.",
		}
	}
	return CheckResult{
		Name:   CheckConfig,
		Status: StatusOK,
		Detail: fmt.Sprintf("proyecto: %s; global: %s", pathOrNotFound(input.Effective.ProjectConfigPath), pathOrNotFound(input.Effective.GlobalConfigPath)),
	}
}

// checkProject reports whether an active project is defined.
func checkProject(input Input) CheckResult {
	if input.Effective.Project.Value == "" {
		return CheckResult{
			Name:        CheckProject,
			Status:      StatusFail,
			Detail:      "proyecto sin definir",
			Remediation: "Ejecuta gandalf config init --project <id> o define GANDALF_PROJECT.",
		}
	}
	return CheckResult{Name: CheckProject, Status: StatusOK, Detail: input.Effective.Project.Value}
}

// checkRomeroURL reports whether a ROMERO base URL is configured.
func checkRomeroURL(input Input) CheckResult {
	if input.Effective.RomeroBaseURL.Value == "" {
		return CheckResult{
			Name:        CheckRomeroURL,
			Status:      StatusFail,
			Detail:      "sin URL de ROMERO",
			Remediation: "Define romero.base_url en la configuración o GANDALF_ROMERO_BASE_URL.",
		}
	}
	return CheckResult{Name: CheckRomeroURL, Status: StatusOK, Detail: input.Effective.RomeroBaseURL.Value}
}

// checkCredential reports credential presence without ever exposing its value.
func checkCredential(input Input) CheckResult {
	if !input.APIKeyPresent {
		return CheckResult{
			Name:        CheckCredential,
			Status:      StatusFail,
			Detail:      "ROMERO_API_KEY no está definida",
			Remediation: "Define ROMERO_API_KEY en el entorno de la sesión.",
		}
	}
	return CheckResult{Name: CheckCredential, Status: StatusOK, Detail: "ROMERO_API_KEY está definida"}
}

// checkRomeroConn probes ROMERO's status endpoint and classifies the failure.
func checkRomeroConn(ctx context.Context, input Input) CheckResult {
	if input.Effective.RomeroBaseURL.Value == "" {
		return CheckResult{
			Name:        CheckRomeroConn,
			Status:      StatusFail,
			Detail:      "conexión no probada: sin URL de ROMERO",
			Remediation: "Define romero.base_url en la configuración o GANDALF_ROMERO_BASE_URL.",
		}
	}
	if input.StatusPort == nil {
		return CheckResult{
			Name:        CheckRomeroConn,
			Status:      StatusFail,
			Detail:      "sonda de estado no disponible",
			Remediation: "Vuelve a instalar gandalf o reporta el error.",
		}
	}

	status, err := input.StatusPort.CheckStatus(ctx)
	if err != nil {
		if errors.Is(err, romero.ErrAccessDenied) {
			return CheckResult{
				Name:        CheckRomeroConn,
				Status:      StatusFail,
				Detail:      "credencial rechazada o ausente (401)",
				Remediation: "Revisa ROMERO_API_KEY.",
			}
		}
		return CheckResult{
			Name:        CheckRomeroConn,
			Status:      StatusFail,
			Detail:      "ROMERO no responde",
			Remediation: "Comprueba que ROMERO esté corriendo y accesible.",
		}
	}
	return CheckResult{
		Name:   CheckRomeroConn,
		Status: StatusOK,
		Detail: fmt.Sprintf("estado: %s; versión: %s; perfil: %s", status.Status, status.Version, status.ActiveProfile),
	}
}

// checkHarness reports the read-only harness and RSC detection results.
func checkHarness(input Input) CheckResult {
	parts := make([]string, 0, len(input.HarnessReport.Harnesses)+1)
	for _, h := range input.HarnessReport.Harnesses {
		state := "no detectado"
		if h.Detected {
			state = "detectado"
		}
		parts = append(parts, fmt.Sprintf("%s: %s", h.Name, state))
	}
	rsc := "ausente"
	if input.HarnessReport.RSCPresent {
		rsc = "presente"
	}
	parts = append(parts, "rsc: "+rsc)
	return CheckResult{Name: CheckHarness, Status: StatusOK, Detail: strings.Join(parts, " · ")}
}

// pathOrNotFound renders a missing configuration path deterministically.
func pathOrNotFound(path string) string {
	if path == "" {
		return "no encontrado"
	}
	return path
}
