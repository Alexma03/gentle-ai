package agentguidance

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/agents"
	"github.com/gentleman-programming/gentle-ai/v2/internal/catalog"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func TestInjectRoutingInstallsGuidanceForEverySupportedAgent(t *testing.T) {
	t.Parallel()

	for _, agent := range catalog.AllAgents() {
		t.Run(string(agent.ID), func(t *testing.T) {
			t.Parallel()

			targetDir := t.TempDir()

			result, err := InjectRouting(targetDir, agent.ID)
			if err != nil {
				t.Fatalf("InjectRouting(%q) error = %v", agent.ID, err)
			}
			if !result.Changed {
				t.Fatalf("InjectRouting(%q) reported no change on a fresh target", agent.ID)
			}
			if len(result.Files) != 1 {
				t.Fatalf("InjectRouting(%q) touched %v, want exactly one file", agent.ID, result.Files)
			}

			path := result.Files[0]
			if !filepath.IsAbs(path) {
				t.Fatalf("InjectRouting(%q) reported non-absolute path %q", agent.ID, path)
			}
			if !strings.HasPrefix(path, targetDir) {
				t.Fatalf("InjectRouting(%q) wrote outside the target dir: %q", agent.ID, path)
			}

			// Read the scope the agent actually loads, not merely the bytes on
			// disk: adapters whose guidance lives inside a settings document
			// carry the block as an encoded string, never as raw markdown.
			written := deliveredGuidance(t, path)
			rendered, err := RenderRouting(agent.ID)
			if err != nil {
				t.Fatalf("RenderRouting(%q) error = %v", agent.ID, err)
			}
			if !strings.Contains(written, rendered) {
				t.Fatalf("InjectRouting(%q) did not write the rendered guidance:\n%s", agent.ID, written)
			}
			if !strings.Contains(written, "<!-- gentle-ai:"+RoutingSectionID+" -->") {
				t.Fatalf("InjectRouting(%q) did not open the managed section:\n%s", agent.ID, written)
			}
			if !strings.Contains(written, "<!-- /gentle-ai:"+RoutingSectionID+" -->") {
				t.Fatalf("InjectRouting(%q) did not close the managed section:\n%s", agent.ID, written)
			}
			if !strings.Contains(written, "First establish whether the requested outcome explicitly authorizes a change.") {
				t.Fatalf("InjectRouting(%q) did not deliver the outcome-authorization guard:\n%s", agent.ID, written)
			}
		})
	}
}

// TestInjectRoutingStaysContainedUnderHostileEnvironment pins containment
// against ambient config-redirection environment. CI runners (and real user
// shells) export XDG_CONFIG_HOME and APPDATA; an adapter that resolves paths
// from the environment instead of the passed installation root writes guidance
// outside the target dir — into the real user config — and the idempotency
// guarantee silently breaks because state leaks across runs.
//
// No t.Parallel here: t.Setenv is process-wide and forbids it.
func TestInjectRoutingStaysContainedUnderHostileEnvironment(t *testing.T) {
	hostile := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(hostile, "xdg"))
	t.Setenv("APPDATA", filepath.Join(hostile, "AppData", "Roaming"))

	for _, agent := range catalog.AllAgents() {
		t.Run(string(agent.ID), func(t *testing.T) {
			targetDir := t.TempDir()

			declared, err := RoutingPaths(targetDir, agent.ID)
			if err != nil {
				t.Fatalf("RoutingPaths(%q) error = %v", agent.ID, err)
			}
			for _, path := range declared {
				if !strings.HasPrefix(path, targetDir) {
					t.Fatalf("RoutingPaths(%q) declared a path outside the target dir: %q", agent.ID, path)
				}
			}

			result, err := InjectRouting(targetDir, agent.ID)
			if err != nil {
				t.Fatalf("InjectRouting(%q) error = %v", agent.ID, err)
			}
			if !result.Changed {
				t.Fatalf("InjectRouting(%q) reported no change on a fresh target", agent.ID)
			}
			for _, path := range result.Files {
				if !strings.HasPrefix(path, targetDir) {
					t.Fatalf("InjectRouting(%q) wrote outside the target dir: %q", agent.ID, path)
				}
			}

			entries, err := os.ReadDir(hostile)
			if err != nil {
				t.Fatalf("ReadDir(hostile) error = %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("InjectRouting(%q) created %d entries under the hostile config root", agent.ID, len(entries))
			}
		})
	}
}

func TestInjectRoutingIsIdempotent(t *testing.T) {
	t.Parallel()

	targetDir := t.TempDir()

	first, err := InjectRouting(targetDir, model.AgentClaudeCode)
	if err != nil {
		t.Fatalf("first InjectRouting error = %v", err)
	}
	if !first.Changed {
		t.Fatalf("first InjectRouting reported no change")
	}
	afterFirst := readFile(t, first.Files[0])

	second, err := InjectRouting(targetDir, model.AgentClaudeCode)
	if err != nil {
		t.Fatalf("second InjectRouting error = %v", err)
	}
	if second.Changed {
		t.Fatalf("second identical InjectRouting reported a change")
	}
	if got := readFile(t, first.Files[0]); got != afterFirst {
		t.Fatalf("second InjectRouting rewrote the file:\n%s", got)
	}
}

func TestInjectRoutingPreservesUnmanagedUserContent(t *testing.T) {
	t.Parallel()

	targetDir := t.TempDir()

	adapter, err := agents.NewAdapter(model.AgentClaudeCode)
	if err != nil {
		t.Fatalf("NewAdapter error = %v", err)
	}
	path := adapter.SystemPromptFile(targetDir)

	const (
		above = "# My own prompt\n\nHand-written rules that must survive.\n"
		below = "\n## My trailing notes\n\nAlso hand-written.\n"
	)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(path, []byte(above), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	if _, err := InjectRouting(targetDir, model.AgentClaudeCode); err != nil {
		t.Fatalf("InjectRouting error = %v", err)
	}

	// Append unmanaged content after the closing marker, then re-inject: both
	// the content above and below the managed block must survive verbatim.
	withTrailing := readFile(t, path) + below
	if err := os.WriteFile(path, []byte(withTrailing), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	result, err := InjectRouting(targetDir, model.AgentClaudeCode)
	if err != nil {
		t.Fatalf("re-inject error = %v", err)
	}
	if result.Changed {
		t.Fatalf("re-injecting identical guidance reported a change")
	}

	final := readFile(t, path)
	if !strings.HasPrefix(final, above) {
		t.Fatalf("unmanaged content above the block was altered:\n%s", final)
	}
	if !strings.HasSuffix(final, below) {
		t.Fatalf("unmanaged content below the block was altered:\n%s", final)
	}
}

func TestInjectRoutingRejectsUnregisteredAgent(t *testing.T) {
	t.Parallel()

	targetDir := t.TempDir()

	result, err := InjectRouting(targetDir, model.AgentID("totally-unregistered-agent"))
	if err == nil {
		t.Fatalf("InjectRouting accepted an unregistered agent: %+v", result)
	}
	if result.Changed || len(result.Files) != 0 {
		t.Fatalf("InjectRouting reported work for an unregistered agent: %+v", result)
	}

	entries, readErr := os.ReadDir(targetDir)
	if readErr != nil {
		t.Fatalf("ReadDir error = %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("InjectRouting wrote %d entries for an unregistered agent", len(entries))
	}
}

func TestInjectRoutingRejectsBlankTargetDir(t *testing.T) {
	t.Parallel()

	result, err := InjectRouting("   ", model.AgentClaudeCode)
	if err == nil {
		t.Fatalf("InjectRouting accepted a blank target dir: %+v", result)
	}
	if !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("InjectRouting error = %v, want it to wrap ErrInvalidTarget", err)
	}
}

func TestInjectRoutingKeepsMarkdownSectionAgentsOnTheirPromptFile(t *testing.T) {
	t.Parallel()

	for _, agent := range markdownSectionAgents(t) {
		t.Run(string(agent), func(t *testing.T) {
			t.Parallel()

			targetDir := t.TempDir()

			adapter, err := agents.NewAdapter(agent)
			if err != nil {
				t.Fatalf("NewAdapter error = %v", err)
			}
			path := adapter.SystemPromptFile(targetDir)

			const (
				above = "# My own prompt\n\nHand-written rules that must survive.\n"
				below = "\n## My trailing notes\n\nAlso hand-written.\n"
			)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("MkdirAll error = %v", err)
			}
			if err := os.WriteFile(path, []byte(above), 0o644); err != nil {
				t.Fatalf("WriteFile error = %v", err)
			}

			result, err := InjectRouting(targetDir, agent)
			if err != nil {
				t.Fatalf("InjectRouting(%q) error = %v", agent, err)
			}
			if len(result.Files) != 1 || result.Files[0] != path {
				t.Fatalf("InjectRouting(%q) touched %v, want exactly %q", agent, result.Files, path)
			}

			withTrailing := readFile(t, path) + below
			if err := os.WriteFile(path, []byte(withTrailing), 0o644); err != nil {
				t.Fatalf("WriteFile error = %v", err)
			}

			second, err := InjectRouting(targetDir, agent)
			if err != nil {
				t.Fatalf("re-inject(%q) error = %v", agent, err)
			}
			if second.Changed {
				t.Fatalf("re-injecting identical guidance for %q reported a change", agent)
			}

			final := readFile(t, path)
			if !strings.HasPrefix(final, above) {
				t.Fatalf("unmanaged content above the block was altered for %q:\n%s", agent, final)
			}
			if !strings.HasSuffix(final, below) {
				t.Fatalf("unmanaged content below the block was altered for %q:\n%s", agent, final)
			}
		})
	}
}

func TestInjectRoutingIsIdempotentForEverySupportedAgent(t *testing.T) {
	t.Parallel()

	for _, agent := range catalog.AllAgents() {
		t.Run(string(agent.ID), func(t *testing.T) {
			t.Parallel()

			targetDir := t.TempDir()

			first, err := InjectRouting(targetDir, agent.ID)
			if err != nil {
				t.Fatalf("first InjectRouting(%q) error = %v", agent.ID, err)
			}
			if !first.Changed {
				t.Fatalf("first InjectRouting(%q) reported no change", agent.ID)
			}
			afterFirst := readFile(t, first.Files[0])

			second, err := InjectRouting(targetDir, agent.ID)
			if err != nil {
				t.Fatalf("second InjectRouting(%q) error = %v", agent.ID, err)
			}
			if second.Changed {
				t.Fatalf("second identical InjectRouting(%q) reported a change", agent.ID)
			}
			if got := readFile(t, first.Files[0]); got != afterFirst {
				t.Fatalf("second InjectRouting(%q) rewrote the file:\n%s", agent.ID, got)
			}
		})
	}
}

func TestInjectRoutingDeliversNoRetiredControlPlaneVocabulary(t *testing.T) {
	t.Parallel()

	for _, agent := range catalog.AllAgents() {
		t.Run(string(agent.ID), func(t *testing.T) {
			t.Parallel()

			targetDir := t.TempDir()

			result, err := InjectRouting(targetDir, agent.ID)
			if err != nil {
				t.Fatalf("InjectRouting(%q) error = %v", agent.ID, err)
			}

			block := managedRoutingBlock(deliveredGuidance(t, result.Files[0]))
			if strings.TrimSpace(block) == "" {
				t.Fatalf("InjectRouting(%q) delivered no managed routing block", agent.ID)
			}
			lowered := strings.ToLower(block)
			for _, forbidden := range retiredRemoteControlPlaneVocabulary {
				if strings.Contains(lowered, strings.ToLower(forbidden)) {
					t.Fatalf("InjectRouting(%q) delivered retired vocabulary %q:\n%s", agent.ID, forbidden, block)
				}
			}
		})
	}
}

// markdownSectionAgents lists every supported agent whose guidance is delivered
// as a marker section inside its own system prompt file — that is, everything
// except the Jinja router and the orchestrator-scoped OpenCode family.
func markdownSectionAgents(t *testing.T) []model.AgentID {
	t.Helper()

	var selected []model.AgentID
	for _, agent := range catalog.AllAgents() {
		if agent.ID == model.AgentOpenCode {
			continue
		}
		_, err := agents.NewAdapter(agent.ID)
		if err != nil {
			t.Fatalf("NewAdapter(%q) error = %v", agent.ID, err)
		}
		selected = append(selected, agent.ID)
	}
	const wantMarkdownSectionAgents = 5
	if len(selected) != wantMarkdownSectionAgents {
		t.Fatalf("selected %d markdown-section agents, want %d", len(selected), wantMarkdownSectionAgents)
	}
	return selected
}

// deliveredGuidance returns the guidance text an agent actually loads from the
// written file: a settings document carries it as an encoded prompt string,
// every other target carries it as raw markdown.
func deliveredGuidance(t *testing.T, path string) string {
	t.Helper()

	return readFile(t, path)
}

// managedRoutingBlock returns only the content Gentle AI owns, so assertions
// about the block never accidentally inspect surrounding user content.
func managedRoutingBlock(content string) string {
	open := "<!-- gentle-ai:" + RoutingSectionID + " -->"
	closing := "<!-- /gentle-ai:" + RoutingSectionID + " -->"

	start := strings.Index(content, open)
	end := strings.Index(content, closing)
	if start < 0 || end <= start {
		return ""
	}
	return content[start+len(open) : end]
}

func writeJSON(t *testing.T, path string, value map[string]any) {
	t.Helper()

	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()

	decoded := map[string]any{}
	if err := json.Unmarshal([]byte(readFile(t, path)), &decoded); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", path, err)
	}
	return decoded
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return string(data)
}
