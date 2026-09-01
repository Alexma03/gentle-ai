package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/state"
)

// MigrateInstallState runs the versioned state migration while holding the
// same lock used by install and sync. Callers supply the managed paths that
// must be included in the rollback manifest; the state package owns the
// snapshot and restore semantics.
func MigrateInstallState(homeDir string, managedPaths []string) (state.MigrationResult, error) {
	return state.MigrateWithLock(homeDir, managedPaths, func(operation func() error) error {
		return withInstallStateLock(homeDir, operation)
	})
}

// requireInstallStateMigrationResolved prevents install/sync planning from
// reading a state file whose migration report still contains retired values.
// Resolution is explicit: the caller must present the report and invoke
// state.ResolveSelection before trying again.
func requireInstallStateMigrationResolved(homeDir string) error {
	return state.RequireMigrationResolved(homeDir)
}

// migrateInstallStateBeforePlanning upgrades a legacy state file while the
// install-state lock is held, then enforces the explicit unresolved-selection
// gate. Keeping both operations at this boundary prevents install and sync
// from planning against a state shape that the canonical registry cannot
// represent.
func migrateInstallStateBeforePlanning(homeDir string) error {
	_, err := migrateInstallStateFromDisk(homeDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("state migration: %w", err)
	}
	if err := requireInstallStateMigrationResolved(homeDir); err != nil {
		return fmt.Errorf("state migration: %w", err)
	}
	return nil
}

// migrateInstallStateFromDisk derives the rollback targets from persisted
// selections inside the same lock as the migration read/write. A missing
// state file is returned unchanged so fresh homes retain their legacy no-op
// behavior.
func migrateInstallStateFromDisk(homeDir string) (result state.MigrationResult, err error) {
	err = withInstallStateLock(homeDir, func() error {
		if _, statErr := os.Stat(state.Path(homeDir)); statErr != nil {
			return statErr
		}
		managedPaths, pathsErr := migrationManagedPaths(homeDir)
		if pathsErr != nil {
			return fmt.Errorf("derive migration rollback paths: %w", pathsErr)
		}
		result, err = state.Migrate(homeDir, managedPaths)
		return err
	})
	return result, err
}

// migrationManagedPaths mirrors sync's managed-path contract using the
// persisted selection, including legacy adapters that still exist solely for
// migration/rollback. It intentionally avoids planner resolution: migration
// must run before canonical planning can reject retired IDs.
func migrationManagedPaths(homeDir string) ([]string, error) {
	persisted, err := state.Read(homeDir)
	if err != nil {
		return nil, err
	}
	selection := model.Selection{
		Components: append([]model.ComponentID(nil), persisted.Components...),
		Skills:     append([]model.SkillID(nil), persisted.Skills...),
		Preset:     persisted.Preset,
		Persona:    model.PersonaID(persisted.Persona),
	}
	for _, id := range persisted.InstalledAgents {
		selection.Agents = append(selection.Agents, model.AgentID(id))
	}
	for _, tool := range persisted.CommunityTools {
		selection.CommunityTools = append(selection.CommunityTools, model.CommunityToolID(tool))
	}
	return syncBackupTargets(homeDir, "", selection, resolveAdapters(selection.Agents))
}
