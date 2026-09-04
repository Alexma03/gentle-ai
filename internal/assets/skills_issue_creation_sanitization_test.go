package assets

import (
	"strings"
	"testing"
)

func TestIssueCreationSkillHasSanitizationRule(t *testing.T) {
	content := MustRead("skills/issue-creation/SKILL.md")

	for _, term := range []string{
		"only when the user explicitly requests a GitHub issue operation",
		"Issues are optional tracking artifacts",
		"Sanitize credentials, private paths, hostnames, source contents, and unrelated project details before publishing",
		"Perform one bounded mutation, read it back from GitHub",
		"without blocking unrelated technical work",
	} {
		if !strings.Contains(content, term) {
			t.Errorf("issue-creation skill is missing personal workflow marker %q", term)
		}
	}
	if strings.Contains(content, "status:approved") || strings.Contains(content, "require an approved issue") {
		t.Error("issue-creation skill reintroduced an issue approval gate")
	}
}
