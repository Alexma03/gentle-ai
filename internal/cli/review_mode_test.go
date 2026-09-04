package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gentleman-programming/gentle-ai/v2/internal/reviewtransaction"
	"github.com/gentleman-programming/gentle-ai/v2/internal/state"
)

func TestReviewModeDisableGlobalWinsOverEveryRepository(t *testing.T) {
	home := reviewModeHome(t)
	repo := initReviewCLIRepo(t)

	var output bytes.Buffer
	if err := RunReviewMode([]string{"disable", "--cwd", repo, "--json"}, &output); err != nil {
		t.Fatalf("RunReviewMode(disable) error = %v", err)
	}
	if result := decodeReviewModeResult(t, output.Bytes()); result.Status.Effective != reviewtransaction.RDDModeOff ||
		result.Status.Source != reviewtransaction.RDDModeSourceGlobal {
		t.Fatalf("global disable result = %#v", result.Status)
	}
	persisted, err := state.Read(home)
	if err != nil {
		t.Fatalf("state.Read error = %v", err)
	}
	if persisted.RDDMode != string(reviewtransaction.RDDModeOff) || persisted.RDDModeRecordedAt == nil {
		t.Fatalf("global disable was not persisted in user state: %#v", persisted)
	}

	output.Reset()
	if err := RunReviewMode([]string{"enable", "--cwd", repo, "--json"}, &output); err != nil {
		t.Fatalf("RunReviewMode(enable) error = %v", err)
	}
	if result := decodeReviewModeResult(t, output.Bytes()); result.Status.Effective != reviewtransaction.RDDModeOn {
		t.Fatalf("global enable result = %#v", result.Status)
	}
}

func TestReviewModeGlobalScopeWorksFromNonGitDirectory(t *testing.T) {
	home := reviewModeHome(t)
	nonGit := t.TempDir()

	var output bytes.Buffer
	if err := RunReviewMode([]string{"status", "--cwd", nonGit, "--json"}, &output); err != nil {
		t.Fatalf("unset global status from non-Git cwd error = %v\n%s", err, output.String())
	}
	if before := decodeReviewModeResult(t, output.Bytes()); before.Status.Effective != reviewtransaction.RDDModeOn ||
		before.Status.Source != reviewtransaction.RDDModeSourceDefault ||
		before.Status.Global != reviewtransaction.RDDModeUnset || before.Status.CloneLocal != reviewtransaction.RDDModeUnset {
		t.Fatalf("unset global status from non-Git cwd = %#v", before.Status)
	}

	output.Reset()
	if err := RunReviewMode([]string{"enable", "--cwd", nonGit, "--scope", "global", "--json"}, &output); err != nil {
		t.Fatalf("global enable from non-Git cwd error = %v\n%s", err, output.String())
	}
	result := decodeReviewModeResult(t, output.Bytes())
	if result.Operation != "enable" || result.Scope != reviewModeScopeGlobal ||
		result.Status.Effective != reviewtransaction.RDDModeOn ||
		result.Status.Source != reviewtransaction.RDDModeSourceGlobal ||
		result.Status.Global != reviewtransaction.RDDModeOn ||
		result.Status.CloneLocal != reviewtransaction.RDDModeUnset {
		t.Fatalf("global enable from non-Git cwd = %#v", result)
	}
	persisted, err := state.Read(home)
	if err != nil {
		t.Fatalf("state.Read error = %v", err)
	}
	if persisted.RDDMode != string(reviewtransaction.RDDModeOn) || persisted.RDDModeRecordedAt == nil {
		t.Fatalf("global enable did not persist an explicit on: %#v", persisted)
	}
	if entries, err := os.ReadDir(nonGit); err != nil || len(entries) != 0 {
		t.Fatalf("global enable touched non-Git cwd: entries=%v err=%v", entries, err)
	}

	output.Reset()
	if err := RunReviewMode([]string{"status", "--cwd", nonGit, "--json"}, &output); err != nil {
		t.Fatalf("global status from non-Git cwd error = %v\n%s", err, output.String())
	}
	status := decodeReviewModeResult(t, output.Bytes())
	if status.Operation != "status" || status.Scope != reviewModeScopeBoth ||
		status.Status.Effective != reviewtransaction.RDDModeOn ||
		status.Status.Source != reviewtransaction.RDDModeSourceGlobal ||
		status.Status.Global != reviewtransaction.RDDModeOn ||
		status.Status.CloneLocal != reviewtransaction.RDDModeUnset {
		t.Fatalf("global status from non-Git cwd = %#v", status)
	}
}

func TestReviewModeCloneScopeOutsideGitFailsBeforeWriting(t *testing.T) {
	home := reviewModeHome(t)
	nonGit := t.TempDir()

	var output bytes.Buffer
	err := RunReviewMode([]string{"disable", "--cwd", nonGit, "--scope", "clone", "--json"}, &output)
	if err == nil || !strings.Contains(err.Error(), "clone-local review mode requires a Git repository") ||
		!strings.Contains(err.Error(), "--cwd") || !strings.Contains(err.Error(), "--scope global") ||
		strings.Contains(err.Error(), "fatal:") || strings.Contains(err.Error(), "git rev-parse") || strings.Contains(err.Error(), "exit code 128") {
		t.Fatalf("clone disable outside Git error = %v", err)
	}
	if !reviewtransaction.ReviewRootResolutionReportsNoRepository(err) {
		t.Fatalf("clone disable outside Git lost its typed no-repository classification: %v", err)
	}
	if _, readErr := state.Read(home); !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("clone disable outside Git mutated global state: %v", readErr)
	}
	if entries, readErr := os.ReadDir(nonGit); readErr != nil || len(entries) != 0 {
		t.Fatalf("clone disable outside Git touched cwd: entries=%v err=%v", entries, readErr)
	}
}

func TestReviewModeRepositoryRequiredRefusalDoesNotDependOnGitStderrLanguage(t *testing.T) {
	localized := &reviewtransaction.GitCommandError{Args: []string{"rev-parse", "--show-toplevel"}, ExitCode: 128, Output: "fatal: no es un repositorio Git"}
	refusal := reviewModeRepositoryRequiredRefusal(localized)
	if refusal == nil || !reviewtransaction.ReviewRootResolutionReportsNoRepository(refusal) {
		t.Fatalf("localized no-repository error was not classified: %v", refusal)
	}
	if strings.Contains(refusal.Error(), localized.Output) {
		t.Fatalf("localized Git stderr reached the operator refusal: %v", refusal)
	}
}

// TestReviewModeGlobalEnableRemainsAnExplicitOpinion proves an explicit enable
// remains distinguishable from the personal default and survives across clones.
func TestReviewModeGlobalEnableRemainsAnExplicitOpinion(t *testing.T) {
	home := reviewModeHome(t)
	repo := initReviewCLIRepo(t)

	var output bytes.Buffer
	if err := RunReviewMode([]string{"status", "--cwd", repo, "--json"}, &output); err != nil {
		t.Fatalf("RunReviewMode(status) error = %v", err)
	}
	if before := decodeReviewModeResult(t, output.Bytes()); before.Status.Effective != reviewtransaction.RDDModeOn ||
		before.Status.Source != reviewtransaction.RDDModeSourceDefault {
		t.Fatalf("a fresh clone did not inherit the personal default: %#v", before.Status)
	}

	output.Reset()
	if err := RunReviewMode([]string{"enable", "--cwd", repo, "--scope", "global", "--json"}, &output); err != nil {
		t.Fatalf("RunReviewMode(enable global) error = %v", err)
	}
	if result := decodeReviewModeResult(t, output.Bytes()); result.Status.Effective != reviewtransaction.RDDModeOn ||
		result.Status.Source != reviewtransaction.RDDModeSourceGlobal ||
		result.Status.Global != reviewtransaction.RDDModeOn {
		t.Fatalf("global enable result = %#v", result.Status)
	}

	persisted, err := state.Read(home)
	if err != nil {
		t.Fatalf("state.Read error = %v", err)
	}
	if persisted.RDDMode != string(reviewtransaction.RDDModeOn) || persisted.RDDModeRecordedAt == nil {
		t.Fatalf("global enable did not persist an explicit on: %#v", persisted)
	}

	// The persisted opinion, not the process that wrote it, is what survives an
	// upgrade: a later status in a different clone reads the same explicit on.
	other := initReviewCLIRepo(t)
	output.Reset()
	if err := RunReviewMode([]string{"status", "--cwd", other, "--json"}, &output); err != nil {
		t.Fatalf("RunReviewMode(status other clone) error = %v", err)
	}
	if after := decodeReviewModeResult(t, output.Bytes()); after.Status.Effective != reviewtransaction.RDDModeOn ||
		after.Status.Source != reviewtransaction.RDDModeSourceGlobal {
		t.Fatalf("an explicitly enabled user lost reviews: %#v", after.Status)
	}
}

// TestReviewModeCloneScopeDisablesOnlyThisClone needs a user who opted in
// globally: the property under test is that a clone-local off does not travel
// to a second clone, and that is only observable when something other than the
// override says on.
func TestReviewModeCloneScopeDisablesOnlyThisClone(t *testing.T) {
	reviewEnabledHome(t)
	repo := initReviewCLIRepo(t)

	var output bytes.Buffer
	if err := RunReviewMode([]string{"disable", "--cwd", repo, "--scope", "clone", "--json"}, &output); err != nil {
		t.Fatalf("RunReviewMode(disable clone) error = %v", err)
	}
	result := decodeReviewModeResult(t, output.Bytes())
	if result.Status.Effective != reviewtransaction.RDDModeOff ||
		result.Status.Source != reviewtransaction.RDDModeSourceCloneLocal ||
		result.Status.Revision == "" {
		t.Fatalf("clone disable result = %#v", result.Status)
	}

	clone := filepath.Join(t.TempDir(), "clone")
	runReviewCLIGit(t, repo, "clone", "-q", repo, clone)
	output.Reset()
	if err := RunReviewMode([]string{"status", "--cwd", clone, "--json"}, &output); err != nil {
		t.Fatalf("RunReviewMode(status clone) error = %v", err)
	}
	if cloned := decodeReviewModeResult(t, output.Bytes()); cloned.Status.Effective != reviewtransaction.RDDModeOn {
		t.Fatalf("second clone inherited the override: %#v", cloned.Status)
	}
}

func TestReviewModeCloneScopeEnableIsIdempotentWhenGlobalOn(t *testing.T) {
	home := reviewModeHome(t)
	repo := initReviewCLIRepo(t)
	if err := state.Write(home, state.InstallState{RDDMode: string(reviewtransaction.RDDModeOn)}); err != nil {
		t.Fatalf("state.Write error = %v", err)
	}

	var output bytes.Buffer
	err := RunReviewMode([]string{"enable", "--cwd", repo, "--scope", "clone", "--expected-revision", "", "--json"}, &output)
	if err != nil {
		t.Fatalf("clearing an absent clone override must succeed while global mode is on: %v", err)
	}
	if result := decodeReviewModeResult(t, output.Bytes()); result.Status.Effective != reviewtransaction.RDDModeOn ||
		result.Status.Source != reviewtransaction.RDDModeSourceGlobal || result.Status.Revision != "" {
		t.Fatalf("clone enable result = %#v", result.Status)
	}
	if _, err := os.Lstat(filepath.Join(repo, ".git", "gentle-ai")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("idempotent clone enable created repository state: %v", err)
	}
}

// TestReviewModeCloneScopeEnableMigratesLegacyRevision seeds the clone-local
// override against an explicit global "on", so the fixture opts in the same
// way: clearing the override has to land back on that global opinion, and
// against the opt-in default it would land on off and hide the migration.
func TestReviewModeCloneScopeEnableMigratesLegacyRevision(t *testing.T) {
	reviewEnabledHome(t)
	repo := initReviewCLIRepo(t)
	ctx := context.Background()
	disabled, err := reviewtransaction.SetCloneLocalRDDMode(ctx, repo, reviewtransaction.RDDModeOff, "", reviewtransaction.RDDGlobalMode{Value: "on"})
	if err != nil {
		t.Fatalf("seed clone-local override: %v", err)
	}
	current, err := reviewtransaction.CloneLocalRDDModeRecordPath(ctx, repo)
	if err != nil {
		t.Fatalf("current record path: %v", err)
	}
	legacyBytes, err := os.ReadFile(current)
	if err != nil {
		t.Fatalf("read current record: %v", err)
	}
	legacyRoot := filepath.Join(repo, ".git", "gentle-ai", "review-transactions")
	// The seeding write publishes into both locations, because the switch is
	// machine state rather than build state (#3284). This fixture is the clone
	// that only ever had the pre-relocation one, so its mirror is dropped
	// before the relocated root takes that name.
	if err := os.RemoveAll(legacyRoot); err != nil {
		t.Fatalf("drop the mirrored fixture copy: %v", err)
	}
	if err := os.Rename(filepath.Join(repo, ".git", "gentle-ai", "review-mode"), legacyRoot); err != nil {
		t.Fatalf("relocate secure legacy fixture: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(repo, ".git", "gentle-ai", "review-mode")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy fixture left a separately created private directory: %v", err)
	}
	legacy := filepath.Join(legacyRoot, "rar-authority", "v1", "rdd-mode", filepath.Base(current))

	var output bytes.Buffer
	if err := RunReviewMode([]string{"status", "--cwd", repo, "--json"}, &output); err != nil {
		t.Fatalf("legacy status: %v", err)
	}
	status := decodeReviewModeResult(t, output.Bytes()).Status
	if status.Revision != disabled.Revision || status.CloneLocal != reviewtransaction.RDDModeOff {
		t.Fatalf("legacy CLI status = %#v", status)
	}
	output.Reset()
	if err := RunReviewMode([]string{"enable", "--cwd", repo, "--scope", "clone", "--expected-revision", status.Revision, "--json"}, &output); err != nil {
		t.Fatalf("legacy CLI enable: %v", err)
	}
	migrated := decodeReviewModeResult(t, output.Bytes()).Status
	if !migrated.Enabled() || migrated.Revision == "" || migrated.Revision == status.Revision {
		t.Fatalf("migrated CLI status = %#v", migrated)
	}
	if after, err := os.ReadFile(legacy); err != nil || !bytes.Equal(after, legacyBytes) {
		t.Fatalf("legacy CLI bytes changed: err=%v", err)
	}
}

func TestReviewModeCloneScopeEnableRejectsGlobalOffWithoutLocalOverride(t *testing.T) {
	home := reviewModeHome(t)
	repo := initReviewCLIRepo(t)
	if err := state.Write(home, state.InstallState{RDDMode: string(reviewtransaction.RDDModeOff)}); err != nil {
		t.Fatalf("state.Write error = %v", err)
	}

	var output bytes.Buffer
	err := RunReviewMode([]string{"enable", "--cwd", repo, "--scope", "clone", "--json"}, &output)
	var disabled *reviewtransaction.RDDDisabledError
	if !errors.As(err, &disabled) || !errors.Is(err, reviewtransaction.ErrRDDDisabled) ||
		disabled.Source != reviewtransaction.RDDModeSourceGlobal {
		t.Fatalf("clone enable error = %v, want global typed disabled error", err)
	}
	if !strings.Contains(err.Error(), "gentle-ai review mode enable --scope=global") {
		t.Fatalf("clone enable error does not name the global continuation: %v", err)
	}
	if result := decodeReviewModeResult(t, output.Bytes()); result.Status.Effective != reviewtransaction.RDDModeOff ||
		result.Status.Source != reviewtransaction.RDDModeSourceGlobal || result.Status.Revision != "" {
		t.Fatalf("clone enable result = %#v", result.Status)
	}
	if _, err := os.Lstat(filepath.Join(repo, ".git", "gentle-ai")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected clone enable created repository state: %v", err)
	}
}

func TestReviewModeCloneScopeEnableRejectsLegacyInheritWhileGlobalOff(t *testing.T) {
	home := reviewModeHome(t)
	repo := initReviewCLIRepo(t)
	global := reviewtransaction.RDDGlobalMode{Value: string(reviewtransaction.RDDModeOn)}
	disabled, err := reviewtransaction.SetCloneLocalRDDMode(context.Background(), repo, reviewtransaction.RDDModeOff, "", global)
	if err != nil {
		t.Fatalf("SetCloneLocalRDDMode(off) error = %v", err)
	}
	inherited, err := reviewtransaction.SetCloneLocalRDDMode(context.Background(), repo, reviewtransaction.RDDModeUnset, disabled.Revision, global)
	if err != nil {
		t.Fatalf("SetCloneLocalRDDMode(inherit) error = %v", err)
	}
	if err := state.Write(home, state.InstallState{RDDMode: string(reviewtransaction.RDDModeOff)}); err != nil {
		t.Fatalf("state.Write error = %v", err)
	}
	record, err := reviewtransaction.CloneLocalRDDModeRecordPath(context.Background(), repo)
	if err != nil {
		t.Fatalf("CloneLocalRDDModeRecordPath error = %v", err)
	}
	before, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("read inherited record: %v", err)
	}

	var output bytes.Buffer
	err = RunReviewMode([]string{"enable", "--cwd", repo, "--scope", "clone", "--json"}, &output)
	var blocked *reviewtransaction.RDDDisabledError
	if !errors.As(err, &blocked) || blocked.Source != reviewtransaction.RDDModeSourceGlobal {
		t.Fatalf("legacy inherit clone enable error = %v, want global typed disabled error", err)
	}
	recordAfter, err := reviewtransaction.CloneLocalRDDModeRecordPath(context.Background(), repo)
	if err != nil {
		t.Fatalf("CloneLocalRDDModeRecordPath after retry error = %v", err)
	}
	after, err := os.ReadFile(recordAfter)
	if err != nil {
		t.Fatalf("read inherited record after retry: %v", err)
	}
	if recordAfter != record || !bytes.Equal(after, before) {
		t.Fatalf("legacy inherit retry published a new generation")
	}
	if result := decodeReviewModeResult(t, output.Bytes()); result.Status.Revision != inherited.Revision ||
		result.Status.Source != reviewtransaction.RDDModeSourceGlobal {
		t.Fatalf("legacy inherit clone enable result = %#v", result.Status)
	}
}

func TestReviewModeCloneScopeEnableRejectsExplicitOffWhileGlobalOff(t *testing.T) {
	home := reviewModeHome(t)
	repo := initReviewCLIRepo(t)
	disabled, err := reviewtransaction.SetCloneLocalRDDMode(
		context.Background(), repo, reviewtransaction.RDDModeOff, "", reviewtransaction.RDDGlobalMode{Value: string(reviewtransaction.RDDModeOn)})
	if err != nil {
		t.Fatalf("SetCloneLocalRDDMode(off) error = %v", err)
	}
	if err := state.Write(home, state.InstallState{RDDMode: string(reviewtransaction.RDDModeOff)}); err != nil {
		t.Fatalf("state.Write error = %v", err)
	}
	record, err := reviewtransaction.CloneLocalRDDModeRecordPath(context.Background(), repo)
	if err != nil {
		t.Fatalf("CloneLocalRDDModeRecordPath error = %v", err)
	}
	before, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("read explicit-off record: %v", err)
	}

	var output bytes.Buffer
	err = RunReviewMode([]string{"enable", "--cwd", repo, "--scope", "clone", "--json"}, &output)
	var blocked *reviewtransaction.RDDDisabledError
	if !errors.As(err, &blocked) || !errors.Is(err, reviewtransaction.ErrRDDDisabled) ||
		blocked.Source != reviewtransaction.RDDModeSourceGlobal {
		t.Fatalf("explicit-off clone enable error = %v, want global typed disabled error", err)
	}
	if !strings.Contains(err.Error(), "gentle-ai review mode enable --scope=global") {
		t.Fatalf("explicit-off clone enable error does not name the global continuation: %v", err)
	}
	result := decodeReviewModeResult(t, output.Bytes())
	if result.Status.Effective != reviewtransaction.RDDModeOff || result.Status.CloneLocal != reviewtransaction.RDDModeOff ||
		result.Status.Revision != disabled.Revision {
		t.Fatalf("explicit-off clone enable result = %#v", result.Status)
	}
	recordAfter, err := reviewtransaction.CloneLocalRDDModeRecordPath(context.Background(), repo)
	if err != nil {
		t.Fatalf("CloneLocalRDDModeRecordPath after rejected enable error = %v", err)
	}
	after, err := os.ReadFile(recordAfter)
	if err != nil {
		t.Fatalf("read explicit-off record after rejected enable: %v", err)
	}
	if recordAfter != record || !bytes.Equal(after, before) {
		t.Fatalf("explicit-off clone enable published a new generation")
	}
}

func TestReviewModeCloneScopeDisableIsIdempotent(t *testing.T) {
	home := reviewModeHome(t)
	repo := initReviewCLIRepo(t)
	if err := state.Write(home, state.InstallState{RDDMode: string(reviewtransaction.RDDModeOn)}); err != nil {
		t.Fatalf("state.Write error = %v", err)
	}

	var output bytes.Buffer
	if err := RunReviewMode([]string{"disable", "--cwd", repo, "--scope", "clone", "--json"}, &output); err != nil {
		t.Fatalf("first clone disable: %v", err)
	}
	first := decodeReviewModeResult(t, output.Bytes()).Status
	if first.Global != reviewtransaction.RDDModeOn || first.CloneLocal != reviewtransaction.RDDModeOff ||
		first.Source != reviewtransaction.RDDModeSourceCloneLocal {
		t.Fatalf("seeded clone disable status = %#v", first)
	}
	record, err := reviewtransaction.CloneLocalRDDModeRecordPath(context.Background(), repo)
	if err != nil {
		t.Fatalf("CloneLocalRDDModeRecordPath error = %v", err)
	}
	before, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("read disabled record: %v", err)
	}

	output.Reset()
	if err := RunReviewMode([]string{"disable", "--cwd", repo, "--scope", "clone", "--json"}, &output); err != nil {
		t.Fatalf("repeated clone disable: %v", err)
	}
	second := decodeReviewModeResult(t, output.Bytes()).Status
	recordAfter, err := reviewtransaction.CloneLocalRDDModeRecordPath(context.Background(), repo)
	if err != nil {
		t.Fatalf("CloneLocalRDDModeRecordPath after retry error = %v", err)
	}
	after, err := os.ReadFile(recordAfter)
	if err != nil {
		t.Fatalf("read disabled record after retry: %v", err)
	}
	if second.Revision != first.Revision || recordAfter != record || !bytes.Equal(after, before) {
		t.Fatalf("repeated clone disable published a new generation: first=%#v second=%#v", first, second)
	}
}

func TestReviewModeReportsUnknownPersistedModeAsDisabled(t *testing.T) {
	home := reviewModeHome(t)
	repo := initReviewCLIRepo(t)
	if err := state.Write(home, state.InstallState{RDDMode: "sometimes"}); err != nil {
		t.Fatalf("state.Write error = %v", err)
	}

	var output bytes.Buffer
	err := RunReviewMode([]string{"status", "--cwd", repo, "--json"}, &output)
	if !errors.Is(err, reviewtransaction.ErrRDDModeUnknown) {
		t.Fatalf("unknown persisted mode error = %v, want ErrRDDModeUnknown", err)
	}
	if !strings.Contains(output.String(), string(reviewtransaction.RDDModeOff)) {
		t.Fatalf("unknown persisted mode did not report a disabled projection:\n%s", output.String())
	}
}

func TestReviewModeRejectsUnknownSubcommandAndScope(t *testing.T) {
	reviewModeHome(t)
	repo := initReviewCLIRepo(t)
	var output bytes.Buffer
	if err := RunReviewMode([]string{"toggle", "--cwd", repo}, &output); err == nil ||
		!strings.Contains(err.Error(), "unknown review mode command") {
		t.Fatalf("unknown subcommand error = %v", err)
	}
	if err := RunReviewMode([]string{"disable", "--cwd", repo, "--scope", "team"}, &output); err == nil ||
		!strings.Contains(err.Error(), "unknown review mode scope") {
		t.Fatalf("unknown scope error = %v", err)
	}
}

// TestReviewStartIsRejectedWhileTheKillSwitchIsOff proves a disabled START
// reports its actual refusal without manufacturing persistent authority or a
// receipt merely to exercise the negative path.
func TestReviewStartIsRejectedWhileTheKillSwitchIsOff(t *testing.T) {
	reviewEnabledHome(t)
	repo := initReviewCLIRepo(t)
	writeReviewStartCandidate(t, repo, "docs/guide.md", "ordinary documentation\n", 0o644)

	var modeOutput bytes.Buffer
	if err := RunReviewMode([]string{"disable", "--cwd", repo, "--scope", "clone", "--json"}, &modeOutput); err != nil {
		t.Fatalf("disable clone-local review mode: %v", err)
	}

	var startOutput bytes.Buffer
	err := RunReviewFacadeStart([]string{"--cwd", repo, "--lineage", "review-kill-switch"}, &startOutput)
	var disabled *reviewtransaction.RDDDisabledError
	if !errors.As(err, &disabled) {
		t.Fatalf("disabled review start error = %v, want *RDDDisabledError", err)
	}
	if disabled.Operation != reviewtransaction.RDDOperationStart ||
		disabled.Source != reviewtransaction.RDDModeSourceCloneLocal {
		t.Fatalf("disabled review start = %#v", disabled)
	}
	if !errors.Is(err, reviewtransaction.ErrRDDDisabled) {
		t.Fatalf("disabled review start does not unwrap to ErrRDDDisabled: %v", err)
	}
	stores, err := reviewtransaction.DiscoverCompactStores(context.Background(), repo)
	if err != nil {
		t.Fatalf("discover stores after disabled START: %v", err)
	}
	if len(stores) != 0 {
		t.Fatalf("disabled START created persistent review authority: %#v", stores)
	}
}

func TestTierZeroReviewStartNeverAsksForConsent(t *testing.T) {
	reviewEnabledHome(t)
	repo := initReviewCLIRepo(t)
	console := stubReviewConsole(t, true, "1\n")
	writeReviewStartCandidate(t, repo, "docs/guide.md", "ordinary documentation\n", 0o644)

	var output bytes.Buffer
	if err := RunReviewFacadeStart([]string{"--cwd", repo, "--lineage", "review-tier-zero"}, &output); err != nil {
		t.Fatalf("tier 0 start: %v\n%s", err, output.String())
	}
	var started ReviewFacadeStartResult
	decodeStrictReviewJSON(t, output.Bytes(), &started)
	if started.RiskLevel != reviewtransaction.RiskLow || len(started.SelectedLenses) != 0 {
		t.Fatalf("tier 0 START = %#v", started)
	}
	if console.Len() != 0 {
		t.Fatalf("tier 0 emitted a consent prompt:\n%s", console.String())
	}
}

// stubReviewConsole replaces the console seam so the one-time question can be
// driven without a terminal. It returns the buffer the question is written to.
func stubReviewConsole(t *testing.T, interactive bool, answer string) *bytes.Buffer {
	t.Helper()
	previous := reviewConsole
	buffer := &bytes.Buffer{}
	reviewConsole = func() reviewConsentSession {
		return reviewConsentSession{Interactive: interactive, Input: strings.NewReader(answer), Output: buffer}
	}
	t.Cleanup(func() { reviewConsole = previous })
	return buffer
}

func reviewModeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

// reviewEnabledHome is reviewModeHome for a user who opted in. Receipt-driven
// development is off until someone explicitly enables it, so a test whose
// subject is the review lifecycle -- rather than the switch itself -- has to
// opt in the way a real user does before a review will start at all. It writes
// the same explicit global "on" that `gentle-ai review mode enable` persists,
// rather than reaching past the switch, so these fixtures keep exercising the
// resolution path they are meant to run through.
//
// The opinion lives in the user's home directory, which is process-wide state
// reached through t.Setenv. Go forbids t.Setenv in a test that also calls
// t.Parallel, so a test that opts in cannot be parallel: there is no
// repository-scoped way to assert "on" (a clone may only ever assert "off").
func reviewEnabledHome(t *testing.T) string {
	t.Helper()
	home := reviewModeHome(t)
	recordedAt := time.Now().UTC()
	if err := state.Write(home, state.InstallState{
		RDDMode:           string(reviewtransaction.RDDModeOn),
		RDDModeRecordedAt: &recordedAt,
	}); err != nil {
		t.Fatalf("enable review mode for this test: %v", err)
	}
	return home
}

func decodeReviewModeResult(t *testing.T, payload []byte) ReviewModeResult {
	t.Helper()
	var result ReviewModeResult
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("decode review mode result: %v\n%s", err, payload)
	}
	return result
}
