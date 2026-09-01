package update

import "testing"

func TestToolsExcludeRetiredEntries(t *testing.T) {
	for _, tool := range Tools {
		switch tool.Name {
		case "gga", "opencode-subagent-statusline", "opencode-sdd-engram-manage":
			t.Fatalf("Tools exposed retired updater entry %#v", tool)
		}
	}
}
