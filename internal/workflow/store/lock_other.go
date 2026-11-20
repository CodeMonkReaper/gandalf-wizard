//go:build !windows

package store

import (
	"errors"
	"os"
	"runtime"
	"syscall"
)

// acquireFileLock takes an exclusive, non-blocking advisory lock on f.
// It maps "lock held elsewhere" to ErrRunBusy.
func acquireFileLock(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	runtime.KeepAlive(f)
	if err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return ErrRunBusy
		}
		return err
	}
	return nil
}

// releaseFileLock releases the advisory lock held on f.
func releaseFileLock(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	runtime.KeepAlive(f)
	return err
}

// isReparseOrSymlink reports whether info describes a symbolic link.
func isReparseOrSymlink(info os.FileInfo) bool {
	return info != nil && info.Mode()&os.ModeSymlink != 0
}
