package sdd

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gentleman-programming/gentle-ai/v2/internal/assets"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

// requiredLedgerClauses is the OpenCode binding of the shared clause set: the
// only consumer is the preserved OpenCode orchestrator prompt.
var requiredLedgerClauses = boundedReviewRequiredClausesFor(model.AgentOpenCode)

const requiredOrchestratorMergeModeClause = "Native Compact Review Orchestration"

func TestDedicatedReviewAndJudgmentAssetsRenderRoleContracts(t *testing.T) {
	assetsByFamily := map[string][]string{
		"claude": {
			"claude/agents/review-risk.md", "claude/agents/review-readability.md",
			"claude/agents/review-reliability.md", "claude/agents/review-resilience.md",
			"claude/agents/jd-judge-a.md", "claude/agents/jd-judge-b.md",
		},
		"cursor": {
			"cursor/agents/review-risk.md", "cursor/agents/review-readability.md",
			"cursor/agents/review-reliability.md", "cursor/agents/review-resilience.md",
		},
	}
	for family, paths := range assetsByFamily {
		for _, path := range paths {
			t.Run(family+"/"+path, func(t *testing.T) {
				content := renderBoundedReviewAsset(agentForAssetPath(t, path), path)
				assertTextContainsClauses(t, path, content, []string{"candidate", "BLOCKER", "CRITICAL", "causal", "proof"})
				if !strings.Contains(content, "read-only") && !strings.Contains(content, "Never edit") {
					t.Errorf("%s does not state its non-mutating role", path)
				}
				assertNoReviewerLifecycleInstructions(t, path, content)
			})
		}
	}
}

func TestDedicatedReviewersAndRefutersAreStructurallyReadOnly(t *testing.T) {
	for _, path := range []string{
		"claude/agents/review-risk.md", "claude/agents/review-readability.md",
		"claude/agents/review-reliability.md", "claude/agents/review-resilience.md",
	} {
		frontmatter := markdownFrontmatter(t, path)
		if !strings.Contains(frontmatter, "tools: []") {
			t.Errorf("%s grants live reviewer tools: %s", path, frontmatter)
		}
		if strings.Contains(frontmatter, "Bash") {
			t.Errorf("%s grants unrestricted Bash without a per-command policy", path)
		}
		for _, forbidden := range []string{"Write", "Edit"} {
			if strings.Contains(frontmatter, forbidden) {
				t.Errorf("%s frontmatter grants %s", path, forbidden)
			}
		}
	}
	if frontmatter := markdownFrontmatter(t, "claude/agents/review-refuter.md"); strings.Contains(frontmatter, "Bash") || strings.Contains(frontmatter, "Write") || strings.Contains(frontmatter, "Edit") {
		t.Errorf("Claude refuter grants an execution or mutation tool: %s", frontmatter)
	}
	for _, path := range []string{
		"cursor/agents/review-risk.md", "cursor/agents/review-readability.md",
		"cursor/agents/review-reliability.md", "cursor/agents/review-resilience.md",
		"cursor/agents/review-refuter.md",
	} {
		if frontmatter := markdownFrontmatter(t, path); !strings.Contains(frontmatter, "readonly: true") {
			t.Errorf("%s is not read-only", path)
		}
	}
	for _, path := range []string{"claude/agents/review-refuter.md", "cursor/agents/review-refuter.md"} {
		assertNoReviewerLifecycleInstructions(t, path, renderBoundedReviewAsset(agentForAssetPath(t, path), path))
	}
}

func assertOpenCodeTargetedValidator(t *testing.T, label string, agents map[string]any) {
	t.Helper()

	orchestrator, ok := agents["gentle-orchestrator"].(map[string]any)
	if !ok {
		t.Fatalf("%s missing gentle-orchestrator", label)
	}
	permission, ok := orchestrator["permission"].(map[string]any)
	if !ok {
		t.Fatalf("%s gentle-orchestrator permission = %#v, want object", label, orchestrator["permission"])
	}
	task, ok := permission["task"].(map[string]any)
	if !ok {
		t.Fatalf("%s gentle-orchestrator permission.task = %#v, want object", label, permission["task"])
	}
	allowlist, ok := task["__replace__"].(map[string]any)
	if !ok || allowlist["review-validator"] != "allow" {
		t.Fatalf("%s gentle-orchestrator does not allow task review-validator: %#v", label, task)
	}

	validator, ok := agents["review-validator"].(map[string]any)
	if !ok {
		t.Fatalf("%s missing review-validator subagent", label)
	}
	if validator["mode"] != "subagent" || validator["hidden"] != true {
		t.Fatalf("%s review-validator visibility = mode:%#v hidden:%#v, want hidden subagent", label, validator["mode"], validator["hidden"])
	}
	prompt, ok := validator["prompt"].(string)
	if !ok {
		t.Fatalf("%s review-validator prompt = %#v, want string", label, validator["prompt"])
	}
	for _, required := range []string{"provider-issued targeted validation", "Do NOT edit", "Do NOT delegate", "exact requested JSON"} {
		if !strings.Contains(prompt, required) {
			t.Errorf("%s review-validator prompt missing %q: %s", label, required, prompt)
		}
	}

	tools, ok := validator["tools"].(map[string]any)
	if !ok {
		t.Fatalf("%s review-validator tools = %#v, want object", label, validator["tools"])
	}
	wantTools := map[string]bool{"read": true, "bash": true, "write": false, "edit": false, "task": false}
	if len(tools) != len(wantTools) {
		t.Errorf("%s review-validator tool count = %d, want %d: %#v", label, len(tools), len(wantTools), tools)
	}
	for name, want := range wantTools {
		if got, exists := tools[name]; !exists || got != want {
			t.Errorf("%s review-validator tool %q = %#v, want %t", label, name, got, want)
		}
	}
}

// assertOpenCodeProviderInjectedReviewer proves the genuinely restored
// shape: the reviewer prompt names the provider-injected context block
// (never the disabled "unsupported-capability" refusal) and its permission
// map denies bash and edit outright, with no wildcarded allow list — the
// dynamic-binding problem the wildcard existed for cannot exist when there
// is nothing left to allow.
func assertOpenCodeProviderInjectedReviewer(t *testing.T, label string, agent map[string]any) {
	t.Helper()
	prompt, _ := agent["prompt"].(string)
	if strings.Contains(prompt, "unsupported-capability") {
		t.Fatalf("%s prompt still refuses immutable inspection as unsupported: %s", label, prompt)
	}
	if !strings.Contains(prompt, "GENTLE_AI_REVIEW_CONTEXT") || !strings.Contains(prompt, "You have no execution tools") {
		t.Fatalf("%s prompt does not name the provider-injected context block: %s", label, prompt)
	}
	permission, ok := agent["permission"].(map[string]any)
	if !ok || permission["bash"] != "deny" || permission["edit"] != "deny" || len(permission) != 2 {
		t.Fatalf("%s permission = %#v, want bash/edit deny only", label, agent["permission"])
	}
}

func TestReviewerInspectionCommandsReturnIndependentValues(t *testing.T) {
	first := reviewerInspectionCommands()
	second := reviewerInspectionCommands()
	if len(first) == 0 || len(second) != len(first) {
		t.Fatalf("inspection commands = %#v / %#v", first, second)
	}
	first[0] = "mutated"
	if second[0] == "mutated" || reviewerInspectionCommands()[0] == "mutated" {
		t.Fatal("reviewer inspection commands share mutable backing storage")
	}
}

// TestReviewerBashPromptIsNativeAndWindowsPortable pins the shared
// bash-command reviewer prompt (reviewerPrompt) still used by markdown-based
// runtime that keeps its own shell (Cursor). OpenCode uses a provider-injected
// prompt with no Bash permission wildcard: it gets
// openCodeProviderInjectedReviewerPrompt with no bash and no read tool
// instead (see TestOpenCodeOverlaysRenderBoundedReadOnlyReviewRoles).
func TestReviewerBashPromptIsNativeAndWindowsPortable(t *testing.T) {
	prompt, ok := reviewerPrompt("review-reliability")
	if !ok {
		t.Fatal("review-reliability prompt missing")
	}
	for _, forbidden := range []string{"env -i", " git ", "--text", "PowerShell", "cmd /", "Git Bash"} {
		if strings.Contains(prompt, forbidden) {
			t.Errorf("review inspection still depends on %q", forbidden)
		}
	}
	for _, operation := range []string{"name-status", "numstat", "stat", "patch", "object"} {
		if !strings.Contains(prompt, "gentle-ai review inspect-candidate") || !strings.Contains(prompt, "--operation "+operation) {
			t.Errorf("review prompt omits native %s inspection recipe", operation)
		}
	}
}

func measurePromptCost(prompt string) (characters, estimatedTokens int) {
	characters = utf8.RuneCountInString(prompt)
	return characters, characters / 4
}

func markdownFrontmatter(t *testing.T, path string) string {
	t.Helper()
	parts := strings.SplitN(assets.MustRead(path), "---", 3)
	if len(parts) != 3 {
		t.Fatalf("%s missing frontmatter", path)
	}
	return parts[1]
}

func assertOpenCodeReadOnlyTools(t *testing.T, label string, tools map[string]any, read, bash bool) {
	t.Helper()
	want := map[string]bool{"*": false, "read": read, "write": false, "edit": false, "bash": bash, "task": false}
	if len(tools) != len(want) {
		t.Fatalf("%s tools = %#v", label, tools)
	}
	for name, expected := range want {
		if got, ok := tools[name].(bool); !ok || got != expected {
			t.Errorf("%s tool %s = %v, want %v", label, name, tools[name], expected)
		}
	}
}

func assertTextContainsClauses(t *testing.T, label, content string, clauses []string) {
	t.Helper()
	for _, clause := range clauses {
		if !strings.Contains(content, clause) {
			t.Errorf("%s missing required clause %q", label, clause)
		}
	}
}

func assertNoReviewerLifecycleInstructions(t *testing.T, label, content string) {
	t.Helper()
	forbidden := regexp.MustCompile(`(?i)\b(bundle|receipt|mirror|release|gate)s?\b`)
	if match := forbidden.FindString(content); match != "" {
		t.Errorf("%s reviewer prompt contains lifecycle instruction term %q", label, match)
	}
	lower := strings.ToLower(content)
	for _, phrase := range []string{"ordinary 4r", "fix/re-judgment", "launches review-refuter", "review/start", "review-resume", "correction transaction", "scoped validator"} {
		if strings.Contains(lower, phrase) {
			t.Errorf("%s reviewer prompt contains lifecycle routing phrase %q", label, phrase)
		}
	}
}

func readGentleOrchestratorPrompt(t *testing.T, settingsPath string) string {
	t.Helper()
	payload, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(payload, &root); err != nil {
		t.Fatal(err)
	}
	agentsMap := root["agent"].(map[string]any)
	orchestrator := agentsMap["gentle-orchestrator"].(map[string]any)
	return orchestrator["prompt"].(string)
}

func assertOpenCodeRefuterToolsReadOnly(t *testing.T, label string, tools map[string]any) {
	t.Helper()
	assertOpenCodeReadOnlyTools(t, label, tools, true, false)
}
