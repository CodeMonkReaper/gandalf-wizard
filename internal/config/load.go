package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// configFile mirrors the admitted JSON schema of Gandalf configuration files.
type configFile struct {
	Project *string           `json:"project"`
	Caller  *string           `json:"caller"`
	Romero  *configFileRomero `json:"romero"`
}

// configFileRomero holds the ROMERO connection block of a configuration file.
type configFileRomero struct {
	BaseURL *string `json:"base_url"`
	Timeout *string `json:"timeout"`
}

// fileValues holds validated values read from a single configuration file.
type fileValues struct {
	Project  *string
	Caller   *string
	BaseURL  *string
	Timeout  *time.Duration
	Warnings []string
}

// Load resolves the effective Gandalf configuration from five layers:
// flag > entorno > proyecto > global > predeterminado.
// It never stores or returns the ROMERO_API_KEY value.
func Load(input LoadInput) (EffectiveConfig, error) {
	getenv := input.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}

	projectPath := discoverProjectConfig(input.WorkDir)
	projectFile, err := parseConfigFile(projectPath)
	if err != nil {
		return EffectiveConfig{}, err
	}

	globalPath := globalConfigPath(input.UserConfigDir)
	globalFile, err := parseConfigFile(globalPath)
	if err != nil {
		return EffectiveConfig{}, err
	}

	warnings := append([]string{}, projectFile.Warnings...)
	warnings = append(warnings, globalFile.Warnings...)

	overrides := input.Overrides

	flagProject, err := validateOptionalProject(overrides.Project, OriginFlag)
	if err != nil {
		return EffectiveConfig{}, err
	}
	envProject, err := validateOptionalProject(getenv(EnvKeyProject), OriginEnv)
	if err != nil {
		return EffectiveConfig{}, err
	}
	flagURL, err := validateOptionalBaseURL(overrides.RomeroBaseURL, OriginFlag)
	if err != nil {
		return EffectiveConfig{}, err
	}
	envURL, err := validateOptionalBaseURL(getenv(EnvKeyRomeroBaseURL), OriginEnv)
	if err != nil {
		return EffectiveConfig{}, err
	}
	flagTimeout, err := validateOptionalTimeout(overrides.RomeroTimeout, OriginFlag)
	if err != nil {
		return EffectiveConfig{}, err
	}
	envTimeout, err := validateOptionalTimeout(getenv(EnvKeyRomeroTimeout), OriginEnv)
	if err != nil {
		return EffectiveConfig{}, err
	}

	cfg := EffectiveConfig{
		Project:           resolveString(flagProject, envProject, projectFile.Project, globalFile.Project, "", OriginDefault),
		Caller:            resolveCaller(input, overrides.Caller, getenv(EnvKeyCaller), projectFile.Caller, globalFile.Caller),
		RomeroBaseURL:     resolveString(flagURL, envURL, projectFile.BaseURL, globalFile.BaseURL, DefaultBaseURL, OriginDefault),
		RomeroTimeout:     resolveDuration(flagTimeout, envTimeout, projectFile.Timeout, globalFile.Timeout, DefaultTimeout),
		APIKeyPresent:     getenv(EnvKeyRomeroAPIKey) != "",
		APIKeyOrigin:      OriginEnv,
		ProjectConfigPath: projectPath,
		GlobalConfigPath:  globalPath,
		Warnings:          warnings,
	}
	return cfg, nil
}

// discoverProjectConfig searches .gandalf.json from workDir upward; the nearest file wins.
func discoverProjectConfig(workDir string) string {
	dir := workDir
	if dir == "" {
		if wd, err := os.Getwd(); err == nil {
			dir = wd
		}
	}
	for dir != "" {
		candidate := filepath.Join(dir, ProjectConfigFileName)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	return ""
}

// globalConfigPath returns the user-level configuration file path.
func globalConfigPath(userConfigDir string) string {
	if userConfigDir == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			userConfigDir = dir
		} else {
			return ""
		}
	}
	return filepath.Join(userConfigDir, GlobalConfigDirName, GlobalConfigFileName)
}

// parseConfigFile reads, inspects, and validates a configuration file.
// An empty path means no file; a missing file is not an error.
func parseConfigFile(path string) (fileValues, error) {
	if path == "" {
		return fileValues{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fileValues{}, nil
		}
		return fileValues{}, &ConfigError{Path: path, Reason: "no se puede leer el archivo", Err: ErrInvalidConfig}
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fileValues{}, &ConfigError{Path: path, Reason: "JSON inválido", Err: ErrInvalidConfig}
	}

	values := fileValues{}
	if err := inspectKeys(raw, path, &values); err != nil {
		return fileValues{}, err
	}

	var cf configFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return fileValues{}, &ConfigError{Path: path, Reason: "JSON inválido", Err: ErrInvalidConfig}
	}

	if cf.Project != nil {
		if err := ValidateProjectID(*cf.Project); err != nil {
			return fileValues{}, &ConfigError{Path: path, Reason: "identificador de proyecto inválido", Err: ErrInvalidConfig}
		}
		values.Project = cf.Project
	}
	if cf.Caller != nil && strings.TrimSpace(*cf.Caller) != "" {
		values.Caller = cf.Caller
	}
	if cf.Romero != nil {
		if cf.Romero.BaseURL != nil {
			if err := validateBaseURL(*cf.Romero.BaseURL); err != nil {
				return fileValues{}, &ConfigError{Path: path, Reason: "URL de ROMERO inválida", Err: ErrInvalidConfig}
			}
			values.BaseURL = cf.Romero.BaseURL
		}
		if cf.Romero.Timeout != nil {
			d, err := parseTimeout(*cf.Romero.Timeout)
			if err != nil {
				return fileValues{}, &ConfigError{Path: path, Reason: "tiempo límite de ROMERO inválido", Err: ErrInvalidConfig}
			}
			values.Timeout = &d
		}
	}
	return values, nil
}

// rootKnownKeys and romeroKnownKeys define the admitted schema at each object level.
var (
	rootKnownKeys   = map[string]bool{"project": true, "caller": true, "romero": true}
	romeroKnownKeys = map[string]bool{"base_url": true, "timeout": true}
)

// inspectKeys walks the raw JSON tree rejecting credential keys and collecting
// unknown-key warnings at the root and romero levels.
func inspectKeys(obj map[string]any, path string, values *fileValues) error {
	return walkKeys(obj, rootKnownKeys, path, values)
}

func walkKeys(obj map[string]any, known map[string]bool, path string, values *fileValues) error {
	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if IsForbiddenCredentialKey(key) {
			return &ConfigError{Path: path, Reason: "credencial no permitida en archivo de configuración", Err: ErrForbiddenCredentialKey}
		}
		if known != nil && !known[key] {
			values.Warnings = append(values.Warnings, fmt.Sprintf("clave desconocida %q en %s", key, path))
		}

		childKnown := knownForChild(known, key)
		switch val := obj[key].(type) {
		case map[string]any:
			if err := walkKeys(val, childKnown, path, values); err != nil {
				return err
			}
		case []any:
			for _, item := range val {
				if child, ok := item.(map[string]any); ok {
					if err := walkKeys(child, nil, path, values); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// knownForChild returns the admitted key set one level deeper; nil means "warn nothing".
func knownForChild(known map[string]bool, key string) map[string]bool {
	if known == nil || !known[key] {
		return nil
	}
	if key == "romero" {
		return romeroKnownKeys
	}
	return nil
}

// validateBaseURL checks that raw parses as an http/https URL with a host.
func validateBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ErrInvalidConfig
	}
	return nil
}

// parseTimeout parses a positive duration string.
func parseTimeout(raw string) (time.Duration, error) {
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, ErrInvalidConfig
	}
	return d, nil
}

// validateOptionalProject validates a non-empty project value from flag or environment.
func validateOptionalProject(raw string, origin Origin) (string, error) {
	if raw == "" {
		return "", nil
	}
	if err := ValidateProjectID(raw); err != nil {
		return "", &ConfigError{Reason: fmt.Sprintf("identificador de proyecto inválido (capa %s)", origin), Err: ErrInvalidConfig}
	}
	return raw, nil
}

// validateOptionalBaseURL validates a non-empty URL value from flag or environment.
func validateOptionalBaseURL(raw string, origin Origin) (string, error) {
	if raw == "" {
		return "", nil
	}
	if err := validateBaseURL(raw); err != nil {
		return "", &ConfigError{Reason: fmt.Sprintf("URL de ROMERO inválida (capa %s)", origin), Err: ErrInvalidConfig}
	}
	return raw, nil
}

// validateOptionalTimeout validates a non-empty timeout value from flag or environment.
func validateOptionalTimeout(raw string, origin Origin) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	d, err := parseTimeout(raw)
	if err != nil {
		return 0, &ConfigError{Reason: fmt.Sprintf("tiempo límite de ROMERO inválido (capa %s)", origin), Err: ErrInvalidConfig}
	}
	return d, nil
}

// resolveString picks the winning string layer by precedence.
func resolveString(flag, env string, project, global *string, def string, defOrigin Origin) TrackedValue[string] {
	switch {
	case flag != "":
		return TrackedValue[string]{Value: flag, Origin: OriginFlag}
	case env != "":
		return TrackedValue[string]{Value: env, Origin: OriginEnv}
	case project != nil:
		return TrackedValue[string]{Value: *project, Origin: OriginProject}
	case global != nil:
		return TrackedValue[string]{Value: *global, Origin: OriginGlobal}
	default:
		return TrackedValue[string]{Value: def, Origin: defOrigin}
	}
}

// resolveDuration picks the winning duration layer by precedence.
func resolveDuration(flag, env time.Duration, project, global *time.Duration, def time.Duration) TrackedValue[time.Duration] {
	switch {
	case flag != 0:
		return TrackedValue[time.Duration]{Value: flag, Origin: OriginFlag}
	case env != 0:
		return TrackedValue[time.Duration]{Value: env, Origin: OriginEnv}
	case project != nil:
		return TrackedValue[time.Duration]{Value: *project, Origin: OriginProject}
	case global != nil:
		return TrackedValue[time.Duration]{Value: *global, Origin: OriginGlobal}
	default:
		return TrackedValue[time.Duration]{Value: def, Origin: OriginDefault}
	}
}

// resolveCaller picks the winning caller layer, falling back to the OS user.
func resolveCaller(input LoadInput, flag, env string, project, global *string) TrackedValue[string] {
	switch {
	case flag != "":
		return TrackedValue[string]{Value: flag, Origin: OriginFlag}
	case env != "":
		return TrackedValue[string]{Value: env, Origin: OriginEnv}
	case project != nil:
		return TrackedValue[string]{Value: *project, Origin: OriginProject}
	case global != nil:
		return TrackedValue[string]{Value: *global, Origin: OriginGlobal}
	default:
		return TrackedValue[string]{Value: defaultCaller(input), Origin: OriginDefault}
	}
}

// defaultCaller resolves the OS user name through the injected hook or os/user.
func defaultCaller(input LoadInput) string {
	if input.DefaultCaller != nil {
		return input.DefaultCaller()
	}
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return ""
}
