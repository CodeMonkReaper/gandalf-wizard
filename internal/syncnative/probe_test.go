package syncnative

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	probeSkillName     = "gandalf-native-conformance"
	probeSkillPath     = ".agents/skills/" + probeSkillName + "/SKILL.md"
	probeReferencePath = ".agents/skills/" + probeSkillName + "/references/sample.md"
	vibeSkillPath      = ".vibe/skills/" + probeSkillName + "/SKILL.md"
	probeSkillLabel    = "GandalfNativeProbeSkill_4eb8790"
	vibeSkillLabel     = "GandalfNativeProbeVibeSkill_e3000ca"
	probeRule          = "# GandalfNativeProbeRule_1da9b4c\n\nReport which checks ran.\n"
	probeOverride      = "# GandalfNativeProbeOverride_17836a7\n"
	probeReference     = "Synthetic reference for the native discovery test.\n"
	probeOverridePath  = "AGENTS.override.md"
	probeRulePath      = "AGENTS.md"
)

func TestSyncNativeProbe(t *testing.T) {
	if os.Getenv("GANDALF_SYNC_NATIVE_TEST") != "1" {
		t.Skip("set GANDALF_SYNC_NATIVE_TEST=1 to run the installed CLI probes")
	}

	tests := []struct {
		target, version, trust            string
		collisionSkill, hiddenSkill       string
		collisionRule, hiddenRule         string
		winningSkillPath, winningRulePath string
	}{
		{
			target: "codex", version: "0.169.0", trust: "temporary project trust",
			collisionSkill: probeSkillLabel, hiddenSkill: vibeSkillLabel,
			collisionRule: probeOverride, hiddenRule: probeRule,
			winningSkillPath: probeSkillPath, winningRulePath: probeOverridePath,
		},
		{
			target: "vibe", version: "3.1.2", trust: "invocation-only --trust",
			collisionSkill: vibeSkillLabel, hiddenSkill: probeSkillLabel,
			collisionRule: probeRule, hiddenRule: probeOverride,
			winningSkillPath: vibeSkillPath, winningRulePath: probeRulePath,
		},
	}
	for _, tc := range tests {
		t.Run(tc.target, func(t *testing.T) {
			t.Run("project", func(t *testing.T) {
				project := t.TempDir()
				writeProbeProject(t, project)
				got := captureProbe(t, tc.target, tc.version, project)
				requireText(t, "skill", got.skillContext, probeSkillLabel, "")
				requireText(t, "rule", got.ruleContext, strings.TrimSpace(probeRule), "")
				t.Logf("target=%s version=%s platform=%s/%s trust=%q proxy_denials=%d", tc.target, tc.version, runtime.GOOS, runtime.GOARCH, tc.trust, got.blockedRequests)
				for _, path := range []string{probeSkillPath, probeRulePath, probeReferencePath} {
					t.Logf("fixture=%s sha256=%x", path, fixtureDigest(t, project, path))
				}
			})
			t.Run("precedence", func(t *testing.T) {
				project := t.TempDir()
				writeProbeProject(t, project)
				writeNativeFixture(t, project, vibeSkillPath, probeSkill(vibeSkillLabel))
				writeNativeFixture(t, project, probeOverridePath, probeOverride)

				got := captureProbe(t, tc.target, tc.version, project)
				requireText(t, "skill", got.skillContext, tc.collisionSkill, tc.hiddenSkill)
				requireText(t, "rule", got.ruleContext, strings.TrimSpace(tc.collisionRule), strings.TrimSpace(tc.hiddenRule))
				t.Logf("observed skill=%s rule=%s proxy_denials=%d", tc.winningSkillPath, tc.winningRulePath, got.blockedRequests)
			})
			t.Run("empty-project", func(t *testing.T) {
				got := captureProbe(t, tc.target, tc.version, t.TempDir())
				requireText(t, "skill", got.skillContext, "", probeSkillLabel)
				requireText(t, "rule", got.ruleContext, "", strings.TrimSpace(probeRule))
				t.Logf("observed no project skill or rule, proxy_denials=%d", got.blockedRequests)
			})
		})
	}
}

func probeSkill(description string) string {
	return "---\nname: " + probeSkillName + "\ndescription: " + description + "\n---\n\nReport which checks ran.\n"
}

func writeProbeProject(t *testing.T, project string) {
	t.Helper()
	writeNativeFixture(t, project, probeSkillPath, probeSkill(probeSkillLabel))
	writeNativeFixture(t, project, probeReferencePath, probeReference)
	writeNativeFixture(t, project, probeRulePath, probeRule)
}

func writeNativeFixture(t *testing.T, directory, path, content string) {
	t.Helper()
	full := filepath.Join(directory, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func fixtureDigest(t *testing.T, directory, path string) [sha256.Size]byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("read synthetic fixture %s", path)
	}
	return sha256.Sum256(data)
}

func requireText(t *testing.T, artifact, observed, present, absent string) {
	t.Helper()
	if present != "" && !strings.Contains(observed, present) {
		t.Fatalf("expected %s absent from native context", artifact)
	}
	if absent != "" && strings.Contains(observed, absent) {
		t.Fatalf("unexpected %s present in native context", artifact)
	}
}

func TestVibeProbeEnvironmentDisablesKeyring(t *testing.T) {
	t.Setenv("VIBE_TEST_DISABLE_KEYRING", "0")
	env := nativeEnvironment(t.TempDir(), t.TempDir(), "vibe", "http://127.0.0.1:1")
	var guards []string
	for _, entry := range env {
		if strings.HasPrefix(entry, "VIBE_TEST_DISABLE_KEYRING=") {
			guards = append(guards, entry)
		}
	}
	if len(guards) != 1 || guards[0] != "VIBE_TEST_DISABLE_KEYRING=1" {
		t.Fatalf("expected one effective keyring guard, got %v", guards)
	}
}
