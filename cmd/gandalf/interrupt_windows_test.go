//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup isolates the child in its own process group so the
// console interrupt reaches only the child, never this test process.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// modkernel32 reaches the console control API that the public syscall
// package does not export.
var modkernel32 = syscall.NewLazyDLL("kernel32.dll")

var procGenerateConsoleCtrlEvent = modkernel32.NewProc("GenerateConsoleCtrlEvent")

// sendInterrupt delivers a real console interrupt to the child process.
// The Go runtime maps both CTRL_C_EVENT and CTRL_BREAK_EVENT to os.Interrupt.
func sendInterrupt(cmd *exec.Cmd) error {
	r1, _, callErr := procGenerateConsoleCtrlEvent.Call(uintptr(syscall.CTRL_BREAK_EVENT), uintptr(cmd.Process.Pid))
	if r1 == 0 {
		return callErr
	}
	return nil
}
