package assets

import "github.com/gentleman-programming/gentle-ai/v2/internal/model"

// SDDCommandsAssetDir returns the embedded slash-command asset directory for an
// agent. Claude owns the canonical command contract; agents without a
// dedicated command set use the retained Claude-compatible assets.
func SDDCommandsAssetDir(agent model.AgentID) string {
	switch agent {
	case model.AgentClaudeCode:
		return "claude/commands"
	default:
		return "claude/commands"
	}
}
