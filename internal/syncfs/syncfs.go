// Package syncfs confines local synchronization to a verified project filesystem.
package syncfs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

var (
	ErrUnsafePath   = errors.New("unsafe project path")
	ErrBusy         = errors.New("project writer is busy")
	ErrConflict     = errors.New("file state conflicts with the expected change")
	ErrInvalidLease = errors.New("invalid project writer lease")
	ErrUnsupported  = errors.New("project filesystem is unsupported")
)

// FileState is a regular file snapshot. The zero value denotes absence.
// FileID and SHA256 identify an observation. A desired state uses its bytes.
type FileState struct {
	Exists bool
	SHA256 string
	FileID string
	Bytes  []byte
}

// NewFileState prepares a desired state for a present file, including an empty one.
func NewFileState(data []byte) FileState {
	return stateOf(bytes.Clone(data))
}

func stateOf(data []byte) FileState {
	digest := sha256.Sum256(data)
	return FileState{Exists: true, SHA256: hex.EncodeToString(digest[:]), Bytes: data}
}

func (s FileState) matches(expected FileState) bool {
	return s.Exists == expected.Exists &&
		(!s.Exists || s.FileID == expected.FileID && s.SHA256 == expected.SHA256)
}

// ScopedProject holds the effective project root and any active writer lease.
type ScopedProject struct {
	mu     sync.RWMutex
	files  *projectFS
	id     string
	writer *Lease
}

// OpenProject opens an existing project on a supported local filesystem.
func OpenProject(root string) (*ScopedProject, error) {
	files, id, err := openProjectFS(root)
	if err != nil {
		return nil, err
	}
	return &ScopedProject{files: files, id: id}, nil
}

// ID identifies the effective root directory on its volume.
func (p *ScopedProject) ID() string {
	return p.id
}

// Close releases the writer lease and root handles.
func (p *ScopedProject) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.files == nil {
		return nil
	}
	var leaseErr error
	if p.writer != nil {
		leaseErr = p.writer.release()
		p.writer = nil
	}
	closeErr := p.files.close()
	p.files = nil
	return errors.Join(leaseErr, closeErr)
}

// Observe reads a relative path without creating any project state.
func (p *ScopedProject) Observe(path string) (FileState, error) {
	parts, err := pathParts(path)
	if err != nil {
		return FileState{}, err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.files == nil {
		return FileState{}, fs.ErrClosed
	}
	return p.files.observe(parts)
}

// Lease authorizes a single project writer until it closes or the process exits.
type Lease struct {
	project *ScopedProject
	release func() error
}

// TryExclusive acquires the project writer lease immediately or returns ErrBusy.
func (p *ScopedProject) TryExclusive() (*Lease, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.files == nil {
		return nil, fs.ErrClosed
	}
	if p.writer != nil {
		return nil, ErrBusy
	}
	release, err := takeWriter(p.id)
	if err != nil {
		return nil, err
	}
	p.writer = &Lease{project: p, release: release}
	return p.writer, nil
}

// Close releases the writer lease. Multiple calls are safe.
func (l *Lease) Close() error {
	if l == nil || l.project == nil {
		return nil
	}
	p := l.project
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.writer != l {
		return nil
	}
	p.writer = nil
	return l.release()
}

// Change rechecks the expected file state and publishes the desired bytes or
// absence. It creates missing ordinary directories when adding a file.
func (p *ScopedProject) Change(lease *Lease, path string, expected, desired FileState) (bool, error) {
	parts, err := pathParts(path)
	if err != nil {
		return false, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.files == nil {
		return false, fs.ErrClosed
	}
	if lease == nil || p.writer != lease {
		return false, ErrInvalidLease
	}
	if desired.Exists {
		digest := stateOf(desired.Bytes).SHA256
		if desired.SHA256 != "" && desired.SHA256 != digest {
			return false, fmt.Errorf("%w: desired bytes and digest differ", ErrConflict)
		}
		desired.SHA256 = digest
	}
	return p.files.change(parts, expected, desired)
}

func pathParts(path string) ([]string, error) {
	if !utf8.ValidString(path) || !filepath.IsLocal(path) {
		return nil, ErrUnsafePath
	}
	parts := strings.Split(strings.ReplaceAll(path, "\\", "/"), "/")
	for _, part := range parts {
		// Keep DOS device stems invalid even on Windows versions that allow
		// an extension after one of those names.
		stem, _, _ := strings.Cut(part, ".")
		if part == "" || strings.TrimRight(part, " .") != part ||
			stem != "" && !filepath.IsLocal(strings.TrimRight(stem, " ")) ||
			strings.ContainsAny(part, ":*?\"<>|") ||
			strings.ContainsFunc(part, func(r rune) bool { return r < 32 }) {
			return nil, ErrUnsafePath
		}
	}
	return parts, nil
}
