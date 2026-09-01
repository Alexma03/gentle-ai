package mcp

import (
	"fmt"

	"github.com/gentleman-programming/gentle-ai/v2/internal/versions"
)

var defaultContext7ServerJSON = []byte(fmt.Sprintf("{\n  \"command\": \"npx\",\n  \"args\": [\n    \"-y\",\n    \"--package=@upstash/context7-mcp@%s\",\n    \"--\",\n    \"context7-mcp\"\n  ]\n}\n", versions.Context7MCP))

var defaultContext7OverlayJSON = []byte(fmt.Sprintf("{\n  \"mcpServers\": {\n    \"context7\": {\n      \"command\": \"npx\",\n      \"args\": [\n        \"-y\",\n        \"--package=@upstash/context7-mcp@%s\",\n        \"--\",\n        \"context7-mcp\"\n      ]\n    }\n  }\n}\n", versions.Context7MCP))

// openCodeContext7OverlayJSON is the opencode.json overlay using the new MCP format.
// Context7 is a remote MCP server — no npx needed.
// The context7 entry must replace atomically so legacy local keys do not survive
// deep merge into OpenCode/KiloCode's strict MCP schema.
var openCodeContext7OverlayJSON = []byte("{\n  \"mcp\": {\n    \"context7\": {\n      \"__replace__\": {\n        \"type\": \"remote\",\n        \"url\": \"https://mcp.context7.com/mcp\",\n        \"enabled\": true\n      }\n    }\n  }\n}\n")

// antigravityContext7OverlayJSON is the Antigravity mcp_config.json overlay.
// Uses mcpServers key (same schema as Claude Code) with serverUrl for HTTP remote.
// The context7 entry must replace atomically so legacy local keys do not survive
// deep merge into this managed MCP server entry.
var antigravityContext7OverlayJSON = []byte("{\n  \"mcpServers\": {\n    \"context7\": {\n      \"__replace__\": {\n        \"serverUrl\": \"https://mcp.context7.com/mcp\"\n      }\n    }\n  }\n}\n")

func DefaultContext7ServerJSON() []byte {
	content := make([]byte, len(defaultContext7ServerJSON))
	copy(content, defaultContext7ServerJSON)
	return content
}

func DefaultContext7OverlayJSON() []byte {
	content := make([]byte, len(defaultContext7OverlayJSON))
	copy(content, defaultContext7OverlayJSON)
	return content
}

func OpenCodeContext7OverlayJSON() []byte {
	content := make([]byte, len(openCodeContext7OverlayJSON))
	copy(content, openCodeContext7OverlayJSON)
	return content
}

func AntigravityContext7OverlayJSON() []byte {
	content := make([]byte, len(antigravityContext7OverlayJSON))
	copy(content, antigravityContext7OverlayJSON)
	return content
}
