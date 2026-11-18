package romero

import (
	"context"
	"errors"
)

// Sentinel errors classifying read-only ROMERO port failures.
var (
	ErrAccessDenied       = errors.New("romero: access denied")
	ErrServiceInterrupted = errors.New("romero: service interrupted")
	ErrDetailInterrupted  = errors.New("romero: detail interrupted")
)

// RomeroReadPort defines the read-only boundary toward ROMERO's public memory contract.
type RomeroReadPort interface {
	Search(ctx context.Context, scope ResolvedScope, query string) (SearchResult, error)
	GetDetail(ctx context.Context, scope ResolvedScope, memoryID string) (MemoryDetail, error)
}
