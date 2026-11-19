package syncfs

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const targetName = "rules.txt"

func TestSyncFilesystem(t *testing.T) {
	t.Run("rutas y enlaces", testConfinement)
	t.Run("estados y precondiciones", testFileStates)
	t.Run("ACL de un destino existente", testDestinationACL)
	t.Run("flujos adicionales del destino", testNamedStreams)
	t.Run("identidad y anclaje de la raiz", testProjectIdentity)
	t.Run("anclaje de directorios intermedios", testParentPins)
	t.Run("exclusion sin archivos de bloqueo", testExclusiveProcess)
	t.Run("terminacion antes y despues del reemplazo", testInterruptedProcess)
	t.Run("precondicion de identidad antes de publicar", testPublicationGuard)
	t.Run("lease de operacion", testOperationLease)
}

func testConfinement(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeFixture(t, filepath.Join(outside, targetName), "outside")
	writeFixture(t, filepath.Join(root, targetName), "inside")
	dir := filepath.Join(root, "nested")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(dir, targetName), "nested")
	for name, destination := range map[string]string{"outside-junction": outside, "inside-junction": dir} {
		path := filepath.Join(root, name)
		out, err := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", path, destination).CombinedOutput()
		if err != nil {
			t.Fatalf("create junction fixture: %v (%s)", err, out)
		}
		if _, err := OpenProject(path); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("open project through junction: got %v, want ErrUnsafePath", err)
		}
	}
	if err := os.Link(filepath.Join(outside, targetName), filepath.Join(root, "hard-link.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(root, targetName), filepath.Join(root, "second-link.txt")); err != nil {
		t.Fatal(err)
	}
	project := openFixture(t, root)
	lease := acquireFixture(t, project)
	paths := []string{"../rules.txt", "nested/../rules.txt", "..\\rules.txt",
		"nested//rules.txt", "nested/rules.txt.", "nested/rules.txt ", "NUL.txt", "nested/rules.txt:alternate",
		"C:rules.txt", "\\rules.txt", filepath.Join(outside, targetName),
		"outside-junction/" + targetName, "inside-junction/" + targetName, "outside-junction",
		"hard-link.txt", "second-link.txt", targetName, "nested", "hard-link.txt/child",
		"absent/" + strings.Repeat("x", 256), "absent/" + strings.Repeat("界", 256) + "/child.txt"}
	for _, path := range paths {
		if _, err := project.Observe(path); !errors.Is(err, ErrUnsafePath) {
			t.Errorf("Observe(%q): got %v, want ErrUnsafePath", path, err)
		}
		if changed, err := project.Change(lease, path, FileState{}, NewFileState([]byte("changed"))); changed || !errors.Is(err, ErrUnsafePath) {
			t.Errorf("Change(%q): got changed=%v, err=%v, want ErrUnsafePath", path, changed, err)
		}
	}
	checkContents(t, filepath.Join(outside, targetName), "outside")
	checkContents(t, filepath.Join(dir, targetName), "nested")
}

func testFileStates(t *testing.T) {
	root := t.TempDir()
	project := openFixture(t, root)
	lease := acquireFixture(t, project)
	path := "nested/recipe.txt"
	absent := observeFixture(t, project, path)
	if absent.Exists || absent.SHA256 != "" || absent.FileID != "" || len(absent.Bytes) != 0 {
		t.Fatalf("expected absence for missing parent, got %+v", absent)
	}
	if _, err := os.Stat(filepath.Join(root, "nested")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Observe created a parent or could not inspect it: %v", err)
	}

	before := NewFileState([]byte("before"))
	assertChange(t, project, lease, path, absent, before, true)
	observed := observeFixture(t, project, path)
	if !observed.Exists || observed.FileID == "" || observed.SHA256 != before.SHA256 || !bytes.Equal(observed.Bytes, before.Bytes) {
		t.Fatalf("created state does not match desired bytes or have an identity: %+v", observed)
	}
	assertChange(t, project, lease, path, observed, before, false)
	if unchanged := observeFixture(t, project, path); unchanged.FileID != observed.FileID {
		t.Fatal("a no-op replaced the file")
	}

	after := NewFileState([]byte("after"))
	replaced := filepath.Join(root, "nested", "replacement.txt")
	writeFixture(t, replaced, "before")
	if err := os.Rename(replaced, filepath.Join(root, filepath.FromSlash(path))); err != nil {
		t.Fatal(err)
	}
	replacement := observeFixture(t, project, path)
	if replacement.FileID == observed.FileID || replacement.SHA256 != observed.SHA256 {
		t.Fatal("fixture did not replace the inode while preserving the digest")
	}
	if changed, err := project.Change(lease, path, observed, after); changed || !errors.Is(err, ErrConflict) {
		t.Fatalf("replaced identity: got changed=%v, err=%v, want ErrConflict", changed, err)
	}
	writeFixture(t, filepath.Join(root, filepath.FromSlash(path)), "external")
	if changed, err := project.Change(lease, path, replacement, after); changed || !errors.Is(err, ErrConflict) {
		t.Fatalf("external bytes: got changed=%v, err=%v, want ErrConflict", changed, err)
	}
	checkContents(t, filepath.Join(root, filepath.FromSlash(path)), "external")
	current := observeFixture(t, project, path)
	editor, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(path)), os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := project.Change(lease, path, current, after); changed || !errors.Is(err, ErrConflict) {
		_ = editor.Close()
		t.Fatalf("open external writer: got changed=%v, err=%v, want ErrConflict", changed, err)
	}
	if err := editor.Close(); err != nil {
		t.Fatal(err)
	}
	checkContents(t, filepath.Join(root, filepath.FromSlash(path)), "external")
	assertChange(t, project, lease, path, current, after, true)
	applied := observeFixture(t, project, path)
	if !applied.Exists || applied.SHA256 != after.SHA256 || !bytes.Equal(applied.Bytes, after.Bytes) {
		t.Fatal("replacement did not produce the expected bytes")
	}
	assertChange(t, project, lease, path, applied, FileState{}, true)
	if observeFixture(t, project, path).Exists {
		t.Fatal("restoring the original absence left a file")
	}
	assertChange(t, project, lease, path, FileState{}, FileState{}, false)

	other := openFixture(t, t.TempDir())
	if changed, err := other.Change(lease, targetName, FileState{}, before); changed || !errors.Is(err, ErrInvalidLease) {
		t.Fatalf("foreign lease: got changed=%v, err=%v, want ErrInvalidLease", changed, err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if changed, err := project.Change(lease, targetName, FileState{}, before); changed || !errors.Is(err, ErrInvalidLease) {
		t.Fatalf("released lease: got changed=%v, err=%v, want ErrInvalidLease", changed, err)
	}
}

func testDestinationACL(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, targetName)
	writeFixture(t, path, "private before")
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid, err := user.User.Sid.String()
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("icacls.exe", path, "/inheritance:r", "/grant:r", "*"+sid+":(F)").CombinedOutput()
	if err != nil {
		t.Fatalf("restrict fixture ACL: %v (%s)", err, out)
	}
	project := openFixture(t, root)
	lease := acquireFixture(t, project)
	before := observeFixture(t, project, targetName)
	changed, err := project.Change(lease, targetName, before, NewFileState([]byte("private after")))
	if changed || !errors.Is(err, ErrConflict) {
		t.Fatalf("replacing restricted ACL: got changed=%v, err=%v, want ErrConflict", changed, err)
	}
	checkContents(t, path, "private before")
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != targetName {
		t.Fatalf("ACL conflict left a stage file: %v (%v)", entries, err)
	}
}

func testProjectIdentity(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "project")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	project := openFixture(t, root)
	second := openFixture(t, root)
	unrelated := openFixture(t, t.TempDir())
	if project.ID() == "" || project.ID() != second.ID() || project.ID() == unrelated.ID() {
		t.Fatal("project identity must agree for the same directory and differ for another")
	}
	lease := acquireFixture(t, project)
	acquireFixture(t, unrelated)
	for _, handle := range []*ScopedProject{project, second} {
		if _, err := handle.TryExclusive("fixture"); !errors.Is(err, ErrBusy) {
			t.Fatalf("a second acquisition in one process returned %v, want ErrBusy", err)
		}
	}
	renamed := filepath.Join(base, "renamed")
	if err := os.Rename(root, renamed); err == nil {
		t.Fatal("an open project root could be moved")
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if err := project.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, renamed); err != nil {
		t.Fatalf("closed root remained pinned: %v", err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
}

func testParentPins(t *testing.T) {
	for _, fixture := range []struct {
		name   string
		create bool
	}{{"existente", false}, {"creado", true}} {
		t.Run(fixture.name, func(t *testing.T) {
			root := t.TempDir()
			ancestor := filepath.Join(root, "ancestor")
			parent := filepath.Join(ancestor, "parent")
			if !fixture.create {
				if err := os.MkdirAll(parent, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			project := openFixture(t, root)
			_, release, err := project.files.pinParent([]string{"ancestor", "parent", targetName}, fixture.create)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if release != nil {
					release()
				}
			}()

			moved := filepath.Join(ancestor, "moved")
			if err := os.Rename(parent, moved); err == nil {
				t.Fatal("the destination parent could be moved while pinned")
			}
			if err := os.Rename(ancestor, filepath.Join(root, "moved-ancestor")); err == nil {
				t.Fatal("an ancestor could be moved while pinned")
			}
			release()
			release = nil
			if err := os.Rename(parent, moved); err != nil {
				t.Fatalf("released parent remained pinned: %v", err)
			}
		})
	}
}

func testExclusiveProcess(t *testing.T) {
	root := t.TempDir()
	holder := startFilesystemProcess(t, root, "hold")
	runContender(t, root, "busy")
	before, err := os.ReadDir(root)
	if err != nil || len(before) != 0 {
		t.Fatalf("leasing populated a fresh project: %v (%v)", before, err)
	}
	holder.stop(t)
	runContender(t, root, "free")
	after, err := os.ReadDir(root)
	if err != nil || len(after) != 0 {
		t.Fatalf("leasing left project artifacts: %v (%v)", after, err)
	}
}

func testInterruptedProcess(t *testing.T) {
	for _, point := range []struct {
		mode, want string
	}{{"before", "before"}, {"after", "after"}} {
		t.Run(point.mode, func(t *testing.T) {
			root := t.TempDir()
			writeFixture(t, filepath.Join(root, targetName), "before")
			project := openFixture(t, root)
			holder := startFilesystemProcess(t, root, point.mode)
			checkContents(t, filepath.Join(root, targetName), point.want)
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if point.mode == "before" {
				want = 2
				attributed := false
				for _, entry := range entries {
					attributed = attributed || strings.HasPrefix(entry.Name(), ".gandalf-sync-fixture-")
				}
				if !attributed {
					t.Fatalf("surviving stage is not attributable to the operation: %v", entries)
				}
			}
			if len(entries) != want {
				t.Fatalf("expected %d project files at %s replacement, got %v", want, point.mode, entries)
			}
			runContender(t, root, "busy")
			holder.stop(t)
			entries, err = os.ReadDir(root)
			if err != nil || len(entries) != 1 || entries[0].Name() != targetName {
				t.Fatalf("interrupted replacement left unexpected files: %v (%v)", entries, err)
			}
			lease := acquireFixture(t, project)
			current := observeFixture(t, project, targetName)
			if !current.Exists || current.SHA256 != NewFileState([]byte(point.want)).SHA256 {
				t.Fatal("interrupted target was neither the expected old nor new state")
			}
			assertChange(t, project, lease, targetName, current, NewFileState([]byte("before")), point.mode == "after")
			checkContents(t, filepath.Join(root, targetName), "before")
		})
	}
}

func TestSyncFilesystemProcess(t *testing.T) {
	mode := os.Getenv("GANDALF_SYNCFS_TEST_PROCESS")
	if mode == "" {
		return
	}
	project := openFixture(t, os.Getenv("GANDALF_SYNCFS_TEST_ROOT"))
	if mode == "busy" {
		if _, err := project.TryExclusive("fixture"); !errors.Is(err, ErrBusy) {
			t.Fatalf("competitor got %v, want ErrBusy", err)
		}
		return
	}
	lease := acquireFixture(t, project)
	switch mode {
	case "free":
		return
	case "before":
		staged, err := stageFixture(project, lease.operation, NewFileState([]byte("after")))
		if err != nil {
			t.Fatal(err)
		}
		defer staged()
	case "after":
		before := observeFixture(t, project, targetName)
		assertChange(t, project, lease, targetName, before, NewFileState([]byte("after")), true)
	case "hold":
	default:
		t.Fatalf("unexpected test mode %q", mode)
	}
	fmt.Println("READY")
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}
}

type filesystemProcess struct {
	cmd    *exec.Cmd
	waited bool
}

func filesystemCommand(t *testing.T, root, mode string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSyncFilesystemProcess$")
	cmd.Env = append(os.Environ(), "GANDALF_SYNCFS_TEST_PROCESS="+mode, "GANDALF_SYNCFS_TEST_ROOT="+root)
	return cmd
}

func startFilesystemProcess(t *testing.T, root, mode string) *filesystemProcess {
	t.Helper()
	cmd := filesystemCommand(t, root, mode)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close() })
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	process := &filesystemProcess{cmd: cmd}
	t.Cleanup(func() {
		if !process.waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	var output []string
	s := bufio.NewScanner(stdout)
	for s.Scan() {
		if s.Text() == "READY" {
			return process
		}
		output = append(output, s.Text())
	}
	t.Fatalf("holder did not report readiness: %v (%s)", s.Err(), strings.Join(output, "\n"))
	return nil
}

func (p *filesystemProcess) stop(t *testing.T) {
	t.Helper()
	if err := p.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	err := p.cmd.Wait()
	p.waited = true
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("killed process exit result: %v", err)
	}
}

func runContender(t *testing.T, root, mode string) {
	t.Helper()
	cmd := filesystemCommand(t, root, mode)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("contender %s: %v (%s)", mode, err, out)
	}
}

func openFixture(t *testing.T, root string) *ScopedProject {
	t.Helper()
	project, err := OpenProject(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := project.Close(); err != nil {
			t.Error(err)
		}
	})
	return project
}

func acquireFixture(t *testing.T, project *ScopedProject) *Lease {
	t.Helper()
	lease, err := project.TryExclusive("fixture")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
	})
	return lease
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func checkContents(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("read fixture: got %q (%v), want %q", got, err, want)
	}
}

func observeFixture(t *testing.T, project *ScopedProject, path string) FileState {
	t.Helper()
	state, err := project.Observe(path)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func assertChange(t *testing.T, p *ScopedProject, lease *Lease, path string, expected, desired FileState, want bool) {
	t.Helper()
	if changed, err := p.Change(lease, path, expected, desired); changed != want || err != nil {
		t.Fatalf("change: got changed=%v, err=%v, want changed=%v", changed, err, want)
	}
}

func testPublicationGuard(t *testing.T) {
	for _, seam := range []struct {
		name  string
		apply func(root string)
	}{
		{"reemplazo externo", func(root string) {
			name := filepath.Join(root, targetName)
			if err := os.Remove(name); err != nil {
				t.Errorf("seam could not remove the target: %v", err)
			}
			writeFixture(t, name, "external")
		}},
		{"nombre eliminado", func(root string) {
			if err := os.Remove(filepath.Join(root, targetName)); err != nil {
				t.Errorf("seam could not remove the target: %v", err)
			}
		}},
	} {
		t.Run(seam.name, func(t *testing.T) {
			root := t.TempDir()
			writeFixture(t, filepath.Join(root, targetName), "before")
			project := openFixture(t, root)
			lease := acquireFixture(t, project)
			expected := observeFixture(t, project, targetName)
			after := NewFileState([]byte("after"))
			testSeamBeforePublication = func() { seam.apply(root) }
			defer func() { testSeamBeforePublication = nil }()
			if changed, err := project.Change(lease, targetName, expected, after); changed || !errors.Is(err, ErrConflict) {
				t.Fatalf("target changed before publication: got changed=%v, err=%v, want ErrConflict", changed, err)
			}
			if seam.name == "reemplazo externo" {
				checkContents(t, filepath.Join(root, targetName), "external")
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".gandalf-sync-") {
					t.Fatalf("guard left a stage file: %v", entries)
				}
			}
		})
	}
}

func testOperationLease(t *testing.T) {
	root := t.TempDir()
	project := openFixture(t, root)
	for _, operation := range []string{"", "op/x", "op x", strings.Repeat("x", 65), "op\n"} {
		lease, err := project.TryExclusive(operation)
		if err == nil {
			_ = lease.Close()
			t.Fatalf("TryExclusive(%q) succeeded, want ErrInvalidOperation", operation)
		}
		if !errors.Is(err, ErrInvalidOperation) {
			t.Fatalf("TryExclusive(%q): got %v, want ErrInvalidOperation", operation, err)
		}
	}
	lease := acquireFixture(t, project)
	if _, err := project.TryExclusive("fixture"); !errors.Is(err, ErrBusy) {
		t.Fatalf("a second acquisition returned %v, want ErrBusy", err)
	}
	assertChange(t, project, lease, targetName, FileState{}, NewFileState([]byte("op")), true)
}

func testNamedStreams(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, targetName)
	writeFixture(t, path, "main")
	project := openFixture(t, root)
	lease := acquireFixture(t, project)
	expected := observeFixture(t, project, targetName)
	writeFixture(t, path+":gandalf-metadata", "metadata")
	if _, err := project.Observe(targetName); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("Observe with named stream: got %v, want ErrUnsafePath", err)
	}
	if changed, err := project.Change(lease, targetName, expected, NewFileState([]byte("replacement"))); changed || !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("Change with named stream: got changed=%v, err=%v", changed, err)
	}
	checkContents(t, path, "main")
	checkContents(t, path+":gandalf-metadata", "metadata")
}
