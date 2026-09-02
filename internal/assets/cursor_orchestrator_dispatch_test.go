package assets

import (
	"io/fs"
	"path"
	"strings"
	"testing"
)

func TestCursorOrchestratorListsEverySDDSubagentAndOwnsDispatch(t *testing.T) {
	content := MustRead("cursor/sdd-orchestrator.md")
	if strings.Contains(content, "Skills (appear in autocomplete)") {
		t.Fatal("Cursor does not install user slash commands; the orchestrator owns agent and type dispatch")
	}
	if !strings.Contains(content, "The user never types them") {
		t.Fatal("Cursor orchestrator must tell the parent it decides agent and type, not the user")
	}

	entries, err := fs.ReadDir(FS, "cursor/agents")
	if err != nil {
		t.Fatalf("read cursor/agents: %v", err)
	}
	var missing []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		if !strings.HasPrefix(name, "sdd-") {
			continue
		}
		row := "| `" + name + "` | `" + entry.Name() + "` |"
		if !strings.Contains(content, row) {
			missing = append(missing, row)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("cursor orchestrator table missing subagent rows:\n%s", strings.Join(missing, "\n"))
	}
}

func TestCursorOrchestratorDoesNotInstallSlashCommandAssets(t *testing.T) {
	_, err := fs.Stat(FS, path.Join("cursor", "commands"))
	if err == nil {
		t.Fatal("cursor/commands must not exist; Cursor slash commands are not a user surface")
	}
}
