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

	lock, err := reviewtransaction.AcquireAuthorityFileLock(mustInstallStateLockPath(t, home))
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

func TestRunInstallMigratesLegacyStateBeforePlanning(t *testing.T) {
	home := t.TempDir()
	if err := state.Write(home, state.InstallState{
		InstalledAgents: []string{"opencode"},
		CommunityTools:  []string{string(model.CommunityToolCodeGraph)},
	}); err != nil {
		t.Fatal(err)
	}

	previousHome := osUserHomeDir
	osUserHomeDir = func() (string, error) { return home, nil }
	defer func() { osUserHomeDir = previousHome }()

	_, err := RunInstall([]string{"--dry-run", "--agent", "claude-code"}, system.DetectionResult{})
	if !errors.Is(err, state.ErrMigrationSelectionRequired) {
		t.Fatalf("RunInstall() error = %v, want automatic migration selection gate", err)
	}
	migrated, readErr := state.Read(home)
	if readErr != nil {
		t.Fatal(readErr)
	}
	hasCodeGraph := false
	for _, component := range migrated.Components {
		if component == model.ComponentCodeGraph {
			hasCodeGraph = true
			break
		}
	}
	if migrated.SchemaVersion != state.CurrentSchemaVersion || !hasCodeGraph {
		t.Fatalf("migrated state = %#v, want canonical schema and CodeGraph component", migrated)
	}
	if _, readErr := state.ReadMigrationReport(home); readErr != nil {
		t.Fatalf("automatic install migration report missing: %v", readErr)
	}
}

func TestRunSyncMigratesLegacyStateBeforePlanning(t *testing.T) {
	home := t.TempDir()
	if err := state.Write(home, state.InstallState{InstalledAgents: []string{"opencode"}}); err != nil {
		t.Fatal(err)
	}

	previousHome := osUserHomeDir
	osUserHomeDir = func() (string, error) { return home, nil }
	defer func() { osUserHomeDir = previousHome }()

	_, err := RunSync([]string{"--dry-run", "--agent", "claude-code"})
	if !errors.Is(err, state.ErrMigrationSelectionRequired) {
		t.Fatalf("RunSync() error = %v, want automatic migration selection gate", err)
	}
	if _, readErr := state.ReadMigrationReport(home); readErr != nil {
		t.Fatalf("automatic sync migration report missing: %v", readErr)
	}
}
