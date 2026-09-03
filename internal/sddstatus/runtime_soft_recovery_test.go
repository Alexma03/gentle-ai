package sddstatus

import (
	"context"
	"strings"
	"testing"
)

func TestRuntimeAttemptCeilingsAreAdvisoryAfterFailure(t *testing.T) {
	repo := initRuntimeLedgerRepo(t)
	store := mustRuntimeStore(t, repo, "soft-attempt-ceiling")

	started, err := store.Begin(context.Background(), BeginAttemptRequest{
		RequestID: "soft-ceiling-begin-1", WorkUnit: "apply-calendar",
		EvidenceGoal: "prove calendar integration", MaxAttempts: 1, MaxChangedLines: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	failed, err := store.Finish(context.Background(), FinishAttemptRequest{
		ExpectedRevision: started.Revision, RequestID: "soft-ceiling-finish-1", Outcome: AttemptFailed,
		EvidenceRevision: runtimeTestHash('a'), Diagnosis: "focused gate found stale assertions",
		HarnessDisposition: HarnessInvalidated, CleanupEvidence: "test process exited",
		ProcessEvidence: "partial test evidence was retained",
	})
	if err != nil {
		t.Fatal(err)
	}
	if failed.DecisionRequired || failed.NextAction != RuntimeActionBegin {
		t.Fatalf("first failed one-shot attempt became a hard blocker: %#v", failed)
	}

	retried, err := store.Begin(context.Background(), BeginAttemptRequest{
		ExpectedRevision: failed.Revision, RequestID: "soft-ceiling-begin-2", WorkUnit: "apply-calendar",
		EvidenceGoal: "prove calendar integration", MaxAttempts: 1, MaxChangedLines: 1,
	})
	if err != nil {
		t.Fatalf("diagnosed retry was refused by advisory accounting: %v", err)
	}
	if retried.ActiveAttempt == nil || retried.ActiveAttempt.Ordinal != 2 {
		t.Fatalf("retry did not open the next recorded attempt: %#v", retried)
	}
}

func TestRuntimeReadinessTreatsLegacyBudgetDecisionAsRetryable(t *testing.T) {
	status := RuntimeStatus{
		DecisionRequired: true,
		NextAction:       RuntimeActionReset,
		Objective: &RuntimeObjective{
			WorkUnit: "apply-calendar", EvidenceGoal: "prove calendar integration", MaxAttempts: 1,
		},
		CumulativeAttempts: 1,
	}

	result, terminal := runtimeReadiness(runtimeReadinessInput{Status: status})
	if terminal || result.State == CompactStateBlocked {
		t.Fatalf("legacy accounting decision stayed a hard blocker: result=%#v terminal=%v", result, terminal)
	}
}

func TestPassedAttemptCompletesWhenHistoricalLineCeilingIsExceeded(t *testing.T) {
	repo := initRuntimeLedgerRepo(t)
	store := mustRuntimeStore(t, repo, "soft-line-ceiling")
	started, err := store.Begin(context.Background(), BeginAttemptRequest{
		RequestID: "soft-lines-begin", WorkUnit: "apply-calendar",
		EvidenceGoal: "prove calendar integration", MaxAttempts: 1, MaxChangedLines: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	appendRuntimeLedgerFile(t, repo, "one\ntwo\n")
	finished, err := store.Finish(context.Background(), FinishAttemptRequest{
		ExpectedRevision: started.Revision, RequestID: "soft-lines-finish", Outcome: AttemptPassed,
		EvidenceRevision: runtimeTestHash('b'), Diagnosis: "focused and broad gates passed",
		HarnessDisposition: HarnessReused, CleanupEvidence: "test process exited",
		ProcessEvidence: "no descendants remained",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !finished.Complete || finished.DecisionRequired || finished.NextAction != RuntimeActionComplete {
		t.Fatalf("passing evidence was rejected by advisory line telemetry: %#v", finished)
	}
	last := finished.Attempts[len(finished.Attempts)-1]
	if !last.ChangedLineBudgetExceeded {
		t.Fatalf("advisory line telemetry was not preserved: %#v", last)
	}
	next, err := store.Begin(context.Background(), BeginAttemptRequest{
		ExpectedRevision: finished.Revision, RequestID: "soft-lines-next", WorkUnit: "verify-calendar",
		EvidenceGoal: "verify calendar integration", MaxAttempts: 1, MaxChangedLines: 1,
	})
	if err != nil || next.ActiveAttempt == nil || next.ActiveAttempt.WorkUnit != "verify-calendar" {
		t.Fatalf("advisory line telemetry blocked the next work unit: status=%#v err=%v", next, err)
	}
}

func TestRuntimeInstructionsDoNotCreateOneShotBudgets(t *testing.T) {
	status := Status{RuntimeStatus: &RuntimeStatus{}}
	status.ActionContext.WorkspaceRoot = "/workspace/repo"
	joined := strings.Join(nativeRuntimeInstructions(status, "calendar-change"), "\n")
	if strings.Contains(joined, "--max-attempts") {
		t.Fatalf("routine instructions still delegate attempt policy to the caller:\n%s", joined)
	}
	for _, want := range []string{"diagnosed acquire", "change strategy", "without asking a human to reset accounting"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("runtime instructions omit %q:\n%s", want, joined)
		}
	}
}
