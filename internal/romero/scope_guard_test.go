package romero

import (
	"errors"
	"slices"
	"testing"
)

func TestProjectScopeGuard(t *testing.T) {
	t.Run("valid single active project", func(t *testing.T) {
		got, err := ResolveScope(WorkspaceContext{
			ActiveProjectID: "Faro",
			CallerIdentity:  "dev-user",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := ResolvedScope{
			ProjectID:      "Faro",
			CallerIdentity: "dev-user",
		}
		if got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("missing project", func(t *testing.T) {
		_, err := ResolveScope(WorkspaceContext{
			ActiveProjectID:     "",
			CandidateProjectIDs: nil,
			CallerIdentity:      "dev-user",
		})
		if err == nil {
			t.Fatal("expected ScopeError, got nil")
		}
		var scopeErr *ScopeError
		if !errors.As(err, &scopeErr) {
			t.Fatalf("expected *ScopeError, got %T (%v)", err, err)
		}
		if scopeErr.Kind != ScopeMissing {
			t.Fatalf("Kind = %q, want %q", scopeErr.Kind, ScopeMissing)
		}
	})

	t.Run("ambiguous candidate projects", func(t *testing.T) {
		_, err := ResolveScope(WorkspaceContext{
			ActiveProjectID:     "",
			CandidateProjectIDs: []string{"Faro", "OtroProyecto"},
			CallerIdentity:      "dev-user",
		})
		if err == nil {
			t.Fatal("expected ScopeError, got nil")
		}
		var scopeErr *ScopeError
		if !errors.As(err, &scopeErr) {
			t.Fatalf("expected *ScopeError, got %T (%v)", err, err)
		}
		if scopeErr.Kind != ScopeAmbiguous {
			t.Fatalf("Kind = %q, want %q", scopeErr.Kind, ScopeAmbiguous)
		}
		wantCandidates := []string{"Faro", "OtroProyecto"}
		if !slices.Equal(scopeErr.Candidates, wantCandidates) {
			t.Fatalf("Candidates = %v, want %v", scopeErr.Candidates, wantCandidates)
		}
	})

	t.Run("triangulation: whitespace, caller, candidates, duplicates, and defensive copy", func(t *testing.T) {
		tests := []struct {
			name           string
			ctx            WorkspaceContext
			wantScope      ResolvedScope
			wantErrKind    ScopeErrorKind
			wantCandidates []string
		}{
			{
				name: "whitespace active and candidates treated as missing",
				ctx: WorkspaceContext{
					ActiveProjectID:     "   ",
					CandidateProjectIDs: []string{"  ", "\t"},
					CallerIdentity:      "dev-user",
				},
				wantErrKind: ScopeMissing,
			},
			{
				name: "missing caller identity even with active project",
				ctx: WorkspaceContext{
					ActiveProjectID: "Faro",
					CallerIdentity:  "   ",
				},
				wantErrKind: ScopeMissing,
			},
			{
				name: "explicit active selection disambiguates multiple candidates",
				ctx: WorkspaceContext{
					ActiveProjectID:     "  Faro ",
					CandidateProjectIDs: []string{"Faro", "OtroProyecto"},
					CallerIdentity:      " dev-user ",
				},
				wantScope: ResolvedScope{ProjectID: "Faro", CallerIdentity: "dev-user"},
			},
			{
				name: "active project conflicting with candidate list returns ambiguous",
				ctx: WorkspaceContext{
					ActiveProjectID:     "Faro",
					CandidateProjectIDs: []string{"OtroProyecto"},
					CallerIdentity:      "dev-user",
				},
				wantErrKind:    ScopeAmbiguous,
				wantCandidates: []string{"OtroProyecto"},
			},
			{
				name: "duplicate single candidate deduplicates to valid scope",
				ctx: WorkspaceContext{
					ActiveProjectID:     "",
					CandidateProjectIDs: []string{"Faro", " Faro ", ""},
					CallerIdentity:      "dev-user",
				},
				wantScope: ResolvedScope{ProjectID: "Faro", CallerIdentity: "dev-user"},
			},
		}

		var guard ProjectScopeGuard
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := guard.ResolveScope(tt.ctx)
				if tt.wantErrKind == "" {
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					if got != tt.wantScope {
						t.Fatalf("got %+v, want %+v", got, tt.wantScope)
					}
					return
				}

				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErrKind)
				}
				var scopeErr *ScopeError
				if !errors.As(err, &scopeErr) {
					t.Fatalf("expected *ScopeError, got %T (%v)", err, err)
				}
				if scopeErr.Kind != tt.wantErrKind {
					t.Fatalf("Kind = %q, want %q", scopeErr.Kind, tt.wantErrKind)
				}
				if tt.wantCandidates != nil && !slices.Equal(scopeErr.Candidates, tt.wantCandidates) {
					t.Fatalf("Candidates = %v, want %v", scopeErr.Candidates, tt.wantCandidates)
				}
			})
		}

		t.Run("defensive copy isolates ScopeError candidates from caller slice mutation", func(t *testing.T) {
			inputCandidates := []string{"Faro", "OtroProyecto"}
			_, err := ResolveScope(WorkspaceContext{
				CandidateProjectIDs: inputCandidates,
				CallerIdentity:      "dev-user",
			})
			var scopeErr *ScopeError
			if !errors.As(err, &scopeErr) {
				t.Fatalf("expected *ScopeError, got %T (%v)", err, err)
			}
			inputCandidates[0] = "Mutated"
			if scopeErr.Candidates[0] != "Faro" {
				t.Fatalf("Candidates mutated via caller slice: %v", scopeErr.Candidates)
			}
		})
	})
}
