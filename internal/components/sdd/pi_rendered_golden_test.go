package sdd

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

var updatePiGolden = flag.Bool("update-pi-golden", false, "update the rendered Pi orchestrator golden")

func TestRenderedPiOrchestratorGolden(t *testing.T) {
	wantPath := filepath.Join("..", "..", "..", "testdata", "golden", "sdd-pi-orchestrator.golden")
	got := []byte(renderSDDOrchestratorAsset(model.AgentPi))
	if *updatePiGolden {
		if err := os.WriteFile(wantPath, got, 0o644); err != nil {
			t.Fatalf("update Pi golden: %v", err)
		}
	}
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read Pi golden: %v", err)
	}
	if string(got) != string(want) {
		t.Fatal("rendered Pi orchestrator differs from sdd-pi-orchestrator.golden")
	}
}
