package codegraph

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func TestCodeGraphComponentExposesCanonicalStatusAndManagedPaths(t *testing.T) {
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
	paths := CodeGraphManagedPaths(home)
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			t.Fatalf("CodeGraphManagedPaths() returned non-absolute path %q", path)
		}
	}
}
