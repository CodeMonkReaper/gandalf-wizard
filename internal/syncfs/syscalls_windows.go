package syncfs

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const (
	shareRead           = syscall.FILE_SHARE_READ
	shareReadWrite      = shareRead | syscall.FILE_SHARE_WRITE
	shareReadDelete     = shareRead | syscall.FILE_SHARE_DELETE
	shareAll            = shareReadWrite | syscall.FILE_SHARE_DELETE
	deleteAccess        = 0x00010000
	fileOpen            = 1
	fileCreate          = 2
	fileOpenIf          = 3
	directoryFile       = 0x00000001
	synchronousIO       = 0x00000020
	nonDirectoryFile    = 0x00000040
	deleteOnClose       = 0x00001000
	openReparsePoint    = 0x00200000
	caseInsensitive     = 0x40
	renameInformationEx = 65
	dispositionInfoEx   = 64
	renameReplace       = 0x00000001
	renamePOSIX         = 0x00000002
	dispositionDelete   = 0x00000001
	dispositionPOSIX    = 0x00000002
	dispositionOnClose  = 0x00000008
	volumeNameGUID      = 0x00000001
	mutexModifyState    = 0x00000001
	sharingViolation    = syscall.Errno(32)
	encryptedAttribute  = 0x00004000
	ownerGroupDACLLabel = 0x00000001 | 0x00000002 | 0x00000004 | 0x00000010
)

// These system DLLs are loaded by the Windows process before Go code runs.
var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	advapi32             = syscall.NewLazyDLL("advapi32.dll")
	ntdll                = syscall.NewLazyDLL("ntdll.dll")
	createMutex          = kernel32.NewProc("CreateMutexExW")
	releaseMutex         = kernel32.NewProc("ReleaseMutex")
	finalPathByHandle    = kernel32.NewProc("GetFinalPathNameByHandleW")
	volumeInfoByHandle   = kernel32.NewProc("GetVolumeInformationByHandleW")
	fileInfoByHandle     = kernel32.NewProc("GetFileInformationByHandleEx")
	getKernelSecurity    = advapi32.NewProc("GetKernelObjectSecurity")
	ntCreateFile         = ntdll.NewProc("NtCreateFile")
	ntSetInformationFile = ntdll.NewProc("NtSetInformationFile")
	statusToError        = ntdll.NewProc("RtlNtStatusToDosError")
)

type unicodeString struct {
	length, capacity uint16
	buffer           *uint16
}

type objectAttributes struct {
	length           uint32
	parent           syscall.Handle
	name             *unicodeString
	flags            uint32
	security         uintptr
	qualityOfService uintptr
}

type ioStatusBlock struct {
	status, information uintptr
}

// openAt receives one validated component. The kernel uses the supplied
// directory handle and opens any reparse point itself for inspection.
func openAt(parent syscall.Handle, name string, access, share, disposition, options uint32) (syscall.Handle, error) {
	name16, err := syscall.UTF16FromString(name)
	if err != nil {
		return 0, ErrUnsafePath
	}
	nameBytes := (len(name16) - 1) * 2
	if nameBytes > 510 {
		return 0, ErrUnsafePath
	}
	objectName := unicodeString{uint16(nameBytes), uint16(nameBytes + 2), &name16[0]}
	attrs := objectAttributes{parent: parent, name: &objectName, flags: caseInsensitive}
	attrs.length = uint32(unsafe.Sizeof(attrs))
	var handle syscall.Handle
	var status ioStatusBlock
	result, _, _ := ntCreateFile.Call(
		uintptr(unsafe.Pointer(&handle)), uintptr(access|syscall.SYNCHRONIZE),
		uintptr(unsafe.Pointer(&attrs)), uintptr(unsafe.Pointer(&status)),
		0, syscall.FILE_ATTRIBUTE_NORMAL, uintptr(share), uintptr(disposition),
		uintptr(options|synchronousIO|openReparsePoint), 0, 0,
	)
	return handle, ntError(result)
}

func ntError(status uintptr) error {
	if int32(status) >= 0 {
		return nil
	}
	code, _, _ := statusToError.Call(uintptr(uint32(status)))
	return syscall.Errno(code)
}

func setFileInfo(handle syscall.Handle, class uint32, data unsafe.Pointer, size uintptr) error {
	var status ioStatusBlock
	result, _, _ := ntSetInformationFile.Call(
		uintptr(handle), uintptr(unsafe.Pointer(&status)), uintptr(data), size, uintptr(class),
	)
	return ntError(result)
}

func unlinkHandle(handle syscall.Handle) error {
	flags := uint32(dispositionDelete | dispositionPOSIX)
	return setFileInfo(handle, dispositionInfoEx, unsafe.Pointer(&flags), unsafe.Sizeof(flags))
}

func keepStagedFile(handle syscall.Handle) error {
	flags := uint32(dispositionOnClose)
	return setFileInfo(handle, dispositionInfoEx, unsafe.Pointer(&flags), unsafe.Sizeof(flags))
}

func renameHandle(handle, parent syscall.Handle, name string, replace bool) error {
	name16, err := syscall.UTF16FromString(name)
	if err != nil || len(name16) > 256 {
		return ErrUnsafePath
	}
	// FILE_RENAME_INFORMATION for FileRenameInformationEx.
	var info struct {
		flags      uint32
		parent     syscall.Handle
		nameLength uint32
		name       [256]uint16
	}
	info.parent = parent
	info.nameLength = uint32((len(name16) - 1) * 2)
	copy(info.name[:], name16)
	if replace {
		info.flags = renameReplace | renamePOSIX
	}
	return setFileInfo(handle, renameInformationEx, unsafe.Pointer(&info), unsafe.Sizeof(info))
}

func inspectHandle(handle syscall.Handle, directory bool) (syscall.ByHandleFileInformation, error) {
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &info); err != nil {
		return info, err
	}
	isDirectory := info.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0
	if info.FileAttributes&(syscall.FILE_ATTRIBUTE_REPARSE_POINT|encryptedAttribute) != 0 ||
		isDirectory != directory || !directory && info.NumberOfLinks != 1 {
		return info, ErrUnsafePath
	}
	return info, nil
}

func requireDefaultStream(handle syscall.Handle) error {
	const fileStreamInfo = 7
	const moreData = syscall.Errno(234)
	// FILE_STREAM_INFO begins with the next record offset. The default
	// data stream fits in this buffer. Additional streams are unsupported.
	var stream struct {
		nextOffset uint32
		nameLength uint32
		size       int64
		allocation int64
		name       [256]uint16
	}
	ok, _, err := fileInfoByHandle.Call(uintptr(handle), fileStreamInfo,
		uintptr(unsafe.Pointer(&stream)), unsafe.Sizeof(stream))
	if ok == 0 {
		if errors.Is(err, moreData) {
			return ErrUnsafePath
		}
		return fmt.Errorf("inspect file streams: %w", err)
	}
	if stream.nextOffset != 0 || stream.nameLength != 14 ||
		syscall.UTF16ToString(stream.name[:7]) != "::$DATA" {
		return ErrUnsafePath
	}
	return nil
}

func fileID(info syscall.ByHandleFileInformation) string {
	return fmt.Sprintf("%08x-%08x%08x", info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow)
}

func localNTFSVolume(handle syscall.Handle) (string, error) {
	var filesystem [32]uint16
	result, _, err := volumeInfoByHandle.Call(uintptr(handle), 0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&filesystem[0])), uintptr(len(filesystem)))
	if result == 0 {
		return "", fmt.Errorf("identify filesystem: %w", err)
	}
	if syscall.UTF16ToString(filesystem[:]) != "NTFS" {
		return "", ErrUnsupported
	}

	// Network shares have no local volume GUID.
	required, _, _ := finalPathByHandle.Call(uintptr(handle), 0, 0, volumeNameGUID)
	if required == 0 || required > 32768 {
		return "", ErrUnsupported
	}
	path := make([]uint16, required)
	size, _, _ := finalPathByHandle.Call(uintptr(handle), uintptr(unsafe.Pointer(&path[0])), required, volumeNameGUID)
	if size == 0 || size >= required {
		return "", ErrUnsupported
	}
	prefix, ok := strings.CutPrefix(syscall.UTF16ToString(path), "\\\\?\\")
	if !ok {
		return "", ErrUnsupported
	}
	volume, _, ok := strings.Cut(prefix, "\\")
	if !ok || !strings.HasPrefix(volume, "Volume{") {
		return "", ErrUnsupported
	}
	return strings.ToLower(volume), nil
}

func accessDescriptor(handle syscall.Handle) ([]byte, error) {
	var needed uint32
	_, _, err := getKernelSecurity.Call(uintptr(handle), ownerGroupDACLLabel, 0, 0, uintptr(unsafe.Pointer(&needed)))
	if needed == 0 {
		return nil, err
	}
	data := make([]byte, needed)
	ok, _, err := getKernelSecurity.Call(uintptr(handle), ownerGroupDACLLabel,
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&needed)))
	if ok == 0 {
		return nil, err
	}
	return data[:needed], nil
}

func takeWriter(id string) (func() error, error) {
	name, err := syscall.UTF16PtrFromString("Global\\GandalfWizard.Sync." + id)
	if err != nil {
		return nil, err
	}
	raw, _, callErr := createMutex.Call(0, uintptr(unsafe.Pointer(name)), 0, syscall.SYNCHRONIZE|mutexModifyState)
	if raw == 0 {
		return nil, fmt.Errorf("open project writer mutex: %w", callErr)
	}
	handle := syscall.Handle(raw)
	acquired := make(chan error)
	release := make(chan struct{})
	done := make(chan error)
	go func() {
		// Windows grants mutex ownership to the OS thread making the wait call.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		state, err := syscall.WaitForSingleObject(handle, 0)
		switch state {
		case syscall.WAIT_OBJECT_0, syscall.WAIT_ABANDONED:
			acquired <- nil
		case syscall.WAIT_TIMEOUT:
			acquired <- errors.Join(ErrBusy, syscall.CloseHandle(handle))
			return
		default:
			acquired <- errors.Join(fmt.Errorf("wait for project writer mutex: %w", err), syscall.CloseHandle(handle))
			return
		}
		<-release
		ok, _, err := releaseMutex.Call(uintptr(handle))
		if ok != 0 {
			err = nil
		}
		done <- errors.Join(err, syscall.CloseHandle(handle))
	}()
	if err := <-acquired; err != nil {
		return nil, err
	}
	return func() error {
		close(release)
		return <-done
	}, nil
}

func changeError(err error) error {
	if errors.Is(err, sharingViolation) || errors.Is(err, os.ErrExist) || errors.Is(err, os.ErrNotExist) {
		return errors.Join(ErrConflict, err)
	}
	return err
}
