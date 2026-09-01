package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/state"
	"github.com/gentleman-programming/gentle-ai/v2/internal/system"
)

func TestRunInstallPersistsConfiguredSelection(t *testing.T) {
	home := t.TempDir()
	original := osUserHomeDir
	osUserHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { osUserHomeDir = original })
	// This test targets state persistence, not agent install behavior, so
	// simulate Cursor as already installed (its Detect checks for ~/.cursor)
	// — otherwise gentle-ai correctly refuses to proceed for an undetected
	// desktop-app agent.
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.cursor): %v", err)
	}
	if _, err := RunInstall([]string{"--agent", "cursor", "--preset", "custom", "--sdd-mode", "multi"}, system.DetectionResult{}); err != nil {
		t.Fatal(err)
	}
	got, err := state.Read(home)
	if err != nil || !got.SelectionConfigured || got.Preset != model.PresetCustom || got.SDDMode != model.SDDModeMulti || len(got.Components) != 0 {
		t.Fatalf("persisted selection = %#v, err = %v", got, err)
	}
	wantDigest, err := managedAssetDigest()
	if err != nil {
		t.Fatal(err)
	}
	if got.ManagedAssetDigest != wantDigest {
		t.Fatalf("managed asset digest = %q, want %q", got.ManagedAssetDigest, wantDigest)
	}
}

// TestMergeExplicitAgentInstallStateFailsHonestlyOnCorruptState was renamed
// from TestMergeExplicitAgentInstallStateSkipsCorruptState (install/sync
// surface audit finding 2). The old assertion (ok == false, no error) let
// RunInstall silently return (result, nil) on an unreadable/corrupted
// ~/.gentle-ai/state.json — the pipeline ran to completion, but the user's
// agent selection was never persisted and the CLI reported success anyway.
// The honest contract is: an unreadable existing state during an explicit
// `--agent` install must fail loudly instead of vanishing.
func TestMergeExplicitAgentInstallStateFailsHonestlyOnCorruptState(t *testing.T) {
	home := t.TempDir()
	statePath := state.Path(home)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(statePath, []byte("{not valid json\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := mergeExplicitAgentInstallState(home, state.InstallState{InstalledAgents: []string{"codex"}}, []string{"codex"}, InstallFlags{})
	if err == nil {
		t.Fatal("mergeExplicitAgentInstallState() error = nil for corrupt state, want a non-nil error")
	}
}

// TestRunInstallFailsHonestlyWhenExistingStateIsCorruptDuringExplicitAgentInstall
// closes install/sync surface audit finding 2: previously, `gentle-ai install
// --agent X` against a corrupted ~/.gentle-ai/state.json completed the whole
// pipeline (files written, verification passed) and RunInstall returned
// (result, nil) -- reported success -- WITHOUT ever calling state.Write. The
// user believed the install fully completed; state.json stayed corrupted
// forever, silently breaking every future `gentle-ai sync`.
func TestRunInstallFailsHonestlyWhenExistingStateIsCorruptDuringExplicitAgentInstall(t *testing.T) {
	home := t.TempDir()
	original := osUserHomeDir
	osUserHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { osUserHomeDir = original })

	statePath := state.Path(home)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(statePath, []byte("{not valid json\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := RunInstall([]string{"--agent", "cursor", "--preset", "custom"}, system.DetectionResult{})
	if err == nil {
		t.Fatal("RunInstall() error = nil, want an error naming the unreadable install state instead of a silent success")
	}
}
