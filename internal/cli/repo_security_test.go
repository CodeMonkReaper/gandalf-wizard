package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type secretRule struct {
	name    string
	pattern *regexp.Regexp
}

var repoSecretRules = []secretRule{
	{name: "clave Mistral/Vibe", pattern: regexp.MustCompile(`mstrl_[A-Za-z0-9_-]{12,}`)},
	{name: "clave OpenAI/Anthropic", pattern: regexp.MustCompile(`sk-(?:proj-|live-|ant-|)[A-Za-z0-9_-]{24,}`)},
	{name: "clave Google/Gemini", pattern: regexp.MustCompile(`AIza[0-9A-Za-z_-]{32,}`)},
	{name: "token GitHub", pattern: regexp.MustCompile(`(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{30,})`)},
	{name: "clave privada PEM", pattern: regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH |)PRIVATE KEY-----`)},
}

// scanPathAndContent checks a repository-relative path and its content for
// prohibited secret patterns, credential files, or session state.
func scanPathAndContent(relPath string, content []byte) []string {
	var findings []string
	slash := filepath.ToSlash(relPath)
	lowerSlash := strings.ToLower(slash)
	base := strings.ToLower(filepath.Base(slash))

	if base == ".env" || (strings.HasPrefix(base, ".env.") && base != ".env.example") {
		findings = append(findings, "archivo de entorno prohibido: "+slash)
	}
	switch base {
	case ".gandalf.json", "whoami_cache.json", "google_accounts.json":
		findings = append(findings, "archivo de configuracion o sesion local prohibido: "+slash)
	default:
		if strings.Contains(base, "credentials") && strings.HasSuffix(base, ".json") {
			findings = append(findings, "archivo de credenciales prohibido: "+slash)
		}
	}

	switch strings.ToLower(filepath.Ext(base)) {
	case ".pem", ".key", ".pfx", ".exe", ".test":
		findings = append(findings, "extension binaria o de clave prohibida: "+slash)
	}

	if lowerSlash == ".vibe/config.toml" {
		findings = append(findings, "configuracion local prohibida: "+slash)
	}
	// A file named exactly .gandalf is neither a directory for the .gitignore
	// pattern nor caught by the prefix rules below.
	if base == ".gandalf" {
		findings = append(findings, "estado local de workflows prohibido: "+slash)
	}
	for _, prefix := range []string{".vibe/sessions/", ".vibe/logs/", ".vibe/worktrees/", ".rsc/", ".gandalf/", "01-tools/vibe-bridge/", "02-docs/security/"} {
		// A workflow can use a subproject as its root.
		if strings.HasPrefix(lowerSlash, prefix) || (prefix == ".gandalf/" && strings.Contains(lowerSlash, "/"+prefix)) {
			findings = append(findings, "directorio local o sensible prohibido: "+slash)
			break
		}
	}

	findings = append(findings, scanContentSecrets(slash, content)...)
	return findings
}

func scanContentSecrets(label string, content []byte) []string {
	var findings []string
	for _, rule := range repoSecretRules {
		if rule.pattern.Match(content) {
			findings = append(findings, rule.name+" detectada en "+label)
		}
	}
	return findings
}

func gitRepoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("git rev-parse --show-toplevel: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestRepoSecurityGate enforces deterministic secret and session hygiene
// across tracked files, branch commits, and synthetic blocking cases (P5, P6).
func TestRepoSecurityGate(t *testing.T) {
	t.Run("arbol e historial de la rama limpios de claves y sesiones", func(t *testing.T) {
		root := gitRepoRoot(t)

		lsOut, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
		if err != nil {
			t.Fatalf("git ls-files: %v", err)
		}
		for _, entry := range bytes.Split(lsOut, []byte{0}) {
			if len(entry) == 0 {
				continue
			}
			rel := string(entry)
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatalf("leer %s: %v", rel, err)
			}
			if findings := scanPathAndContent(rel, data); len(findings) > 0 {
				t.Errorf("hallazgo en %s: %v", rel, findings)
			}
		}

		revRange := "HEAD"
		if exec.Command("git", "-C", root, "rev-parse", "--verify", "origin/master").Run() == nil {
			revRange = "origin/master..HEAD"
		}
		patchOut, err := exec.Command("git", "-C", root, "log", revRange, "-p").Output()
		if err != nil {
			t.Fatalf("git log %s -p: %v", revRange, err)
		}
		if findings := scanContentSecrets("historial "+revRange, patchOut); len(findings) > 0 {
			t.Errorf("secreto en historial de commits: %v", findings)
		}
	})

	t.Run("bloquea claves API y archivos de sesion simulados", func(t *testing.T) {
		blockedCases := []struct {
			name    string
			relPath string
			content []byte
		}{
			{
				name:    "clave Mistral/Vibe",
				relPath: "tmp/config.txt",
				content: []byte("key=" + "mstr" + "l_" + strings.Repeat("aB3d", 4)),
			},
			{
				name:    "clave OpenAI",
				relPath: "tmp/config.txt",
				content: []byte("key=" + "sk-" + "proj-" + strings.Repeat("xY9z", 7)),
			},
			{
				name:    "clave Google",
				relPath: "tmp/config.txt",
				content: []byte("key=" + "AI" + "za" + strings.Repeat("0AbC", 9)),
			},
			{
				name:    "token GitHub",
				relPath: "tmp/config.txt",
				content: []byte("tok=" + "gh" + "p_" + strings.Repeat("kLmN", 8)),
			},
			{
				name:    "clave privada PEM",
				relPath: "tmp/key.txt",
				content: []byte("-----BEGIN " + "PRIVATE KEY-----\nMIIB..."),
			},
			{
				name:    "archivo .env",
				relPath: ".env",
				content: []byte("FOO=bar"),
			},
			{
				name:    "archivo .env.local",
				relPath: ".env.local",
				content: []byte("FOO=bar"),
			},
			{
				name:    "sesion vibe whoami_cache.json",
				relPath: "tmp/whoami_cache.json",
				content: []byte(`{"user":"demo"}`),
			},
			{
				name:    "sesion antigravity google_accounts.json",
				relPath: "tmp/google_accounts.json",
				content: []byte(`{"active":"demo"}`),
			},
			{
				name:    "configuracion local .gandalf.json",
				relPath: ".gandalf.json",
				content: []byte(`{"project":"demo"}`),
			},
			{
				name:    "checkpoint local de workflow",
				relPath: ".gandalf/workflows/runs/run-1/state.json",
				content: []byte(`{"status":"paused"}`),
			},
			{
				name:    "artefacto local de un subproyecto",
				relPath: "examples/demo/.gandalf/workflows/runs/run-1/artifacts/report.txt",
				content: []byte("synthetic artifact"),
			},
			{
				name:    "fichero con el nombre del estado local",
				relPath: "examples/demo/.gandalf",
				content: []byte("not a directory"),
			},
			{
				name:    "archivo credentials json",
				relPath: "config/client_credentials.json",
				content: []byte(`{}`),
			},
			{
				name:    "artefacto vibe-bridge",
				relPath: "01-TOOLS/vibe-bridge/runs/out.log",
				content: []byte("log"),
			},
			{
				name:    "configuracion local .vibe/config.toml",
				relPath: ".vibe/config.toml",
				content: []byte("model = 'x'"),
			},
			{
				name:    "sesion local en .vibe/sessions",
				relPath: ".vibe/sessions/s1.json",
				content: []byte(`{"id":"1"}`),
			},
			{
				name:    "directorio de auditoria de seguridad",
				relPath: "02-DOCS/security/audit-2026-11-18/report.html",
				content: []byte("<html></html>"),
			},
			{
				name:    "binario ejecutable .exe",
				relPath: "gandalf.exe",
				content: []byte("MZ"),
			},
		}

		for _, tc := range blockedCases {
			if findings := scanPathAndContent(tc.relPath, tc.content); len(findings) == 0 {
				t.Errorf("%s (%s): esperaba bloqueo, pero no hubo hallazgos", tc.name, tc.relPath)
			}
		}

		allowedCases := []struct {
			relPath string
			content []byte
		}{
			{relPath: ".env.example", content: []byte("ROMERO_API_KEY=\nMISTRAL_API_KEY=\n")},
			{relPath: ".vibe/hooks.toml", content: []byte("# hooks\n")},
			{relPath: "internal/cli/router.go", content: []byte("package cli\n")},
			{relPath: "02-DOCS/wiki/index.md", content: []byte("# Conocimiento\n")},
			{relPath: "testdata/.gandalf-example/workflow.json", content: []byte(`{}`)},
		}
		for _, tc := range allowedCases {
			if findings := scanPathAndContent(tc.relPath, tc.content); len(findings) > 0 {
				t.Errorf("%s: no debia bloquearse, hallazgos=%v", tc.relPath, findings)
			}
		}
	})

	t.Run("git ignora el estado local de workflows", func(t *testing.T) {
		root := gitRepoRoot(t)
		for _, tc := range []struct {
			path string
			rule string
		}{
			{".gandalf/workflows/runs/run-1/state.json", ":.gandalf/"},
			{".gandalf", ":.gandalf"},
			{"examples/demo/.gandalf", ":.gandalf"},
		} {
			out, err := exec.Command("git", "-C", root, "check-ignore", "--no-index", "-v", "--", tc.path).CombinedOutput()
			rule, _, _ := strings.Cut(string(out), "\t")
			if err != nil || !strings.HasPrefix(rule, ".gitignore:") || !strings.HasSuffix(rule, tc.rule) {
				t.Errorf("la regla %s de .gitignore debe excluir %s: %v (%q)", tc.rule, tc.path, err, out)
			}
		}
	})
}
