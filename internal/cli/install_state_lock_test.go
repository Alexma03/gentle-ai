package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/reviewtransaction"
	"github.com/gentleman-programming/gentle-ai/v2/internal/state"
	"github.com/gentleman-programming/gentle-ai/v2/internal/statecoord"
)

func TestPersistSyncManagedAssetStateReReadsLatestStateAfterLockContention(t *testing.T) {
	home := t.TempDir()
	held, err := reviewtransaction.AcquireAuthorityFileLock(mustInstallStateLockPath(t, home))
	if err != nil {
		t.Fatal(err)
	}
	if err := persistSyncManagedAssetState(home, model.Selection{}, "sha256:current-writer", ""); err == nil || !strings.Contains(err.Error(), "acquire install state lock") {
		t.Fatalf("contended sync state persistence error = %v", err)
	}
	if err := state.Write(home, state.InstallState{Persona: "concurrent"}); err != nil {
		t.Fatal(err)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if err := persistSyncManagedAssetState(home, model.Selection{CommunityTools: []model.CommunityToolID{model.CommunityToolCodeGraph}}, "sha256:current-writer", ""); err != nil {
		t.Fatal(err)
	}
	got, err := state.Read(home)
	if err != nil || got.Persona != "concurrent" || got.ManagedAssetDigest != "sha256:current-writer" || !got.CommunityToolsConfigured || len(got.CommunityTools) != 1 || got.CommunityTools[0] != string(model.CommunityToolCodeGraph) {
		t.Fatalf("persisted sync state = %#v, err = %v", got, err)
	}
}

func TestPersistSyncManagedAssetStateRefusesCorruptLatestState(t *testing.T) {
	home := t.TempDir()
	if err := state.Write(home, state.InstallState{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state.Path(home), []byte("{not valid json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := persistSyncManagedAssetState(home, model.Selection{}, "sha256:current-writer", ""); err == nil || !strings.Contains(err.Error(), "run `gentle-ai install`") {
		t.Fatalf("corrupt sync state persistence error = %v", err)
	}
}

func mustInstallStateLockPath(t *testing.T, home string) string {
	t.Helper()
	lockPath, err := statecoord.LockPath(home)
	if err != nil {
		t.Fatalf("install state lock path: %v", err)
	}
	return lockPath
}
