package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	acceptanceSecret = "SECRET_MARKER_T008_ACCEPTANCE"

	acceptanceSkillID   = "revision-cambios"
	acceptanceRuleID    = "evidencia-de-pruebas"
	acceptancePatternID = "revision-por-pares"
	acceptanceVersion   = "1.0.0"

	acceptanceSkillPath     = ".agents/skills/revision-cambios/SKILL.md"
	acceptanceReferencePath = ".agents/skills/revision-cambios/references/checklist.md"
	acceptanceRulePath      = "AGENTS.md"
)

// acceptanceHarness runs the CLI against one temporary project copy and keeps
// every byte the CLI wrote so AC-08 can assert the secret never leaks.
type acceptanceHarness struct {
	t        *testing.T
	workDir  string
	sources  string
	observed bytes.Buffer
}

func newAcceptanceHarness(t *testing.T, name string) *acceptanceHarness {
	t.Helper()
	root := t.TempDir()
	workDir := filepath.Join(root, name)
	sources := filepath.Join(root, name+"-sources")
	for _, dir := range []string{workDir, sources} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}
	return &acceptanceHarness{t: t, workDir: workDir, sources: sources}
}

func (h *acceptanceHarness) run(args ...string) (int, string) {
	h.t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), args, Dependencies{
		Version: "acceptance",
		WorkDir: h.workDir,
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	combined := stdout.String() + stderr.String()
	h.observed.WriteString(combined)
	if strings.Contains(combined, acceptanceSecret) {
		h.t.Fatalf("AC-08 violated: secret marker leaked by %v", args)
	}
	return code, combined
}

func (h *acceptanceHarness) runJSON(t *testing.T, args ...string) map[string]any {
	t.Helper()
	code, out := h.run(args...)
	if code != ExitOK {
		t.Fatalf("%v exited %d: %s", args, code, out)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("%v produced non-JSON output: %v", args, err)
	}
	return payload
}

func (h *acceptanceHarness) write(rel, content string) string {
	h.t.Helper()
	full := filepath.Join(h.workDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		h.t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0600); err != nil {
		h.t.Fatalf("write %s: %v", rel, err)
	}
	return full
}

func (h *acceptanceHarness) writeSource(name, content string) string {
	h.t.Helper()
	full := filepath.Join(h.sources, name)
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		h.t.Fatalf("mkdir for source %s: %v", name, err)
	}
	if err := os.WriteFile(full, []byte(content), 0600); err != nil {
		h.t.Fatalf("write source %s: %v", name, err)
	}
	return full
}

func (h *acceptanceHarness) read(rel string) []byte {
	h.t.Helper()
	data, err := os.ReadFile(filepath.Join(h.workDir, filepath.FromSlash(rel)))
	if err != nil {
		h.t.Fatalf("read %s: %v", rel, err)
	}
	return data
}

func (h *acceptanceHarness) digest(rel string) [sha256.Size]byte {
	h.t.Helper()
	return sha256.Sum256(h.read(rel))
}

func (h *acceptanceHarness) exists(rel string) bool {
	h.t.Helper()
	_, err := os.Stat(filepath.Join(h.workDir, filepath.FromSlash(rel)))
	return err == nil
}

func acceptanceSkillSource() string {
	return "---\nname: " + acceptanceSkillID + "\ndescription: Revisa los cambios antes de publicarlos.\n---\n\n" +
		"# Revisión de cambios\n\n" +
		"1. Lee el diff completo.\n" +
		"2. Recorre la lista de comprobación.\n" +
		"3. Reporta los hallazgos.\n"
}

func acceptanceReferenceSource() string {
	return "# Lista de comprobación\n\n" +
		"- ¿El cambio tiene pruebas ejecutadas?\n" +
		"- ¿El diff está acotado al objetivo?\n"
}

func acceptanceRuleSource() string {
	return "# Evidencia de pruebas\n\n" +
		"Reporta siempre las pruebas que se ejecutaron y su resultado.\n"
}

func acceptancePatternSource() string {
	return "# Revisión por pares\n\n" +
		"Toda revisión por pares deja constancia de quién revisó y qué encontró.\n"
}

func acceptanceAgentsFile() string {
	return "# Proyecto de aceptación\n\n" +
		"Notas previas del proyecto que deben sobrevivir a la sincronización.\n\n" +
		"Marcador reservado: " + acceptanceSecret + "\n"
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

// registerAcceptanceComponents registers the approved reference example and
// returns the rule source path so AC-01 can mutate it afterwards.
func (h *acceptanceHarness) registerAcceptanceComponents(t *testing.T) string {
	t.Helper()
	skillDir := filepath.Join(h.sources, "skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0700); err != nil {
		t.Fatalf("create skill source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(acceptanceSkillSource()), 0600); err != nil {
		t.Fatalf("write skill source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "checklist.md"), []byte(acceptanceReferenceSource()), 0600); err != nil {
		t.Fatalf("write reference source: %v", err)
	}
	ruleSource := h.writeSource("rule.md", acceptanceRuleSource())
	patternSource := h.writeSource("pattern.md", acceptancePatternSource())

	h.runJSON(t, "catalog", "register", "--json",
		"--kind", "skill", "--id", acceptanceSkillID, "--version", acceptanceVersion,
		"--source", skillDir, "--origin", "acceptance")
	h.runJSON(t, "catalog", "register", "--json",
		"--kind", "rule", "--id", acceptanceRuleID, "--version", acceptanceVersion,
		"--source", ruleSource, "--origin", "acceptance")
	h.runJSON(t, "catalog", "register", "--json",
		"--kind", "pattern", "--id", acceptancePatternID, "--version", acceptanceVersion,
		"--source", patternSource, "--origin", "acceptance")
	return ruleSource
}

func acceptanceChanges(t *testing.T, payload map[string]any) []any {
	t.Helper()
	raw, ok := payload["changes"]
	if !ok {
		t.Fatalf("payload without changes: %v", payload)
	}
	changes, ok := raw.([]any)
	if !ok {
		t.Fatalf("changes is not a list: %v", raw)
	}
	return changes
}

func acceptanceString(t *testing.T, payload map[string]any, key string) string {
	t.Helper()
	value, ok := payload[key].(string)
	if !ok {
		t.Fatalf("payload without string %q: %v", key, payload)
	}
	return value
}

func TestSyncAcceptance(t *testing.T) {
	copyA := newAcceptanceHarness(t, "copyA")
	copyB := newAcceptanceHarness(t, "copyB")

	originalAgents := acceptanceAgentsFile()
	for _, h := range []*acceptanceHarness{copyA, copyB} {
		h.write(acceptanceRulePath, originalAgents)
		h.write(".gandalf-sync.json", acceptanceConfig())
	}

	// AC-01: registration freezes the bytes; later source edits must not leak.
	ruleSourceA := copyA.registerAcceptanceComponents(t)
	copyB.registerAcceptanceComponents(t)
	if err := os.WriteFile(ruleSourceA, []byte("# Mutado\n\nEste texto no debe llegar al proyecto.\n"), 0600); err != nil {
		t.Fatalf("mutate rule source: %v", err)
	}

	// AC-02: preview is read-only and reports the three pending changes.
	for _, h := range []*acceptanceHarness{copyA, copyB} {
		before := h.digest(acceptanceRulePath)
		preview := h.runJSON(t, "sync", "preview", "--json")
		if got := acceptanceString(t, preview, "status"); got != "ok" {
			t.Fatalf("preview status = %q, want ok", got)
		}
		if viewID := acceptanceString(t, preview, "view_id"); viewID == "" {
			t.Fatal("preview returned an empty view_id")
		}
		if changes := acceptanceChanges(t, preview); len(changes) != 3 {
			t.Fatalf("preview reported %d changes, want 3", len(changes))
		}
		if after := h.digest(acceptanceRulePath); after != before {
			t.Fatal("preview mutated AGENTS.md")
		}
		if h.exists(acceptanceSkillPath) {
			t.Fatal("preview projected the skill before apply")
		}
	}

	// AC-03: apply converges both copies to identical bytes.
	applied := make(map[string]string, 2)
	for name, h := range map[string]*acceptanceHarness{"copyA": copyA, "copyB": copyB} {
		preview := h.runJSON(t, "sync", "preview", "--json")
		viewID := acceptanceString(t, preview, "view_id")
		result := h.runJSON(t, "sync", "apply", "--json", "--view-id", viewID)
		if got := acceptanceString(t, result, "status"); got != "applied" {
			t.Fatalf("%s apply status = %q, want applied", name, got)
		}
		if opID := acceptanceString(t, result, "operation_id"); opID == "" {
			t.Fatalf("%s apply returned an empty operation_id", name)
		}
		applied[name] = acceptanceString(t, result, "operation_id")
	}

	for _, rel := range []string{acceptanceSkillPath, acceptanceReferencePath, acceptanceRulePath} {
		if copyA.digest(rel) != copyB.digest(rel) {
			t.Fatalf("AC-03: %s differs between copyA and copyB", rel)
		}
	}

	// AC-01 continued: the projected rule carries the registered bytes.
	projected := string(copyA.read(acceptanceRulePath))
	if !strings.Contains(projected, strings.TrimSpace(acceptanceRuleSource())) {
		t.Fatalf("AC-01: projected AGENTS.md lacks the registered rule text:\n%s", projected)
	}
	if strings.Contains(projected, "Este texto no debe llegar al proyecto") {
		t.Fatal("AC-01: projected AGENTS.md used the mutated source")
	}
	if !strings.Contains(projected, "Notas previas del proyecto") {
		t.Fatal("AC-01: projected AGENTS.md dropped pre-existing content")
	}

	// AC-04: a second preview/apply is a noop.
	repeatPreview := copyA.runJSON(t, "sync", "preview", "--json")
	if rawChanges, ok := repeatPreview["changes"]; ok && rawChanges != nil {
		if changes, ok := rawChanges.([]any); !ok || len(changes) != 0 {
			t.Fatalf("AC-04: repeat preview reported unexpected changes: %v", rawChanges)
		}
	}
	repeatApply := copyA.runJSON(t, "sync", "apply", "--json",
		"--view-id", acceptanceString(t, repeatPreview, "view_id"))
	if got := acceptanceString(t, repeatApply, "status"); got != "noop" {
		t.Fatalf("AC-04: repeat apply status = %q, want noop", got)
	}
	if opID, _ := repeatApply["operation_id"].(string); opID != "" {
		t.Fatalf("AC-04: noop apply returned operation_id %q, want empty", opID)
	}

	// AC-06: restore returns AGENTS.md to its original bytes and removes the skill.
	restored := copyA.runJSON(t, "sync", "restore", "--json", "--operation-id", applied["copyA"])
	if got := acceptanceString(t, restored, "status"); got != "restored" {
		t.Fatalf("AC-06: restore status = %q, want restored", got)
	}
	if got := string(copyA.read(acceptanceRulePath)); got != originalAgents {
		t.Fatalf("AC-06: AGENTS.md not restored to original bytes:\n%s", got)
	}
	for _, rel := range []string{acceptanceSkillPath, acceptanceReferencePath} {
		if copyA.exists(rel) {
			t.Fatalf("AC-06: %s survived restore", rel)
		}
	}

	// AC-07: a local edit blocks restore until it is reconciled.
	edited := originalAgents + "\nEdición local posterior al apply.\n"
	copyB.write(acceptanceRulePath, edited)
	code, out := copyB.run("sync", "restore", "--json", "--operation-id", applied["copyB"])
	if code != ExitConfigError {
		t.Fatalf("AC-07: conflicting restore exited %d, want %d: %s", code, ExitConfigError, out)
	}
	var conflict map[string]any
	if err := json.Unmarshal([]byte(out), &conflict); err != nil {
		t.Fatalf("AC-07: conflicting restore produced non-JSON output: %v", err)
	}
	diags, _ := conflict["diagnostics"].([]any)
	foundRestoreConflict := false
	for _, raw := range diags {
		if m, ok := raw.(map[string]any); ok && m["code"] == "restore_conflict" {
			foundRestoreConflict = true
		}
	}
	if !foundRestoreConflict {
		t.Fatalf("AC-07: expected restore_conflict diagnostic, got %v", conflict)
	}
	if got := string(copyB.read(acceptanceRulePath)); got != edited {
		t.Fatalf("AC-07: conflicting restore discarded the local edit:\n%s", got)
	}

	copyB.write(acceptanceRulePath, originalAgents)
	retried := copyB.runJSON(t, "sync", "restore", "--json", "--operation-id", applied["copyB"])
	if got := acceptanceString(t, retried, "status"); got != "restored" {
		t.Fatalf("AC-07: retried restore status = %q, want restored", got)
	}
	if got := string(copyB.read(acceptanceRulePath)); got != originalAgents {
		t.Fatalf("AC-07: retried restore left AGENTS.md as:\n%s", got)
	}
	for _, rel := range []string{acceptanceSkillPath, acceptanceReferencePath} {
		if copyB.exists(rel) {
			t.Fatalf("AC-07: %s survived the retried restore", rel)
		}
	}

	// AC-08: no CLI invocation ever echoed the secret marker.
	for name, h := range map[string]*acceptanceHarness{"copyA": copyA, "copyB": copyB} {
		if strings.Contains(h.observed.String(), acceptanceSecret) {
			t.Fatalf("AC-08: %s leaked the secret marker", name)
		}
	}
}
