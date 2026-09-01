package screens

import (
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/components/codegraph"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func TestRenderCommunityToolsShowsCodeGraph(t *testing.T) {
	out := RenderCommunityTools([]model.CommunityToolID{model.CommunityToolCodeGraph}, 0, nil, false, nil)
	for _, want := range []string{"Community Tools/Plugins", "[x] CodeGraph", "View repo: https://github.com/colbymchenry/codegraph", "Continue", "Back"} {
		if !strings.Contains(out, want) {
			t.Fatalf("RenderCommunityTools missing %q; output:\n%s", want, out)
		}
	}
}

func TestRenderCommunityToolsShowsStatusLoadingAndAgentState(t *testing.T) {
	loading := RenderCommunityTools(nil, 0, nil, true, nil)
	if !strings.Contains(loading, "Detecting installed tool and agent wiring") {
		t.Fatalf("loading output missing detection text:\n%s", loading)
	}

	status := codegraph.Status{
		Tool: model.CommunityToolCodeGraph,
		CLI:  codegraph.AvailabilityAvailable,
		Agents: []codegraph.AgentStatus{
			{Agent: model.AgentClaudeCode, Name: "Claude Code", Detected: true, Configured: true, Status: codegraph.AgentStatusConfigured, Path: "/tmp/.claude/mcp/codegraph.json"},
			{Agent: model.AgentOpenCode, Name: "OpenCode", Detected: true, Configured: false, Status: codegraph.AgentStatusMissing, Path: "/tmp/.config/opencode"},
		},
	}
	out := RenderCommunityTools(nil, 0, []codegraph.Status{status}, false, nil)
	for _, want := range []string{"CodeGraph CLI: available", "Agent wiring: 2 detected • 1 configured • 1 missing", "Claude Code: configured", "OpenCode: missing"} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q; output:\n%s", want, out)
		}
	}
}

func TestRenderCommunityToolResultShowsPartialContextOnError(t *testing.T) {
	result := codegraph.Result{
		Tool: model.CommunityToolCodeGraph,
		StatusAfter: &codegraph.Status{
			Tool: model.CommunityToolCodeGraph,
			CLI:  codegraph.AvailabilityMissing,
			Agents: []codegraph.AgentStatus{
				{Agent: model.AgentOpenCode, Name: "OpenCode", Detected: true, Configured: false, Status: codegraph.AgentStatusMissing},
			},
		},
	}
	out := RenderCommunityToolResult([]codegraph.Result{result}, assertErr("validation failed"))
	for _, want := range []string{"Community tool setup failed", "validation failed", "CodeGraph: CLI missing", "OpenCode: missing"} {
		if !strings.Contains(out, want) {
			t.Fatalf("result output missing %q; output:\n%s", want, out)
		}
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
