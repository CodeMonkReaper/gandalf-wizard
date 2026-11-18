package harness

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func lookPathFunc(found map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		if path, ok := found[name]; ok {
			return path, nil
		}
		return "", errors.New("not found")
	}
}

func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		info, err := d.Info()
		if err != nil {
			return err
		}
		paths = append(paths, rel+"|"+info.Mode().String())
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	sort.Strings(paths)
	return strings.Join(paths, "\n")
}

func TestDetectHarnesses(t *testing.T) {
	t.Run("detecta los tres harnesses y rsc presente", func(t *testing.T) {
		root := t.TempDir()
		rscPath := filepath.Join(root, ".rsc.json")
		if err := os.WriteFile(rscPath, []byte(`{"version":1}`), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		before := snapshotTree(t, root)

		report := Detect(root, lookPathFunc(map[string]string{
			"codex":       "/bin/codex",
			"vibe":        "/bin/vibe",
			"antigravity": "/bin/antigravity",
		}))

		if len(report.Harnesses) != 3 {
			t.Fatalf("harnesses = %d, want 3", len(report.Harnesses))
		}
		want := []struct {
			name     string
			detected bool
			path     string
		}{
			{HarnessCodex, true, "/bin/codex"},
			{HarnessVibe, true, "/bin/vibe"},
			{HarnessAntigravity, true, "/bin/antigravity"},
		}
		for i, w := range want {
			got := report.Harnesses[i]
			if got.Name != w.name || got.Detected != w.detected || got.Path != w.path {
				t.Errorf("harnesses[%d] = %+v, want %s/%v/%s", i, got, w.name, w.detected, w.path)
			}
		}
		if !report.RSCPresent || report.RSCConfigPath != rscPath {
			t.Errorf("rsc = %v/%q, want true/%q", report.RSCPresent, report.RSCConfigPath, rscPath)
		}

		if after := snapshotTree(t, root); after != before {
			t.Error("Detect modificó el árbol de archivos; debe ser solo lectura")
		}
	})

	t.Run("antigravity cae a agy como fallback", func(t *testing.T) {
		root := t.TempDir()
		report := Detect(root, lookPathFunc(map[string]string{
			"codex": "/bin/codex",
			"vibe":  "/bin/vibe",
			"agy":   "/bin/agy",
		}))
		ag := report.Harnesses[2]
		if ag.Name != HarnessAntigravity || !ag.Detected || ag.Path != "/bin/agy" {
			t.Errorf("antigravity = %+v, want detectado vía /bin/agy", ag)
		}
	})

	t.Run("nada detectado y sin rsc", func(t *testing.T) {
		root := t.TempDir()
		report := Detect(root, lookPathFunc(nil))
		for i, want := range []string{HarnessCodex, HarnessVibe, HarnessAntigravity} {
			if report.Harnesses[i].Name != want || report.Harnesses[i].Detected {
				t.Errorf("harnesses[%d] = %+v, want %s no detectado", i, report.Harnesses[i], want)
			}
		}
		if report.RSCPresent || report.RSCConfigPath != "" {
			t.Errorf("rsc = %v/%q, want false/\"\"", report.RSCPresent, report.RSCConfigPath)
		}
	})

	t.Run("lookPath nil no entra en pánico", func(t *testing.T) {
		root := t.TempDir()
		report := Detect(root, nil)
		if len(report.Harnesses) != 3 {
			t.Fatalf("harnesses = %d, want 3", len(report.Harnesses))
		}
	})
}
