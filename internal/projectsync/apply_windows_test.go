package projectsync

import (
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

func assertOwnerOnlyACL(t *testing.T, targetPath string, requireProtected bool) {
	t.Helper()
	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getNamedSecurityInfo := advapi32.NewProc("GetNamedSecurityInfoW")
	convertSDToString := advapi32.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW")
	localFree := kernel32.NewProc("LocalFree")

	p16, err := syscall.UTF16PtrFromString(targetPath)
	if err != nil {
		t.Fatalf("UTF16PtrFromString: %v", err)
	}
	var sd uintptr
	const (
		seFileObject     = 1
		daclSecurityInfo = 0x00000004
		sddlRevision1    = 1
	)
	r1, _, _ := getNamedSecurityInfo.Call(
		uintptr(unsafe.Pointer(p16)),
		seFileObject,
		daclSecurityInfo,
		0, 0, 0, 0,
		uintptr(unsafe.Pointer(&sd)),
	)
	if r1 != 0 {
		t.Fatalf("GetNamedSecurityInfoW(%s): %v", targetPath, syscall.Errno(r1))
	}
	defer localFree.Call(sd)

	var strPtr *uint16
	r1, _, e1 := convertSDToString.Call(
		sd,
		sddlRevision1,
		daclSecurityInfo,
		uintptr(unsafe.Pointer(&strPtr)),
		0,
	)
	if r1 == 0 {
		t.Fatalf("ConvertSecurityDescriptorToStringSecurityDescriptorW: %v", e1)
	}
	defer localFree.Call(uintptr(unsafe.Pointer(strPtr)))

	sddl := syscall.UTF16ToString((*[4096]uint16)(unsafe.Pointer(strPtr))[:])
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		t.Fatalf("OpenCurrentProcessToken: %v", err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		t.Fatalf("GetTokenUser: %v", err)
	}
	sidStr, err := user.User.Sid.String()
	if err != nil {
		t.Fatalf("SID.String: %v", err)
	}
	if requireProtected && !strings.HasPrefix(sddl, "D:P") {
		t.Fatalf("expected protected DACL (D:P) for %s, got SDDL %q", targetPath, sddl)
	}
	if !strings.HasPrefix(sddl, "D:") || !strings.Contains(sddl, sidStr) || strings.Count(sddl, "(") != 1 {
		t.Fatalf("expected owner-only single-ACE DACL for %s (sid=%s), got SDDL %q", targetPath, sidStr, sddl)
	}
}
