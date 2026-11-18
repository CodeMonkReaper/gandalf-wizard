package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/cli"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router"
)

// TestMain routes the test binary into helper mode when the interrupt test
// re-executes it; otherwise it runs the normal tests.
func TestMain(m *testing.M) {
	if os.Getenv("GANDALF_INTERRUPT_HELPER") == "1" {
		os.Exit(interruptHelperMain())
	}
	os.Exit(m.Run())
}

// hangAdapter blocks every execution until the context is canceled and
// records each invocation.
type hangAdapter struct {
	invocations string
}

func (h *hangAdapter) Probe(ctx context.Context, ex config.RouterExecutor) router.Probe {
	return router.Probe{Installed: true, Auth: router.AuthReady, TextOnly: router.TextOnlyEnforced}
}

func (h *hangAdapter) Discover(ctx context.Context, ex config.RouterExecutor) ([]router.Candidate, error) {
	return nil, router.ErrDiscoveryUnavailable
}

func (h *hangAdapter) Execute(ctx context.Context, execution router.Execution) (string, error) {
	f, err := os.OpenFile(h.invocations, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		fmt.Fprintf(f, "%s\n", execution.Profile.ID)
		f.Close()
	}
	<-ctx.Done()
	return "", ctx.Err()
}

// interruptHelperMain replicates main.go's wiring — signal.NotifyContext
// feeding cli.Run — with a hanging executor, so the parent test can prove
// that a real Ctrl+C cancels the current executor and never starts the
// next one.
func interruptHelperMain() int {
	reg := router.NewRegistry()
	reg.Register("hang", &hangAdapter{invocations: os.Getenv("GANDALF_HELPER_INVOCATIONS")})
	deps := cli.Dependencies{
		Version:        "test",
		WorkDir:        os.Getenv("GANDALF_HELPER_WORK_DIR"),
		UserConfigDir:  os.Getenv("GANDALF_HELPER_USER_CONFIG_DIR"),
		Stdout:         os.Stdout,
		Stderr:         os.Stderr,
		Getenv:         func(string) string { return "" },
		DefaultCaller:  func() string { return "tester" },
		RouterRegistry: reg,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return cli.Run(ctx, []string{"run", "tarea"}, deps)
}

// TestRouterInterrupt proves against a real process that Ctrl+C cancels
// the running executor and no further profile is started (AC11, AC17).
func TestRouterInterrupt(t *testing.T) {
	workDir := t.TempDir()
	userConfigDir := t.TempDir()
	invocations := filepath.Join(t.TempDir(), "invocations.txt")

	globalDir := filepath.Join(userConfigDir, "gandalf")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	globalConfig := `{"router":{"executors":[{"id":"hang","kind":"cli","authorized":true}]}}`
	if err := os.WriteFile(filepath.Join(globalDir, "config.json"), []byte(globalConfig), 0o600); err != nil {
		t.Fatalf("WriteFile global: %v", err)
	}
	projectConfig := `{"project":"demo","router":{"profiles":[
  {"id":"hang/uno","executor_id":"hang","model":"uno"},
  {"id":"hang/dos","executor_id":"hang","model":"dos"}
]}}`
	if err := os.WriteFile(filepath.Join(workDir, ".gandalf.json"), []byte(projectConfig), 0o600); err != nil {
		t.Fatalf("WriteFile project: %v", err)
	}

	bin := filepath.Join(t.TempDir(), "gandalf-test.test")
	build := exec.Command("go", "test", "-c", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compilar binario de prueba: %v: %s", err, out)
	}

	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(),
		"GANDALF_INTERRUPT_HELPER=1",
		"GANDALF_HELPER_WORK_DIR="+workDir,
		"GANDALF_HELPER_USER_CONFIG_DIR="+userConfigDir,
		"GANDALF_HELPER_INVOCATIONS="+invocations,
	)
	configureProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("arrancar proceso: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	deadline := time.Now().Add(20 * time.Second)
	for {
		if strings.TrimSpace(readInvocations(invocations)) != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("el ejecutor nunca arrancó; invocaciones: %q", readInvocations(invocations))
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := sendInterrupt(cmd); err != nil {
		t.Fatalf("enviar interrupción: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			if ee, ok := err.(*exec.ExitError); !ok {
				t.Fatalf("esperaba exit 10, got %v", err)
			} else if ee.ExitCode() != cli.ExitCanceled {
				t.Fatalf("esperaba exit 10, got %d", ee.ExitCode())
			}
		} else {
			t.Fatal("esperaba exit 10, got 0")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("el proceso no terminó tras la interrupción")
	}

	raw := readInvocations(invocations)
	lines := strings.Count(strings.TrimSpace(raw), "\n") + 1
	if strings.TrimSpace(raw) == "" {
		lines = 0
	}
	if lines != 1 {
		t.Fatalf("esperaba exactamente 1 invocación tras cancelar, got %d: %q", lines, raw)
	}
	if !strings.Contains(raw, "hang/uno") {
		t.Fatalf("la invocación registrada no es la esperada: %q", raw)
	}
}

func readInvocations(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(raw)
}
