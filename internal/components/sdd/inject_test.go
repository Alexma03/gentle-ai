package sdd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/agents"
	"github.com/gentleman-programming/gentle-ai/v2/internal/agents/claude"
	"github.com/gentleman-programming/gentle-ai/v2/internal/assets"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	// agents/cursor, agents/gemini, agents/vscode used via agents.NewAdapter()
)

func claudeAdapter() agents.Adapter { return claude.NewAdapter() }

func mockNoPackageManager(t *testing.T) {
	t.Helper()
}

// TestInjectHermesWritesSDDOrchestratorToSOULMD verifies that sdd.Inject writes
// the Hermes-specific SDD orchestrator content into ~/.hermes/SOUL.md via
// StrategyMarkdownSections markers. Content is preserved across re-runs.

// TestInjectHermesSDDIdempotent verifies that Inject for the Hermes adapter writes
// the SDD orchestrator markdown into ~/.hermes/SOUL.md via markdown-section injection,
// and that a second Inject call converges to Changed=false (idempotent).

func TestInjectClaudeWritesSectionMarkers(t *testing.T) {
	home := t.TempDir()

	result, err := Inject(home, claudeAdapter(), "")
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("Inject() first changed = false")
	}

	path := filepath.Join(home, ".claude", "CLAUDE.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	text := string(content)

	if !strings.Contains(text, "<!-- gentle-ai:sdd-orchestrator -->") {
		t.Fatal("CLAUDE.md missing open marker for sdd-orchestrator")
	}
	if !strings.Contains(text, "<!-- /gentle-ai:sdd-orchestrator -->") {
		t.Fatal("CLAUDE.md missing close marker for sdd-orchestrator")
	}
	if !strings.Contains(text, "sub-agent") {
		t.Fatal("CLAUDE.md missing real SDD orchestrator content (expected 'sub-agent')")
	}
	if !strings.Contains(text, "dependency") {
		t.Fatal("CLAUDE.md missing real SDD orchestrator content (expected 'dependency')")
	}
}

func TestInjectClaudeKeepsHeavySDDWorkflowLazy(t *testing.T) {
	home := t.TempDir()

	_, err := Inject(home, claudeAdapter(), "")
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	promptPath := filepath.Join(home, ".claude", "CLAUDE.md")
	promptContent, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", promptPath, err)
	}
	prompt := string(promptContent)
	for _, heavy := range []string{
		"## SDD Workflow (Spec-Driven Development)",
		"### Automatic Mode Gatekeeper (MANDATORY)",
		"### Native SDD Dispatcher Guard",
	} {
		if strings.Contains(prompt, heavy) {
			t.Fatalf("CLAUDE.md eagerly includes heavy SDD workflow detail %q:\n%s", heavy, prompt)
		}
	}
	for _, eager := range []string{
		"### Delegation Rules",
		"#### Mandatory Delegation Triggers",
		"#### Native Checking Contract",
		"#### Cost and Context Balance",
		"~/.claude/skills/_shared/sdd-orchestrator-workflow.md",
	} {
		if !strings.Contains(prompt, eager) {
			t.Fatalf("CLAUDE.md missing eager bootstrap %q:\n%s", eager, prompt)
		}
	}

	lazyPath := filepath.Join(home, ".claude", "skills", "_shared", "sdd-orchestrator-workflow.md")
	lazyContent, err := os.ReadFile(lazyPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", lazyPath, err)
	}
	lazy := string(lazyContent)
	for _, want := range []string{
		"## SDD Workflow (Spec-Driven Development)",
		"### Automatic Mode Gatekeeper (MANDATORY)",
		"### Native SDD Dispatcher Guard",
	} {
		if !strings.Contains(lazy, want) {
			t.Fatalf("lazy SDD workflow missing %q:\n%s", want, lazy)
		}
	}
}

func TestInjectClaudePreservesExistingSections(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	existing := "# My Config\n\nSome user content.\n"
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Inject(home, claudeAdapter(), "")
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "Some user content.") {
		t.Fatal("Existing user content was clobbered")
	}
	if !strings.Contains(text, "<!-- gentle-ai:sdd-orchestrator -->") {
		t.Fatal("SDD section was not injected")
	}
}

func TestInjectClaudeIsIdempotent(t *testing.T) {
	home := t.TempDir()

	first, err := Inject(home, claudeAdapter(), "")
	if err != nil {
		t.Fatalf("Inject() first error = %v", err)
	}
	if !first.Changed {
		t.Fatalf("Inject() first changed = false")
	}

	second, err := Inject(home, claudeAdapter(), "")
	if err != nil {
		t.Fatalf("Inject() second error = %v", err)
	}
	if second.Changed {
		t.Fatalf("Inject() second changed = true")
	}
}

func TestInjectClaudeCustomModelAssignments(t *testing.T) {
	home := t.TempDir()

	opts := InjectOptions{ClaudeModelAssignments: map[string]model.ClaudeModelAlias{
		"sdd-design":  model.ClaudeModelSonnet,
		"sdd-propose": model.ClaudeModelFable,
		"default":     model.ClaudeModelHaiku,
	}}

	result, err := Inject(home, claudeAdapter(), "", opts)
	if err != nil {
		t.Fatalf("Inject(claude, custom assignments) error = %v", err)
	}
	if !result.Changed {
		t.Fatal("Inject(claude, custom assignments) changed = false")
	}

	content, err := os.ReadFile(filepath.Join(home, ".claude", "skills", "_shared", "sdd-orchestrator-workflow.md"))
	if err != nil {
		t.Fatalf("ReadFile(sdd-orchestrator-workflow.md) error = %v", err)
	}

	text := string(content)
	if strings.Contains(text, "| orchestrator |") {
		t.Fatal("lazy workflow should not expose orchestrator as a configurable model row")
	}
	for _, want := range []string{
		"| sdd-design | sonnet | default | Architecture decisions |",
		"| sdd-propose | fable | default | Architectural decisions |",
		"| default | haiku | default | SDD/JD phase fallback |",
		"Gentle AI does not configure the main orchestrator model",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("lazy workflow missing custom table row %q", want)
		}
	}

	if !strings.Contains(text, "<!-- gentle-ai:sdd-model-assignments -->") {
		t.Fatal("lazy workflow missing model assignment open marker")
	}
	if !strings.Contains(text, "<!-- /gentle-ai:sdd-model-assignments -->") {
		t.Fatal("lazy workflow missing model assignment close marker")
	}
	for _, want := range []string{
		"Agent tool calls for SDD/Judgment-Day phase agents MUST include `model`",
		"Generic/non-SDD delegation MUST NOT use this table",
		"omit `model` unless the user explicitly requested an override",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("lazy workflow missing scoped model gate text %q", want)
		}
	}
	for _, forbidden := range []string{
		"Every Agent tool call MUST include `model`",
		"for general/non-SDD delegation use `default`",
		"Non-SDD general delegation",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("lazy workflow contains legacy generic delegation model routing text %q", forbidden)
		}
	}
}

func TestInjectClaudeCustomModelAssignmentsIsIdempotent(t *testing.T) {
	home := t.TempDir()
	opts := InjectOptions{ClaudeModelAssignments: map[string]model.ClaudeModelAlias{
		"sdd-design": model.ClaudeModelSonnet,
	}}

	first, err := Inject(home, claudeAdapter(), "", opts)
	if err != nil {
		t.Fatalf("Inject() first error = %v", err)
	}
	if !first.Changed {
		t.Fatal("Inject() first changed = false")
	}

	second, err := Inject(home, claudeAdapter(), "", opts)
	if err != nil {
		t.Fatalf("Inject() second error = %v", err)
	}
	if second.Changed {
		t.Fatal("Inject() second changed = true")
	}
}

// A preserved prompt that carries the retired prompt-owned lens router is
// upgraded in place to native route/status/transition authority.

// Vocabulary from the retired work-routing contracts. A preserved prompt is a
// user-visible artifact, so none of it may survive a migration: the commands it
// names no longer exist and would send the orchestrator after dead authority.
var retiredWorkRoutingTokens = []string{
	"work-capabilities",
	"work-start",
	"work-advance",
	"work-route",
	"work-status",
	"work-transition",
	"work-reconcile",
	"work-verification-decide",
	"WorkRun",
	"authorizedTransition",
}

// The exact rule 7 a previous install wrote into every preserved OpenCode and
// Kilocode orchestrator prompt.
const retiredWorkRoutingAuthorityRule = "7. **Authority rule**: when a WorkRun exists, request `gentle-ai.work-status/v1`" +
	" and apply only its exact provider-issued `gentle-ai.work-transition/v1` authorization." +
	" Never select lenses, synthesize transitions, or infer PASS from prose."

// The replacement rule 7 retains one current-worktree transaction binding and
// never lets compatibility gates decide delivery.
const nativeReviewAuthorityRuleText = "7. **Authority rule**: use selectorless `gentle-ai review status` only to preflight the current worktree" +
	" and execute its exact START; retain that transaction's lineage, revision, and target for every later lifecycle call." +
	" Gates are informational only. Never select lenses, synthesize transitions, infer PASS, or authorize delivery from prose."

// previouslyInstalledDelegationHardGates reproduces, byte for byte, the managed
// block that shipped before this migration, including the retired rule 7.
func previouslyInstalledDelegationHardGates(userHead string) string {
	return userHead + "\n\n" +
		"<!-- gentle-ai:delegation-hard-gates-migration -->\n" +
		"### Mandatory Delegation Triggers (Non-Skippable)\n\n" +
		"These routing boundaries are fully mandatory. They protect context quality without making SDD the universal implementation workflow.\n\n" +
		"Semantic guard: **delegate** means using OpenCode's native Task tool to invoke a configured sub-agent. Running local scripts, Python, or Bash inline is execution, not delegation.\n\n" +
		"Do not pass these rules to child agents as permission to spawn more agents; children receive concrete role work and must not orchestrate.\n\n" +
		"1. **Bounded read rule**: read 1–3 files inline to decide or verify.\n" +
		"2. **4-file rule**: if understanding requires 4+ files, delegate one narrow exploration/mapping task.\n" +
		"3. **Write rule**: keep one mechanical, already-understood file inline; delegate one writer for 2+ non-trivial files.\n" +
		"4. **Context rule**: delegate reading that prepares a write and broad research.\n" +
		"5. **Optional SDD rule**: propose SDD only when durable proposal/spec/design/tasks materially reduce substantial ambiguity. Select it only after explicit request or accepted proposal.\n" +
		"6. **Per-action rule**: tests, builds, installs, and native review actors may use fresh workers without changing the implementation route or creating SDD state.\n" +
		retiredWorkRoutingAuthorityRule + "\n" +
		"<!-- /gentle-ai:delegation-hard-gates-migration -->\n"
}

func TestInjectCursorWritesSDDOrchestratorAndSkills(t *testing.T) {
	home := t.TempDir()

	cursorAdapter, err := agents.NewAdapter("cursor")
	if err != nil {
		t.Fatalf("NewAdapter(cursor) error = %v", err)
	}

	result, injectErr := Inject(home, cursorAdapter, "")
	if injectErr != nil {
		t.Fatalf("Inject(cursor) error = %v", injectErr)
	}

	if !result.Changed {
		t.Fatal("Inject(cursor) changed = false")
	}

	// Should have SDD skill files AND the system prompt file.
	if len(result.Files) == 0 {
		t.Fatal("Inject(cursor) returned no files")
	}

	// Verify SDD orchestrator was injected into the system prompt file.
	promptPath := filepath.Join(home, ".cursor", "rules", "gentle-ai.mdc")
	content, readErr := os.ReadFile(promptPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%q) error = %v", promptPath, readErr)
	}

	text := string(content)
	if !strings.Contains(text, "Spec-Driven Development") {
		t.Fatal("Cursor system prompt missing SDD orchestrator content")
	}
	if !strings.Contains(text, "sub-agent") {
		t.Fatal("Cursor system prompt missing SDD sub-agent references")
	}
}

func TestInjectAntigravityPreservesSharedGeminiPrompt(t *testing.T) {
	home := t.TempDir()
	adapter, err := agents.NewAdapter("antigravity")
	if err != nil {
		t.Fatalf("NewAdapter(antigravity) error = %v", err)
	}

	promptPath := adapter.SystemPromptFile(home)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(prompt dir) error = %v", err)
	}
	const existing = "# Existing Antigravity rules\n\nKeep this user-authored guidance exactly as written.\n"
	if err := os.WriteFile(promptPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile(prompt) error = %v", err)
	}

	result, err := Inject(home, adapter, "")
	if err != nil {
		t.Fatalf("Inject(antigravity) error = %v", err)
	}
	if !result.Changed {
		t.Fatal("Inject(antigravity) changed = false")
	}

	content, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", promptPath, err)
	}
	text := string(content)
	if !strings.Contains(text, existing) {
		t.Fatalf("existing Antigravity prompt was not preserved; got:\n%s", text)
	}
	if !strings.Contains(text, "Spec-Driven Development") {
		t.Fatalf("Antigravity prompt missing SDD orchestrator content; got:\n%s", text)
	}
}

func TestInjectFileAppendSkipsIfAlreadyPresent(t *testing.T) {
	home := t.TempDir()

	cursorAdapter, err := agents.NewAdapter("cursor")
	if err != nil {
		t.Fatalf("NewAdapter(cursor) error = %v", err)
	}

	// First injection.
	first, firstErr := Inject(home, cursorAdapter, "")
	if firstErr != nil {
		t.Fatalf("Inject() first error = %v", firstErr)
	}
	if !first.Changed {
		t.Fatal("first Inject() changed = false")
	}

	// Second injection — SDD content is already there, should not duplicate.
	second, secondErr := Inject(home, cursorAdapter, "")
	if secondErr != nil {
		t.Fatalf("Inject() second error = %v", secondErr)
	}
	if second.Changed {
		t.Fatal("second Inject() changed = true — SDD orchestrator was duplicated")
	}
}

func TestInjectFileAppendMigratesLegacyHeading(t *testing.T) {
	home := t.TempDir()

	cursorAdapter, err := agents.NewAdapter("cursor")
	if err != nil {
		t.Fatalf("NewAdapter(cursor) error = %v", err)
	}

	promptPath := cursorAdapter.SystemPromptFile(home)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	existing := "# Existing\n\n## Spec-Driven Development (SDD) Orchestrator\nAlready present.\n"
	if err := os.WriteFile(promptPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, injectErr := Inject(home, cursorAdapter, "")
	if injectErr != nil {
		t.Fatalf("Inject() error = %v", injectErr)
	}
	if len(result.Files) == 0 {
		t.Fatal("Inject() returned no files")
	}

	content, readErr := os.ReadFile(promptPath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}

	text := string(content)
	if strings.Contains(text, "Already present.") {
		t.Fatal("legacy SDD orchestrator content survived after migration")
	}
	if !strings.Contains(text, "<!-- gentle-ai:sdd-orchestrator -->") {
		t.Fatal("missing open marker after migration")
	}
	if !strings.Contains(text, "<!-- /gentle-ai:sdd-orchestrator -->") {
		t.Fatal("missing close marker after migration")
	}
	if strings.Count(text, "## Agent Teams Orchestrator") != 1 {
		t.Fatal("agent teams heading duplicated after migration")
	}
	if !strings.Contains(text, "## Skills to load before work") {
		t.Fatal("SDD orchestrator was not refreshed to current skill-path loading format")
	}
}

func TestInjectFileAppendMigratesFullLegacyOrchestratorBlock(t *testing.T) {
	home := t.TempDir()

	cursorAdapter, err := agents.NewAdapter("cursor")
	if err != nil {
		t.Fatalf("NewAdapter(cursor) error = %v", err)
	}

	promptPath := cursorAdapter.SystemPromptFile(home)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	existing := "## Rules\n\nLegacy intro.\n\n" +
		"## Agent Teams Orchestrator\n\n" +
		"### Result Contract\n" +
		"Each phase returns: `status`, `executive_summary`, `artifacts`, `next_recommended`, `risks`.\n\n" +
		"### Sub-Agent Launch Pattern\n\n" +
		"SKILL: Load `{skill-path}` before starting.\n\n" +
		"<!-- gentle-ai:engram-protocol -->\n" +
		"## Engram Persistent Memory - Protocol\n" +
		"<!-- /gentle-ai:engram-protocol -->\n"

	if err := os.WriteFile(promptPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, injectErr := Inject(home, cursorAdapter, "")
	if injectErr != nil {
		t.Fatalf("Inject() error = %v", injectErr)
	}
	if len(result.Files) == 0 {
		t.Fatal("Inject() returned no files")
	}

	content, readErr := os.ReadFile(promptPath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}

	text := string(content)
	if strings.Contains(text, "SKILL: Load `{skill-path}` before starting.") {
		t.Fatal("legacy sub-agent launch content survived after migration")
	}
	if strings.Count(text, "### Result Contract") != 1 {
		t.Fatal("result contract section duplicated after migration")
	}
	if !strings.Contains(text, "`skill_resolution`") {
		t.Fatal("result contract was not refreshed to current format")
	}
	if !strings.Contains(text, "## Skills to load before work") {
		t.Fatal("current skill-path launch pattern missing after migration")
	}
	if strings.Count(text, "<!-- gentle-ai:engram-protocol -->") != 1 {
		t.Fatal("engram protocol marker should be preserved exactly once")
	}
}

func TestInjectFileAppendRemovesLegacyBlockWhenMarkedSectionAlreadyExists(t *testing.T) {
	home := t.TempDir()

	cursorAdapter, err := agents.NewAdapter("cursor")
	if err != nil {
		t.Fatalf("NewAdapter(cursor) error = %v", err)
	}

	promptPath := cursorAdapter.SystemPromptFile(home)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	canonical := assets.MustRead("generic/sdd-orchestrator.md")
	existing := "## Agent Teams Orchestrator\n\nLegacy duplicate block.\n\n" +
		"<!-- gentle-ai:sdd-orchestrator -->\n" + canonical + "\n<!-- /gentle-ai:sdd-orchestrator -->\n"

	if err := os.WriteFile(promptPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, injectErr := Inject(home, cursorAdapter, "")
	if injectErr != nil {
		t.Fatalf("Inject() error = %v", injectErr)
	}

	content, readErr := os.ReadFile(promptPath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}

	text := string(content)
	if strings.Contains(text, "Legacy duplicate block.") {
		t.Fatal("legacy duplicate block survived even with marked section present")
	}
	if strings.Count(text, "## Agent Teams Orchestrator") != 1 {
		t.Fatal("orchestrator heading should exist exactly once after cleanup")
	}
}

func TestInjectMarkdownSections_stripsLegacyATLBlockWithMarkedSection(t *testing.T) {
	home := t.TempDir()

	claudeAdpt := claudeAdapter()
	promptPath := claudeAdpt.SystemPromptFile(home)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	const legacyATLBlock = `<!-- BEGIN:agent-teams-lite -->
## Agent Teams Orchestrator

You are a COORDINATOR, not an executor.

### Delegation Rules (ALWAYS ACTIVE)

| Rule | Instruction |
|------|------------|
| No inline work | Reading/writing code → delegate to sub-agent |
<!-- END:agent-teams-lite -->`

	sddSection := "<!-- gentle-ai:sdd-orchestrator -->\nYou are a COORDINATOR.\n<!-- /gentle-ai:sdd-orchestrator -->\n"
	existing := legacyATLBlock + "\n\n" + sddSection

	if err := os.WriteFile(promptPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, injectErr := Inject(home, claudeAdpt, "")
	if injectErr != nil {
		t.Fatalf("Inject() error = %v", injectErr)
	}

	content, readErr := os.ReadFile(promptPath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}

	text := string(content)

	if strings.Contains(text, "<!-- BEGIN:agent-teams-lite -->") {
		t.Fatal("ATL open marker should have been stripped during inject")
	}
	if strings.Contains(text, "<!-- END:agent-teams-lite -->") {
		t.Fatal("ATL close marker should have been stripped during inject")
	}
	if !strings.Contains(text, "<!-- gentle-ai:sdd-orchestrator -->") {
		t.Fatal("sdd-orchestrator section must be present after ATL strip")
	}
	if !strings.Contains(text, "<!-- /gentle-ai:sdd-orchestrator -->") {
		t.Fatal("sdd-orchestrator close marker must be present after ATL strip")
	}
}

// TestInjectOpenCodeRemovesStaleProfileToolsWithoutExplicitProfiles proves the
// install/reinstall path: no profile is passed in InjectOptions, yet stale
// tools blocks on profile-derived managed agents already present on disk
// (sdd-orchestrator-{name}, {phase}-{name}, {jd}-{name}) are still removed so
// they cannot preserve a sensitive-path read-deny bypass. Agents whose names
// only resemble profile keys but carry an invalid profile-name suffix stay
// user-owned and untouched.

func TestInjectFileAppendSkipsAgentTeamsHeading(t *testing.T) {
	home := t.TempDir()

	cursorAdapter, err := agents.NewAdapter("cursor")
	if err != nil {
		t.Fatalf("NewAdapter(cursor) error = %v", err)
	}

	promptPath := cursorAdapter.SystemPromptFile(home)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	existing := "# Existing\n\n## Agent Teams Orchestrator\nAlready present.\n"
	if err := os.WriteFile(promptPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, injectErr := Inject(home, cursorAdapter, "")
	if injectErr != nil {
		t.Fatalf("Inject() error = %v", injectErr)
	}
	if len(result.Files) == 0 {
		t.Fatal("Inject() returned no files")
	}

	content, readErr := os.ReadFile(promptPath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}

	text := string(content)
	if strings.Count(text, "## Agent Teams Orchestrator") != 1 {
		t.Fatal("agent teams heading duplicated")
	}
}

func TestInjectClaudeDeduplicatesBareOrchestratorSection(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Pre-existing file with a BARE (no HTML markers) Agent Teams Orchestrator section.
	existing := "# My Rules\n\n## Rules\n\nBe excellent.\n\n## Agent Teams Orchestrator\n\nYou are a COORDINATOR.\n\n### Delegation Rules\n\nSome old rules.\n\n## Other Section\n\nOther content.\n"
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := Inject(home, claudeAdapter(), "")
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if len(result.Files) == 0 {
		t.Fatal("Inject() returned no files")
	}

	content, readErr := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}

	text := string(content)

	// Must have exactly ONE "## Agent Teams Orchestrator" heading — no duplication.
	if count := strings.Count(text, "## Agent Teams Orchestrator"); count != 1 {
		t.Fatalf("expected 1 Agent Teams Orchestrator heading, got %d\n\ncontent:\n%s", count, text)
	}

	// The injected marked version must be present.
	if !strings.Contains(text, "<!-- gentle-ai:sdd-orchestrator -->") {
		t.Fatal("missing open marker after injection")
	}
	if !strings.Contains(text, "<!-- /gentle-ai:sdd-orchestrator -->") {
		t.Fatal("missing close marker after injection")
	}

	// Content outside the orchestrator section must be preserved.
	if !strings.Contains(text, "Be excellent.") {
		t.Fatal("user content outside orchestrator section was lost")
	}
	if !strings.Contains(text, "## Other Section") {
		t.Fatal("section after orchestrator was lost")
	}
	if !strings.Contains(text, "Other content.") {
		t.Fatal("content after orchestrator section was lost")
	}

	// The old bare content must NOT survive (replaced by the marked version).
	if strings.Contains(text, "Some old rules.") {
		t.Fatal("old bare orchestrator content was not stripped")
	}
}

func TestInjectClaudeDeduplicatesBareOrchestratorAtEndOfFile(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Bare orchestrator section at the END of file (no following ## heading).
	existing := "# My Rules\n\n## Rules\n\nBe excellent.\n\n## Agent Teams Orchestrator\n\nYou are a COORDINATOR, not an executor.\n"
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Inject(home, claudeAdapter(), "")
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	content, readErr := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}

	text := string(content)

	if count := strings.Count(text, "## Agent Teams Orchestrator"); count != 1 {
		t.Fatalf("expected 1 Agent Teams Orchestrator heading, got %d\n\ncontent:\n%s", count, text)
	}
	if !strings.Contains(text, "<!-- gentle-ai:sdd-orchestrator -->") {
		t.Fatal("missing open marker after injection")
	}
	if !strings.Contains(text, "Be excellent.") {
		t.Fatal("user content outside orchestrator section was lost")
	}
}

// TestInjectOpenCodeMultiModeJDAgentsExcludedFromRootModel verifies that JD
// agents are NOT injected with the root model when no explicit assignment
// exists, even though SDD agents do receive root model propagation.
// This preserves model diversity — JD agents inherit the runtime default
// instead of being coupled to the root model.

// TestInjectOpenCodeMultiModePreservesExistingAgentModels verifies that
// a pre-existing agent definition with an explicit model is not overwritten
// by the root model, while a NEW agent (not yet in the user's config) gets
// the root model as a default.

// TestInjectOpenCodeMultiModeExistingAgentWithNoModelIsNotTouched verifies
// that a pre-existing agent WITHOUT a model field is respected — the root model
// is NOT injected for that agent. The user intentionally set up the agent
// without a model (they may rely on per-project overrides or session context).

// ---------------------------------------------------------------------------
// Fix 1: shared SDD support files written to disk
// ---------------------------------------------------------------------------

// TestInjectWritesAllSharedFilesToDisk verifies that all _shared
// convention files (including SDD phase/status contracts) are
// actually written to the agent's skills/_shared/ directory during Inject().
// This is a disk-level test; assets_test.go only checks the embedded FS.

// TestInjectSharedDirCreatedWithAllFiles verifies that Inject() creates the
// _shared directory when it does not exist and writes all shared files into it.

func TestInjectSkillDirectoryRemovesLegacySharedSkillMarker(t *testing.T) {
	skillDir := filepath.Join(t.TempDir(), "compatibility-root")
	sharedDir := filepath.Join(skillDir, "_shared")
	legacyMarker := filepath.Join(sharedDir, "SKILL.md")

	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", sharedDir, err)
	}
	if err := os.WriteFile(legacyMarker, []byte("legacy generated shared skill marker\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", legacyMarker, err)
	}

	result, err := InjectSkillDirectory(skillDir, "")
	if err != nil {
		t.Fatalf("InjectSkillDirectory() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("InjectSkillDirectory() changed = false, want legacy marker cleanup to report a change")
	}
	if _, err := os.Stat(legacyMarker); !os.IsNotExist(err) {
		t.Fatalf("legacy shared marker %q still exists or could not be checked: %v", legacyMarker, err)
	}

	readmePath := filepath.Join(sharedDir, "README.md")
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", readmePath, err)
	}
	if string(readme) != assets.MustRead("skills/_shared/README.md") {
		t.Fatalf("README.md = %q, want current embedded support-directory documentation", readme)
	}

	for _, name := range mustSharedSkillFileNames(t) {
		path := filepath.Join(sharedDir, filepath.FromSlash(name))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("shared reference %q was not preserved: %v", path, err)
		}
	}
}

func TestInjectSkillDirectoryRefusesToRemoveNonRegularLegacySharedMarker(t *testing.T) {
	skillDir := filepath.Join(t.TempDir(), "compatibility-root")
	legacyMarker := filepath.Join(skillDir, "_shared", "SKILL.md")

	if err := os.MkdirAll(legacyMarker, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", legacyMarker, err)
	}

	_, err := InjectSkillDirectory(skillDir, "")
	if err == nil {
		t.Fatal("InjectSkillDirectory() error = nil, want refusal to remove a non-regular legacy marker")
	}
	info, statErr := os.Stat(legacyMarker)
	if statErr != nil {
		t.Fatalf("Stat(%q) error = %v", legacyMarker, statErr)
	}
	if !info.IsDir() {
		t.Fatalf("legacy marker mode = %v, want directory preserved after refusal", info.Mode())
	}
}

func mustSharedSkillFileNames(t *testing.T) []string {
	t.Helper()

	names, err := assets.SharedSkillFileNames()
	if err != nil {
		t.Fatalf("SharedSkillFileNames() error = %v", err)
	}
	return names
}

// ---------------------------------------------------------------------------
// Fix 2: orchestrator dedup — stripBareOrchestratorSection unit tests
// ---------------------------------------------------------------------------

// TestStripBareOrchestratorSection_BareAtBeginning verifies that a bare
// orchestrator section that appears BEFORE any other content is stripped.
func TestStripBareOrchestratorSection_BareAtBeginning(t *testing.T) {
	input := "## Agent Teams Orchestrator\n\nYou are a COORDINATOR.\n\n## Other Section\n\nSome content.\n"
	result := stripBareOrchestratorSection(input)

	if strings.Contains(result, "You are a COORDINATOR.") {
		t.Fatal("bare orchestrator at beginning was not stripped")
	}
	if !strings.Contains(result, "## Other Section") {
		t.Fatal("content after bare orchestrator was lost")
	}
	if !strings.Contains(result, "Some content.") {
		t.Fatal("content after bare orchestrator section was lost")
	}
}

// TestStripBareOrchestratorSection_OnlyOrchestratorContent verifies that a
// file containing ONLY the bare orchestrator section (no surrounding content)
// is reduced to an empty string (or just a newline).
func TestStripBareOrchestratorSection_OnlyOrchestratorContent(t *testing.T) {
	input := "## Agent Teams Orchestrator\n\nYou are a COORDINATOR, not an executor.\n"
	result := stripBareOrchestratorSection(input)

	if strings.Contains(result, "COORDINATOR") {
		t.Fatalf("solo bare orchestrator section was not stripped: %q", result)
	}
}

// TestStripBareOrchestratorSection_PreservesBeforeAndAfter verifies that
// stripBareOrchestratorSection keeps content both BEFORE and AFTER the section.
func TestStripBareOrchestratorSection_PreservesBeforeAndAfter(t *testing.T) {
	input := "# My Rules\n\n## Rules\n\nBe excellent.\n\n## Agent Teams Orchestrator\n\nYou are a COORDINATOR.\n\n### Delegation Rules\n\nOld rules.\n\n## Other Section\n\nOther content.\n"
	result := stripBareOrchestratorSection(input)

	if strings.Contains(result, "You are a COORDINATOR.") {
		t.Fatal("bare orchestrator content was not removed")
	}
	if strings.Contains(result, "Old rules.") {
		t.Fatal("orchestrator sub-content was not removed")
	}
	if !strings.Contains(result, "Be excellent.") {
		t.Fatal("content BEFORE bare orchestrator was lost")
	}
	if !strings.Contains(result, "## Other Section") {
		t.Fatal("heading AFTER bare orchestrator was lost")
	}
	if !strings.Contains(result, "Other content.") {
		t.Fatal("content AFTER bare orchestrator was lost")
	}
}

// TestStripBareOrchestratorSection_NoOpWhenNoSection verifies that a file
// without any orchestrator heading is returned unchanged.
func TestStripBareOrchestratorSection_NoOpWhenNoSection(t *testing.T) {
	input := "# My Rules\n\n## Rules\n\nBe excellent.\n"
	result := stripBareOrchestratorSection(input)

	if result != input {
		t.Fatalf("no-op case mutated content:\ngot:  %q\nwant: %q", result, input)
	}
}

// TestStripBareOrchestratorSection_DoesNotStripIfMarkersPresent verifies that
// a section that already has HTML comment markers is NOT stripped by
// stripBareOrchestratorSection (the markers are handled by InjectMarkdownSection).
// This ensures the migration guard in injectMarkdownSections() is correct.
func TestStripBareOrchestratorSection_DoesNotStripIfMarkersPresent(t *testing.T) {
	input := "# My Rules\n\n<!-- gentle-ai:sdd-orchestrator -->\n## Agent Teams Orchestrator\n\nYou are a COORDINATOR.\n<!-- /gentle-ai:sdd-orchestrator -->\n"

	// The function sees "## Agent Teams Orchestrator" and would normally strip it.
	// But the caller (injectMarkdownSections) is supposed to check for markers
	// first and skip the strip call. This test documents what happens if
	// stripBareOrchestratorSection is called on already-marked content:
	// the heading will be removed, which is WRONG — this validates the guard.
	result := stripBareOrchestratorSection(input)

	// Because stripBareOrchestratorSection does not check for markers itself,
	// calling it on marked content would damage the file. The real protection is
	// the `!strings.Contains(existing, "<!-- gentle-ai:sdd-orchestrator -->")` guard
	// in injectMarkdownSections(). This test confirms that guard works end-to-end.
	_ = result
}

// ---------------------------------------------------------------------------
// Task 6: StrictTDD marker injected into system prompt files
// ---------------------------------------------------------------------------

// TestInjectStrictTDDEnabledInjectsMarkerIntoClaude verifies that when
// InjectOptions.StrictTDD = true, the injected content in CLAUDE.md contains
// the <!-- gentle-ai:strict-tdd-mode --> marker with its content.
func TestInjectStrictTDDEnabledInjectsMarkerIntoClaude(t *testing.T) {
	home := t.TempDir()

	opts := InjectOptions{StrictTDD: true}
	result, err := Inject(home, claudeAdapter(), "", opts)
	if err != nil {
		t.Fatalf("Inject(claude, StrictTDD=true) error = %v", err)
	}
	if !result.Changed {
		t.Fatal("Inject() changed = false")
	}

	content, err := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatalf("ReadFile(CLAUDE.md) error = %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "<!-- gentle-ai:strict-tdd-mode -->") {
		t.Fatal("CLAUDE.md missing <!-- gentle-ai:strict-tdd-mode --> open marker")
	}
	if !strings.Contains(text, "<!-- /gentle-ai:strict-tdd-mode -->") {
		t.Fatal("CLAUDE.md missing <!-- /gentle-ai:strict-tdd-mode --> close marker")
	}
	if !strings.Contains(text, "Strict TDD Mode: enabled") {
		t.Fatal("CLAUDE.md missing 'Strict TDD Mode: enabled' content")
	}
}

// TestInjectStrictTDDDisabledDoesNotInjectMarker verifies that when
// InjectOptions.StrictTDD = false (default), the strict-tdd marker is NOT injected.
func TestInjectStrictTDDDisabledDoesNotInjectMarker(t *testing.T) {
	home := t.TempDir()

	// Default (no opts) — strict TDD disabled.
	_, err := Inject(home, claudeAdapter(), "")
	if err != nil {
		t.Fatalf("Inject(claude, default) error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatalf("ReadFile(CLAUDE.md) error = %v", err)
	}

	text := string(content)
	if strings.Contains(text, "<!-- gentle-ai:strict-tdd-mode -->") {
		t.Fatal("CLAUDE.md should NOT contain strict-tdd-mode marker when StrictTDD=false")
	}
}

// TestInjectStrictTDDIsIdempotent verifies that injecting with StrictTDD=true
// twice does not duplicate the marker.
func TestInjectStrictTDDIsIdempotent(t *testing.T) {
	home := t.TempDir()

	opts := InjectOptions{StrictTDD: true}

	first, err := Inject(home, claudeAdapter(), "", opts)
	if err != nil {
		t.Fatalf("Inject() first error = %v", err)
	}
	if !first.Changed {
		t.Fatal("first Inject() changed = false")
	}

	second, err := Inject(home, claudeAdapter(), "", opts)
	if err != nil {
		t.Fatalf("Inject() second error = %v", err)
	}
	if second.Changed {
		t.Fatal("second Inject() changed = true — strict-tdd marker was duplicated")
	}
}

// ---------------------------------------------------------------------------
// Task 1: All files from each skill directory are copied (not just SKILL.md)
// ---------------------------------------------------------------------------

// TestInjectCopiesAllFilesFromSkillDirectory verifies that Inject() copies
// ALL .md files from each skill directory, not just SKILL.md.
// Specifically, sdd-apply/strict-tdd.md and sdd-verify/strict-tdd-verify.md
// must be written to disk alongside their SKILL.md files.

func assertNonEmptyFile(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected file %q: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected file %q to be non-empty", path)
	}
}

// TestInjectCopiesAllFilesReportedInResult verifies that all skill files
// (including extra files beyond SKILL.md) are included in result.Files.

// TestInjectClaudeDeduplicatesBareOrchestratorAtBeginning verifies that a bare
// orchestrator section at the very START of CLAUDE.md is handled correctly.
func TestInjectClaudeDeduplicatesBareOrchestratorAtBeginning(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Bare orchestrator at the very start, followed by other content.
	existing := "## Agent Teams Orchestrator\n\nYou are a COORDINATOR.\n\n## Other Rules\n\nBe excellent.\n"
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Inject(home, claudeAdapter(), "")
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	content, readErr := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	text := string(content)

	if count := strings.Count(text, "## Agent Teams Orchestrator"); count != 1 {
		t.Fatalf("expected 1 Agent Teams Orchestrator heading, got %d\n\ncontent:\n%s", count, text)
	}
	if !strings.Contains(text, "<!-- gentle-ai:sdd-orchestrator -->") {
		t.Fatal("missing open marker after injection")
	}
	if !strings.Contains(text, "## Other Rules") {
		t.Fatal("content after bare orchestrator was lost")
	}
	if !strings.Contains(text, "Be excellent.") {
		t.Fatal("content after bare orchestrator section was lost")
	}
}

// TestInjectClaudeDeduplicatesFileWithOnlyBareOrchestrator verifies that a
// CLAUDE.md containing ONLY the bare orchestrator (no other sections) is
// correctly replaced with the marker-based version.
func TestInjectClaudeDeduplicatesFileWithOnlyBareOrchestrator(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Use a unique phrase that does NOT appear in the canonical orchestrator
	// asset so we can confirm the bare version was stripped.
	existing := "## Agent Teams Orchestrator\n\nYou are a COORDINATOR.\n\n### Delegation Rules\n\nLEGACY-RULE-MARKER-XYZ\n"
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Inject(home, claudeAdapter(), "")
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	content, readErr := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	text := string(content)

	// Should have exactly one orchestrator heading (the injected one).
	if count := strings.Count(text, "## Agent Teams Orchestrator"); count != 1 {
		t.Fatalf("expected 1 Agent Teams Orchestrator heading, got %d\n\ncontent:\n%s", count, text)
	}
	// Must have markers.
	if !strings.Contains(text, "<!-- gentle-ai:sdd-orchestrator -->") {
		t.Fatal("missing open marker")
	}
	if !strings.Contains(text, "<!-- /gentle-ai:sdd-orchestrator -->") {
		t.Fatal("missing close marker")
	}
	// The unique legacy phrase must be gone — the bare section was stripped.
	if strings.Contains(text, "LEGACY-RULE-MARKER-XYZ") {
		t.Fatal("old bare orchestrator content (unique marker) survived after injection")
	}
}

// TestInjectClaudeDeduplicatesBareOrchestratorIsIdempotent verifies that
// running Inject() TWICE on a file that started with a bare orchestrator
// section produces exactly one orchestrator section (no accumulation).
func TestInjectClaudeDeduplicatesBareOrchestratorIsIdempotent(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Start from bare state.
	existing := "# My Rules\n\n## Agent Teams Orchestrator\n\nYou are a COORDINATOR.\n"
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// First inject — strips bare, inserts marked section.
	if _, err := Inject(home, claudeAdapter(), ""); err != nil {
		t.Fatalf("Inject() first error = %v", err)
	}

	// Second inject — must be a no-op (already has markers).
	second, err := Inject(home, claudeAdapter(), "")
	if err != nil {
		t.Fatalf("Inject() second error = %v", err)
	}
	if second.Changed {
		t.Fatal("second Inject() changed = true — idempotency broken after dedup migration")
	}

	content, readErr := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	text := string(content)

	if count := strings.Count(text, "## Agent Teams Orchestrator"); count != 1 {
		t.Fatalf("expected 1 Agent Teams Orchestrator heading after 2 injects, got %d\n\ncontent:\n%s", count, text)
	}
}

// TestInjectClaudeDoesNotStripMarkedSection verifies that an existing
// CLAUDE.md with a properly-marked orchestrator section is NOT stripped and
// re-written as bare content (the migration guard must only fire when markers
// are absent).
func TestInjectClaudeDoesNotStripMarkedSection(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Pre-inject once to produce the canonical marked state.
	if _, err := Inject(home, claudeAdapter(), ""); err != nil {
		t.Fatalf("first Inject() error = %v", err)
	}

	// Read and verify markers.
	after1, err := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(after1), "<!-- gentle-ai:sdd-orchestrator -->") {
		t.Fatal("markers not present after first inject — test precondition failed")
	}

	// Second inject — must not change the file.
	second, err := Inject(home, claudeAdapter(), "")
	if err != nil {
		t.Fatalf("second Inject() error = %v", err)
	}
	if second.Changed {
		t.Fatal("second Inject() changed = true — marked section was incorrectly re-processed")
	}
}

// ---------------------------------------------------------------------------
// OpenCode plugin tests
// ---------------------------------------------------------------------------

// TestInjectModelAssignments_ReasoningEffortInjected verifies that when an
// assignment has a non-empty Effort, the "variant" key is written into
// the agent map alongside "model".

// TestInjectModelAssignments_EmptyEffortSetsEmptyVariant verifies that when
// Effort is empty, the "variant" key is set to "" so the deep merge overwrites
// any pre-existing variant in the user's config.

// TestInjectModelAssignments_StaleVariantOverwritten verifies that when switching
// from a reasoning model to a non-reasoning model (Effort=""), a pre-existing
// "variant" key in the overlay is overwritten with "".

// TestInjectModelAssignments_RootModelFallbackClearsVariant verifies that
// case 3 (rootModelID fallback — no TUI assignment, agent absent from user
// config, root model set) writes variant:"" alongside the model. Mirrors the
// case 1 contract so case 3 cannot leak a stale variant from the overlay
// through to the user's settings file. See PR #440 review.

// ---------------------------------------------------------------------------
// Windsurf workflow injection tests
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Agent-specific SDD orchestrator asset selection tests
// ---------------------------------------------------------------------------

// TestSDDOrchestratorAssetSelection verifies that sddOrchestratorAsset()
// returns agent-specific paths for agents that have dedicated orchestrators,
// and falls back to generic for all others.

// TestInjectCodexWritesSDDOrchestratorAndSkills verifies that Codex injection
// creates agents.md with the SDD orchestrator and writes skill files.
func TestInjectCodexWritesSDDOrchestratorAndSkills(t *testing.T) {
	home := t.TempDir()

	codexAdapter, err := agents.NewAdapter("codex")
	if err != nil {
		t.Fatalf("NewAdapter(codex) error = %v", err)
	}

	result, injectErr := Inject(home, codexAdapter, "")
	if injectErr != nil {
		t.Fatalf("Inject(codex) error = %v", injectErr)
	}
	if !result.Changed {
		t.Fatal("Inject(codex) changed = false")
	}

	// Verify SDD orchestrator was injected into AGENTS.md.
	promptPath := filepath.Join(home, ".codex", "AGENTS.md")
	content, readErr := os.ReadFile(promptPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%q) error = %v", promptPath, readErr)
	}

	text := string(content)
	if !strings.Contains(text, "Spec-Driven Development") {
		t.Fatal("agents.md missing SDD orchestrator content")
	}

	// Codex-specific asset must reference Codex skill paths.
	if !strings.Contains(text, "~/.codex/skills/_shared/") {
		t.Fatal("agents.md missing ~/.codex/skills/_shared/ path — agent-specific asset not used")
	}

	// Codex-specific asset must NOT reference Gemini paths.
	if strings.Contains(text, "~/.gemini/") {
		t.Fatal("agents.md contains Gemini-specific paths — wrong asset was injected")
	}

	for _, want := range []string{
		"`spawn_agent`",
		"`wait_agent(timeout_ms=<bounded timeout>)`",
		"`list_agents()`",
		"`send_message`",
		"`followup_task`",
		"`interrupt_agent`",
		"Completed or idle agents remain reusable",
		"Repeat `wait_agent(timeout_ms=<bounded timeout>)` and `list_agents()` until the target agent reaches a terminal state.",
		"If the target reaches a non-success terminal state, stop and surface its final output or status",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("agents.md missing Codex multi-agent v2 lifecycle fragment %q", want)
		}
	}
	for _, stale := range []string{
		"`close_agent`",
		"`send_input`",
		"`wait_agent(task_name=",
	} {
		if strings.Contains(text, stale) {
			t.Errorf("agents.md retained legacy Codex multi-agent lifecycle fragment %q", stale)
		}
	}

	// Should also write SDD skill files.
	skillPath := filepath.Join(home, ".codex", "skills", "sdd-init", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected SDD skill file %q: %v", skillPath, err)
	}

	// Codex requires YAML frontmatter to start at byte 0. Section extraction must
	// not leave a leading newline before the frontmatter delimiter.
	extractedSkillPath := filepath.Join(home, ".codex", "skills", "sdd-apply", "SKILL.md")
	extractedSkill, err := os.ReadFile(extractedSkillPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", extractedSkillPath, err)
	}
	if !strings.HasPrefix(string(extractedSkill), "---\n") {
		t.Fatalf("Codex SDD skill must start with YAML frontmatter delimiter, got prefix %q", string(extractedSkill[:min(len(extractedSkill), 16)]))
	}

	// Shared files should also be written.
	sharedPath := filepath.Join(home, ".codex", "skills", "_shared", "engram-convention.md")
	if _, err := os.Stat(sharedPath); err != nil {
		t.Fatalf("expected shared SDD convention file %q: %v", sharedPath, err)
	}
}

// TestInjectCodexIsIdempotent verifies that injecting Codex twice does not
// duplicate the SDD orchestrator content.
func TestInjectCodexIsIdempotent(t *testing.T) {
	home := t.TempDir()

	codexAdapter, err := agents.NewAdapter("codex")
	if err != nil {
		t.Fatalf("NewAdapter(codex) error = %v", err)
	}

	first, err := Inject(home, codexAdapter, "")
	if err != nil {
		t.Fatalf("Inject(codex) first error = %v", err)
	}
	if !first.Changed {
		t.Fatal("first Inject(codex) changed = false")
	}

	second, err := Inject(home, codexAdapter, "")
	if err != nil {
		t.Fatalf("Inject(codex) second error = %v", err)
	}
	if second.Changed {
		t.Fatal("second Inject(codex) changed = true — SDD orchestrator was duplicated")
	}
}

// ---------------------------------------------------------------------------
// Regression: post-check must validate in-memory merged bytes, not re-read disk
// (Windows/WSL2 atomic-write visibility bug — "missing sdd-apply sub-agent")
// ---------------------------------------------------------------------------

// TestInjectOpenCodeMultiModeWithPreExistingMinimalConfig reproduces the
// Windows/WSL2 regression where a pre-existing minimal opencode.json (e.g.
// only {"model": "anthropic/..."}) caused the post-check to fail with:
//
//	post-check: .../opencode.json missing sdd-apply sub-agent
//
// The root cause was re-reading the file from disk after the atomic rename,
// which could see stale content on Windows/WSL2. The fix validates against
// the in-memory merged bytes returned by the production merge helper instead.

// TestInjectOpenCodeMultiModeWithPreExistingFullConfig verifies that a
// pre-existing opencode.json with a non-trivial structure (multiple keys,
// provider settings, etc.) is correctly merged with the multi-mode overlay
// and passes the post-check without any disk re-read race.

// ---------------------------------------------------------------------------
// gentle-orchestrator agent model assignment from SDD coordinator selection
// ---------------------------------------------------------------------------

// TestInjectOpenCodeMultiModeAssignsGentleOrchestratorModelFromLegacyOrchestratorKey
// verifies that historical TUI assignments keyed by sdd-orchestrator are
// migrated to the current gentle-orchestrator base coordinator.

// TestInjectOpenCodeMultiModeInstallsGentleOrchestratorWithModel verifies that the base
// SDD overlay owns the gentle-orchestrator coordinator.

// TestMergeOpenCodeCompatibleJSONFileReturnsMergedBytes verifies that the
// production merge path returns merged bytes in-memory, so callers never need
// to re-read from disk to validate the result (the Windows/WSL2 post-check fix).

// ---------------------------------------------------------------------------
// Fix 1: Cursor sub-agent files written to disk
// ---------------------------------------------------------------------------

func TestInjectCursorWritesSubAgentFiles(t *testing.T) {
	home := t.TempDir()

	cursorAdapter, err := agents.NewAdapter("cursor")
	if err != nil {
		t.Fatalf("NewAdapter(cursor) error = %v", err)
	}

	promptPath := cursorAdapter.SystemPromptFile(home)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	result, injectErr := Inject(home, cursorAdapter, "")
	if injectErr != nil {
		t.Fatalf("Inject() error = %v", injectErr)
	}

	agentsDir := filepath.Join(home, ".cursor", "agents")
	phases := []string{"sdd-init", "sdd-explore", "sdd-propose", "sdd-spec", "sdd-design", "sdd-tasks", "sdd-apply", "sdd-verify", "sdd-archive", "review-risk", "review-readability", "review-reliability", "review-resilience"}

	for _, phase := range phases {
		agentPath := filepath.Join(agentsDir, phase+".md")
		info, err := os.Stat(agentPath)
		if err != nil {
			t.Fatalf("agent file %s not found: %v", phase, err)
		}
		if info.Size() < 100 {
			t.Fatalf("agent file %s too small: %d bytes", phase, info.Size())
		}
	}

	// Verify readonly flags: sdd-explore and sdd-verify must use readonly: false
	// so they can use terminal commands and MCP tools (issue #156).
	for _, phase := range []string{"sdd-explore", "sdd-verify"} {
		content, err := os.ReadFile(filepath.Join(agentsDir, phase+".md"))
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", phase, err)
		}
		if !strings.Contains(string(content), "readonly: false") {
			t.Fatalf("agent %s should have readonly: false (terminal/MCP access required)", phase)
		}
	}

	// Verify result.Files includes agent paths
	hasAgentFile := false
	for _, f := range result.Files {
		// Normalize for Windows paths
		if strings.Contains(strings.ReplaceAll(f, `\`, `/`), ".cursor/agents/") {
			hasAgentFile = true
			break
		}
	}
	if !hasAgentFile {
		t.Fatal("result.Files should include at least one cursor agent path")
	}

	// Idempotency: second run should not change files
	result2, err := Inject(home, cursorAdapter, "")
	if err != nil {
		t.Fatalf("second Inject() error = %v", err)
	}
	for _, f := range result2.Files {
		if strings.Contains(f, ".cursor/agents/") {
			t.Fatalf("second inject should not report changed agent files, but got %s", f)
		}
	}
}

func mustAdapter(t *testing.T, id model.AgentID) agents.Adapter {
	t.Helper()
	adapter, err := agents.NewAdapter(id)
	if err != nil {
		t.Fatalf("NewAdapter(%s) error = %v", id, err)
	}
	return adapter
}

func assertNativeAgentFile(t *testing.T, path string, contains string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if len(content) < 50 {
		t.Fatalf("native agent file %q is suspiciously short: %d bytes", path, len(content))
	}
	if !strings.Contains(string(content), contains) {
		t.Fatalf("native agent file %q missing %q", path, contains)
	}
}

// TestInjectKiroFallsBackToClaudeModelAssignmentsWhenKiroMapUnset verifies that
// when KiroModelAssignments is nil, the injector falls back to ClaudeModelAssignments
// for Kiro phase model resolution (legacy backward-compatible path).

// TestInjectKiroModelAssignmentsTakePrecedenceOverClaude verifies that when
// both KiroModelAssignments and ClaudeModelAssignments are provided,
// KiroModelAssignments wins for Kiro subagent file generation.

// ---------------------------------------------------------------------------
// Fix 2: findProjectRoot — monorepo and enhanced workspace root detection
// ---------------------------------------------------------------------------

// TestFindProjectRootPnpmMonorepo verifies that when the starting directory
// has a package.json but a parent has pnpm-workspace.yaml, the function
// returns the monorepo root (parent), not the sub-package directory.

// TestFindProjectRootNxMonorepo verifies that nx.json is recognized as a
// monorepo root marker.

// TestFindProjectRootTurboMonorepo verifies that turbo.json is recognized as
// a monorepo root marker.

// TestFindProjectRootGitTakesPrecedence verifies that a .git directory at a
// higher level takes precedence over a package.json in a subdirectory.

// TestFindProjectRootPackageJsonFallback verifies that when only package.json
// exists (no .git, go.mod, or monorepo markers), it is returned as the best
// candidate root.

// TestFindProjectRootEmptyDirReturnsNotFound verifies that an empty directory
// (no markers at all) returns false.

// TestFindProjectRootEmptyStringReturnsNotFound verifies the early-return for
// empty dir input.

// TestFindProjectRootDeepNested verifies that findProjectRoot handles deeply
// nested directories without panicking or infinite looping, and that it
// correctly returns ("", false) when the marker is beyond maxAncestorDepth.

// TestFindProjectRootMultiplePackageJsonPicksHighest verifies that when
// multiple package.json files exist in ancestor directories, findProjectRoot
// returns the highest ancestor (closest to filesystem root), not the first
// (closest to starting dir).

// TestFindProjectRootAllMarkers verifies that each project marker (beyond .git,
// go.mod, and package.json) is correctly recognized as a project root.

// ---------------------------------------------------------------------------
// Fix: SDD post-check disk fallback on Windows
// ---------------------------------------------------------------------------

// TestInjectOpenCodePostCheckDiskFallback tests that the SDD post-check
// correctly falls back to reading from disk when the in-memory merged bytes
// are stale or empty. This simulates the Windows scenario where os.ReadFile
// returns stale data due to NTFS caching, but the file on disk is correct.

// TestInjectOpenCodeWithProfile_PostCheckVerifiesOrchestrator verifies that
// when a named profile is injected, the post-check confirms sdd-orchestrator-{name}
// is present in the merged opencode.json.

// TestInjectOpenCodeWithProfile_DefaultProfileSkipped verifies that the default
// profile (Name="" or Name="default") is skipped in the profile injection loop.

// TestInjectOpenCodeWithTwoProfiles_BothOrchestratorsPresent verifies that
// two named profiles both get their orchestrators injected and verified.

// TestInjectClaudeSubAgentsResolveModels verifies that when SDD is injected
// for the Claude adapter, the embedded sub-agent files are copied to
// ~/.claude/agents/ and the {{CLAUDE_MODEL}} placeholder is substituted per
// phase using opts.ClaudeModelAssignments.
func TestInjectClaudeSubAgentsResolveModels(t *testing.T) {
	home := t.TempDir()

	assignments := map[string]model.ClaudeModelAlias{
		"sdd-design":  model.ClaudeModelOpus,
		"sdd-propose": model.ClaudeModelFable,
		"sdd-archive": model.ClaudeModelHaiku,
		"default":     model.ClaudeModelSonnet,
	}

	result, err := Inject(home, claudeAdapter(), "", InjectOptions{ClaudeModelAssignments: assignments})
	if err != nil {
		t.Fatalf("Inject(claude, custom assignments) error = %v", err)
	}
	if !result.Changed {
		t.Fatal("Inject(claude, custom assignments) changed = false")
	}

	tests := []struct {
		phase string
		want  string
	}{
		{phase: "sdd-design", want: "model: opus"},
		{phase: "sdd-propose", want: "model: fable"},
		{phase: "sdd-archive", want: "model: haiku"},
		{phase: "sdd-spec", want: "model: sonnet"},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			path := filepath.Join(home, ".claude", "agents", tt.phase+".md")
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("ReadFile(%s) error = %v", tt.phase, readErr)
			}
			text := string(content)
			if strings.Contains(text, "{{CLAUDE_MODEL}}") {
				t.Fatalf("agent %s still contains unresolved {{CLAUDE_MODEL}} placeholder", tt.phase)
			}
			if !strings.Contains(text, tt.want) {
				t.Fatalf("agent %s missing %q\n--- file ---\n%s", tt.phase, tt.want, text)
			}
		})
	}
}

func TestInjectClaudeSubAgentsRenderConfiguredEffort(t *testing.T) {
	home := t.TempDir()

	assignments := map[string]model.ClaudePhaseAssignment{
		"sdd-design":  {Model: model.ClaudeModelOpus, Effort: model.ClaudeEffortXHigh},
		"sdd-propose": {Model: model.ClaudeModelFable, Effort: model.ClaudeEffortMax},
		"sdd-spec":    {Model: model.ClaudeModelSonnet, Effort: model.ClaudeEffortMax},
		"sdd-archive": {Model: model.ClaudeModelHaiku, Effort: model.ClaudeEffortHigh},
	}

	_, err := Inject(home, claudeAdapter(), "", InjectOptions{ClaudePhaseAssignments: assignments})
	if err != nil {
		t.Fatalf("Inject(claude, model+effort assignments) error = %v", err)
	}

	checks := []struct {
		phase      string
		wantModel  string
		wantEffort string
		denyEffort bool
	}{
		{phase: "sdd-design", wantModel: "model: opus", wantEffort: "effort: xhigh"},
		{phase: "sdd-propose", wantModel: "model: fable", wantEffort: "effort: max"},
		{phase: "sdd-spec", wantModel: "model: sonnet", wantEffort: "effort: max"},
		// Haiku is not listed as effort-compatible in the official Claude Code matrix.
		{phase: "sdd-archive", wantModel: "model: haiku", denyEffort: true},
	}

	for _, tt := range checks {
		t.Run(tt.phase, func(t *testing.T) {
			path := filepath.Join(home, ".claude", "agents", tt.phase+".md")
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("ReadFile(%s) error = %v", tt.phase, readErr)
			}
			text := string(content)
			if !strings.Contains(text, tt.wantModel) {
				t.Fatalf("agent %s missing %q\n--- file ---\n%s", tt.phase, tt.wantModel, text)
			}
			if tt.denyEffort {
				if strings.Contains(text, "effort:") {
					t.Fatalf("agent %s should omit unsupported/default effort\n--- file ---\n%s", tt.phase, text)
				}
				return
			}
			if !strings.Contains(text, tt.wantEffort) {
				t.Fatalf("agent %s missing %q\n--- file ---\n%s", tt.phase, tt.wantEffort, text)
			}
		})
	}
}

func TestInjectClaudeSubAgentsDefaultEffortOmitted(t *testing.T) {
	home := t.TempDir()

	assignments := map[string]model.ClaudePhaseAssignment{
		"sdd-design": {Model: model.ClaudeModelOpus, Effort: model.ClaudeEffortDefault},
	}

	_, err := Inject(home, claudeAdapter(), "", InjectOptions{ClaudePhaseAssignments: assignments})
	if err != nil {
		t.Fatalf("Inject(claude, default effort) error = %v", err)
	}

	path := filepath.Join(home, ".claude", "agents", "sdd-design.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(sdd-design) error = %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "model: opus") {
		t.Fatalf("agent missing model: opus\n--- file ---\n%s", text)
	}
	if strings.Contains(text, "effort:") {
		t.Fatalf("agent should omit default effort\n--- file ---\n%s", text)
	}
	if strings.Contains(text, "{{CLAUDE_EFFORT_FRONTMATTER}}") {
		t.Fatalf("agent still contains unresolved effort placeholder\n--- file ---\n%s", text)
	}
}

func TestInjectClaudeSubAgentsUseBalancedDefaultsWhenAssignmentsUnset(t *testing.T) {
	home := t.TempDir()

	result, err := Inject(home, claudeAdapter(), "")
	if err != nil {
		t.Fatalf("Inject(claude, default assignments) error = %v", err)
	}
	if !result.Changed {
		t.Fatal("Inject(claude, default assignments) changed = false")
	}

	tests := []struct {
		phase string
		want  string
	}{
		{phase: "sdd-design", want: "model: opus"},
		{phase: "sdd-spec", want: "model: sonnet"},
		{phase: "sdd-archive", want: "model: haiku"},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			path := filepath.Join(home, ".claude", "agents", tt.phase+".md")
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("ReadFile(%s) error = %v", tt.phase, readErr)
			}
			if !strings.Contains(string(content), tt.want) {
				t.Fatalf("agent %s missing balanced default %q\n--- file ---\n%s", tt.phase, tt.want, string(content))
			}
		})
	}
}

func TestInjectClaudeSubAgentsIgnoreInvalidAliases(t *testing.T) {
	home := t.TempDir()

	assignments := map[string]model.ClaudeModelAlias{
		"sdd-design":  model.ClaudeModelAlias("claude-opus-4-1"),
		"sdd-archive": model.ClaudeModelAlias("bad-value"),
		"default":     model.ClaudeModelHaiku,
	}

	_, err := Inject(home, claudeAdapter(), "", InjectOptions{ClaudeModelAssignments: assignments})
	if err != nil {
		t.Fatalf("Inject(claude, invalid aliases) error = %v", err)
	}

	checks := []struct {
		phase string
		want  string
	}{
		{phase: "sdd-design", want: "model: opus"},
		{phase: "sdd-archive", want: "model: haiku"},
		{phase: "sdd-spec", want: "model: sonnet"},
	}

	for _, tt := range checks {
		path := filepath.Join(home, ".claude", "agents", tt.phase+".md")
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("ReadFile(%s) error = %v", tt.phase, readErr)
		}
		text := string(content)
		if !strings.Contains(text, tt.want) {
			t.Fatalf("agent %s missing sanitized model %q\n--- file ---\n%s", tt.phase, tt.want, text)
		}
		if strings.Contains(text, "bad-value") || strings.Contains(text, "claude-opus-4-1") {
			t.Fatalf("agent %s contains invalid alias in frontmatter\n--- file ---\n%s", tt.phase, text)
		}
	}
}

// TestInjectClaudeSubAgentsScopedTools verifies that each generated Claude
// sub-agent carries a scoped tools: frontmatter entry so the phase cannot use
// tools outside its contract (e.g. sdd-explore cannot Edit/Write; no phase
// carries Task so recursion is impossible).
func TestInjectClaudeSubAgentsScopedTools(t *testing.T) {
	home := t.TempDir()

	_, err := Inject(home, claudeAdapter(), "", InjectOptions{ClaudeModelAssignments: model.ClaudeModelPresetBalanced()})
	if err != nil {
		t.Fatalf("Inject(claude, balanced preset) error = %v", err)
	}

	tests := []struct {
		phase       string
		mustContain []string
		mustNotHave []string
	}{
		{
			phase:       "sdd-explore",
			mustContain: []string{"Read", "Grep", "Glob", "WebFetch", "WebSearch", "mcp__plugin_engram_engram__mem_save"},
			mustNotHave: []string{"Edit", "Write", "Bash", "Task"},
		},
		{
			phase:       "sdd-propose",
			mustContain: []string{"Read", "Edit", "Write", "Grep", "Glob", "mcp__plugin_engram_engram__mem_search", "mcp__plugin_engram_engram__mem_get_observation", "mcp__plugin_engram_engram__mem_save"},
			mustNotHave: []string{"Bash", "Task"},
		},
		{
			phase:       "sdd-spec",
			mustContain: []string{"Read", "Edit", "Write", "Grep", "Glob", "mcp__plugin_engram_engram__mem_search", "mcp__plugin_engram_engram__mem_get_observation", "mcp__plugin_engram_engram__mem_save"},
			mustNotHave: []string{"Bash", "Task"},
		},
		{
			phase:       "sdd-design",
			mustContain: []string{"Read", "Edit", "Write", "Grep", "Glob", "mcp__plugin_engram_engram__mem_search", "mcp__plugin_engram_engram__mem_get_observation", "mcp__plugin_engram_engram__mem_save"},
			mustNotHave: []string{"Bash", "Task"},
		},
		{
			phase:       "sdd-tasks",
			mustContain: []string{"Read", "Edit", "Write", "Grep", "Glob", "mcp__plugin_engram_engram__mem_search", "mcp__plugin_engram_engram__mem_get_observation", "mcp__plugin_engram_engram__mem_save"},
			mustNotHave: []string{"Bash", "Task"},
		},
		{
			phase:       "sdd-apply",
			mustContain: []string{"Read", "Edit", "Write", "Bash", "mcp__plugin_engram_engram__mem_search", "mcp__plugin_engram_engram__mem_get_observation", "mcp__plugin_engram_engram__mem_save", "mcp__plugin_engram_engram__mem_update"},
			mustNotHave: []string{"Task"},
		},
		{
			phase:       "sdd-verify",
			mustContain: []string{"Read", "Bash", "mcp__plugin_engram_engram__mem_search", "mcp__plugin_engram_engram__mem_get_observation", "mcp__plugin_engram_engram__mem_save"},
			mustNotHave: []string{"Edit", "Write", "Task"},
		},
		{
			phase:       "sdd-archive",
			mustContain: []string{"Read", "Edit", "Write", "Bash", "mcp__plugin_engram_engram__mem_search", "mcp__plugin_engram_engram__mem_get_observation", "mcp__plugin_engram_engram__mem_save"},
			mustNotHave: []string{"Task"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			path := filepath.Join(home, ".claude", "agents", tt.phase+".md")
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("ReadFile(%s) error = %v", tt.phase, readErr)
			}
			text := string(content)

			toolsLine := ""
			for _, line := range strings.Split(text, "\n") {
				if strings.HasPrefix(line, "tools:") {
					toolsLine = line
					break
				}
			}
			if toolsLine == "" {
				t.Fatalf("agent %s missing tools: frontmatter line\n--- file ---\n%s", tt.phase, text)
			}

			for _, want := range tt.mustContain {
				if !strings.Contains(toolsLine, want) {
					t.Errorf("agent %s tools line %q missing required tool %q", tt.phase, toolsLine, want)
				}
			}
			for _, forbidden := range tt.mustNotHave {
				if strings.Contains(toolsLine, forbidden) {
					t.Errorf("agent %s tools line %q must not grant %q", tt.phase, toolsLine, forbidden)
				}
			}
		})
	}
}

func TestEnsureClaudeSkillRegistryHookAppendsIdempotently(t *testing.T) {
	home := t.TempDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {"type": "command", "command": "echo keep"}
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "matcher": "startup",
        "hooks": [
          {"type": "command", "command": "echo existing"}
        ]
      }
    ]
  }
}`
	if err := os.WriteFile(settingsPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := ensureClaudeSkillRegistryHook(settingsPath)
	if err != nil {
		t.Fatalf("ensureClaudeSkillRegistryHook() error = %v", err)
	}
	if !changed {
		t.Fatal("first call changed = false, want true")
	}
	changed, err = ensureClaudeSkillRegistryHook(settingsPath)
	if err != nil {
		t.Fatalf("second ensureClaudeSkillRegistryHook() error = %v", err)
	}
	if changed {
		t.Fatal("second call changed = true, want false")
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Count(text, "gentle-ai skill-registry refresh") != 1 {
		t.Fatalf("hook command count mismatch:\n%s", text)
	}
	if !strings.Contains(text, "echo keep") || !strings.Contains(text, "echo existing") {
		t.Fatalf("existing hooks not preserved:\n%s", text)
	}
}

func TestEnsureClaudeSkillRegistryHookRejectsMalformedSettings(t *testing.T) {
	home := t.TempDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"permissions":`)
	if err := os.WriteFile(settingsPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := ensureClaudeSkillRegistryHook(settingsPath)
	if err == nil {
		t.Fatal("ensureClaudeSkillRegistryHook() error = nil, want parse error")
	}
	if changed {
		t.Fatal("changed = true, want false")
	}
	after, readErr := os.ReadFile(settingsPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(original) {
		t.Fatalf("malformed settings were modified: %q", after)
	}
}

func TestEnsureClaudeSkillRegistryHookRejectsUnexpectedHookSchema(t *testing.T) {
	home := t.TempDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"hooks":{"UserPromptSubmit":{"bad":true}}}`)
	if err := os.WriteFile(settingsPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := ensureClaudeSkillRegistryHook(settingsPath)
	if err == nil {
		t.Fatal("ensureClaudeSkillRegistryHook() error = nil, want schema error")
	}
	if changed {
		t.Fatal("changed = true, want false")
	}
	after, readErr := os.ReadFile(settingsPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(original) {
		t.Fatalf("settings were modified: %q", after)
	}
}

func TestEnsureCodexSkillRegistryHookWritesSessionStartHookIdempotently(t *testing.T) {
	home := t.TempDir()
	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {"type": "command", "command": "echo keep"}
        ]
      }
    ]
  }
}`
	if err := os.WriteFile(hooksPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := ensureCodexSkillRegistryHook(hooksPath)
	if err != nil {
		t.Fatalf("ensureCodexSkillRegistryHook() error = %v", err)
	}
	if !changed {
		t.Fatal("first call changed = false, want true")
	}
	changed, err = ensureCodexSkillRegistryHook(hooksPath)
	if err != nil {
		t.Fatalf("second ensureCodexSkillRegistryHook() error = %v", err)
	}
	if changed {
		t.Fatal("second call changed = true, want false")
	}

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Count(text, "gentle-ai skill-registry refresh") != 1 {
		t.Fatalf("hook command count mismatch:\n%s", text)
	}
	if !strings.Contains(text, `"SessionStart"`) {
		t.Fatalf("Codex hook should use SessionStart, got:\n%s", text)
	}
	if !strings.Contains(text, `startup|resume|clear|compact`) {
		t.Fatalf("Codex SessionStart hook should cover supported startup sources, got:\n%s", text)
	}
	if !strings.Contains(text, "echo keep") {
		t.Fatalf("existing hooks not preserved:\n%s", text)
	}
}

// ---------------------------------------------------------------------------
// Codex inject tests (T3.1)
// ---------------------------------------------------------------------------

func codexInjectAdapter() agents.Adapter {
	// Import inline to avoid adding to the import block of existing file
	// We use agents.NewAdapter to get the codex adapter.
	a, err := agents.NewAdapter("codex")
	if err != nil {
		panic("agents.NewAdapter(codex): " + err.Error())
	}
	return a
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}

func TestInject_CodexSubstitutesPhaseEfforts(t *testing.T) {
	home := t.TempDir()
	adapter := codexInjectAdapter()

	opts := InjectOptions{
		CodexModelAssignments: model.CodexModelPresetRecommended(),
	}
	result, err := Inject(home, adapter, "", opts)
	if err != nil {
		t.Fatalf("Inject(codex, Recommended) error = %v", err)
	}
	if !result.Changed {
		t.Fatal("Inject(codex, Recommended) changed = false, want true")
	}

	agentsMD, readErr := os.ReadFile(filepath.Join(home, ".codex", "AGENTS.md"))
	if readErr != nil {
		t.Fatalf("ReadFile(AGENTS.md) error = %v", readErr)
	}
	text := string(agentsMD)
	if strings.Contains(text, "{{") {
		t.Errorf("AGENTS.md contains unresolved placeholder '{{' after Inject:\n%s", text)
	}
	// Table should be present.
	if !strings.Contains(text, "sdd-strong") {
		t.Error("AGENTS.md missing sdd-strong tier row in rendered table")
	}
	if !strings.Contains(text, "sdd-mid") {
		t.Error("AGENTS.md missing sdd-mid tier row")
	}
	if !strings.Contains(text, "sdd-cheap") {
		t.Error("AGENTS.md missing sdd-cheap tier row")
	}
}

func TestInject_CodexOrchestratorUsesSkillRegistry(t *testing.T) {
	home := t.TempDir()
	adapter := codexInjectAdapter()

	if _, err := Inject(home, adapter, ""); err != nil {
		t.Fatalf("Inject(codex) error = %v", err)
	}

	agentsMD, readErr := os.ReadFile(filepath.Join(home, ".codex", "AGENTS.md"))
	if readErr != nil {
		t.Fatalf("ReadFile(AGENTS.md) error = %v", readErr)
	}
	text := string(agentsMD)
	for _, want := range []string{
		"Skill Resolver Protocol",
		`mem_search(query: "skill-registry"`,
		".atl/skill-registry.md",
		"## Skills to load before work",
		"skill_resolution",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Codex orchestrator missing skill-registry contract %q:\n%s", want, text)
		}
	}
}

func TestInject_CodexNoAssignmentsUsesRecommended(t *testing.T) {
	home := t.TempDir()
	adapter := codexInjectAdapter()

	// No CodexModelAssignments → should use Recommended preset as fallback.
	result, err := Inject(home, adapter, "")
	if err != nil {
		t.Fatalf("Inject(codex, nil opts) error = %v", err)
	}
	if !result.Changed {
		t.Fatal("Inject(codex, nil opts) changed = false")
	}

	agentsMD, readErr := os.ReadFile(filepath.Join(home, ".codex", "AGENTS.md"))
	if readErr != nil {
		t.Fatalf("ReadFile(AGENTS.md) error = %v", readErr)
	}
	text := string(agentsMD)
	if strings.Contains(text, "{{") {
		t.Errorf("AGENTS.md contains unresolved '{{' with nil assignments:\n%s", text)
	}
}

func TestInject_CodexInstallsSkillRegistryHook(t *testing.T) {
	home := t.TempDir()
	adapter := codexInjectAdapter()

	result, err := Inject(home, adapter, "")
	if err != nil {
		t.Fatalf("Inject(codex) error = %v", err)
	}

	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	if _, err := os.Stat(hooksPath); err != nil {
		t.Fatalf("Codex hooks.json not installed: %v", err)
	}
	if !containsPath(result.Files, hooksPath) {
		t.Fatalf("result.Files missing Codex hooks path %q: %v", hooksPath, result.Files)
	}
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "gentle-ai skill-registry refresh") {
		t.Fatalf("Codex hooks.json missing skill-registry refresh:\n%s", data)
	}
}

func TestInject_CodexIdempotent(t *testing.T) {
	home := t.TempDir()
	adapter := codexInjectAdapter()
	opts := InjectOptions{
		CodexModelAssignments: model.CodexModelPresetRecommended(),
	}

	_, err := Inject(home, adapter, "", opts)
	if err != nil {
		t.Fatalf("first Inject(codex) error = %v", err)
	}
	second, err := Inject(home, adapter, "", opts)
	if err != nil {
		t.Fatalf("second Inject(codex) error = %v", err)
	}
	if second.Changed {
		t.Error("second Inject(codex) Changed = true, want false (idempotent)")
	}
}

// TestInject_CodexPerPhaseModelAssignments covers inject.go:1585 — the
// CodexPhaseModelAssignments branch. When InjectOptions.CodexPhaseModelAssignments
// is non-empty, the injected AGENTS.md must contain the per-phase table
// (| Phase | Model | reasoning_effort |) with the custom model in the correct
// phase row. When empty (carril/preset path), it must use the per-carril table.
func TestInject_CodexPerPhaseModelAssignments_InjectsPerPhaseTable(t *testing.T) {
	home := t.TempDir()
	adapter := codexInjectAdapter()

	// Custom per-phase: sdd-propose gets gpt-5.4, while unassigned phases
	// preserve the selected/saved carril models instead of reverting to the
	// Recommended preset.
	opts := InjectOptions{
		CodexModelAssignments: model.CodexModelPresetRecommended(),
		CodexPhaseModelAssignments: map[string]string{
			"sdd-propose": "gpt-5.4",
		},
		CodexCarrilModelAssignments: map[string]string{
			"sdd-strong": "gpt-5.4-mini",
			"sdd-mid":    "gpt-5.5",
			"sdd-cheap":  "gpt-5.3-codex",
		},
	}

	result, err := Inject(home, adapter, "", opts)
	if err != nil {
		t.Fatalf("Inject(codex, per-phase opts) error = %v", err)
	}
	if !result.Changed {
		t.Fatal("Inject(codex, per-phase opts) changed = false, want true")
	}

	agentsMD, readErr := os.ReadFile(filepath.Join(home, ".codex", "AGENTS.md"))
	if readErr != nil {
		t.Fatalf("ReadFile(AGENTS.md) error = %v", readErr)
	}
	text := string(agentsMD)

	// The per-phase table header must be present (not the per-carril header).
	if !strings.Contains(text, "| Phase | Model |") {
		t.Error("AGENTS.md missing per-phase table header '| Phase | Model |'")
	}
	// The per-carril profile rows must NOT be present when per-phase mode is active.
	if strings.Contains(text, "| `sdd-strong`") {
		t.Error("AGENTS.md contains per-carril row '| `sdd-strong`' but per-phase mode is active")
	}
	// The custom model must appear in the sdd-propose row.
	wantRow := "| `sdd-propose` | `gpt-5.4` |"
	if !strings.Contains(text, wantRow) {
		t.Errorf("AGENTS.md missing expected sdd-propose row %q:\n%s", wantRow, text)
	}
	// An unassigned strong phase must preserve the supplied carril model.
	wantFallbackRow := "| `sdd-design` | `gpt-5.4-mini` | `medium` |"
	if !strings.Contains(text, wantFallbackRow) {
		t.Errorf("AGENTS.md missing preserved carril fallback row %q:\n%s", wantFallbackRow, text)
	}
	// The delegation example must refer readers back to the generated table,
	// not hardcode values that are wrong for some presets.
	if strings.Contains(text, `model="gpt-5.6-sol", reasoning_effort="xhigh"`) {
		t.Errorf("AGENTS.md contains a hardcoded delegation example that can contradict the generated table:\n%s", text)
	}
	if !strings.Contains(text, `model="<assigned-model>"`) || !strings.Contains(text, `reasoning_effort="<assigned-effort>"`) {
		t.Errorf("AGENTS.md delegation example must use assigned-value placeholders:\n%s", text)
	}
	// No unresolved template placeholders.
	if strings.Contains(text, "{{") {
		t.Errorf("AGENTS.md contains unresolved placeholder '{{' after Inject:\n%s", text)
	}

	second, err := Inject(home, adapter, "", opts)
	if err != nil {
		t.Fatalf("second Inject(codex, per-phase opts) error = %v", err)
	}
	if second.Changed {
		t.Fatal("second Inject(codex, per-phase opts) changed = true, want false")
	}
	afterAgentsMD, readErr := os.ReadFile(filepath.Join(home, ".codex", "AGENTS.md"))
	if readErr != nil {
		t.Fatalf("ReadFile(AGENTS.md) after second inject error = %v", readErr)
	}
	if !bytes.Equal(afterAgentsMD, agentsMD) {
		t.Fatal("AGENTS.md changed after idempotent per-phase Codex inject")
	}
}

// TestInject_CodexNilPhaseModelAssignments_UsesCarrilTable verifies that when
// CodexPhaseModelAssignments is empty/nil, the carril-level table is rendered.
func TestInject_CodexNilPhaseModelAssignments_UsesCarrilTable(t *testing.T) {
	home := t.TempDir()
	adapter := codexInjectAdapter()

	// No per-phase assignments → preset/carril path.
	opts := InjectOptions{
		CodexModelAssignments: model.CodexModelPresetRecommended(),
	}

	result, err := Inject(home, adapter, "", opts)
	if err != nil {
		t.Fatalf("Inject(codex, carril opts) error = %v", err)
	}
	if !result.Changed {
		t.Fatal("Inject(codex, carril opts) changed = false, want true")
	}

	agentsMD, readErr := os.ReadFile(filepath.Join(home, ".codex", "AGENTS.md"))
	if readErr != nil {
		t.Fatalf("ReadFile(AGENTS.md) error = %v", readErr)
	}
	text := string(agentsMD)

	// The per-carril profile rows must be present.
	for _, carril := range []string{"sdd-strong", "sdd-mid", "sdd-cheap"} {
		needle := "| `" + carril + "`"
		if !strings.Contains(text, needle) {
			t.Errorf("AGENTS.md missing carril row %q in per-carril mode", needle)
		}
	}
	// The per-phase table header must NOT be present.
	if strings.Contains(text, "| Phase | Model |") {
		t.Error("AGENTS.md contains per-phase table header '| Phase | Model |' but carril mode is active")
	}
}

func TestInject_NonCodexAdapterUnaffected(t *testing.T) {
	// Retained non-Codex adapters must not be affected by CodexModelAssignments.
	adapters := []struct {
		name    string
		adapter agents.Adapter
	}{
		{"cursor", func() agents.Adapter {
			a, _ := agents.NewAdapter("cursor")
			return a
		}()},
		{"antigravity", func() agents.Adapter {
			a, _ := agents.NewAdapter("antigravity")
			return a
		}()},
	}

	for _, tc := range adapters {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			opts := InjectOptions{
				CodexModelAssignments: model.CodexModelPresetRecommended(),
			}
			result, err := Inject(home, tc.adapter, "", opts)
			if err != nil {
				t.Fatalf("Inject(%s) error = %v", tc.name, err)
			}
			if !result.Changed {
				t.Fatalf("Inject(%s) changed = false", tc.name)
			}
			// Non-codex adapters must produce no unresolved placeholders.
			for _, f := range result.Files {
				data, readErr := os.ReadFile(f)
				if readErr != nil {
					continue
				}
				if strings.Contains(string(data), "{{CODEX_PHASE_EFFORTS}}") {
					t.Errorf("%s adapter file %q contains unresolved {{CODEX_PHASE_EFFORTS}}", tc.name, f)
				}
			}
		})
	}
}

// ─── WU-3 RED: InjectOptions.CodexCarrilModelAssignments threading ───────────

// TestInjectCodexWithCarrilModels verifies that InjectOptions.CodexCarrilModelAssignments
// is threaded into the rendered AGENTS.md Model column.
func TestInjectCodexWithCarrilModels(t *testing.T) {
	home := t.TempDir()
	adapter := codexInjectAdapter()

	carrilModels := map[string]string{
		"sdd-strong": "gpt-5.5",
		"sdd-mid":    "gpt-5.5",
		"sdd-cheap":  "gpt-5.4-mini",
	}

	_, err := Inject(home, adapter, "", InjectOptions{
		CodexModelAssignments:       model.CodexModelPresetRecommended(),
		CodexCarrilModelAssignments: carrilModels,
	})
	if err != nil {
		t.Fatalf("Inject(codex, carrilModels) error = %v", err)
	}

	agentsMD, readErr := os.ReadFile(filepath.Join(home, ".codex", "AGENTS.md"))
	if readErr != nil {
		t.Fatalf("ReadFile(AGENTS.md) error = %v", readErr)
	}
	text := string(agentsMD)

	// Table must have a Model column.
	if !strings.Contains(text, "Model") {
		t.Error("AGENTS.md missing Model column in phase-efforts table")
	}
	// gpt-5.5 and gpt-5.4-mini must appear in the table.
	if !strings.Contains(text, "gpt-5.5") {
		t.Error("AGENTS.md missing gpt-5.5 in phase-efforts table")
	}
	if !strings.Contains(text, "gpt-5.4-mini") {
		t.Error("AGENTS.md missing gpt-5.4-mini in phase-efforts table")
	}
}

// TestInjectCodexNilCarrilModels verifies that nil CodexCarrilModelAssignments
// causes the render to use canonical GPT-5.6 defaults.
func TestInjectCodexNilCarrilModels(t *testing.T) {
	home := t.TempDir()
	adapter := codexInjectAdapter()

	_, err := Inject(home, adapter, "", InjectOptions{
		CodexModelAssignments: model.CodexModelPresetRecommended(),
		// CodexCarrilModelAssignments intentionally nil
	})
	if err != nil {
		t.Fatalf("Inject(codex, nil carrilModels) error = %v", err)
	}

	agentsMD, readErr := os.ReadFile(filepath.Join(home, ".codex", "AGENTS.md"))
	if readErr != nil {
		t.Fatalf("ReadFile(AGENTS.md) error = %v", readErr)
	}
	text := string(agentsMD)

	if !strings.Contains(text, "Model") {
		t.Error("AGENTS.md missing Model column — nil carrilModels should fall back to defaults")
	}
	for _, want := range []string{"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna"} {
		if !strings.Contains(text, want) {
			t.Errorf("AGENTS.md missing %s — nil carrilModels should show GPT-5.6 defaults", want)
		}
	}
}

// TestInjectNonCodexAdapterCarrilUnaffected verifies that non-Codex adapters
// are completely unaffected by the new CodexCarrilModelAssignments field.
func TestInjectNonCodexAdapterCarrilUnaffected(t *testing.T) {
	home := t.TempDir()
	// Use Claude adapter — it must not attempt to resolve carril models.
	_, err := Inject(home, claudeAdapter(), "", InjectOptions{
		CodexCarrilModelAssignments: map[string]string{
			"sdd-strong": "gpt-5.5",
		},
	})
	if err != nil {
		t.Fatalf("Inject(claude, carrilModels) should not error; got: %v", err)
	}
}

// TestRemoveLegacyOpenCodePlainChatPreflightLinesPreservesUserContentAfterGroup
// is the regression for the group-scoped legacy scrub: the retired `D. Review`
// preflight group must be removed together with its D1/D2/D3 legacy options, but
// the scrub must NOT spill past the group boundary into later user-authored
// content. A blank line ends the group, and a later line whose text merely
// mentions "D. Review" (for example "See D. Review notes below") must survive.

// TestRemoveLegacyOpenCodePlainChatPreflightLinesGroupBoundarySemantics
// triangulates the scrub boundary: only the retired D1/D2/D3 option lines that
// directly follow the exact legacy group header are removed, everything else
// survives, and the group ends at the first blank or non-option line.

// TestRefreshInstalledOpenCodePluginsSkipsSymlinksAndDirectories pins the
// Lstat safety property of RefreshInstalledOpenCodePlugins: only regular
// files at managed plugin paths are refreshed. A symlink must be skipped
// without following it (the user-owned target must stay untouched) and a
// directory at a plugin path must be left alone.

// TestInjectRefreshesStaleArchiveSkillWithFinalStateAuthority proves the
// freshness path for the sdd-archive instruction fix end to end: an installed
// config carrying pre-Final-State-Authority skill text must actually receive
// the new contract on the next inject (community issue #1882 was reported from
// a mixed install whose managed skills lagged the binary).
func TestInjectRefreshesStaleArchiveSkillWithFinalStateAuthority(t *testing.T) {
	home := t.TempDir()

	skillPath := filepath.Join(home, ".claude", "skills", "sdd-archive", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	stale := "---\nname: sdd-archive\n---\n\n## Purpose\n\nOld archive skill text without the snapshot contract.\n"
	if err := os.WriteFile(skillPath, []byte(stale), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := Inject(home, claudeAdapter(), "")
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("Inject() over a stale archive skill reported no change")
	}

	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", skillPath, err)
	}
	text := string(content)
	for _, want := range []string{
		"## Final-State Authority",
		"`apply-progress` and `verify-report` are intermediate snapshots",
		"outranks intermediate snapshots",
		"Do NOT echo the stale claim",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("installed sdd-archive skill still stale — missing %q", want)
		}
	}

	workflowPath := filepath.Join(home, ".claude", "skills", "_shared", "sdd-orchestrator-workflow.md")
	workflowContent, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", workflowPath, err)
	}
	if !strings.Contains(string(workflowContent), "### Archive Final-State Handoff (MANDATORY)") {
		t.Fatal("installed lazy SDD workflow missing archive final-state handoff section")
	}
}
