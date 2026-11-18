package romero

import (
	"context"
	"fmt"
	"net/http"
)

// systemStatusResponse mirrors the stable fields of GET /api/v1/system/status
// (pkg/romeroclient, rev 3f46f9a).
type systemStatusResponse struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	Database      string `json:"database"`
	ActiveProfile string `json:"active_profile"`
}

// CheckStatus implements RomeroStatusPort via GET /api/v1/system/status.
// It maps 401/403 to ErrAccessDenied and any other failure to ErrServiceInterrupted.
func (a *RomeroPublicClientAdapter) CheckStatus(ctx context.Context) (SystemStatus, error) {
	resp, err := a.do(ctx, http.MethodGet, "/api/v1/system/status", nil)
	if err != nil {
		return SystemStatus{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return SystemStatus{}, ErrAccessDenied
	}
	if resp.StatusCode != http.StatusOK {
		return SystemStatus{}, fmt.Errorf("%w: checkStatus status %d", ErrServiceInterrupted, resp.StatusCode)
	}

	var sr systemStatusResponse
	if err := decodeCapped(resp.Body, &sr); err != nil {
		return SystemStatus{}, fmt.Errorf("%w: decode status: %v", ErrServiceInterrupted, err)
	}
	return SystemStatus{
		Status:        sr.Status,
		Version:       sr.Version,
		Database:      sr.Database,
		ActiveProfile: sr.ActiveProfile,
	}, nil
}

// RomeroFakeStatusAdapter is a deterministic in-memory double implementing RomeroStatusPort.
type RomeroFakeStatusAdapter struct {
	Status SystemStatus
	Err    error
}

// CheckStatus returns the configured status or error.
func (a *RomeroFakeStatusAdapter) CheckStatus(ctx context.Context) (SystemStatus, error) {
	if a.Err != nil {
		return SystemStatus{}, a.Err
	}
	return a.Status, nil
}
