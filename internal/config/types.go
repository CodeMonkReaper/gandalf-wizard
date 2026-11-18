// Package config resolves layered Gandalf configuration without storing or exposing credentials.
package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Origin identifies which configuration layer provided an effective value.
type Origin string

const (
	OriginFlag    Origin = "flag"
	OriginEnv     Origin = "entorno"
	OriginProject Origin = "proyecto"
	OriginGlobal  Origin = "global"
	OriginDefault Origin = "predeterminado"
)

// Canonical defaults, file names, and environment variable names.
const (
	DefaultBaseURL        = "http://127.0.0.1:8420"
	DefaultTimeout        = 10 * time.Second
	ProjectConfigFileName = ".gandalf.json"
	GlobalConfigDirName   = "gandalf"
	GlobalConfigFileName  = "config.json"

	EnvKeyProject       = "GANDALF_PROJECT"
	EnvKeyCaller        = "GANDALF_CALLER"
	EnvKeyRomeroBaseURL = "GANDALF_ROMERO_BASE_URL"
	EnvKeyRomeroTimeout = "GANDALF_ROMERO_TIMEOUT"
	EnvKeyRomeroAPIKey  = "ROMERO_API_KEY"
)

// Sentinel errors classifying configuration loading and initialization failures.
var (
	ErrInvalidConfig          = errors.New("config: invalid configuration")
	ErrConfigExists           = errors.New("config: project configuration already exists")
	ErrInvalidProjectID       = errors.New("config: invalid project identifier")
	ErrForbiddenCredentialKey = errors.New("config: credential key not allowed in config file")
)

var validProjectIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

// ConfigError describes a sanitized configuration failure citing the file path and reason.
type ConfigError struct {
	Path   string
	Reason string
	Err    error
}

// Error formats the configuration error without revealing secret values.
func (e *ConfigError) Error() string {
	if e == nil {
		return ""
	}
	if e.Path != "" {
		return fmt.Sprintf("error de configuración en %s: %s", e.Path, e.Reason)
	}
	return fmt.Sprintf("error de configuración: %s", e.Reason)
}

// Unwrap exposes the underlying sentinel error for errors.Is checks.
func (e *ConfigError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ValidateProjectID checks that id is non-empty and matches ^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$.
func ValidateProjectID(id string) error {
	if id == "" || strings.TrimSpace(id) != id || !validProjectIDPattern.MatchString(id) {
		return ErrInvalidProjectID
	}
	return nil
}

// IsForbiddenCredentialKey reports whether a config key attempts to store a credential.
func IsForbiddenCredentialKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "api_key", "token", "secret", "password", "credential", "romero_api_key":
		return true
	default:
		return false
	}
}

// TrackedValue holds a resolved configuration value alongside its source layer.
type TrackedValue[T any] struct {
	Value  T
	Origin Origin
}

// EffectiveConfig represents the merged, validated configuration for a Gandalf invocation.
// It intentionally never stores the ROMERO_API_KEY value.
type EffectiveConfig struct {
	Project           TrackedValue[string]
	Caller            TrackedValue[string]
	RomeroBaseURL     TrackedValue[string]
	RomeroTimeout     TrackedValue[time.Duration]
	APIKeyPresent     bool
	APIKeyOrigin      Origin
	ProjectConfigPath string
	GlobalConfigPath  string
	Warnings          []string
}

// FlagOverrides holds per-invocation CLI flags that override lower layers.
type FlagOverrides struct {
	Project       string
	Caller        string
	RomeroBaseURL string
	RomeroTimeout string
}

// LoadInput carries the environment, paths, and flag overrides needed to resolve config.
type LoadInput struct {
	WorkDir       string
	UserConfigDir string
	Getenv        func(string) string
	DefaultCaller func() string
	Overrides     FlagOverrides
}
