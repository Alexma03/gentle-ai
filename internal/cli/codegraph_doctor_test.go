package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/doctor"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/state"
)

func TestCheckCodeGraphReportsUnavailableWithoutMutatingClients(t *testing.T) {
	original := lookPathFn
	lookPathFn = func(string) (string, error) { return "", errors.New("not installed") }
	defer func() { lookPathFn = original }()

	result := checkCodeGraph(t.TempDir())
	if result.Name != doctor.CheckCodeGraph || result.Status != CheckStatusWarn {
		t.Fatalf("checkCodeGraph() = %#v, want warning", result)
	}
	for _, want := range []string{"CodeGraph unavailable", "parity evidence", "retained clients are unchanged"} {
		if !strings.Contains(result.Detail, want) {
			t.Fatalf("checkCodeGraph() detail = %q, missing %q", result.Detail, want)
		}
	}
}

func TestCheckCodeGraphReportsParityDegradation(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	original := lookPathFn
	lookPathFn = func(string) (string, error) { return "/usr/local/bin/codegraph", nil }
	defer func() { lookPathFn = original }()

	result := checkCodeGraph(home)
	if result.Status != CheckStatusWarn || !strings.Contains(result.Detail, "parity degraded") {
		t.Fatalf("checkCodeGraph() = %#v, want parity warning", result)
	}
	if result.Remedy == nil || result.Remedy.ID != doctor.RemedyInspectCodeGraph {
		t.Fatalf("checkCodeGraph() remedy = %#v, want CodeGraph inspection", result.Remedy)
	}
}

func TestCodeGraphEnabledRecognizesCanonicalAndLegacyState(t *testing.T) {
	if !codeGraphEnabled(state.InstallState{Components: []model.ComponentID{model.ComponentCodeGraph}}) {
		t.Fatal("canonical CodeGraph component was not recognized")
	}
	if !codeGraphEnabled(state.InstallState{CommunityTools: []string{string(model.CommunityToolCodeGraph)}}) {
		t.Fatal("legacy CodeGraph community tool was not recognized")
	}
	if codeGraphEnabled(state.InstallState{Components: []model.ComponentID{model.ComponentEngram}}) {
		t.Fatal("unrelated component enabled CodeGraph")
	}
}
