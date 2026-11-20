//go:build windows

package store

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	lockfileExclusiveLock   = 0x00000002
	lockfileFailImmediately = 0x00000001
)

const (
	errSharingViolation = syscall.Errno(32)
	errLockViolation    = syscall.Errno(33)
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

// acquireFileLock takes an exclusive, non-blocking byte-range lock on the
// first byte of f. It maps lock contention to ErrRunBusy.
func acquireFileLock(f *os.File) error {
	var overlapped syscall.Overlapped
	r1, _, callErr := procLockFileEx.Call(
		f.Fd(),
		uintptr(lockfileExclusiveLock|lockfileFailImmediately),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	runtime.KeepAlive(f)
	if r1 == 0 {
		if errno, ok := callErr.(syscall.Errno); ok {
			if errno == errLockViolation || errno == errSharingViolation {
				return ErrRunBusy
			}
		}
		return callErr
	}
	return nil
}

// releaseFileLock releases the byte-range lock held on f.
func releaseFileLock(f *os.File) error {
	var overlapped syscall.Overlapped
	r1, _, callErr := procUnlockFileEx.Call(
		f.Fd(),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	runtime.KeepAlive(f)
	if r1 == 0 {
		return callErr
	}
	return nil
}

// isReparseOrSymlink reports whether info describes a symbolic link or any
// reparse point (junctions included).
func isReparseOrSymlink(info os.FileInfo) bool {
	if info == nil {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	if attrs, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return attrs.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
	}
	return false
}
