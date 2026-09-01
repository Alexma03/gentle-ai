package cli

import (
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/components/codegraph"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func TestComponentApplyStepRunsFirstClassCodeGraphInstaller(t *testing.T) {
	previous := installCodeGraph
	t.Cleanup(func() { installCodeGraph = previous })

	var gotHome, gotWorkspace string
	installCodeGraph = func(homeDir, workspaceDir string, _ codegraph.Runner, _ codegraph.Detector) (codegraph.Result, error) {
		gotHome, gotWorkspace = homeDir, workspaceDir
		return codegraph.Result{}, nil
	}
	home, workspace := t.TempDir(), t.TempDir()
	step := componentApplyStep{
		component:    model.ComponentCodeGraph,
		homeDir:      home,
		workspaceDir: workspace,
		state:        &runtimeState{},
	}
	if err := step.Run(); err != nil {
		t.Fatalf("CodeGraph component step returned error: %v", err)
	}
	if gotHome != home || gotWorkspace != workspace {
		t.Fatalf("CodeGraph installer received home/workspace %q/%q, want %q/%q", gotHome, gotWorkspace, home, workspace)
	}
}
