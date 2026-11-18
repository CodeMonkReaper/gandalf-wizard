package harness

import (
	"os"
	"path/filepath"
)

// antigravityFallback is the alternative executable name for Antigravity.
const antigravityFallback = "agy"

// rscConfigFileName is the RSC manifest file probed at the project root.
const rscConfigFileName = ".rsc.json"

// Detect probes the known harness executables and the RSC manifest in
// read-only mode: it only calls lookPath and os.Stat, never writes.
func Detect(projectRoot string, lookPath func(string) (string, error)) DetectionReport {
	report := DetectionReport{
		Harnesses: []HarnessStatus{
			probe(HarnessCodex, lookPath),
			probe(HarnessVibe, lookPath),
			probeAntigravity(lookPath),
		},
	}

	rscPath := filepath.Join(projectRoot, rscConfigFileName)
	if st, err := os.Stat(rscPath); err == nil && !st.IsDir() {
		report.RSCPresent = true
		report.RSCConfigPath = rscPath
	}
	return report
}

// probe reports whether a single harness executable is on PATH.
func probe(name string, lookPath func(string) (string, error)) HarnessStatus {
	if lookPath == nil {
		return HarnessStatus{Name: name}
	}
	if path, err := lookPath(name); err == nil && path != "" {
		return HarnessStatus{Name: name, Detected: true, Path: path}
	}
	return HarnessStatus{Name: name}
}

// probeAntigravity checks antigravity first and falls back to agy.
func probeAntigravity(lookPath func(string) (string, error)) HarnessStatus {
	if lookPath == nil {
		return HarnessStatus{Name: HarnessAntigravity}
	}
	if path, err := lookPath(HarnessAntigravity); err == nil && path != "" {
		return HarnessStatus{Name: HarnessAntigravity, Detected: true, Path: path}
	}
	if path, err := lookPath(antigravityFallback); err == nil && path != "" {
		return HarnessStatus{Name: HarnessAntigravity, Detected: true, Path: path}
	}
	return HarnessStatus{Name: HarnessAntigravity}
}
