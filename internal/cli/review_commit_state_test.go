package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/reviewtransaction"
)

func TestReviewCandidatePreservesStagedAndUnstagedCommitState(t *testing.T) {
	repo := initReviewCLIRepo(t)
	path := filepath.Join(repo, "tracked.txt")
	if err := os.WriteFile(path, []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runReviewCLIGit(t, repo, "add", "tracked.txt")
	if err := os.WriteFile(path, []byte("unstaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	staged, err := (reviewtransaction.SnapshotBuilder{Repo: repo}).Build(context.Background(), reviewtransaction.Target{
		Kind: reviewtransaction.TargetCurrentChanges, Projection: reviewtransaction.ProjectionStaged, IntendedUntracked: []string{},
	})
	if err != nil {
		t.Fatalf("staged candidate = %v", err)
	}
	if got := strings.TrimSpace(runReviewCLIGit(t, repo, "show", staged.CandidateTree+":tracked.txt")); got != "staged" {
		t.Fatalf("staged candidate content = %q, want staged bytes", got)
	}

	workspace, err := (reviewtransaction.SnapshotBuilder{Repo: repo}).Build(context.Background(), reviewtransaction.Target{
		Kind: reviewtransaction.TargetCurrentChanges, Projection: reviewtransaction.ProjectionWorkspace, IntendedUntracked: []string{},
	})
	if err != nil {
		t.Fatalf("workspace candidate = %v", err)
	}
	if got := strings.TrimSpace(runReviewCLIGit(t, repo, "show", workspace.CandidateTree+":tracked.txt")); got != "unstaged" {
		t.Fatalf("workspace candidate content = %q, want unstaged bytes", got)
	}

	// Building either projection must never mutate the caller's real index or
	// worktree. The staged candidate remains the exact index snapshot above.
	if got := strings.TrimSpace(runReviewCLIGit(t, repo, "show", staged.CandidateTree+":tracked.txt")); got != "staged" {
		t.Fatalf("staged candidate changed after workspace build = %q", got)
	}
}

func TestReviewCommittedOnlySelectorPreservesCommitAmSemantics(t *testing.T) {
	reviewEnabledHome(t)
	repo := initReviewCLIRepo(t)
	parent := strings.TrimSpace(runReviewCLIGit(t, repo, "rev-parse", "HEAD"))
	path := filepath.Join(repo, "tracked.txt")
	if err := os.WriteFile(path, []byte("commit-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runReviewCLIGit(t, repo, "commit", "-am", "commit tracked change")

	status := negotiatedStartStatus(t, repo, "--base-ref", parent, "--committed-only", "--lineage", "review-commit-a")
	started := executeStartTransition(t, repo, status)
	if status.Projection.Kind != reviewtransaction.TargetBaseDiff || started.Projection != reviewtransaction.ProjectionWorkspace {
		t.Fatalf("committed-only START target = %#v, want base-diff/workspace", started)
	}
	if started.ChangedFiles != 1 || status.TargetIdentity == "" {
		t.Fatalf("committed-only START = %#v, want one committed path", started)
	}
	if got := strings.TrimSpace(runReviewCLIGit(t, repo, "show", status.Projection.CurrentCandidateTree+":tracked.txt")); got != "commit-a" {
		t.Fatalf("committed-only candidate content = %q, want committed bytes", got)
	}
}

func TestReviewStagedProjectionPreservesEmptyIndex(t *testing.T) {
	repo := initReviewCLIRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("worktree-only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runReviewCLIGit(t, repo, "read-tree", "--empty")

	snapshot, err := (reviewtransaction.SnapshotBuilder{Repo: repo}).Build(context.Background(), reviewtransaction.Target{
		Kind: reviewtransaction.TargetCurrentChanges, Projection: reviewtransaction.ProjectionStaged, IntendedUntracked: []string{},
	})
	if err != nil {
		t.Fatalf("empty-index staged candidate = %v", err)
	}
	if got := strings.TrimSpace(runReviewCLIGit(t, repo, "write-tree")); got != snapshot.CandidateTree {
		t.Fatalf("empty-index candidate tree = %q, want real empty-index tree %q", snapshot.CandidateTree, got)
	}
	if len(snapshot.Paths) != 1 || snapshot.Paths[0] != "tracked.txt" {
		t.Fatalf("empty-index changed paths = %#v, want tracked deletion", snapshot.Paths)
	}
}
