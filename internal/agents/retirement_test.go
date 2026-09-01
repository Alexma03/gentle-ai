package agents

import (
	"errors"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func TestFactoryRejectsCohortARetiredAgents(t *testing.T) {
	for _, id := range []model.AgentID{
		model.AgentHermes,
		model.AgentKilocode,
		model.AgentKimi,
	} {
		t.Run(string(id), func(t *testing.T) {
			_, err := NewAdapter(id)
			if !errors.Is(err, ErrAgentNotSupported) {
				t.Fatalf("NewAdapter(%q) error = %v, want ErrAgentNotSupported", id, err)
			}
		})
	}
}

func TestFactoryRejectsCohortBRetiredAgents(t *testing.T) {
	for _, id := range []model.AgentID{
		model.AgentKiroIDE,
		model.AgentOpenClaw,
		model.AgentQwenCode,
		model.AgentTrae,
		model.AgentVSCodeCopilot,
		model.AgentWindsurf,
	} {
		t.Run(string(id), func(t *testing.T) {
			_, err := NewAdapter(id)
			if !errors.Is(err, ErrAgentNotSupported) {
				t.Fatalf("NewAdapter(%q) error = %v, want ErrAgentNotSupported", id, err)
			}
		})
	}
}
