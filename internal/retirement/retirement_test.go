package retirement

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRetiredProductionSurfaceHasNoActiveReferences(t *testing.T) {
	root := repositoryRoot(t)
	forbidden := []string{"hermes", "kilocode", "kimi", "kiro", "openclaw", "qwen", "trae", "vscode", "windsurf", "opencode", "gemini-cli", "gga", "tintinweb", "j0k3r"}
	for _, relative := range []string{
		"internal/agents",
		"internal/catalog",
		"internal/components",
		"internal/cli",
		"internal/installcmd",
		"internal/planner",
		"internal/tui",
		"internal/update",
		"internal/assets",
		"internal/versions",
	} {
		dir := filepath.Join(root, relative)
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if !isProductionFile(path) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			lower := strings.ToLower(string(data))
			for _, token := range forbidden {
				if strings.Contains(lower, token) {
					return fmt.Errorf("%s retains active retired token %q", filepath.ToSlash(path), token)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRetiredAgentDirectoriesAreAbsent(t *testing.T) {
	root := repositoryRoot(t)
	for _, name := range []string{"hermes", "kilocode", "kimi", "kiro", "openclaw", "opencode", "qwen", "trae", "vscode", "windsurf"} {
		path := filepath.Join(root, "internal", "agents", name)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("retired agent directory %q still exists (err=%v)", path, err)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func isProductionFile(path string) bool {
	if filepath.Ext(path) == ".go" {
		return !strings.HasSuffix(path, "_test.go")
	}
	return filepath.Ext(path) != "" && !strings.HasSuffix(path, ".golden")
}
