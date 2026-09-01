package state

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func TestMigrateReportsRetiredValuesAndMapsCodeGraph(t *testing.T) {
	home := t.TempDir()
	managed := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(managed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managed, []byte("before migration\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	original := InstallState{
		InstalledAgents:          []string{"claude-code", "opencode"},
		CommunityTools:           []string{"codegraph"},
		CommunityToolsConfigured: true,
	}
	if err := Write(home, original); err != nil {
		t.Fatal(err)
	}

	result, err := Migrate(home, []string{managed})
	if !errors.Is(err, ErrMigrationSelectionRequired) {
		t.Fatalf("Migrate() error = %v, want selection-required", err)
	}
	if !result.NeedsSelection() || !reflect.DeepEqual(result.Report.Retired, []string{"opencode"}) ||
		!reflect.DeepEqual(result.Report.Unresolved, []string{"agent:opencode"}) {
		t.Fatalf("migration result = %#v, want one unresolved retired agent", result)
	}
	if result.Report.FromVersion != 0 || result.Report.ToVersion != CurrentSchemaVersion || result.Report.RawStateDigest == "" || result.Report.BackupID == "" {
		t.Fatalf("migration report metadata = %#v", result.Report)
	}
	if len(result.Report.Mapped) != 1 || result.Report.Mapped[0].From != "community_tools:codegraph" || result.Report.Mapped[0].To != "components:codegraph" {
		t.Fatalf("migration mappings = %#v", result.Report.Mapped)
	}
	if !containsComponent(result.State.Components, model.ComponentCodeGraph) || result.State.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("migrated state = %#v, want schema and CodeGraph component", result.State)
	}
	if len(result.State.CommunityTools) != 0 {
		t.Fatalf("migrated state retained legacy CodeGraph tools = %#v", result.State.CommunityTools)
	}
	if _, err := os.Stat(MigrationReportPath(home)); err != nil {
		t.Fatalf("migration report not persisted: %v", err)
	}
	backupState, err := os.ReadFile(filepath.Join(MigrationBackupDir(home, result.Report.BackupID), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if digestBytes(backupState) != result.Report.RawStateDigest {
		t.Fatalf("raw backup digest = %q, report = %q", digestBytes(backupState), result.Report.RawStateDigest)
	}
	if len(result.Report.RollbackManifest) != 1 || result.Report.RollbackManifest[0].Digest == "" {
		t.Fatalf("rollback manifest = %#v", result.Report.RollbackManifest)
	}
}

func TestMigrateRejectsFutureStateSchema(t *testing.T) {
	home := t.TempDir()
	if err := Write(home, InstallState{SchemaVersion: CurrentSchemaVersion + 1, InstalledAgents: []string{"claude-code"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Migrate(home, nil); err == nil || !strings.Contains(err.Error(), "unsupported install state schema") {
		t.Fatalf("Migrate() error = %v, want future-schema refusal", err)
	}
}

func TestRequireMigrationResolvedBlocksOnlyPendingReports(t *testing.T) {
	home := t.TempDir()
	if err := Write(home, InstallState{InstalledAgents: []string{"opencode"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Migrate(home, nil); !errors.Is(err, ErrMigrationSelectionRequired) {
		t.Fatalf("Migrate() error = %v, want selection-required", err)
	}
	if err := RequireMigrationResolved(home); !errors.Is(err, ErrMigrationSelectionRequired) {
		t.Fatalf("RequireMigrationResolved() error = %v, want selection-required", err)
	}
	if _, err := ResolveSelection(home, map[string]model.AgentID{"opencode": model.AgentCodex}); err != nil {
		t.Fatalf("ResolveSelection() error = %v", err)
	}
	if err := RequireMigrationResolved(home); err != nil {
		t.Fatalf("RequireMigrationResolved() after selection = %v", err)
	}
}

func TestRestoreMigrationRejectsUntrustedBackupIdentity(t *testing.T) {
	if err := RestoreMigration(t.TempDir(), MigrationReport{Schema: MigrationReportSchema, BackupID: "../../outside"}); err == nil {
		t.Fatal("RestoreMigration() accepted path-traversal backup identity")
	}
}

func TestResolveSelectionAndRestoreMigrationAreReversible(t *testing.T) {
	home := t.TempDir()
	managed := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(managed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managed, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	original := InstallState{InstalledAgents: []string{"opencode"}, CommunityTools: []string{"codegraph"}, CommunityToolsConfigured: true}
	if err := Write(home, original); err != nil {
		t.Fatal(err)
	}
	result, err := Migrate(home, []string{managed})
	if !errors.Is(err, ErrMigrationSelectionRequired) {
		t.Fatalf("Migrate() error = %v, want selection-required", err)
	}
	resolved, err := ResolveSelection(home, map[string]model.AgentID{"opencode": model.AgentCodex})
	if err != nil {
		t.Fatalf("ResolveSelection() error = %v", err)
	}
	if resolved.NeedsSelection() || !reflect.DeepEqual(resolved.State.InstalledAgents, []string{"codex"}) {
		t.Fatalf("resolved migration = %#v", resolved)
	}
	if err := os.WriteFile(managed, []byte("changed after migration\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(home, InstallState{InstalledAgents: []string{"pi"}}); err != nil {
		t.Fatal(err)
	}
	if err := RestoreMigration(home, resolved.Report); err != nil {
		t.Fatalf("RestoreMigration() error = %v", err)
	}
	restored, err := Read(home)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored, original) {
		t.Fatalf("restored state = %#v, want %#v", restored, original)
	}
	content, err := os.ReadFile(managed)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "original\n" {
		t.Fatalf("restored managed path = %q", content)
	}
	restoredReport, err := ReadMigrationReport(home)
	if err != nil {
		t.Fatal(err)
	}
	if !restoredReport.Restored || restoredReport.NeedsSelection() || !strings.Contains(restoredReport.BackupPath, result.Report.BackupID) {
		t.Fatalf("restored report = %#v", restoredReport)
	}
}

func TestMigrateWithLockUsesExistingLockBoundary(t *testing.T) {
	home := t.TempDir()
	if err := Write(home, InstallState{InstalledAgents: []string{"claude-code"}}); err != nil {
		t.Fatal(err)
	}
	called := false
	result, err := MigrateWithLock(home, nil, func(operation func() error) error {
		called = true
		return operation()
	})
	if err != nil {
		t.Fatalf("MigrateWithLock() error = %v", err)
	}
	if !called || result.Report.ToVersion != CurrentSchemaVersion || result.NeedsSelection() {
		t.Fatalf("MigrateWithLock() called=%t result=%#v", called, result)
	}
}
