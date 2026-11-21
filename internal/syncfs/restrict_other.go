//go:build !windows

package syncfs

func (f *projectFS) restrictDir([]string) error {
	return ErrUnsupported
}
