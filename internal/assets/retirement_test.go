package assets

import (
	"io/fs"
	"strings"
	"testing"
)

func TestEmbeddedAssetsExcludeRetiredClientRoots(t *testing.T) {
	for _, entry := range mustReadDir(t, ".") {
		if isRetiredAssetRoot(entry.Name()) {
			t.Fatalf("embedded assets exposed retired client root %q", entry.Name())
		}
	}
}

func mustReadDir(t *testing.T, name string) []fs.DirEntry {
	t.Helper()
	entries, err := FS.ReadDir(name)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", name, err)
	}
	return entries
}

func isRetiredAssetRoot(name string) bool {
	name = strings.ToLower(name)
	switch name {
	case "gemini", "gga", "hermes", "kimi", "kiro", "opencode", "qwen", "windsurf":
		return true
	default:
		return false
	}
}
