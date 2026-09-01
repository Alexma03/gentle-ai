package cli

import (
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func TestAntigravityCollisionCheckNeverWarnsForLegacyGeminiCLI(t *testing.T) {
	checks := antigravityCollisionCheck([]model.AgentID{model.AgentGeminiCLI, model.AgentAntigravity})
	if len(checks) != 0 {
		t.Fatalf("antigravityCollisionCheck() len = %d, want 0", len(checks))
	}
}

func TestAntigravityCollisionCheckNoWarningWithoutGemini(t *testing.T) {
	checks := antigravityCollisionCheck([]model.AgentID{model.AgentAntigravity})
	if len(checks) != 0 {
		t.Fatalf("antigravityCollisionCheck() len = %d, want 0", len(checks))
	}
}
