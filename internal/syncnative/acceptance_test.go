package syncnative

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/cli"
)

const (
	acceptanceSkillID   = "revision-cambios"
	acceptanceRuleID    = "evidencia-de-pruebas"
	acceptancePatternID = "revision-por-pares"
	acceptanceVersion   = "1.0.0"

	acceptanceSkillLabel = "GandalfAcceptanceSkill_9c1a2b"
	acceptanceRuleLabel  = "GandalfAcceptanceRule_7f3e4d"

	acceptanceSkillPath     = ".agents/skills/revision-cambios/SKILL.md"
	acceptanceReferencePath = ".agents/skills/revision-cambios/references/checklist.md"
	acceptanceRulePath      = "AGENTS.md"
)

func acceptanceSkillSource() string {
	return "---\nname: " + acceptanceSkillID + "\ndescription: " + acceptanceSkillLabel + "\n---\n\n" +
		"# Revisión de cambios\n\nLee el diff completo antes de publicar.\n"
}

func acceptanceReferenceSource() string {
	return "# Lista de comprobación\n\n- ¿El cambio tiene pruebas ejecutadas?\n"
}

func acceptanceRuleSource() string {
	return "# Evidencia de pruebas\n\n" + acceptanceRuleLabel + "\n\nReporta las pruebas ejecutadas.\n"
}

func acceptancePatternSource() string {
	return "# Revisión por pares\n\nToda revisión deja constancia de quién revisó.\n"
}

func acceptanceConfig() string {
	return "{\n" +
		"  \"targets\": [\"codex\", \"vibe\"],\n" +
		"  \"components\": [\n" +
		"    {\"kind\": \"skill\", \"id\": \"" + acceptanceSkillID + "\", \"version\": \"" + acceptanceVersion + "\"},\n" +
		"    {\"kind\": \"rule\", \"id\": \"" + acceptanceRuleID + "\", \"version\": \"" + acceptanceVersion + "\"},\n" +
		"    {\"kind\": \"pattern\", \"id\": \"" + acceptancePatternID + "\", \"version\": \"" + acceptanceVersion + "\"}\n" +
		"  ]\n" +
		"}\n"
}

// acceptanceProject drives cli.Run against one temporary project copy.
type acceptanceProject struct {
	t       *testing.T
	workDir string
	sources string
}

func newAcceptanceProject(t *testing.T, name string) *acceptanceProject {
	t.Helper()
	root := t.TempDir()
	project := &acceptanceProject{
		t:       t,
		workDir: filepath.Join(root, name),
		sources: filepath.Join(root, name+"-sources"),
	}
	for _, dir := range []string{project.workDir, project.sources} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}
	return project
}

func (p *acceptanceProject) run(args ...string) (int, string) {
	p.t.Helper()
	var stdout, stderr bytes.Buffer
	code := cli.Run(context.Background(), args, cli.Dependencies{
		Version: "acceptance",
		WorkDir: p.workDir,
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	return code, stdout.String() + stderr.String()
}

func (p *acceptanceProject) runJSON(args ...string) map[string]any {
	p.t.Helper()
	code, out := p.run(args...)
	if code != cli.ExitOK {
		p.t.Fatalf("%v exited %d: %s", args, code, out)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		p.t.Fatalf("%v produced non-JSON output: %v", args, err)
	}
	return payload
}

func (p *acceptanceProject) write(rel, content string) {
	p.t.Helper()
	full := filepath.Join(p.workDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		p.t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0600); err != nil {
		p.t.Fatalf("write %s: %v", rel, err)
	}
}

func (p *acceptanceProject) writeSource(name, content string) string {
	p.t.Helper()
	full := filepath.Join(p.sources, name)
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		p.t.Fatalf("mkdir for source %s: %v", name, err)
	}
	if err := os.WriteFile(full, []byte(content), 0600); err != nil {
		p.t.Fatalf("write source %s: %v", name, err)
	}
	return full
}

func (p *acceptanceProject) register() {
	p.t.Helper()
	skillDir := filepath.Join(p.sources, "skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0700); err != nil {
		p.t.Fatalf("create skill source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(acceptanceSkillSource()), 0600); err != nil {
		p.t.Fatalf("write skill source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "checklist.md"), []byte(acceptanceReferenceSource()), 0600); err != nil {
		p.t.Fatalf("write reference source: %v", err)
	}
	ruleSource := p.writeSource("rule.md", acceptanceRuleSource())
	patternSource := p.writeSource("pattern.md", acceptancePatternSource())

	p.runJSON("catalog", "register", "--json",
		"--kind", "skill", "--id", acceptanceSkillID, "--version", acceptanceVersion,
		"--source", skillDir, "--origin", "acceptance")
	p.runJSON("catalog", "register", "--json",
		"--kind", "rule", "--id", acceptanceRuleID, "--version", acceptanceVersion,
		"--source", ruleSource, "--origin", "acceptance")
	p.runJSON("catalog", "register", "--json",
		"--kind", "pattern", "--id", acceptancePatternID, "--version", acceptanceVersion,
		"--source", patternSource, "--origin", "acceptance")
}

func (p *acceptanceProject) apply() string {
	p.t.Helper()
	preview := p.runJSON("sync", "preview", "--json")
	viewID, ok := preview["view_id"].(string)
	if !ok || viewID == "" {
		p.t.Fatalf("preview returned no view_id: %v", preview)
	}
	result := p.runJSON("sync", "apply", "--json", "--view-id", viewID)
	if status, _ := result["status"].(string); status != "applied" {
		p.t.Fatalf("apply status = %q, want applied", status)
	}
	operationID, ok := result["operation_id"].(string)
	if !ok || operationID == "" {
		p.t.Fatalf("apply returned no operation_id: %v", result)
	}
	return operationID
}

func acceptancePayloadString(t *testing.T, payload map[string]any, key string) string {
	t.Helper()
	value, ok := payload[key].(string)
	if !ok {
		t.Fatalf("payload without string %q: %v", key, payload)
	}
	return value
}

// installedVibeVersion reports the version of the installed Vibe CLI.
func installedVibeVersion(t *testing.T) string {
	t.Helper()
	if version := strings.TrimSpace(os.Getenv("GANDALF_VIBE_VERSION")); version != "" {
		return version
	}
	return "3.1.3"
}

func TestSyncNativeAcceptance(t *testing.T) {
	if os.Getenv("GANDALF_SYNC_NATIVE_TEST") != "1" {
		t.Skip("set GANDALF_SYNC_NATIVE_TEST=1 to run the installed CLI probes")
	}

	copyA := newAcceptanceProject(t, "copyA")
	copyB := newAcceptanceProject(t, "copyB")
	for _, project := range []*acceptanceProject{copyA, copyB} {
		project.write(acceptanceRulePath, "# Proyecto de aceptación\n\nNotas previas.\n")
		project.write(".gandalf-sync.json", acceptanceConfig())
		project.register()
	}

	operationID := copyA.apply()
	copyB.apply()

	for _, rel := range []string{acceptanceSkillPath, acceptanceReferencePath, acceptanceRulePath} {
		digestA := fixtureDigest(t, copyA.workDir, rel)
		digestB := fixtureDigest(t, copyB.workDir, rel)
		if digestA != digestB {
			t.Fatalf("%s differs between copyA and copyB", rel)
		}
		t.Logf("projected=%s sha256=%x", rel, digestA)
	}

	vibeVersion := installedVibeVersion(t)
	targets := []struct{ target, version string }{
		{target: "codex", version: "0.169.0"},
		{target: "vibe", version: vibeVersion},
	}
	for _, tc := range targets {
		got := captureProbe(t, tc.target, tc.version, copyA.workDir)
		requireText(t, "skill", got.skillContext, acceptanceSkillLabel, "")
		requireText(t, "rule", got.ruleContext, acceptanceRuleLabel, "")
		if got.blockedRequests != 0 {
			t.Fatalf("%s blocked %d requests, want 0", tc.target, got.blockedRequests)
		}
		t.Logf("target=%s version=%s platform=%s/%s proxy_denials=%d", tc.target, tc.version, runtime.GOOS, runtime.GOARCH, got.blockedRequests)
	}

	restored := copyA.runJSON("sync", "restore", "--json", "--operation-id", operationID)
	if status := acceptancePayloadString(t, restored, "status"); status != "restored" {
		t.Fatalf("restore status = %q, want restored", status)
	}

	for _, tc := range targets {
		got := captureProbe(t, tc.target, tc.version, copyA.workDir)
		requireText(t, "skill", got.skillContext, "", acceptanceSkillLabel)
		requireText(t, "rule", got.ruleContext, "", acceptanceRuleLabel)
		if got.blockedRequests != 0 {
			t.Fatalf("%s blocked %d requests after restore, want 0", tc.target, got.blockedRequests)
		}
		t.Logf("restored target=%s version=%s platform=%s/%s proxy_denials=%d", tc.target, tc.version, runtime.GOOS, runtime.GOARCH, got.blockedRequests)
	}
}
