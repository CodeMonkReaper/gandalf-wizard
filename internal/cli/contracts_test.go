package cli_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/cli"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/doctor"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/harness"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/render"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/romero"
)

type stubStatusPort struct{}

func (stubStatusPort) CheckStatus(_ context.Context) (romero.SystemStatus, error) {
	return romero.SystemStatus{Status: "ok"}, nil
}

var _ romero.RomeroStatusPort = stubStatusPort{}

func TestContracts_T001(t *testing.T) {
	t.Run("exit codes match spec table 5", func(t *testing.T) {
		got := []int{
			cli.ExitOK,
			cli.ExitInternalError,
			cli.ExitUsageError,
			cli.ExitConfigError,
			cli.ExitAccessDenied,
			cli.ExitPartialQuery,
			cli.ExitIncompleteQuery,
			cli.ExitDoctorFailed,
		}
		for i, code := range got {
			if code != i {
				t.Fatalf("exit code[%d] = %d, want %d", i, code, i)
			}
		}
	})

	t.Run("config origins and defaults", func(t *testing.T) {
		origins := []config.Origin{
			config.OriginFlag,
			config.OriginEnv,
			config.OriginProject,
			config.OriginGlobal,
			config.OriginDefault,
		}
		want := []config.Origin{"flag", "entorno", "proyecto", "global", "predeterminado"}
		for i := range origins {
			if origins[i] != want[i] {
				t.Fatalf("origin[%d] = %q, want %q", i, origins[i], want[i])
			}
		}
		if config.DefaultBaseURL != "http://127.0.0.1:8420" {
			t.Fatalf("DefaultBaseURL = %q", config.DefaultBaseURL)
		}
		if config.ProjectConfigFileName != ".gandalf.json" {
			t.Fatalf("ProjectConfigFileName = %q", config.ProjectConfigFileName)
		}
	})

	t.Run("project id and credential key validation helpers", func(t *testing.T) {
		for _, valid := range []string{"faro", "Faro_1", "proj.v2", "550e8400-e29b-41d4-a716-446655440000"} {
			if err := config.ValidateProjectID(valid); err != nil {
				t.Fatalf("ValidateProjectID(%q) unexpected error: %v", valid, err)
			}
		}
		for _, invalid := range []string{"", " ", " faro", "faro ", "a b", "../escape", "-bad"} {
			if err := config.ValidateProjectID(invalid); !errors.Is(err, config.ErrInvalidProjectID) {
				t.Fatalf("ValidateProjectID(%q) = %v, want ErrInvalidProjectID", invalid, err)
			}
		}
		for _, key := range []string{"api_key", "API_KEY", "token", "secret", "password", "credential", "romero_api_key"} {
			if !config.IsForbiddenCredentialKey(key) {
				t.Fatalf("IsForbiddenCredentialKey(%q) = false, want true", key)
			}
		}
		if config.IsForbiddenCredentialKey("base_url") {
			t.Fatalf("IsForbiddenCredentialKey(\"base_url\") = true, want false")
		}
	})

	t.Run("config error formatting and unwrap", func(t *testing.T) {
		ce := &config.ConfigError{
			Path:   "/tmp/.gandalf.json",
			Reason: "JSON inválido",
			Err:    config.ErrInvalidConfig,
		}
		if !errors.Is(ce, config.ErrInvalidConfig) {
			t.Fatalf("ConfigError did not unwrap ErrInvalidConfig")
		}
		if ce.Error() == "" {
			t.Fatalf("ConfigError.Error() is empty")
		}
	})

	t.Run("doctor, harness, render and cli dependencies types compile", func(t *testing.T) {
		if render.SchemaVersion != "1" {
			t.Fatalf("SchemaVersion = %q, want 1", render.SchemaVersion)
		}
		if doctor.CheckRomeroConn != "romero.conexión" {
			t.Fatalf("CheckRomeroConn = %q", doctor.CheckRomeroConn)
		}
		_ = harness.DetectionReport{
			Harnesses: []harness.HarnessStatus{{Name: harness.HarnessCodex, Detected: true}},
		}
		_ = doctor.Input{
			StatusPort: stubStatusPort{},
		}
		var buf bytes.Buffer
		_ = cli.Dependencies{
			Version:    "0.1.0",
			Stdout:     &buf,
			Stderr:     &buf,
			HTTPClient: http.DefaultClient,
			StatusPort: stubStatusPort{},
		}
	})
}
