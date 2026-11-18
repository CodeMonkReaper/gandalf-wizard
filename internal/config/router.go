package config

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// DefaultRouterAttemptTimeout is the per-attempt deadline when no layer declares one.
const DefaultRouterAttemptTimeout = 60 * time.Second

// Router executor kinds admitted by the policy.
const (
	RouterKindCLI = "cli"
	RouterKindAPI = "api"
)

// Router effort keys admitted in profile declarations.
const (
	RouterEffortLow    = "low"
	RouterEffortMedium = "medium"
	RouterEffortHigh   = "high"
)

// Sentinel reasons for router URL validation; never carry values from files.
var (
	errRouterURLMissing         = errors.New("ejecutor api sin URL")
	errRouterURLInvalid         = errors.New("URL de ejecutor invalida")
	errRouterURLNotHTTPS        = errors.New("URL de ejecutor sin HTTPS")
	errRouterURLWithCredentials = errors.New("URL de ejecutor con credenciales")
	errRouterURLWithoutHost     = errors.New("URL de ejecutor sin host")
)

// validRouterIDPattern admits executor and profile identifiers such as
// "antigravity", "api:mistral" or "antigravity/modelo-a".
var validRouterIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]{0,127}$`)

// validCredentialEnvPattern admits environment variable names such as "MISTRAL_API_KEY".
var validCredentialEnvPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

// RouterExecutor declares an execution target and its authorization state.
// It stores credential variable names, never credential values.
type RouterExecutor struct {
	ID            string
	Kind          string
	Authorized    bool
	Protocol      string
	BaseURL       string
	CredentialEnv string
}

// RouterProfile binds a declared model and its native effort values to an executor.
type RouterProfile struct {
	ID         string
	ExecutorID string
	Model      string
	Efforts    map[string]string
}

// RouterPolicy is the effective router configuration resolved by Load.
// An empty executor list means no execution is authorized.
type RouterPolicy struct {
	Executors      []RouterExecutor
	Profiles       []RouterProfile
	AttemptTimeout time.Duration
}

// routerFileValues holds the validated router block of a single configuration file.
type routerFileValues struct {
	AttemptTimeout   *time.Duration
	Executors        []RouterExecutor
	ExecutorsPresent bool
	Profiles         []RouterProfile
	ProfilesPresent  bool
}

// configFileRouter mirrors the admitted JSON schema of the router block.
type configFileRouter struct {
	AttemptTimeout *string               `json:"attempt_timeout"`
	Executors      *[]configFileExecutor `json:"executors"`
	Profiles       *[]configFileProfile  `json:"profiles"`
}

// configFileExecutor mirrors the admitted JSON schema of a router executor.
type configFileExecutor struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Authorized    *bool  `json:"authorized"`
	Protocol      string `json:"protocol"`
	BaseURL       string `json:"base_url"`
	CredentialEnv string `json:"credential_env"`
}

// configFileProfile mirrors the admitted JSON schema of a router profile.
type configFileProfile struct {
	ID         string            `json:"id"`
	ExecutorID string            `json:"executor_id"`
	Model      string            `json:"model"`
	Efforts    map[string]string `json:"efforts"`
}

// parseRouterBlock validates and converts the router block of one file.
// It never moves credential values: only variable names survive.
func parseRouterBlock(r *configFileRouter, path string) (*routerFileValues, error) {
	out := &routerFileValues{}
	if r.AttemptTimeout != nil {
		d, err := parseTimeout(*r.AttemptTimeout)
		if err != nil {
			return nil, &ConfigError{Path: path, Reason: "plazo por intento del router invalido", Err: ErrInvalidConfig}
		}
		out.AttemptTimeout = &d
	}
	if r.Executors != nil {
		out.ExecutorsPresent = true
		seen := map[string]bool{}
		for _, ex := range *r.Executors {
			e, err := parseRouterExecutor(ex, path)
			if err != nil {
				return nil, err
			}
			if seen[e.ID] {
				return nil, &ConfigError{Path: path, Reason: "identificador de ejecutor duplicado en el router", Err: ErrInvalidConfig}
			}
			seen[e.ID] = true
			out.Executors = append(out.Executors, e)
		}
	}
	if r.Profiles != nil {
		out.ProfilesPresent = true
		seen := map[string]bool{}
		for _, pf := range *r.Profiles {
			p, err := parseRouterProfile(pf, path)
			if err != nil {
				return nil, err
			}
			if seen[p.ID] {
				return nil, &ConfigError{Path: path, Reason: "identificador de perfil duplicado en el router", Err: ErrInvalidConfig}
			}
			seen[p.ID] = true
			out.Profiles = append(out.Profiles, p)
		}
	}
	return out, nil
}

// parseRouterExecutor validates one executor declaration.
func parseRouterExecutor(ex configFileExecutor, path string) (RouterExecutor, error) {
	id := strings.TrimSpace(ex.ID)
	if !validRouterIDPattern.MatchString(id) {
		return RouterExecutor{}, &ConfigError{Path: path, Reason: "identificador de ejecutor del router invalido", Err: ErrInvalidConfig}
	}
	kind := strings.TrimSpace(ex.Kind)
	if kind != RouterKindCLI && kind != RouterKindAPI {
		return RouterExecutor{}, &ConfigError{Path: path, Reason: "tipo de ejecutor del router invalido", Err: ErrInvalidConfig}
	}
	e := RouterExecutor{ID: id, Kind: kind}
	if ex.Authorized != nil {
		e.Authorized = *ex.Authorized
	}
	switch kind {
	case RouterKindAPI:
		e.Protocol = strings.TrimSpace(ex.Protocol)
		if e.Protocol == "" {
			return RouterExecutor{}, &ConfigError{Path: path, Reason: "ejecutor api sin protocolo", Err: ErrInvalidConfig}
		}
		if err := validateRouterBaseURL(strings.TrimSpace(ex.BaseURL)); err != nil {
			return RouterExecutor{}, &ConfigError{Path: path, Reason: err.Error(), Err: ErrInvalidConfig}
		}
		e.BaseURL = strings.TrimSpace(ex.BaseURL)
		e.CredentialEnv = strings.TrimSpace(ex.CredentialEnv)
		if e.CredentialEnv != "" && !validCredentialEnvPattern.MatchString(e.CredentialEnv) {
			return RouterExecutor{}, &ConfigError{Path: path, Reason: "nombre de variable de credencial invalido", Err: ErrInvalidConfig}
		}
	case RouterKindCLI:
		if strings.TrimSpace(ex.Protocol) != "" || strings.TrimSpace(ex.BaseURL) != "" || strings.TrimSpace(ex.CredentialEnv) != "" {
			return RouterExecutor{}, &ConfigError{Path: path, Reason: "ejecutor cli con campos de api", Err: ErrInvalidConfig}
		}
	}
	return e, nil
}

// validateRouterBaseURL requires https, a host, and no credentials embedded in the URL.
func validateRouterBaseURL(raw string) error {
	if raw == "" {
		return errRouterURLMissing
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errRouterURLInvalid
	}
	if u.Scheme != "https" {
		return errRouterURLNotHTTPS
	}
	if u.User != nil {
		return errRouterURLWithCredentials
	}
	if u.Host == "" {
		return errRouterURLWithoutHost
	}
	return nil
}

// parseRouterProfile validates one profile declaration.
func parseRouterProfile(pf configFileProfile, path string) (RouterProfile, error) {
	id := strings.TrimSpace(pf.ID)
	if !validRouterIDPattern.MatchString(id) {
		return RouterProfile{}, &ConfigError{Path: path, Reason: "identificador de perfil del router invalido", Err: ErrInvalidConfig}
	}
	execID := strings.TrimSpace(pf.ExecutorID)
	if !validRouterIDPattern.MatchString(execID) {
		return RouterProfile{}, &ConfigError{Path: path, Reason: "referencia de ejecutor del router invalida", Err: ErrInvalidConfig}
	}
	model := strings.TrimSpace(pf.Model)
	if model == "" {
		return RouterProfile{}, &ConfigError{Path: path, Reason: "perfil del router sin modelo", Err: ErrInvalidConfig}
	}
	p := RouterProfile{ID: id, ExecutorID: execID, Model: model}
	if pf.Efforts != nil {
		p.Efforts = map[string]string{}
		for effort, native := range pf.Efforts {
			if !validRouterEffortKey(effort) {
				return RouterProfile{}, &ConfigError{Path: path, Reason: "esfuerzo de perfil del router invalido", Err: ErrInvalidConfig}
			}
			v := strings.TrimSpace(native)
			if v == "" {
				return RouterProfile{}, &ConfigError{Path: path, Reason: "valor de esfuerzo vacio en el router", Err: ErrInvalidConfig}
			}
			p.Efforts[effort] = v
		}
	}
	return p, nil
}

// validRouterEffortKey reports whether key is an admitted effort name.
func validRouterEffortKey(key string) bool {
	switch key {
	case RouterEffortLow, RouterEffortMedium, RouterEffortHigh:
		return true
	}
	return false
}

// mergeRouterPolicy resolves the effective router policy across layers.
// Executors and their authorization live only in the global layer; the
// project layer may replace the ordered profile list and adjust the timeout.
func mergeRouterPolicy(projectPath string, project *routerFileValues, globalPath string, global *routerFileValues) (RouterPolicy, error) {
	if project != nil && project.ExecutorsPresent {
		return RouterPolicy{}, &ConfigError{Path: projectPath, Reason: "los ejecutores del router se declaran solo en la capa global", Err: ErrInvalidConfig}
	}

	policy := RouterPolicy{Executors: []RouterExecutor{}, Profiles: []RouterProfile{}}
	if global != nil {
		policy.Executors = append(policy.Executors, global.Executors...)
	}

	profiles := global
	profilesPath := globalPath
	if project != nil && project.ProfilesPresent {
		profiles = project
		profilesPath = projectPath
	}
	if profiles != nil {
		policy.Profiles = append(policy.Profiles, profiles.Profiles...)
	}

	known := map[string]bool{}
	for _, e := range policy.Executors {
		known[e.ID] = true
	}
	for _, p := range policy.Profiles {
		if !known[p.ExecutorID] {
			return RouterPolicy{}, &ConfigError{Path: profilesPath, Reason: "perfil del router con ejecutor desconocido", Err: ErrInvalidConfig}
		}
	}

	switch {
	case project != nil && project.AttemptTimeout != nil:
		policy.AttemptTimeout = *project.AttemptTimeout
	case global != nil && global.AttemptTimeout != nil:
		policy.AttemptTimeout = *global.AttemptTimeout
	default:
		policy.AttemptTimeout = DefaultRouterAttemptTimeout
	}
	return policy, nil
}
