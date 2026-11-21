package syncfs

import "syscall"

func stageFixture(p *ScopedProject, operation string, desired FileState) (func(), error) {
	parent := syscall.Handle(p.files.base.Fd())
	target, _, err := openFile(parent, targetName, syscall.GENERIC_READ, shareRead)
	if err != nil {
		return nil, err
	}
	staged, err := prepareReplacement(parent, target, operation, desired.Bytes)
	if err != nil {
		_ = target.Close()
		return nil, err
	}
	return func() {
		_ = staged.Close()
		_ = target.Close()
	}, nil
}
