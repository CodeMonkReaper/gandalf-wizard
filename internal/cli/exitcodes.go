// Package cli implements the Gandalf command router, exit code mapping, and dependency wiring.
package cli

import (
	"io"
	"net/http"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/romero"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/engine"
)

// Exit codes specified by the cli-gandalf specification (Table 5).
const (
	ExitOK              = 0
	ExitInternalError   = 1
	ExitUsageError      = 2
	ExitConfigError     = 3
	ExitAccessDenied    = 4
	ExitPartialQuery    = 5
	ExitIncompleteQuery = 6
	ExitDoctorFailed    = 7
)

// Router exit codes specified by the router-02 specification: 8 when no
// profile is eligible, 9 when every eligible candidate failed, and 10 when
// the invocation was canceled. Codes 0-7 keep their Foundation meaning.
const (
	ExitNoEligible = 8
	ExitExhausted  = 9
	ExitCanceled   = 10
)

// Workflow exit codes reserved by the workflow-runtime-03 plan: 11 waiting for
// a human gate, 12 paused and resumable, 13 partial, 14 blocked, 15 failed and
// 16 conflict. Codes 0-10 keep their Foundation and Router meaning.
const (
	ExitAwaitingApproval = 11
	ExitPaused           = 12
	ExitPartial          = 13
	ExitBlocked          = 14
	ExitFailed           = 15
	ExitConflict         = 16
)

// Dependencies injects runtime capabilities so CLI commands can be tested in-process.
type Dependencies struct {
	Version       string
	WorkDir       string
	UserConfigDir string
	Stdout        io.Writer
	Stderr        io.Writer
	Getenv        func(string) string
	LookPath      func(string) (string, error)
	DefaultCaller func() string
	HTTPClient    *http.Client
	ReadPort      romero.RomeroReadPort
	StatusPort    romero.RomeroStatusPort
	// RouterRegistry overrides the executor adapter wiring; nil wires the
	// production adapters.
	RouterRegistry *router.Registry
	// WorkflowMemory and WorkflowRouter override the provider ports of the
	// workflow engine; nil wires the config-driven production adapters.
	WorkflowMemory engine.MemoryPort
	WorkflowRouter engine.RouterPort
}
