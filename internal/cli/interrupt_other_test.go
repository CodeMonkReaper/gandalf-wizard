//go:build !windows

package cli

import (
	"os"
	"os/exec"
)

// configureProcessGroup is a no-op outside Windows.
func configureProcessGroup(cmd *exec.Cmd) {}

// sendInterrupt delivers a real interrupt to the child process.
func sendInterrupt(cmd *exec.Cmd) error {
	return cmd.Process.Signal(os.Interrupt)
}
