package components_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/agents"
	"github.com/gentleman-programming/gentle-ai/v2/internal/agents/antigravity"
	"github.com/gentleman-programming/gentle-ai/v2/internal/agents/claude"
	codexagent "github.com/gentleman-programming/gentle-ai/v2/internal/agents/codex"
	"github.com/gentleman-programming/gentle-ai/v2/internal/agents/cursor"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/engram"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/persona"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/sdd"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/skills"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

var update = flag.Bool("update", false, "update golden files")

func claudeAdapter() agents.Adapter { return claude.NewAdapter() }

func cursorAdapter() agents.Adapter { return cursor.NewAdapter() }

func codexAdapter() agents.Adapter       { return codexagent.NewAdapter() }
func antigravityAdapter() agents.Adapter { return antigravity.NewAdapter() }

// ---------------------------------------------------------------------------
// Existing golden tests (context7, presets, SDD command)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// SDD Injector golden tests
// ---------------------------------------------------------------------------

func TestGoldenSDD_Claude(t *testing.T) {
	home := t.TempDir()

	adapter := claudeAdapter()

	result, err := sdd.Inject(home, adapter, "")
	if err != nil {
		t.Fatalf("sdd.Inject(claude) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("sdd.Inject(claude) changed = false")
	}

	claudeMD := readTestFile(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	assertGolden(t, "sdd-claude-claudemd.golden", claudeMD)

	for _, name := range []string{
		"sdd-apply", "sdd-archive", "sdd-continue", "sdd-explore",
		"sdd-ff", "sdd-init", "sdd-new", "sdd-onboard", "sdd-research", "sdd-status", "sdd-verify",
	} {
		content := readTestFile(t, filepath.Join(home, ".claude", "commands", name+".md"))
		assertGolden(t, "sdd-claude-cmd-"+name+".golden", content)
	}

	agentsDir := adapter.SubAgentsDir(home)
	for _, name := range []string{
		"sdd-explore", "sdd-research", "sdd-propose", "sdd-spec", "sdd-design",
		"sdd-tasks", "sdd-apply", "sdd-verify", "sdd-archive",
	} {
		agentContent := readTestFile(t, filepath.Join(agentsDir, name+".md"))
		assertGolden(t, "sdd-claude-agent-"+name+".golden", agentContent)
	}
}

func TestGoldenSDD_Cursor(t *testing.T) {
	home := t.TempDir()

	result, err := sdd.Inject(home, cursorAdapter(), "")
	if err != nil {
		t.Fatalf("sdd.Inject(cursor) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("sdd.Inject(cursor) changed = false")
	}

	// Cursor writes SDD orchestrator to ~/.cursor/rules/gentle-ai.mdc.
	rulesFile := readTestFile(t, filepath.Join(home, ".cursor", "rules", "gentle-ai.mdc"))
	assertGolden(t, "sdd-cursor-rules.golden", rulesFile)

	// Golden-check a representative SDD skill file.
	skillInit := readTestFile(t, filepath.Join(home, ".cursor", "skills", "sdd-init", "SKILL.md"))
	assertGolden(t, "sdd-cursor-skill-sdd-init.golden", skillInit)

	// Verify ALL expected SDD skill files exist.
	expectedSkills := []string{
		"sdd-init", "sdd-apply", "sdd-archive", "sdd-explore",
		"sdd-propose", "sdd-research", "sdd-spec", "sdd-design", "sdd-tasks", "sdd-verify",
		"sdd-onboard",
	}
	skillsDir := filepath.Join(home, ".cursor", "skills")
	for _, name := range expectedSkills {
		path := filepath.Join(skillsDir, name, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected SDD skill file %q not found: %v", name, err)
		}
	}
}

func TestGoldenSDD_Codex(t *testing.T) {
	home := t.TempDir()

	result, err := sdd.Inject(home, codexAdapter(), "", sdd.InjectOptions{
		CodexModelAssignments:       model.CodexModelPresetRecommended(),
		CodexCarrilModelAssignments: model.CodexCarrilModelsForPreset("recommended"),
	})
	if err != nil {
		t.Fatalf("sdd.Inject(codex) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("sdd.Inject(codex) changed = false")
	}

	// Codex writes SDD orchestrator to ~/.codex/AGENTS.md.
	agentsMD := readTestFile(t, filepath.Join(home, ".codex", "AGENTS.md"))
	assertGolden(t, "sdd-codex-agentsmd.golden", agentsMD)

	// Golden-check a representative SDD skill file.
	skillInit := readTestFile(t, filepath.Join(home, ".codex", "skills", "sdd-init", "SKILL.md"))
	assertGolden(t, "sdd-codex-skill-sdd-init.golden", skillInit)

	// Verify ALL expected SDD skill files exist.
	expectedSkills := []string{
		"sdd-init", "sdd-apply", "sdd-archive", "sdd-explore",
		"sdd-propose", "sdd-research", "sdd-spec", "sdd-design", "sdd-tasks", "sdd-verify",
		"sdd-onboard",
	}
	skillsDir := filepath.Join(home, ".codex", "skills")
	for _, name := range expectedSkills {
		path := filepath.Join(skillsDir, name, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected SDD skill file %q not found: %v", name, err)
		}
	}
}

func TestGoldenSDD_Codex_LowCost(t *testing.T) {
	home := t.TempDir()

	result, err := sdd.Inject(home, codexAdapter(), "", sdd.InjectOptions{
		CodexModelAssignments:       model.CodexModelPresetLowCost(),
		CodexCarrilModelAssignments: model.CodexCarrilModelsForPreset("low-cost"),
	})
	if err != nil {
		t.Fatalf("sdd.Inject(codex, LowCost) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("sdd.Inject(codex, LowCost) changed = false")
	}

	agentsMD := readTestFile(t, filepath.Join(home, ".codex", "AGENTS.md"))
	assertGolden(t, "sdd-codex-agentsmd-lowcost.golden", agentsMD)
}

func TestGoldenSDD_Codex_Powerful(t *testing.T) {
	home := t.TempDir()

	result, err := sdd.Inject(home, codexAdapter(), "", sdd.InjectOptions{
		CodexModelAssignments:       model.CodexModelPresetPowerful(),
		CodexCarrilModelAssignments: model.CodexCarrilModelsForPreset("powerful"),
	})
	if err != nil {
		t.Fatalf("sdd.Inject(codex, Powerful) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("sdd.Inject(codex, Powerful) changed = false")
	}

	agentsMD := readTestFile(t, filepath.Join(home, ".codex", "AGENTS.md"))
	assertGolden(t, "sdd-codex-agentsmd-powerful.golden", agentsMD)
}

// ---------------------------------------------------------------------------
// Persona Injector golden tests
// ---------------------------------------------------------------------------

func TestGoldenPersona_Claude_Gentleman(t *testing.T) {
	home := t.TempDir()

	result, err := persona.Inject(home, claudeAdapter(), model.PersonaGentleman)
	if err != nil {
		t.Fatalf("persona.Inject(claude, gentleman) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("persona.Inject(claude, gentleman) changed = false")
	}

	claudeMD := readTestFile(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	assertGolden(t, "persona-claude-gentleman.golden", claudeMD)

	outputStyle := readTestFile(t, filepath.Join(home, ".claude", "output-styles", "gentleman.md"))
	assertGolden(t, "persona-claude-gentleman-outputstyle.golden", outputStyle)

	settingsJSON := readTestFile(t, filepath.Join(home, ".claude", "settings.json"))
	assertGolden(t, "persona-claude-gentleman-settings.golden", settingsJSON)
}

func TestGoldenPersona_Claude_Neutral(t *testing.T) {
	home := t.TempDir()

	result, err := persona.Inject(home, claudeAdapter(), model.PersonaNeutral)
	if err != nil {
		t.Fatalf("persona.Inject(claude, neutral) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("persona.Inject(claude, neutral) changed = false")
	}

	claudeMD := readTestFile(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	assertGolden(t, "persona-claude-neutral.golden", claudeMD)

	// Locks the reconciled Neutral output style (Decision 4) — no golden
	// existed for this file before the canonical-channel change.
	outputStyle := readTestFile(t, filepath.Join(home, ".claude", "output-styles", "neutral.md"))
	assertGolden(t, "persona-claude-neutral-outputstyle.golden", outputStyle)
}

func TestGoldenPersona_Claude_Custom(t *testing.T) {
	home := t.TempDir()

	result, err := persona.Inject(home, claudeAdapter(), model.PersonaCustom)
	if err != nil {
		t.Fatalf("persona.Inject(claude, custom) error = %v", err)
	}
	// Custom persona does nothing — no files written.
	if result.Changed {
		t.Fatalf("persona.Inject(claude, custom) changed = true, want false")
	}
	if len(result.Files) != 0 {
		t.Fatalf("persona.Inject(claude, custom) returned files %v, want none", result.Files)
	}
}

// ---------------------------------------------------------------------------
// Engram Injector golden tests
// ---------------------------------------------------------------------------

func TestGoldenEngram_Claude(t *testing.T) {
	home := t.TempDir()

	engram.SetLookPathForTest(t, "/opt/homebrew/bin/engram", "")

	// Pin the engram binary version above the Decision 1 floor (v1.4.0) so
	// this golden reflects the SLIM CLAUDE.md section — the MCP `instructions`
	// channel + SessionStart hook are the verified redundant channels for
	// Claude Code (design.md Decision 1). "1.18.0" matches the live evidence
	// cited in design.md.
	result, err := engram.InjectWithOptions(home, claudeAdapter(), engram.InjectOptions{Version: "1.18.0"})
	if err != nil {
		t.Fatalf("engram.InjectWithOptions(claude) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("engram.InjectWithOptions(claude) changed = false")
	}

	// Claude user MCP registry, the supported user-scope location.
	mcpJSON := readTestFile(t, claude.UserConfigPath(home))
	assertGolden(t, "engram-claude-mcp.golden", mcpJSON)

	// CLAUDE.md with engram-protocol section (slim, per Decision 1).
	claudeMD := readTestFile(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	assertGolden(t, "engram-claude-claudemd.golden", claudeMD)
}

// TestGoldenEngram_Codex captures the rendered Codex model_instructions_file
// and experimental_compact_prompt_file output after the canonical-asset
// consolidation (design.md Decision 3). These goldens catch the content
// growth from consolidating onto the canonical `full` text: the old
// codex/engram-instructions.md (6 "WHEN TO SAVE" bullets, no self-check line)
// is replaced by the fuller canonical text (12 "PROACTIVE SAVE TRIGGERS"
// bullets + a self-check line) concatenated with the unchanged PASSIVE
// CAPTURE section.
func TestGoldenEngram_Codex(t *testing.T) {
	home := t.TempDir()
	restore := codexagent.SetRuntimeVersionCommandForTest("codex-cli 0.144.0", nil)
	t.Cleanup(restore)

	engram.SetLookPathForTest(t, "/opt/homebrew/bin/engram", "")

	result, err := engram.Inject(home, codexAdapter())
	if err != nil {
		t.Fatalf("engram.Inject(codex) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("engram.Inject(codex) changed = false")
	}

	instructions := readTestFile(t, filepath.Join(home, ".codex", "engram-instructions.md"))
	assertGolden(t, "engram-codex-instructions.golden", instructions)

	compactPrompt := readTestFile(t, filepath.Join(home, ".codex", "engram-compact-prompt.md"))
	assertGolden(t, "engram-codex-compact-prompt.golden", compactPrompt)
}

// ---------------------------------------------------------------------------
// Skills Injector golden tests
// ---------------------------------------------------------------------------

func TestGoldenSkills_Claude(t *testing.T) {
	home := t.TempDir()

	skillIDs := []model.SkillID{model.SkillGoTesting, model.SkillCreator}
	result, err := skills.Inject(home, claudeAdapter(), skillIDs)
	if err != nil {
		t.Fatalf("skills.Inject(claude) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("skills.Inject(claude) changed = false")
	}

	goTestingSkill := readTestFile(t, filepath.Join(home, ".claude", "skills", "go-testing", "SKILL.md"))
	assertGolden(t, "skills-claude-go-testing.golden", goTestingSkill)

	skillCreator := readTestFile(t, filepath.Join(home, ".claude", "skills", "skill-creator", "SKILL.md"))
	assertGolden(t, "skills-claude-skill-creator.golden", skillCreator)
}

// ---------------------------------------------------------------------------
// Combined injection golden test (multiple components writing to same CLAUDE.md)
// ---------------------------------------------------------------------------

func TestGoldenCombined_Claude(t *testing.T) {
	home := t.TempDir()

	engram.SetLookPathForTest(t, "/opt/homebrew/bin/engram", "")

	// Inject persona first, then SDD, then Engram — all write sections into CLAUDE.md.
	if _, err := persona.Inject(home, claudeAdapter(), model.PersonaGentleman); err != nil {
		t.Fatalf("persona.Inject error = %v", err)
	}
	if _, err := sdd.Inject(home, claudeAdapter(), ""); err != nil {
		t.Fatalf("sdd.Inject error = %v", err)
	}
	// Pin the engram version above the Decision 1 floor so the combined
	// CLAUDE.md reflects the slim engram-protocol section, matching
	// TestGoldenEngram_Claude above.
	if _, err := engram.InjectWithOptions(home, claudeAdapter(), engram.InjectOptions{Version: "1.18.0"}); err != nil {
		t.Fatalf("engram.InjectWithOptions error = %v", err)
	}

	claudeMD := readTestFile(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	assertGolden(t, "combined-claude-claudemd.golden", claudeMD)
}

// ---------------------------------------------------------------------------
// Antigravity golden tests
// ---------------------------------------------------------------------------

func TestGoldenSDD_Antigravity(t *testing.T) {
	home := t.TempDir()

	result, err := sdd.Inject(home, antigravityAdapter(), "")
	if err != nil {
		t.Fatalf("sdd.Inject(antigravity) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("sdd.Inject(antigravity) changed = false")
	}

	// Antigravity writes SDD orchestrator to ~/.gemini/GEMINI.md (StrategyAppendToFile).
	rulesFile := readTestFile(t, filepath.Join(home, ".gemini", "GEMINI.md"))
	assertGolden(t, "sdd-antigravity-rulesmd.golden", rulesFile)

	// Golden-check a representative SDD skill file.
	skillInit := readTestFile(t, filepath.Join(home, ".gemini", "antigravity-cli", "skills", "sdd-init", "SKILL.md"))
	assertGolden(t, "sdd-antigravity-skill-sdd-init.golden", skillInit)

	// Verify ALL expected SDD skill files exist.
	expectedSkills := []string{
		"sdd-init", "sdd-apply", "sdd-archive", "sdd-explore",
		"sdd-propose", "sdd-research", "sdd-spec", "sdd-design", "sdd-tasks", "sdd-verify",
		"sdd-onboard",
	}
	skillsDir := filepath.Join(home, ".gemini", "antigravity-cli", "skills")
	for _, name := range expectedSkills {
		path := filepath.Join(skillsDir, name, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected SDD skill file %q not found: %v", name, err)
		}
	}
}

func TestGoldenPersona_Antigravity_Gentleman(t *testing.T) {
	home := t.TempDir()

	result, err := persona.Inject(home, antigravityAdapter(), model.PersonaGentleman)
	if err != nil {
		t.Fatalf("persona.Inject(antigravity, gentleman) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("persona.Inject(antigravity, gentleman) changed = false")
	}

	rulesFile := readTestFile(t, filepath.Join(home, ".gemini", "GEMINI.md"))
	assertGolden(t, "persona-antigravity-gentleman.golden", rulesFile)
}

func TestGoldenEngram_Antigravity(t *testing.T) {
	home := t.TempDir()

	engram.SetLookPathForTest(t, "/opt/homebrew/bin/engram", "")

	result, err := engram.Inject(home, antigravityAdapter())
	if err != nil {
		t.Fatalf("engram.Inject(antigravity) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("engram.Inject(antigravity) changed = false")
	}

	// MCP config written to ~/.gemini/antigravity-cli/mcp_config.json.
	mcpJSON := readTestFile(t, filepath.Join(home, ".gemini", "antigravity-cli", "mcp_config.json"))
	assertGolden(t, "engram-antigravity-mcp.golden", mcpJSON)

	// GEMINI.md must contain the engram-protocol section.
	rulesFile := readTestFile(t, filepath.Join(home, ".gemini", "GEMINI.md"))
	assertGolden(t, "engram-antigravity-rulesmd.golden", rulesFile)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func goldenDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "golden")
}

func toStringSlice(ids []model.SkillID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}

func assertGolden(t *testing.T, name string, actual []byte) {
	t.Helper()
	goldenPath := filepath.Join(goldenDir(t), name)

	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("MkdirAll for golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, actual, 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", goldenPath, err)
		}
		t.Logf("updated golden file: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v\n\nRun with -update to generate golden files:\n  go test ./internal/components/ -run %s -update", goldenPath, err, t.Name())
	}

	if string(actual) != string(expected) {
		// Show first difference for easier debugging.
		diffIdx := firstDiffIndex(string(expected), string(actual))
		context := 80
		start := diffIdx - context
		if start < 0 {
			start = 0
		}

		t.Fatalf("golden mismatch for %s (first diff at byte %d)\n\nexpected[%d:%d]:\n%s\n\nactual[%d:%d]:\n%s\n\nRun with -update to regenerate:\n  go test ./internal/components/ -run %s -update",
			name, diffIdx,
			start, min(diffIdx+context, len(string(expected))), string(expected)[start:min(diffIdx+context, len(string(expected)))],
			start, min(diffIdx+context, len(string(actual))), string(actual)[start:min(diffIdx+context, len(string(actual)))],
			t.Name(),
		)
	}
}

func firstDiffIndex(a, b string) int {
	maxLen := len(a)
	if len(b) < maxLen {
		maxLen = len(b)
	}
	for i := 0; i < maxLen; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return maxLen
	}
	return -1
}
