//go:build !windows

package syncfs

type projectFS struct{}

func openProjectFS(string) (*projectFS, string, error) { return nil, "", ErrUnsupported }
func takeWriter(string) (func() error, error)          { return nil, ErrUnsupported }
func (*projectFS) close() error                        { return ErrUnsupported }
func (*projectFS) observe([]string) (FileState, error) { return FileState{}, ErrUnsupported }
func (*projectFS) change([]string, FileState, FileState, string) (bool, error) {
	return false, ErrUnsupported
}
func (*projectFS) hasOperationStage([]string, string, string) (bool, error) {
	return false, ErrUnsupported
}
func (*projectFS) cleanupOperationStages([]string, string) (int, error) {
	return 0, ErrUnsupported
}
