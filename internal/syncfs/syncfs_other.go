//go:build !windows

package syncfs

type projectFS struct{}

func openProjectFS(string) (*projectFS, string, error) { return nil, "", ErrUnsupported }
func takeWriter(string) (func() error, error)          { return nil, ErrUnsupported }
func (*projectFS) close() error                        { return ErrUnsupported }
func (*projectFS) observe([]string) (FileState, error) { return FileState{}, ErrUnsupported }
func (*projectFS) change([]string, FileState, FileState) (bool, error) {
	return false, ErrUnsupported
}
