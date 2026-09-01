package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/reviewtransaction"
	"github.com/gentleman-programming/gentle-ai/v2/internal/state"
	"github.com/gentleman-programming/gentle-ai/v2/internal/system"
)

func TestMigrateInstallStateUsesInstallLock(t *testing.T) {
	home := t.TempDir()
	if err := state.Write(home, state.InstallState{InstalledAgents: []string{"opencode"}}); err != nil {
		t.Fatal(err)
	}
	result, err := MigrateInstallState(home, nil)
	if !errors.Is(err, state.ErrMigrationSelectionRequired) || !result.NeedsSelection() {
		t.Fatalf("MigrateInstallState() error=%v result=%#v, want selection-required", err, result)
	}

	lock, err := reviewtransaction.AcquireAuthorityFileLock(installStateLockPath(home))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	if _, err := MigrateInstallState(home, nil); err == nil || !strings.Contains(err.Error(), "acquire install state lock") {
		t.Fatalf("MigrateInstallState() under lock error=%v, want lock refusal", err)
	}
}

func TestInstallPlanningBlocksUnresolvedStateMigration(t *testing.T) {
	home := t.TempDir()
	if err := state.Write(home, state.InstallState{
		InstalledAgents: []string{"opencode"},
		CommunityTools:  []string{string(model.CommunityToolCodeGraph)},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.Migrate(home, nil); !errors.Is(err, state.ErrMigrationSelectionRequired) {
		t.Fatalf("state.Migrate() error=%v, want selection-required", err)
	}

	previousHome := osUserHomeDir
	osUserHomeDir = func() (string, error) { return home, nil }
	defer func() { osUserHomeDir = previousHome }()
	if _, err := RunInstall([]string{"--dry-run", "--agent", "claude-code"}, system.DetectionResult{}); err == nil || !strings.Contains(err.Error(), "state migration") {
		t.Fatalf("RunInstall() error=%v, want unresolved migration gate", err)
	}
}
