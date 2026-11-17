package romero

import (
	"fmt"
	"slices"
	"strings"
)

// ScopeErrorKind classifies why a project query scope could not be resolved unambiguously.
type ScopeErrorKind string

const (
	ScopeMissing   ScopeErrorKind = "scope_missing"
	ScopeAmbiguous ScopeErrorKind = "scope_ambiguous"
)

// ScopeError describes a deterministic local scope resolution failure.
type ScopeError struct {
	Kind       ScopeErrorKind
	Message    string
	Candidates []string
}

func (e *ScopeError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("scope error: %s", e.Kind)
}

// ProjectScopeGuard validates that a query targets a single active project before any external call.
type ProjectScopeGuard struct{}

// ResolveScope validates that the workspace context identifies exactly one project and caller.
func (ProjectScopeGuard) ResolveScope(ctx WorkspaceContext) (ResolvedScope, error) {
	return ResolveScope(ctx)
}

// ResolveScope checks locally that exactly one non-empty projectId and callerIdentity are present.
func ResolveScope(ctx WorkspaceContext) (ResolvedScope, error) {
	caller := strings.TrimSpace(ctx.CallerIdentity)
	if caller == "" {
		return ResolvedScope{}, &ScopeError{
			Kind:    ScopeMissing,
			Message: "identidad de consulta requerida antes de consultar ROMERO",
		}
	}

	active := strings.TrimSpace(ctx.ActiveProjectID)
	candidates := normalizeCandidates(ctx.CandidateProjectIDs)

	if active != "" {
		if len(candidates) > 0 && !slices.Contains(candidates, active) {
			return ResolvedScope{}, &ScopeError{
				Kind:       ScopeAmbiguous,
				Message:    "el proyecto activo no coincide con los proyectos candidatos; identifica el proyecto",
				Candidates: slices.Clone(candidates),
			}
		}
		return ResolvedScope{
			ProjectID:      active,
			CallerIdentity: caller,
		}, nil
	}

	switch len(candidates) {
	case 0:
		return ResolvedScope{}, &ScopeError{
			Kind:    ScopeMissing,
			Message: "identifica el proyecto activo antes de consultar ROMERO",
		}
	case 1:
		return ResolvedScope{
			ProjectID:      candidates[0],
			CallerIdentity: caller,
		}, nil
	default:
		return ResolvedScope{}, &ScopeError{
			Kind:       ScopeAmbiguous,
			Message:    "hay varios proyectos posibles; identifica cuál consultar",
			Candidates: slices.Clone(candidates),
		}
	}
}

func normalizeCandidates(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, candidate := range raw {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
