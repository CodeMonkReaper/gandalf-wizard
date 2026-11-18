package romero

import "context"

// SystemStatus models the public health/status metadata returned by ROMERO.
type SystemStatus struct {
	Status        string
	Version       string
	Database      string
	ActiveProfile string
}

// RomeroStatusPort defines the read-only status probe toward ROMERO.
type RomeroStatusPort interface {
	CheckStatus(ctx context.Context) (SystemStatus, error)
}
