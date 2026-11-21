package syncfs

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	writeDAC          = 0x00040000
	readControl       = 0x00020000
	seFileObject      = 1
	daclSecurityInfo  = 0x00000004
	protectedDaclInfo = 0x80000000
	sddlRevision1     = 1
)

var (
	convertStringSDToSD = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
	getSecurityDescDacl = advapi32.NewProc("GetSecurityDescriptorDacl")
	setSecurityInfoProc = advapi32.NewProc("SetSecurityInfo")
	localFreeProc       = kernel32.NewProc("LocalFree")
)

func (f *projectFS) restrictDir(parts []string) error {
	parent, release, err := f.pinParent(parts, true)
	if err != nil {
		return err
	}
	defer release()

	name := parts[len(parts)-1]
	handle, err := openAt(parent, name, syscall.GENERIC_READ|writeDAC|readControl, shareReadWrite, fileOpen, 0)
	if errors.Is(err, os.ErrNotExist) {
		handle, err = openAt(parent, name, syscall.GENERIC_READ|writeDAC|readControl, shareReadWrite, fileOpenIf, directoryFile)
	}
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(handle)

	if _, err := inspectHandle(handle, true); err != nil {
		return err
	}

	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return fmt.Errorf("open process token: %w", err)
	}
	defer token.Close()

	user, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("get token user: %w", err)
	}
	sidStr, err := user.User.Sid.String()
	if err != nil {
		return fmt.Errorf("format user sid: %w", err)
	}

	// Protected DACL granting Full Access (FA) with Object+Container Inherit (OICI)
	// exclusively to the current user SID.
	sddl, err := syscall.UTF16PtrFromString("D:P(A;OICI;FA;;;" + sidStr + ")")
	if err != nil {
		return err
	}

	var sd uintptr
	r1, _, e1 := convertStringSDToSD.Call(
		uintptr(unsafe.Pointer(sddl)),
		sddlRevision1,
		uintptr(unsafe.Pointer(&sd)),
		0,
	)
	if r1 == 0 {
		return fmt.Errorf("convert sddl: %w", e1)
	}
	defer localFreeProc.Call(sd)

	var daclPresent uint32
	var dacl uintptr
	var daclDefaulted uint32
	r1, _, e1 = getSecurityDescDacl.Call(
		sd,
		uintptr(unsafe.Pointer(&daclPresent)),
		uintptr(unsafe.Pointer(&dacl)),
		uintptr(unsafe.Pointer(&daclDefaulted)),
	)
	if r1 == 0 || daclPresent == 0 {
		return fmt.Errorf("extract dacl: %w", e1)
	}

	r1, _, _ = setSecurityInfoProc.Call(
		uintptr(handle),
		seFileObject,
		daclSecurityInfo|protectedDaclInfo,
		0,
		0,
		dacl,
		0,
	)
	if r1 != 0 {
		return fmt.Errorf("set security info: %w", syscall.Errno(r1))
	}
	return nil
}
