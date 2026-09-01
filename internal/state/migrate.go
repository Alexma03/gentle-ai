package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gentleman-programming/gentle-ai/v2/internal/components/filemerge"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

const (
	// CurrentSchemaVersion is the first canonical personal-stack state shape.
	// A missing schema_version in an existing state file is version zero.
	CurrentSchemaVersion = 1
	// MigrationReportSchema identifies the durable report contract. Keep this
	// versioned independently from InstallState so readers can fail closed when
	// a future report shape is introduced.
	MigrationReportSchema = "gentle-ai.state-migration/v1"
	migrationReportFile   = "migration-report.json"
	migrationBackupRoot   = "migration-backups"
)

var ErrMigrationSelectionRequired = errors.New("state migration requires user selection")

// MigrationMapping records a value that was carried into the canonical state
// shape rather than silently discarded.
type MigrationMapping struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason,omitempty"`
}

// MigrationPathEntry is one immutable managed-path snapshot in the rollback
// manifest. Snapshot is relative to the migration backup directory.
type MigrationPathEntry struct {
	Path     string `json:"path"`
	Snapshot string `json:"snapshot"`
	Kind     string `json:"kind"`
	Exists   bool   `json:"exists"`
	Mode     uint32 `json:"mode,omitempty"`
	Digest   string `json:"digest,omitempty"`
	Target   string `json:"target,omitempty"`
}

// MigrationReport is the durable audit record for one state migration. The
// report is intentionally additive: the raw state and rollback manifest stay
// available until an operator explicitly resolves or restores the migration.
type MigrationReport struct {
	Schema            string               `json:"schema"`
	FromVersion       int                  `json:"from_version"`
	ToVersion         int                  `json:"to_version"`
	CreatedAt         time.Time            `json:"created_at"`
	RawStateDigest    string               `json:"raw_state_digest"`
	RawStateSnapshot  string               `json:"raw_state_snapshot"`
	BackupID          string               `json:"backup_id"`
	BackupPath        string               `json:"backup_path"`
	Mapped            []MigrationMapping   `json:"mapped,omitempty"`
	Retired           []string             `json:"retired,omitempty"`
	Unresolved        []string             `json:"unresolved,omitempty"`
	AuthorizedRoots   []string             `json:"authorized_roots,omitempty"`
	RollbackManifest  []MigrationPathEntry `json:"rollback_manifest,omitempty"`
	SelectionResolved bool                 `json:"selection_resolved,omitempty"`
	Restored          bool                 `json:"restored,omitempty"`
}

func (r MigrationReport) NeedsSelection() bool {
	return len(r.Unresolved) > 0 && !r.SelectionResolved && !r.Restored
}

// MigrationResult contains both the canonical state proposal and its durable
// report. It is returned together with ErrMigrationSelectionRequired so a UI
// can present the exact unresolved values without guessing.
type MigrationResult struct {
	State  InstallState
	Report MigrationReport
}

func (r MigrationResult) NeedsSelection() bool { return r.Report.NeedsSelection() }

// SelectionRequiredError carries the report that blocked planning.
type SelectionRequiredError struct {
	Report MigrationReport
}

func (e *SelectionRequiredError) Error() string {
	if len(e.Report.Unresolved) == 0 {
		return ErrMigrationSelectionRequired.Error()
	}
	return fmt.Sprintf("%v: resolve %s before planning", ErrMigrationSelectionRequired, strings.Join(e.Report.Unresolved, ", "))
}

func (e *SelectionRequiredError) Unwrap() error { return ErrMigrationSelectionRequired }

// MigrationReportPath returns the report's stable location adjacent to
// state.json.
func MigrationReportPath(homeDir string) string {
	return filepath.Join(homeDir, stateDir, migrationReportFile)
}

// MigrationBackupDir returns the durable backup directory for a report.
func MigrationBackupDir(homeDir, backupID string) string {
	return filepath.Join(homeDir, stateDir, migrationBackupRoot, backupID)
}

// ReadMigrationReport loads the current migration report. A missing report is
// returned as os.ErrNotExist so callers can distinguish a fresh home.
func ReadMigrationReport(homeDir string) (MigrationReport, error) {
	data, err := os.ReadFile(MigrationReportPath(homeDir))
	if err != nil {
		return MigrationReport{}, err
	}
	var report MigrationReport
	if err := json.Unmarshal(data, &report); err != nil {
		return MigrationReport{}, fmt.Errorf("decode migration report: %w", err)
	}
	if report.Schema != MigrationReportSchema {
		return MigrationReport{}, fmt.Errorf("unsupported migration report schema %q", report.Schema)
	}
	if _, _, _, err := validateMigrationReport(homeDir, report); err != nil {
		return MigrationReport{}, err
	}
	return report, nil
}

// RequireMigrationResolved is the planning gate for a home with an active
// migration report. A missing report is a fresh/legacy-compatible home and is
// allowed to proceed; an unresolved report must be handled explicitly rather
// than silently dropping retired selections.
func RequireMigrationResolved(homeDir string) error {
	report, err := ReadMigrationReport(homeDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read state migration report: %w", err)
	}
	if report.NeedsSelection() {
		return &SelectionRequiredError{Report: report}
	}
	return nil
}

// Migrate snapshots raw state and managed paths, maps legacy CodeGraph state
// into the canonical component field, records retired/unresolved values, and
// writes the versioned state plus adjacent report. The caller must hold the
// existing install-state lock; use MigrateWithLock from CLI code.
func Migrate(homeDir string, managedPaths []string) (MigrationResult, error) {
	raw, err := os.ReadFile(Path(homeDir))
	if err != nil {
		return MigrationResult{}, err
	}
	var current InstallState
	if err := json.Unmarshal(raw, &current); err != nil {
		return MigrationResult{}, fmt.Errorf("decode state for migration: %w", err)
	}
	fromVersion := persistedSchemaVersion(raw)
	if fromVersion > CurrentSchemaVersion {
		return MigrationResult{}, fmt.Errorf("unsupported install state schema version %d (current %d)", fromVersion, CurrentSchemaVersion)
	}
	if fromVersion == CurrentSchemaVersion && !legacyCodeGraphPresent(current) {
		return MigrationResult{State: current, Report: MigrationReport{
			Schema: MigrationReportSchema, FromVersion: fromVersion, ToVersion: fromVersion,
			RawStateDigest: digestBytes(raw), SelectionResolved: true,
		}}, nil
	}

	backupID := migrationBackupID(raw, managedPaths)
	report := MigrationReport{
		Schema:           MigrationReportSchema,
		FromVersion:      fromVersion,
		ToVersion:        CurrentSchemaVersion,
		CreatedAt:        time.Now().UTC(),
		RawStateDigest:   digestBytes(raw),
		RawStateSnapshot: "state.json",
		BackupID:         backupID,
		BackupPath:       filepath.ToSlash(filepath.Join(migrationBackupRoot, backupID)),
	}
	authorizedRoots, err := migrationAuthorizedRoots(homeDir)
	if err != nil {
		return MigrationResult{}, err
	}
	report.AuthorizedRoots = authorizedRoots
	for _, path := range dedupAbsolutePaths(managedPaths) {
		if _, err := validateMigrationTargetPath(homeDir, authorizedRoots, path); err != nil {
			return MigrationResult{}, err
		}
	}
	backupDir := MigrationBackupDir(homeDir, backupID)
	if err := os.MkdirAll(filepath.Join(backupDir, "paths"), 0o700); err != nil {
		return MigrationResult{}, fmt.Errorf("create migration backup: %w", err)
	}
	if err := writeMigrationFileAtomically(filepath.Join(backupDir, "state.json"), raw, 0o600); err != nil {
		return MigrationResult{}, fmt.Errorf("snapshot raw state: %w", err)
	}
	manifest, err := snapshotMigrationPaths(homeDir, backupDir, authorizedRoots, managedPaths)
	if err != nil {
		return MigrationResult{}, err
	}
	report.RollbackManifest = manifest

	canonical := current
	canonical.SchemaVersion = CurrentSchemaVersion
	canonical.Components = append([]model.ComponentID(nil), current.Components...)
	if containsComponent(canonical.Components, model.ComponentCodeGraph) == false && legacyCodeGraphPresent(current) {
		canonical.Components = append(canonical.Components, model.ComponentCodeGraph)
		report.Mapped = append(report.Mapped, MigrationMapping{
			From: "community_tools:codegraph", To: "components:codegraph", Reason: "CodeGraph is now a first-class lifecycle component",
		})
	}
	canonical.Components = dedupComponents(canonical.Components)
	// CodeGraph is represented by the canonical component after migration. The
	// old community-tools value is retained only in the raw backup/report so a
	// second runtime allowlist cannot remain active in the migrated state.
	canonical.CommunityTools = slicesWithoutCodeGraph(current.CommunityTools)
	for _, id := range canonical.InstalledAgents {
		if !model.IsPersonalClient(model.AgentID(id)) {
			report.Retired = appendUnique(report.Retired, id)
			report.Unresolved = appendUnique(report.Unresolved, "agent:"+id)
		}
	}
	for _, component := range canonical.Components {
		if !knownComponent(component) {
			report.Unresolved = appendUnique(report.Unresolved, "component:"+string(component))
		}
	}
	for _, tool := range canonical.CommunityTools {
		if tool != string(model.CommunityToolCodeGraph) {
			report.Retired = appendUnique(report.Retired, "community_tool:"+tool)
			report.Unresolved = appendUnique(report.Unresolved, "community_tool:"+tool)
		}
	}
	sort.Strings(report.Retired)
	sort.Strings(report.Unresolved)
	if len(report.Unresolved) == 0 {
		report.SelectionResolved = true
	}

	if err := Write(homeDir, canonical); err != nil {
		return MigrationResult{State: canonical, Report: report}, fmt.Errorf("write migrated state: %w", err)
	}
	if err := writeMigrationReport(homeDir, report); err != nil {
		_ = restoreRawStateAndPaths(homeDir, report)
		return MigrationResult{State: current, Report: report}, fmt.Errorf("write migration report: %w", err)
	}
	result := MigrationResult{State: canonical, Report: report}
	if report.NeedsSelection() {
		return result, &SelectionRequiredError{Report: report}
	}
	return result, nil
}

// MigrateWithLock adapts the repository's existing state-lock helper without
// importing CLI/reviewtransaction into this package (which would create a
// cycle). The callback must execute its operation while the lock is held.
func MigrateWithLock(homeDir string, managedPaths []string, withLock func(func() error) error) (result MigrationResult, err error) {
	if withLock == nil {
		return result, errors.New("state migration lock callback is required")
	}
	err = withLock(func() error {
		result, err = Migrate(homeDir, managedPaths)
		return err
	})
	return result, err
}

// ResolveSelection replaces unresolved retired values with explicitly selected
// retained IDs. The replacement value uses AgentID as the historical string
// token type; for component: and community_tool: entries it is interpreted as
// the corresponding canonical component/tool ID. An empty replacement for a
// component or community tool is an explicit user choice to retire that value.
// A nil/partial mapping is rejected and leaves the report pending, so planning
// cannot silently drop a user's old selection.
func ResolveSelection(homeDir string, replacements map[string]model.AgentID) (MigrationResult, error) {
	report, err := ReadMigrationReport(homeDir)
	if err != nil {
		return MigrationResult{}, err
	}
	current, err := Read(homeDir)
	if err != nil {
		return MigrationResult{}, err
	}
	if !report.NeedsSelection() {
		return MigrationResult{State: current, Report: report}, nil
	}
	for _, unresolved := range report.Unresolved {
		replacement, ok := migrationReplacement(replacements, unresolved)
		if !ok {
			return MigrationResult{State: current, Report: report}, &SelectionRequiredError{Report: report}
		}
		switch {
		case strings.HasPrefix(unresolved, "agent:"):
			if !model.IsPersonalClient(replacement) {
				return MigrationResult{State: current, Report: report}, &SelectionRequiredError{Report: report}
			}
			old := strings.TrimPrefix(unresolved, "agent:")
			for i, installed := range current.InstalledAgents {
				if installed == old {
					current.InstalledAgents[i] = string(replacement)
				}
			}
		case strings.HasPrefix(unresolved, "component:"):
			old := model.ComponentID(strings.TrimPrefix(unresolved, "component:"))
			if replacement != "" && !knownComponent(model.ComponentID(replacement)) {
				return MigrationResult{State: current, Report: report}, &SelectionRequiredError{Report: report}
			}
			filtered := current.Components[:0]
			for _, component := range current.Components {
				if component == old {
					if replacement != "" {
						filtered = append(filtered, model.ComponentID(replacement))
					}
					continue
				}
				filtered = append(filtered, component)
			}
			current.Components = filtered
		case strings.HasPrefix(unresolved, "community_tool:"):
			old := strings.TrimPrefix(unresolved, "community_tool:")
			filtered := current.CommunityTools[:0]
			for _, tool := range current.CommunityTools {
				if tool == old {
					if replacement != "" {
						if model.CommunityToolID(replacement) != model.CommunityToolCodeGraph {
							return MigrationResult{State: current, Report: report}, &SelectionRequiredError{Report: report}
						}
						filtered = append(filtered, string(model.CommunityToolCodeGraph))
					}
					continue
				}
				filtered = append(filtered, tool)
			}
			current.CommunityTools = filtered
		default:
			return MigrationResult{State: current, Report: report}, &SelectionRequiredError{Report: report}
		}
	}
	current.Components = dedupComponents(current.Components)
	current.InstalledAgents = dedupStrings(current.InstalledAgents)
	current.CommunityTools = dedupStrings(current.CommunityTools)
	report.Unresolved = nil
	report.SelectionResolved = true
	if err := Write(homeDir, current); err != nil {
		return MigrationResult{State: current, Report: report}, err
	}
	if err := writeMigrationReport(homeDir, report); err != nil {
		return MigrationResult{State: current, Report: report}, err
	}
	return MigrationResult{State: current, Report: report}, nil
}

func migrationReplacement(replacements map[string]model.AgentID, unresolved string) (model.AgentID, bool) {
	if replacement, ok := replacements[unresolved]; ok {
		return replacement, true
	}
	if separator := strings.IndexByte(unresolved, ':'); separator >= 0 {
		replacement, ok := replacements[unresolved[separator+1:]]
		return replacement, ok
	}
	return "", false
}

func ResolveSelectionWithLock(homeDir string, replacements map[string]model.AgentID, withLock func(func() error) error) (result MigrationResult, err error) {
	if withLock == nil {
		return result, errors.New("state migration lock callback is required")
	}
	err = withLock(func() error {
		result, err = ResolveSelection(homeDir, replacements)
		return err
	})
	return result, err
}

// RestoreMigration restores state.json and every managed path from the report
// backup. It is deliberately explicit and idempotent; the report remains as
// an audit record and is marked restored after the raw state is back.
func RestoreMigration(homeDir string, report MigrationReport) error {
	if _, _, _, err := validateMigrationReport(homeDir, report); err != nil {
		return err
	}
	if err := restoreRawStateAndPaths(homeDir, report); err != nil {
		return err
	}
	report.Restored = true
	report.SelectionResolved = true
	return writeMigrationReport(homeDir, report)
}

func RestoreMigrationFromDisk(homeDir string) error {
	report, err := ReadMigrationReport(homeDir)
	if err != nil {
		return err
	}
	return RestoreMigration(homeDir, report)
}

func writeMigrationReport(homeDir string, report MigrationReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(MigrationReportPath(homeDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	_, err = filemerge.WriteFileAtomic(MigrationReportPath(homeDir), append(data, '\n'), 0o644)
	return err
}

// migrationAuthorizedRoots currently scopes migration rollback to the home
// directory. State migration is a home-level transaction; callers must not
// use it as a way to snapshot or restore arbitrary workspace paths.
func migrationAuthorizedRoots(homeDir string) ([]string, error) {
	root, err := canonicalExistingPath(homeDir)
	if err != nil {
		return nil, fmt.Errorf("resolve migration authorization root: %w", err)
	}
	return []string{root}, nil
}

func validateMigrationReport(homeDir string, report MigrationReport) ([]string, string, string, error) {
	if report.Schema != MigrationReportSchema || !validMigrationBackupID(report.BackupID) {
		return nil, "", "", errors.New("invalid migration report")
	}
	authorizedRoots, err := migrationAuthorizedRoots(homeDir)
	if err != nil {
		return nil, "", "", err
	}
	if len(report.AuthorizedRoots) > 0 {
		authorizedRoots = authorizedRoots[:0]
		canonicalHome := authorizedRootsForHome(homeDir)
		for _, rawRoot := range report.AuthorizedRoots {
			root, rootErr := canonicalExistingPath(rawRoot)
			if rootErr != nil {
				return nil, "", "", fmt.Errorf("invalid migration authorization root %q: %w", rawRoot, rootErr)
			}
			if !pathWithinRoot(canonicalHome, root) {
				return nil, "", "", fmt.Errorf("migration authorization root %q escapes home directory", rawRoot)
			}
			authorizedRoots = appendUniquePath(authorizedRoots, root)
		}
		if len(authorizedRoots) == 0 {
			return nil, "", "", errors.New("migration report has no authorized roots")
		}
	}

	backupDir := MigrationBackupDir(homeDir, report.BackupID)
	expectedBackupPath := filepath.ToSlash(filepath.Join(migrationBackupRoot, report.BackupID))
	if report.BackupPath != expectedBackupPath {
		return nil, "", "", fmt.Errorf("migration backup path %q is not authorized", report.BackupPath)
	}
	if _, err := validateMigrationTargetPath(homeDir, []string{authorizedRootsForHome(homeDir)}, backupDir); err != nil {
		return nil, "", "", fmt.Errorf("validate migration backup path: %w", err)
	}
	backupCanonical, err := canonicalExistingPath(backupDir)
	if err != nil {
		return nil, "", "", fmt.Errorf("resolve migration backup path: %w", err)
	}
	if !pathWithinRoot(authorizedRootsForHome(homeDir), backupCanonical) {
		return nil, "", "", fmt.Errorf("migration backup path %q escapes home directory", backupDir)
	}
	if info, statErr := os.Lstat(backupDir); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, "", "", fmt.Errorf("migration backup path %q is not a directory", backupDir)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, "", "", fmt.Errorf("inspect migration backup path %q: %w", backupDir, statErr)
	}
	stateSnapshot, err := validateMigrationSnapshotPath(backupDir, report.RawStateSnapshot)
	if err != nil {
		return nil, "", "", fmt.Errorf("validate raw state snapshot: %w", err)
	}
	seenTargets := make(map[string]struct{}, len(report.RollbackManifest))
	for _, entry := range report.RollbackManifest {
		target, err := validateMigrationTargetPath(homeDir, authorizedRoots, entry.Path)
		if err != nil {
			return nil, "", "", err
		}
		if _, exists := seenTargets[target]; exists {
			return nil, "", "", fmt.Errorf("duplicate migration rollback path %q", entry.Path)
		}
		seenTargets[target] = struct{}{}
		if !entry.Exists || entry.Kind == "absent" {
			if entry.Exists || entry.Kind != "absent" {
				return nil, "", "", fmt.Errorf("invalid absent migration entry for %q", entry.Path)
			}
			if _, err := validateMigrationSnapshotPath(backupDir, entry.Snapshot); err != nil {
				return nil, "", "", err
			}
			continue
		}
		switch entry.Kind {
		case "file", "symlink":
		default:
			return nil, "", "", fmt.Errorf("unsupported migration snapshot kind %q", entry.Kind)
		}
		if _, err := validateMigrationSnapshotPath(backupDir, entry.Snapshot); err != nil {
			return nil, "", "", err
		}
	}
	return authorizedRoots, backupDir, stateSnapshot, nil
}

func authorizedRootsForHome(homeDir string) string {
	root, err := canonicalExistingPath(homeDir)
	if err != nil {
		return filepath.Clean(homeDir)
	}
	return root
}

func validateMigrationTargetPath(homeDir string, authorizedRoots []string, rawPath string) (string, error) {
	if strings.TrimSpace(rawPath) == "" || !filepath.IsAbs(rawPath) {
		return "", fmt.Errorf("migration rollback path %q is not absolute", rawPath)
	}
	clean := filepath.Clean(rawPath)
	if clean != rawPath {
		return "", fmt.Errorf("migration rollback path %q is not canonical", rawPath)
	}
	parent, err := canonicalExistingPath(filepath.Dir(clean))
	if err != nil {
		return "", fmt.Errorf("resolve migration rollback parent %q: %w", rawPath, err)
	}
	resolved := filepath.Join(parent, filepath.Base(clean))
	for _, rootRaw := range authorizedRoots {
		root, rootErr := canonicalExistingPath(rootRaw)
		if rootErr != nil {
			return "", fmt.Errorf("resolve migration authorized root %q: %w", rootRaw, rootErr)
		}
		if pathWithinRoot(root, resolved) {
			return clean, nil
		}
	}
	return "", fmt.Errorf("migration rollback path %q escapes authorized roots under %q", rawPath, homeDir)
}

func validateMigrationSnapshotPath(baseDir, rawPath string) (string, error) {
	if strings.TrimSpace(rawPath) == "" {
		return "", errors.New("migration snapshot path is empty")
	}
	rel := filepath.FromSlash(rawPath)
	if filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return "", fmt.Errorf("migration snapshot path %q must be relative", rawPath)
	}
	clean := filepath.Clean(rel)
	if clean != rel || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("migration snapshot path %q escapes backup directory", rawPath)
	}
	joined := filepath.Join(baseDir, clean)
	base, err := canonicalExistingPath(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve migration backup directory: %w", err)
	}
	parent, err := canonicalExistingPath(filepath.Dir(joined))
	if err != nil {
		return "", fmt.Errorf("resolve migration snapshot parent: %w", err)
	}
	resolved := filepath.Join(parent, filepath.Base(joined))
	if !pathWithinRoot(base, resolved) {
		return "", fmt.Errorf("migration snapshot path %q escapes backup directory", rawPath)
	}
	return joined, nil
}

func canonicalExistingPath(rawPath string) (string, error) {
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return "", err
	}
	current := filepath.Clean(absPath)
	var suffix []string
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for index := len(suffix) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, suffix[index])
			}
			return filepath.Clean(resolved), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return current, nil
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func pathWithinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func appendUniquePath(paths []string, value string) []string {
	for _, current := range paths {
		if current == value {
			return paths
		}
	}
	return append(paths, value)
}

func snapshotMigrationPaths(homeDir, backupDir string, authorizedRoots, paths []string) ([]MigrationPathEntry, error) {
	paths = dedupAbsolutePaths(paths)
	entries := make([]MigrationPathEntry, 0, len(paths))
	for index, path := range paths {
		if _, err := validateMigrationTargetPath(homeDir, authorizedRoots, path); err != nil {
			return nil, err
		}
		entry := MigrationPathEntry{Path: path, Snapshot: filepath.ToSlash(filepath.Join("paths", fmt.Sprintf("%06d", index)))}
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			entry.Kind = "absent"
			entries = append(entries, entry)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("snapshot managed path %q: %w", path, err)
		}
		entry.Exists = true
		entry.Mode = uint32(info.Mode())
		backupPath := filepath.Join(backupDir, filepath.FromSlash(entry.Snapshot))
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return nil, fmt.Errorf("read managed symlink %q: %w", path, err)
			}
			entry.Kind, entry.Target = "symlink", target
		case info.Mode().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read managed path %q: %w", path, err)
			}
			if err := writeMigrationFileAtomically(backupPath, data, 0o600); err != nil {
				return nil, fmt.Errorf("write managed snapshot %q: %w", path, err)
			}
			entry.Kind, entry.Digest = "file", digestBytes(data)
		default:
			return nil, fmt.Errorf("managed migration path %q is not a regular file or symlink", path)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func restoreRawStateAndPaths(homeDir string, report MigrationReport) error {
	plan, err := prepareMigrationRestore(homeDir, report)
	if err != nil {
		return err
	}

	// Every report and snapshot is validated and read before the first target is
	// touched. This is the important atomicity boundary for a tampered report:
	// a bad later entry cannot leave an earlier managed path restored.
	for index := range plan.paths {
		if err := applyMigrationRestorePath(plan.paths[index].target, plan.paths[index].desired); err != nil {
			rollbackErr := rollbackMigrationRestore(plan.paths[:index+1], plan.statePath, plan.statePrevious)
			if rollbackErr != nil {
				return errors.Join(fmt.Errorf("restore managed path: %w", err), rollbackErr)
			}
			return err
		}
	}
	if err := applyMigrationRestorePath(plan.statePath, plan.stateDesired); err != nil {
		rollbackErr := rollbackMigrationRestore(plan.paths, plan.statePath, plan.statePrevious)
		if rollbackErr != nil {
			return errors.Join(fmt.Errorf("restore raw state: %w", err), rollbackErr)
		}
		return fmt.Errorf("restore raw state: %w", err)
	}
	return nil
}

type migrationRestorePlan struct {
	statePath     string
	stateDesired  migrationRestoreTarget
	statePrevious migrationRestoreTarget
	paths         []migrationRestorePath
}

type migrationRestorePath struct {
	target   string
	desired  migrationRestoreTarget
	previous migrationRestoreTarget
}

type migrationRestoreTarget struct {
	exists bool
	kind   string
	mode   os.FileMode
	data   []byte
	target string
}

func prepareMigrationRestore(homeDir string, report MigrationReport) (migrationRestorePlan, error) {
	_, backupDir, stateSnapshot, err := validateMigrationReport(homeDir, report)
	if err != nil {
		return migrationRestorePlan{}, err
	}
	stateBackup, err := os.ReadFile(stateSnapshot)
	if err != nil {
		return migrationRestorePlan{}, fmt.Errorf("read raw state snapshot: %w", err)
	}
	stateInfo, err := os.Lstat(stateSnapshot)
	if err != nil {
		return migrationRestorePlan{}, fmt.Errorf("inspect raw state snapshot: %w", err)
	}
	if !stateInfo.Mode().IsRegular() {
		return migrationRestorePlan{}, errors.New("raw state snapshot is not a regular file")
	}
	if digestBytes(stateBackup) != report.RawStateDigest {
		return migrationRestorePlan{}, errors.New("raw state snapshot digest does not match migration report")
	}

	plan := migrationRestorePlan{
		statePath: Path(homeDir),
	}
	plan.statePrevious, err = readMigrationRestoreTarget(plan.statePath)
	if err != nil {
		return migrationRestorePlan{}, err
	}
	plan.stateDesired = migrationRestoreTarget{exists: true, kind: "file", mode: 0o644, data: stateBackup}
	plan.paths = make([]migrationRestorePath, 0, len(report.RollbackManifest))
	for _, entry := range report.RollbackManifest {
		desired, snapshotErr := migrationRestoreTargetFromEntry(backupDir, entry)
		if snapshotErr != nil {
			return migrationRestorePlan{}, snapshotErr
		}
		previous, currentErr := readMigrationRestoreTarget(entry.Path)
		if currentErr != nil {
			return migrationRestorePlan{}, currentErr
		}
		plan.paths = append(plan.paths, migrationRestorePath{target: entry.Path, desired: desired, previous: previous})
	}
	return plan, nil
}

func migrationRestoreTargetFromEntry(backupDir string, entry MigrationPathEntry) (migrationRestoreTarget, error) {
	if !entry.Exists || entry.Kind == "absent" {
		if entry.Exists || entry.Kind != "absent" {
			return migrationRestoreTarget{}, fmt.Errorf("invalid absent migration entry for %q", entry.Path)
		}
		return migrationRestoreTarget{}, nil
	}
	switch entry.Kind {
	case "symlink":
		if strings.TrimSpace(entry.Target) == "" {
			return migrationRestoreTarget{}, fmt.Errorf("empty symlink target for %q", entry.Path)
		}
		return migrationRestoreTarget{exists: true, kind: "symlink", mode: os.FileMode(entry.Mode), target: entry.Target}, nil
	case "file":
		path, err := validateMigrationSnapshotPath(backupDir, entry.Snapshot)
		if err != nil {
			return migrationRestoreTarget{}, err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return migrationRestoreTarget{}, fmt.Errorf("inspect managed snapshot %q: %w", entry.Path, err)
		}
		if !info.Mode().IsRegular() {
			return migrationRestoreTarget{}, fmt.Errorf("managed snapshot %q is not a regular file", entry.Path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return migrationRestoreTarget{}, fmt.Errorf("read managed snapshot %q: %w", entry.Path, err)
		}
		if digestBytes(data) != entry.Digest {
			return migrationRestoreTarget{}, fmt.Errorf("managed snapshot digest mismatch for %q", entry.Path)
		}
		return migrationRestoreTarget{exists: true, kind: "file", mode: os.FileMode(entry.Mode), data: data}, nil
	default:
		return migrationRestoreTarget{}, fmt.Errorf("unsupported migration snapshot kind %q", entry.Kind)
	}
}

func readMigrationRestoreTarget(path string) (migrationRestoreTarget, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return migrationRestoreTarget{}, nil
	}
	if err != nil {
		return migrationRestoreTarget{}, fmt.Errorf("inspect current migration path %q: %w", path, err)
	}
	if info.IsDir() {
		return migrationRestoreTarget{}, fmt.Errorf("refuse to restore over directory at migration path %q", path)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return migrationRestoreTarget{}, fmt.Errorf("read current migration symlink %q: %w", path, err)
		}
		return migrationRestoreTarget{exists: true, kind: "symlink", mode: info.Mode(), target: target}, nil
	}
	if !info.Mode().IsRegular() {
		return migrationRestoreTarget{}, fmt.Errorf("current migration path %q is not a regular file or symlink", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return migrationRestoreTarget{}, fmt.Errorf("read current migration path %q: %w", path, err)
	}
	return migrationRestoreTarget{exists: true, kind: "file", mode: info.Mode(), data: data}, nil
}

func applyMigrationRestorePath(path string, desired migrationRestoreTarget) error {
	if !desired.exists {
		return removeMigrationTarget(path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	switch desired.kind {
	case "symlink":
		if err := removeMigrationTarget(path); err != nil {
			return err
		}
		if err := os.Symlink(desired.target, path); err != nil {
			return fmt.Errorf("restore managed symlink %q: %w", path, err)
		}
	case "file":
		if err := writeMigrationFileAtomically(path, desired.data, desired.mode); err != nil {
			return fmt.Errorf("restore managed path %q: %w", path, err)
		}
	default:
		return fmt.Errorf("unsupported migration restore kind %q", desired.kind)
	}
	return nil
}

func writeMigrationFileAtomically(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".gentle-ai-migration-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode.Perm()); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func removeMigrationTarget(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect migration rollback path %q: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("refuse to remove directory at migration rollback path %q", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove current migration path %q: %w", path, err)
	}
	return nil
}

func rollbackMigrationRestore(paths []migrationRestorePath, statePath string, statePrevious migrationRestoreTarget) error {
	var rollbackErr error
	for index := len(paths) - 1; index >= 0; index-- {
		if err := applyMigrationRestorePath(paths[index].target, paths[index].previous); err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	if err := applyMigrationRestorePath(statePath, statePrevious); err != nil {
		rollbackErr = errors.Join(rollbackErr, err)
	}
	return rollbackErr
}

func persistedSchemaVersion(raw []byte) int {
	var fields struct {
		SchemaVersion int `json:"schema_version"`
	}
	if json.Unmarshal(raw, &fields) != nil {
		return 0
	}
	return fields.SchemaVersion
}

func legacyCodeGraphPresent(s InstallState) bool {
	for _, tool := range s.CommunityTools {
		if tool == string(model.CommunityToolCodeGraph) {
			return true
		}
	}
	return false
}

func knownComponent(component model.ComponentID) bool {
	switch component {
	case model.ComponentEngram, model.ComponentSDD, model.ComponentSkills, model.ComponentContext7,
		model.ComponentCodeGraph, model.ComponentPersona, model.ComponentPermission, model.ComponentGGA,
		model.ComponentTheme, model.ComponentClaudeTheme, model.ComponentOpenCodeGentleLogo:
		return true
	default:
		return false
	}
}

func containsComponent(values []model.ComponentID, want model.ComponentID) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func dedupComponents(values []model.ComponentID) []model.ComponentID {
	seen := make(map[model.ComponentID]struct{}, len(values))
	result := make([]model.ComponentID, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func dedupStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func appendUnique(values []string, value string) []string {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

func dedupAbsolutePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		abs = filepath.Clean(abs)
		if _, exists := seen[abs]; exists {
			continue
		}
		seen[abs] = struct{}{}
		result = append(result, abs)
	}
	sort.Strings(result)
	return result
}

func migrationBackupID(raw []byte, paths []string) string {
	hash := sha256.New()
	_, _ = hash.Write(raw)
	for _, path := range dedupAbsolutePaths(paths) {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(path))
	}
	return hex.EncodeToString(hash.Sum(nil))[:24]
}

func validMigrationBackupID(id string) bool {
	if len(id) != 24 {
		return false
	}
	for _, r := range id {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

func slicesWithoutCodeGraph(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value != string(model.CommunityToolCodeGraph) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func digestBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}
