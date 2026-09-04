package assets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersonalWorkflowNeverRequiresGitHubIssues(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	paths := []string{
		"skills/branch-pr/SKILL.md",
		"skills/gentle-ai-collab-perfect/SKILL.md",
		"skills/issue-root-resolution/SKILL.md",
		"skills/rdd-defect-workflow/SKILL.md",
		"internal/assets/skills/branch-pr/SKILL.md",
		"internal/assets/skills/issue-creation/SKILL.md",
		"internal/assets/skills/rdd-defect-workflow/SKILL.md",
		"internal/assets/antigravity/sdd-orchestrator.md",
		"internal/assets/claude/sdd-orchestrator.md",
		"internal/assets/codex/sdd-orchestrator.md",
		"internal/assets/cursor/sdd-orchestrator.md",
		"internal/assets/generic/sdd-orchestrator.md",
		"CONTRIBUTING.md",
		".github/PULL_REQUEST_TEMPLATE.md",
		".github/workflows/pr-check.yml",
	}
	for _, relative := range paths {
		content, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		lower := strings.ToLower(string(content))
		for _, forbidden := range []string{
			"every pr must link an approved issue",
			"require an approved issue",
			"issue-first is mandatory",
			"issue-first: every pr links",
			"check issue reference",
			"check issue has status:approved",
			"pr is linked to an issue with `status:approved`",
			"report the apparent defect",
			"complete a definitive lookup across open and closed issues",
		} {
			if strings.Contains(lower, forbidden) {
				t.Errorf("%s still contains mandatory issue policy %q", relative, forbidden)
			}
		}
	}
}
