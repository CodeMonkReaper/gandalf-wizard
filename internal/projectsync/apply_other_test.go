//go:build !windows

package projectsync

import "testing"

func assertOwnerOnlyACL(t *testing.T, targetPath string, requireProtected bool) {
	t.Helper()
}
