package sdd

import (
	"path/filepath"
	"strings"

	"github.com/gentleman-programming/gentle-ai/v2/internal/assets"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/filemerge"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

const (
	claudeCodeGraphToolGrant = "mcp__codegraph__codegraph_explore"
)

// readSkillContent reads the embedded skill content for the given phase.

func injectCodeGraphGuidanceIntoPrompt(prompt, guidance string) string {
	if strings.TrimSpace(guidance) == "" {
		return prompt
	}
	return filemerge.InjectMarkdownSection(prompt, "codegraph-guidance", guidance)
}

// agentLanguageContract returns the canonical executor language contract
// (issue #1702 defect 4). Single source of truth: injected into every
// rendered sub-agent prompt so executors spawned inside a non-English
// conversation never mimic its dialect when writing artifacts.
func agentLanguageContract() string {
	return assets.MustRead("generic/agent-language-contract.md")
}

// injectLanguageContractIntoPrompt appends the canonical language contract
// as a managed markdown section. Marker-bound, so re-rendering an already
// injected prompt is a no-op (same mechanism as the CodeGraph guidance).
func injectLanguageContractIntoPrompt(prompt string) string {
	return filemerge.InjectMarkdownSection(prompt, "agent-language-contract", strings.TrimSpace(agentLanguageContract()))
}

func injectCodeGraphToolGrantIntoPrompt(prompt string, agentID model.AgentID, guidance string) string {
	if strings.TrimSpace(guidance) == "" {
		return prompt
	}

	grant := ""
	switch agentID {
	case model.AgentClaudeCode:
		grant = claudeCodeGraphToolGrant
	default:
		return prompt
	}

	frontmatterEnd := strings.Index(prompt, "\n---\n")
	if frontmatterEnd < 0 {
		return prompt
	}
	lines := strings.Split(prompt[:frontmatterEnd], "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "tools:") {
			continue
		}
		if strings.Contains(line, grant) {
			return prompt
		}
		// A deliberately empty tools contract (tool-free reviewers, issue
		// #3168/#3648) must stay empty: appending the grant would produce
		// unparseable frontmatter and contradict the agent's own contract.
		if value := strings.TrimSpace(strings.TrimPrefix(line, "tools:")); value == "" || value == "[]" {
			return prompt
		}
		if agentID == model.AgentClaudeCode {
			lines[i] = line + ", " + grant
		} else if strings.HasSuffix(line, "]") {
			lines[i] = strings.TrimSuffix(line, "]") + `, "` + grant + `"]`
		} else {
			return prompt
		}
		return strings.Join(lines, "\n") + prompt[frontmatterEnd:]
	}
	return prompt
}

func isMarkdownSubAgentPromptFile(fileName string) bool {
	if filepath.Ext(fileName) != ".md" {
		return false
	}
	return !strings.HasPrefix(filepath.Base(fileName), ".")
}
