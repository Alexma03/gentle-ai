package screens

import (
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func TestRenderABEngine_HeadingPresent(t *testing.T) {
	engines := []model.AgentID{model.AgentClaudeCode}
	out := RenderABEngine(engines, 0)
	if !strings.Contains(out, "Choose Your AI Engine") {
		t.Errorf("heading not found; output:\n%s", out)
	}
}

func TestRenderABEngine_EngineLabelsPresent(t *testing.T) {
	engines := []model.AgentID{model.AgentClaudeCode, model.AgentCursor}
	out := RenderABEngine(engines, 0)
	if !strings.Contains(out, string(model.AgentClaudeCode)) {
		t.Errorf("claude-code label not found; output:\n%s", out)
	}
	if !strings.Contains(out, string(model.AgentCursor)) {
		t.Errorf("cursor label not found; output:\n%s", out)
	}
}

func TestRenderABEngine_BackOptionPresent(t *testing.T) {
	engines := []model.AgentID{model.AgentClaudeCode}
	out := RenderABEngine(engines, 0)
	if !strings.Contains(out, "Back") {
		t.Errorf("Back option not found; output:\n%s", out)
	}
}

func TestRenderABEngine_EmptyEngines_ShowsWarning(t *testing.T) {
	out := RenderABEngine([]model.AgentID{}, 0)
	if !strings.Contains(out, "No supported AI agent") {
		t.Errorf("expected warning for no engines; output:\n%s", out)
	}
}
