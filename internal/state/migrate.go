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
	if fromVersion >= CurrentSchemaVersion && !legacyCodeGraphPresent(current) {
		return MigrationResult{State: current, Report: MigrationReport{
			Schema: MigrationReportSchema, FromVersion: fromVersion, ToVersion: fromVersion,
			RawStateDigest: digestBytes(raw), SelectionResolved: true,
		}}, nil
	}

	backupID := migrationBackupID(raw, managedPaths)
	backupDir := MigrationBackupDir(homeDir, backupID)
	if err := os.MkdirAll(filepath.Join(backupDir, "paths"), 0o700); err != nil {
		return MigrationResult{}, fmt.Errorf("create migration backup: %w", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "state.json"), raw, 0o600); err != nil {
		return MigrationResult{}, fmt.Errorf("snapshot raw state: %w", err)
	}

	report := MigrationReport{
		Schema:           MigrationReportSchema,
		FromVersion:      fromVersion,
		ToVersion:        CurrentSchemaVersion,
		CreatedAt:        time.Now().UTC(),
		RawStateDigest:   digestBytes(raw),
		RawStateSnapshot: filepath.ToSlash(filepath.Join(filepath.Base(backupDir), "state.json")),
		BackupID:         backupID,
		BackupPath:       filepath.ToSlash(filepath.Join(migrationBackupRoot, backupID)),
	}
	manifest, err := snapshotMigrationPaths(backupDir, managedPaths)
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

// ResolveSelection replaces unresolved retired agent values with an explicitly
// selected retained client. A nil/partial mapping is rejected and leaves the
// report pending, so planning cannot silently drop a user's old selection.
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
		if !strings.HasPrefix(unresolved, "agent:") {
			return MigrationResult{State: current, Report: report}, &SelectionRequiredError{Report: report}
		}
		old := strings.TrimPrefix(unresolved, "agent:")
		replacement, ok := replacements[old]
		if !ok || !model.IsPersonalClient(replacement) {
			return MigrationResult{State: current, Report: report}, &SelectionRequiredError{Report: report}
		}
		for i, installed := range current.InstalledAgents {
			if installed == old {
				current.InstalledAgents[i] = string(replacement)
			}
		}
	}
	current.InstalledAgents = dedupStrings(current.InstalledAgents)
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
	if report.Schema != MigrationReportSchema || strings.TrimSpace(report.BackupID) == "" {
		return errors.New("invalid migration report")
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

func snapshotMigrationPaths(backupDir string, paths []string) ([]MigrationPathEntry, error) {
	paths = dedupAbsolutePaths(paths)
	entries := make([]MigrationPathEntry, 0, len(paths))
	for index, path := range paths {
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
			if err := os.WriteFile(backupPath, data, 0o600); err != nil {
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
	backupDir := MigrationBackupDir(homeDir, report.BackupID)
	stateBackup, err := os.ReadFile(filepath.Join(backupDir, "state.json"))
	if err != nil {
		return fmt.Errorf("read raw state snapshot: %w", err)
	}
	if digestBytes(stateBackup) != report.RawStateDigest {
		return errors.New("raw state snapshot digest does not match migration report")
	}
	for _, entry := range report.RollbackManifest {
		if err := restoreMigrationPath(backupDir, entry); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(Path(homeDir)), 0o755); err != nil {
		return err
	}
	if _, err := filemerge.WriteFileAtomic(Path(homeDir), stateBackup, 0o644); err != nil {
		return fmt.Errorf("restore raw state: %w", err)
	}
	return nil
}

func restoreMigrationPath(backupDir string, entry MigrationPathEntry) error {
	if strings.TrimSpace(entry.Path) == "" || !filepath.IsAbs(entry.Path) {
		return fmt.Errorf("migration rollback path %q is not absolute", entry.Path)
	}
	if err := removeMigrationTarget(entry.Path, entry.Exists); err != nil {
		return err
	}
	if !entry.Exists || entry.Kind == "absent" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(entry.Path), 0o755); err != nil {
		return err
	}
	switch entry.Kind {
	case "symlink":
		if err := os.Symlink(entry.Target, entry.Path); err != nil {
			return fmt.Errorf("restore managed symlink %q: %w", entry.Path, err)
		}
	case "file":
		data, err := os.ReadFile(filepath.Join(backupDir, filepath.FromSlash(entry.Snapshot)))
		if err != nil {
			return fmt.Errorf("read managed snapshot %q: %w", entry.Path, err)
		}
		if digestBytes(data) != entry.Digest {
			return fmt.Errorf("managed snapshot digest mismatch for %q", entry.Path)
		}
		if err := os.WriteFile(entry.Path, data, os.FileMode(entry.Mode)&0o777); err != nil {
			return fmt.Errorf("restore managed path %q: %w", entry.Path, err)
		}
	default:
		return fmt.Errorf("unsupported migration snapshot kind %q", entry.Kind)
	}
	return nil
}

func removeMigrationTarget(path string, previouslyExists bool) error {
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
	_ = previouslyExists
	return nil
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

func digestBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}
