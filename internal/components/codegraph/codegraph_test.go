package codegraph

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func TestCodeGraphComponentExposesCanonicalDefinitionAndStatus(t *testing.T) {
	definitions := Definitions()
	if len(definitions) != 1 {
		t.Fatalf("Definitions() returned %d entries, want one CodeGraph definition", len(definitions))
	}
	definition, ok := DefinitionFor(model.CommunityToolCodeGraph)
	if !ok || definition.ID != model.CommunityToolCodeGraph || definition.CommandName != "codegraph" {
		t.Fatalf("DefinitionFor(codegraph) = %#v, %t", definition, ok)
	}

	home := t.TempDir()
	status := DetectStatus(home, DetectorFunc(func(string) (string, error) {
		return "", errors.New("codegraph unavailable")
	}))
	if status.CLI != AvailabilityMissing {
		t.Fatalf("DetectStatus() CLI = %q, want missing", status.CLI)
	}
	if status.Tool != model.CommunityToolCodeGraph {
		t.Fatalf("DetectStatus() tool = %q, want codegraph", status.Tool)
	}
	paths := BackupPaths(home)
	if len(paths) != len(ManagedPaths(home)) {
		t.Fatalf("BackupPaths() length = %d, ManagedPaths() length = %d", len(paths), len(ManagedPaths(home)))
	}
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			t.Fatalf("BackupPaths() returned non-absolute path %q", path)
		}
	}
}
