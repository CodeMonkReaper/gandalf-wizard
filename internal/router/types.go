// Package router composes the execution catalog, selects profiles, and orchestrates attempts.
package router

import (
	"context"
	"errors"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
)

// DiscoveryMode controls whether catalog building may use documented discovery.
type DiscoveryMode string

const (
	// DiscoveryLocalOnly probes local state only; no remote discovery.
	DiscoveryLocalOnly DiscoveryMode = "local-only"
	// DiscoveryLocalAndDocumentedFree adds documented, quota-free discovery
	// for authorized executors.
	DiscoveryLocalAndDocumentedFree DiscoveryMode = "local-and-documented-free"
)

// AuthState reports the local authentication state of an executor.
type AuthState string

const (
	AuthReady   AuthState = "ready"
	AuthMissing AuthState = "missing"
	AuthUnknown AuthState = "unknown"
)

// TextOnlyState reports whether the text-only execution mode is proven.
type TextOnlyState string

const (
	TextOnlyEnforced TextOnlyState = "enforced"
	TextOnlyUnproven TextOnlyState = "unproven"
)

// Probe is the local, quota-free state of one executor. Presence of the
// binary and authentication are separate states; Reason carries the
// sanitized detail and never a credential.
type Probe struct {
	Installed bool
	Auth      AuthState
	TextOnly  TextOnlyState
	Reason    string
}

// Candidate is a model documented by an executor's discovery. Efforts maps
// effort keys to native values only where the executor documents them.
type Candidate struct {
	Model   string
	Efforts map[string]string
}

// Execution is one attempt specification handed to an adapter.
type Execution struct {
	Executor config.RouterExecutor
	Profile  config.RouterProfile
	Effort   string // empty means no effort is sent
	Task     string
}

// FailureKind classifies a SafeFailure for orchestration.
type FailureKind string

const (
	FailureExecutorError FailureKind = "executor-error"
	FailureTimeout       FailureKind = "timeout"
	FailureUnavailable   FailureKind = "unavailable"
	FailureInvalidOutput FailureKind = "invalid-output"
)

// SafeFailure is the stable, sanitized failure every adapter returns from
// Execute. It never carries provider stderr, raw bodies, or credentials.
type SafeFailure struct {
	Kind   FailureKind
	Reason string
}

// Error implements error with the sanitized reason.
func (f *SafeFailure) Error() string {
	if f == nil {
		return ""
	}
	return f.Reason
}

// ErrDiscoveryUnavailable reports that an executor offers no documented
// discovery; it is distinct from a discovery that ran and failed.
var ErrDiscoveryUnavailable = errors.New("router: descubrimiento no disponible")

// Adapter is the port every executor adapter implements.
type Adapter interface {
	// Probe reports local state without quota, inference, writes, native
	// token reads, or session refresh.
	Probe(ctx context.Context, ex config.RouterExecutor) Probe
	// Discover returns the executor's documented models. It returns
	// ErrDiscoveryUnavailable when the executor offers no documented
	// discovery; any other error means discovery ran and failed.
	Discover(ctx context.Context, ex config.RouterExecutor) ([]Candidate, error)
	// Execute runs one text-only attempt and returns the final text or a
	// SafeFailure.
	Execute(ctx context.Context, exec Execution) (string, error)
}

// Registry resolves the adapter for an executor by its ID.
type Registry struct {
	adapters map[string]Adapter
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{adapters: map[string]Adapter{}}
}

// Register binds an adapter to an executor ID.
func (r *Registry) Register(executorID string, a Adapter) {
	r.adapters[executorID] = a
}

// AdapterFor resolves the adapter of an executor ID.
func (r *Registry) AdapterFor(executorID string) (Adapter, bool) {
	if r == nil || r.adapters == nil {
		return nil, false
	}
	a, ok := r.adapters[executorID]
	return a, ok
}
