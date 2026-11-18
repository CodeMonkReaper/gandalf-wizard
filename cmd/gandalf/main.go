// Command gandalf is the Gandalf CLI entry point; all logic lives in internal/cli.
package main

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"os/user"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/cli"
)

// version is the version reported by `gandalf version`.
const version = "0.1.0"

func main() {
	deps := cli.Dependencies{
		Version:       version,
		WorkDir:       workDir(),
		UserConfigDir: userConfigDir(),
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
		LookPath:      exec.LookPath,
		DefaultCaller: defaultCaller,
	}
	// Ctrl+C (and Ctrl+Break on Windows) cancels the context so a running
	// attempt stops and the CLI exits with the canceled code.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(cli.Run(ctx, os.Args[1:], deps))
}

func workDir() string {
	if dir, err := os.Getwd(); err == nil {
		return dir
	}
	return ""
}

func userConfigDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return dir
	}
	return ""
}

func defaultCaller() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return ""
}
