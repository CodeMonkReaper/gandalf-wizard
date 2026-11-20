package syncfs

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

type projectFS struct {
	root    *os.Root
	base    *os.File
	rootPin syscall.Handle
}

func openProjectFS(name string) (*projectFS, string, error) {
	if name == "" {
		return nil, "", ErrUnsafePath
	}
	path, err := filepath.Abs(name)
	if err != nil {
		return nil, "", err
	}
	if len(filepath.VolumeName(path)) != 2 {
		return nil, "", ErrUnsupported
	}

	// Pin and inspect the selected directory itself before OpenRoot follows any
	// link. Compare the two handles so both opens authorize the same directory.
	path16, err := syscall.UTF16PtrFromString("\\\\?\\" + path)
	if err != nil {
		return nil, "", ErrUnsafePath
	}
	pin, err := syscall.CreateFile(path16, syscall.GENERIC_READ, shareReadWrite, nil,
		syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open project directory: %w", err)
	}
	retained := false
	defer func() {
		if !retained {
			_ = syscall.CloseHandle(pin)
		}
	}()
	selected, err := inspectHandle(pin, true)
	if err != nil {
		return nil, "", err
	}
	volume, err := localNTFSVolume(pin)
	if err != nil {
		return nil, "", err
	}

	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, "", err
	}
	base, err := root.Open(".")
	if err != nil {
		return nil, "", errors.Join(err, root.Close())
	}
	files := &projectFS{root: root, base: base, rootPin: pin}
	actual, err := inspectHandle(syscall.Handle(base.Fd()), true)
	if err != nil {
		return nil, "", errors.Join(err, base.Close(), root.Close())
	}
	if fileID(actual) != fileID(selected) {
		return nil, "", errors.Join(ErrUnsafePath, base.Close(), root.Close())
	}
	retained = true
	return files, volume + "-" + fileID(actual), nil
}

func (f *projectFS) close() error {
	return errors.Join(f.base.Close(), f.root.Close(), syscall.CloseHandle(f.rootPin))
}

// pinParent retains handles to the directory chain until the operation ends.
// Data-read access makes the no-delete sharing rule apply on Windows.
func (f *projectFS) pinParent(parts []string, create bool) (syscall.Handle, func(), error) {
	parent := syscall.Handle(f.base.Fd())
	var pins []syscall.Handle
	release := func() {
		for i := len(pins) - 1; i >= 0; i-- {
			_ = syscall.CloseHandle(pins[i])
		}
	}
	for _, part := range parts[:len(parts)-1] {
		next, err := openAt(parent, part, syscall.GENERIC_READ, shareReadWrite, fileOpen, 0)
		if create && errors.Is(err, os.ErrNotExist) {
			next, err = openAt(parent, part, syscall.GENERIC_READ, shareReadWrite, fileOpenIf, directoryFile)
		}
		if err != nil {
			release()
			return 0, nil, err
		}
		pins = append(pins, next)
		if _, err := inspectHandle(next, true); err != nil {
			release()
			return 0, nil, err
		}
		parent = next
	}
	return parent, release, nil
}

func openFile(parent syscall.Handle, name string, access, share uint32) (*os.File, string, error) {
	handle, err := openAt(parent, name, access, share, fileOpen, 0)
	if err != nil {
		return nil, "", err
	}
	info, err := inspectHandle(handle, false)
	if err != nil {
		_ = syscall.CloseHandle(handle)
		return nil, "", err
	}
	if err := requireDefaultStream(handle); err != nil {
		_ = syscall.CloseHandle(handle)
		return nil, "", err
	}
	return os.NewFile(uintptr(handle), name), fileID(info), nil
}

func readState(file *os.File, id string) (FileState, error) {
	if file == nil {
		return FileState{}, nil
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return FileState{}, err
	}
	state := stateOf(data)
	state.FileID = id
	return state, nil
}

func (f *projectFS) observe(parts []string) (FileState, error) {
	parent, release, err := f.pinParent(parts, false)
	if errors.Is(err, os.ErrNotExist) {
		return FileState{}, nil
	}
	if err != nil {
		return FileState{}, err
	}
	defer release()
	file, id, err := openFile(parent, parts[len(parts)-1], syscall.GENERIC_READ, shareAll)
	if errors.Is(err, os.ErrNotExist) {
		return FileState{}, nil
	}
	if err != nil {
		return FileState{}, err
	}
	defer file.Close()
	return readState(file, id)
}

func (f *projectFS) change(parts []string, expected, desired FileState, operation string) (changed bool, err error) {
	parent, release, err := f.pinParent(parts, desired.Exists && !expected.Exists)
	if errors.Is(err, os.ErrNotExist) && !expected.Exists && !desired.Exists {
		return false, nil
	}
	if err != nil {
		return false, changeError(err)
	}
	defer release()

	name := parts[len(parts)-1]
	access := uint32(syscall.GENERIC_READ)
	if !desired.Exists && expected.Exists {
		access |= deleteAccess
	}
	file, id, err := openFile(parent, name, access, shareRead)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, changeError(err)
	}
	if file != nil {
		defer func() {
			if file != nil {
				_ = file.Close()
			}
		}()
	}
	current, err := readState(file, id)
	if err != nil {
		return false, err
	}
	if !current.matches(expected) {
		return false, ErrConflict
	}
	if !desired.Exists {
		if file == nil {
			return false, nil
		}
		err := changeError(unlinkHandle(syscall.Handle(file.Fd())))
		return err == nil, err
	}
	if current.Exists && current.SHA256 == desired.SHA256 {
		return false, nil
	}

	stage, err := prepareReplacement(parent, file, operation, desired.Bytes)
	if err != nil {
		return false, err
	}
	deleteArmed := true
	defer func() {
		if changed || deleteArmed {
			err = errors.Join(err, stage.Close())
			return
		}
		err = errors.Join(err, discardStage(stage))
	}()

	stageHandle := syscall.Handle(stage.Fd())
	if current.Exists {
		// NTFS needs delete sharing for replacement. Keep the original handle
		// until a second, verified handle has taken over denying data writes.
		final, finalID, err := openFile(parent, name, syscall.GENERIC_READ, shareReadDelete)
		if err != nil {
			return false, changeError(err)
		}
		defer final.Close()
		if finalID != current.FileID {
			return false, ErrConflict
		}
		if err := checkAccess(final, stageHandle); err != nil {
			return false, err
		}
	}

	// NTFS deletes the stage if the owner exits before publication is armed.
	// From here to rename, process termination can leave only the temp name.
	if err := keepStagedFile(stageHandle); err != nil {
		return false, fmt.Errorf("prepare staged publication: %w", err)
	}
	deleteArmed = false
	if testSeamAfterArm != nil {
		testSeamAfterArm()
	}
	if file != nil {
		if err := file.Close(); err != nil {
			return false, err
		}
		file = nil
	}
	if current.Exists {
		// Existing names can be changed by a non-cooperating editor in the
		// interval after closing the original handle and before the rename.
		// The identity precondition rechecks the name, then removes the
		// verified inode's link by handle and publishes without replacement,
		// so a foreign entry is never overwritten.
		if testSeamBeforePublication != nil {
			testSeamBeforePublication()
		}
		guard, guardID, err := openFile(parent, name, syscall.GENERIC_READ|deleteAccess, shareReadDelete)
		if err != nil {
			return false, changeError(err)
		}
		if guardID != current.FileID {
			_ = guard.Close()
			return false, ErrConflict
		}
		if err := unlinkHandle(syscall.Handle(guard.Fd())); err != nil {
			_ = guard.Close()
			return false, changeError(err)
		}
		_ = guard.Close()
	}
	if testSeamBeforeRename != nil {
		testSeamBeforeRename()
	}
	if err := renameHandle(stageHandle, parent, name, false); err != nil {
		return false, changeError(err)
	}
	return true, nil
}

// testSeamBeforePublication lets tests replace or remove the target in the
// interval between closing the original handle and the identity recheck.
// testSeamBeforeRename lets tests act between the recheck and the rename.
// testSeamAfterArm lets tests pause a change after the staged file is armed
// against process termination. All stay nil outside tests.
var (
	testSeamBeforePublication func()
	testSeamBeforeRename      func()
	testSeamAfterArm          func()
)

func prepareReplacement(parent syscall.Handle, current *os.File, operation string, data []byte) (*os.File, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	name := ".gandalf-sync-" + operation + "-" + hex.EncodeToString(random) + ".tmp"
	handle, err := openAt(parent, name, syscall.GENERIC_READ|syscall.GENERIC_WRITE|deleteAccess,
		shareRead, fileCreate, nonDirectoryFile|deleteOnClose)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), name)
	if current != nil {
		if err := checkAccess(current, handle); err != nil {
			return nil, errors.Join(err, file.Close())
		}
	}
	if _, err := file.Write(data); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if err := file.Sync(); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}

func checkAccess(current *os.File, replacement syscall.Handle) error {
	prior, err := accessDescriptor(syscall.Handle(current.Fd()))
	if err != nil {
		return fmt.Errorf("read destination access controls: %w", err)
	}
	staged, err := accessDescriptor(replacement)
	if err != nil {
		return fmt.Errorf("read replacement access controls: %w", err)
	}
	if !bytes.Equal(prior, staged) {
		return fmt.Errorf("%w: destination access controls differ from replacement", ErrConflict)
	}
	return nil
}

func discardStage(file *os.File) error {
	return errors.Join(unlinkHandle(syscall.Handle(file.Fd())), file.Close())
}

// openPinnedDir pins the parent chain of parts and opens the directory that
// holds the target name, verifying both handles name the same directory.
func (f *projectFS) openPinnedDir(parts []string) (syscall.Handle, *os.File, func(), error) {
	parent, release, err := f.pinParent(parts, false)
	if err != nil {
		return 0, nil, nil, err
	}
	parentInfo, err := inspectHandle(parent, true)
	if err != nil {
		release()
		return 0, nil, nil, err
	}
	dirRel := "."
	if len(parts) > 1 {
		dirRel = path.Join(parts[:len(parts)-1]...)
	}
	dirFile, err := f.root.Open(dirRel)
	if err != nil {
		release()
		return 0, nil, nil, err
	}
	openedInfo, err := inspectHandle(syscall.Handle(dirFile.Fd()), true)
	if err != nil {
		_ = dirFile.Close()
		release()
		return 0, nil, nil, err
	}
	if fileID(openedInfo) != fileID(parentInfo) {
		_ = dirFile.Close()
		release()
		return 0, nil, nil, ErrUnsafePath
	}
	return parent, dirFile, func() { _ = dirFile.Close(); release() }, nil
}

// isOperationStageName reports whether name is a staged file of the operation.
func isOperationStageName(name, operation string) bool {
	prefix := ".gandalf-sync-" + operation + "-"
	return strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".tmp") &&
		len(name) > len(prefix)+len(".tmp")
}

func (f *projectFS) hasOperationStage(parts []string, operation, expectedSHA256 string) (bool, error) {
	parent, dirFile, cleanup, err := f.openPinnedDir(parts)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer cleanup()
	entries, err := dirFile.ReadDir(-1)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if !isOperationStageName(e.Name(), operation) {
			continue
		}
		file, id, err := openFile(parent, e.Name(), syscall.GENERIC_READ, shareAll)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		state, err := readState(file, id)
		_ = file.Close()
		if err != nil {
			return false, err
		}
		if expectedSHA256 == "" || state.SHA256 == expectedSHA256 {
			return true, nil
		}
	}
	return false, nil
}

func (f *projectFS) cleanupOperationStages(parts []string, operation string) (int, error) {
	parent, dirFile, cleanup, err := f.openPinnedDir(parts)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, changeError(err)
	}
	defer cleanup()
	entries, err := dirFile.ReadDir(-1)
	if err != nil {
		return 0, changeError(err)
	}
	removed := 0
	for _, e := range entries {
		if !isOperationStageName(e.Name(), operation) {
			continue
		}
		file, _, err := openFile(parent, e.Name(), syscall.GENERIC_READ|deleteAccess, shareRead)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return removed, changeError(err)
		}
		uerr := unlinkHandle(syscall.Handle(file.Fd()))
		cerr := file.Close()
		if uerr != nil {
			return removed, changeError(uerr)
		}
		if cerr != nil {
			return removed, changeError(cerr)
		}
		removed++
	}
	return removed, nil
}
