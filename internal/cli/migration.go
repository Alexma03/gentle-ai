package cli

import (
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
