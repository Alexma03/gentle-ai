package cli

import (
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

// TestComputeSlugSlimVerdictsSkipsAgentsWithoutSetupSlug ensures agents
// without an engram setup slug (e.g. Cursor, VS Code Copilot) are excluded
// from the verdict map entirely, matching the pre-extraction inline loop.
func TestComputeSlugSlimVerdictsSkipsAgentsWithoutSetupSlug(t *testing.T) {
	agentIDs := []model.AgentID{model.AgentCursor}

	verdicts := computeSlugSlimVerdicts(agentIDs, func(model.AgentID) bool {
		return true
	})
	if len(verdicts) != 0 {
		t.Fatalf("computeSlugSlimVerdicts() = %v, want empty map for an agent with no setup slug", verdicts)
	}
}
