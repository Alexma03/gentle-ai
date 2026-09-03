package sddstatus

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRuntimeUnboundedObjectiveIgnoresChangedLineExhaustion(t *testing.T) {
	repo := initRuntimeLedgerRepo(t)
	store := mustRuntimeStore(t, repo, "unbounded-lines")

	started, err := store.Begin(context.Background(), BeginAttemptRequest{
		RequestID: "unbounded-begin", WorkUnit: "large-unit", EvidenceGoal: "finish without a line ceiling",
		MaxAttempts: 2, MaxChangedLines: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if started.Objective == nil || started.Objective.MaxChangedLines != 0 {
		t.Fatalf("new objective line limit = %#v, want unbounded zero", started.Objective)
	}

	appendRuntimeLedgerFile(t, repo, strings.Repeat("changed line\n", DefaultRuntimeChangedLines+25))
	finished, err := store.Finish(context.Background(), FinishAttemptRequest{
		ExpectedRevision: started.Revision, RequestID: "unbounded-finish", Outcome: AttemptPassed,
		EvidenceRevision: runtimeTestHash('a'), Diagnosis: "large work unit passed",
		HarnessDisposition: HarnessReused, CleanupEvidence: "cleanup completed", ProcessEvidence: "no descendants",
	})
	if err != nil {
		t.Fatal(err)
	}
	last := finished.Attempts[len(finished.Attempts)-1]
	if last.ChangedLines <= DefaultRuntimeChangedLines || last.ChangedLineBudgetExceeded || !finished.Complete || finished.DecisionRequired {
		t.Fatalf("unbounded settlement was line-exhausted: status=%#v attempt=%#v", finished, last)
	}

	replayed, err := store.Status()
	if err != nil {
		t.Fatalf("unbounded objective did not replay: %v", err)
	}
	if replayed.Objective == nil || replayed.Objective.MaxChangedLines != 0 || !replayed.Complete {
		t.Fatalf("replayed unbounded status = %#v", replayed)
	}
}

func TestRuntimeZeroContinuationPreservesPositiveHistoricalLimit(t *testing.T) {
	for _, seam := range []string{"begin", "acquire"} {
		t.Run(seam, func(t *testing.T) {
			repo := initRuntimeLedgerRepo(t)
			store := mustRuntimeStore(t, repo, "historical-continuation-"+seam)
			started, err := store.Begin(context.Background(), BeginAttemptRequest{
				RequestID: "historical-begin", WorkUnit: "legacy-unit", EvidenceGoal: "preserve historical ceiling",
				MaxAttempts: 3, MaxChangedLines: 37,
			})
			if err != nil {
				t.Fatal(err)
			}
			failed, err := store.Finish(context.Background(), FinishAttemptRequest{
				ExpectedRevision: started.Revision, RequestID: "historical-finish", Outcome: AttemptFailed,
				EvidenceRevision: runtimeTestHash('b'), Diagnosis: "retry required",
				HarnessDisposition: HarnessReused, CleanupEvidence: "cleanup completed", ProcessEvidence: "no descendants",
			})
			if err != nil {
				t.Fatal(err)
			}

			request := BeginAttemptRequest{
				ExpectedRevision: failed.Revision, RequestID: "historical-continue", WorkUnit: "legacy-unit",
				EvidenceGoal: "preserve historical ceiling", MaxAttempts: 3, MaxChangedLines: 0,
			}
			var continued RuntimeStatus
			switch seam {
			case "begin":
				continued, err = store.Begin(context.Background(), request)
			case "acquire":
				result, acquireErr := store.Acquire(context.Background(), CompactAcquireRequest{BeginAttemptRequest: request})
				err = acquireErr
				if err == nil && result.State != CompactStateProceed {
					t.Fatalf("zero-valued acquire continuation = %#v, want proceed", result)
				}
				continued, _ = store.Status()
			}
			if err != nil {
				t.Fatalf("zero-valued %s continuation did not inherit the positive limit: %v", seam, err)
			}
			if continued.Objective == nil || continued.Objective.MaxChangedLines != 37 || continued.ActiveAttempt == nil {
				t.Fatalf("continued historical objective = %#v", continued)
			}
			beforeReplayRecords := countRuntimeRecords(t, store.Dir)
			switch seam {
			case "begin":
				replayed, replayErr := store.Begin(context.Background(), request)
				if replayErr != nil || replayed.Revision != continued.Revision {
					t.Fatalf("zero-valued begin replay = %#v err=%v", replayed, replayErr)
				}
			case "acquire":
				replayed, replayErr := store.Acquire(context.Background(), CompactAcquireRequest{BeginAttemptRequest: request})
				if replayErr != nil || replayed.State != CompactStateProceed || replayed.Token != continued.Revision {
					t.Fatalf("zero-valued acquire replay = %#v err=%v", replayed, replayErr)
				}
			}
			if records := countRuntimeRecords(t, store.Dir); records != beforeReplayRecords {
				t.Fatalf("zero-valued continuation replay appended a record: before=%d after=%d", beforeReplayRecords, records)
			}
		})
	}
}

func TestRuntimeRescopePreservesLimitKind(t *testing.T) {
	t.Run("zero request inherits a positive historical ceiling", func(t *testing.T) {
		repo := initRuntimeLedgerRepo(t)
		store := mustRuntimeStore(t, repo, "rescope-positive-inherit")
		started, err := store.Begin(context.Background(), BeginAttemptRequest{
			RequestID: "positive-begin", WorkUnit: "legacy-unit", EvidenceGoal: "preserve positive rescope limit",
			MaxAttempts: 3, MaxChangedLines: 40,
		})
		if err != nil {
			t.Fatal(err)
		}
		failed, err := store.Finish(context.Background(), FinishAttemptRequest{
			ExpectedRevision: started.Revision, RequestID: "positive-finish", Outcome: AttemptFailed,
			EvidenceRevision: runtimeTestHash('c'), Diagnosis: "narrower scope required",
			HarnessDisposition: HarnessReused, CleanupEvidence: "cleanup completed", ProcessEvidence: "no descendants",
		})
		if err != nil {
			t.Fatal(err)
		}
		request := RescopeObjectiveRequest{
			ExpectedRevision: failed.Revision, RequestID: "positive-rescope", WorkUnit: "narrower-unit",
			EvidenceGoal: "preserve the historical line ceiling", MaxAttempts: 3, MaxChangedLines: 0,
			Reason: "maintainer narrowed the work unit", Actor: "maintainer",
		}
		rescoped, err := store.Rescope(context.Background(), request)
		if err != nil {
			t.Fatalf("zero-valued rescope did not preserve the positive limit: %v", err)
		}
		if rescoped.Objective == nil || rescoped.Objective.MaxChangedLines != 40 || rescoped.LastRescope == nil || rescoped.LastRescope.MaxChangedLines != 40 {
			t.Fatalf("positive limit was laundered by rescope: %#v", rescoped)
		}
		replayed, err := store.Rescope(context.Background(), request)
		if err != nil || replayed.Revision != rescoped.Revision || countRuntimeRecords(t, store.Dir) != 3 {
			t.Fatalf("zero-valued positive rescope replay = %#v err=%v records=%d", replayed, err, countRuntimeRecords(t, store.Dir))
		}
	})

	t.Run("unbounded objective cannot gain a positive ceiling", func(t *testing.T) {
		repo := initRuntimeLedgerRepo(t)
		store := mustRuntimeStore(t, repo, "rescope-unbounded-kind")
		started, err := store.Begin(context.Background(), BeginAttemptRequest{
			RequestID: "unbounded-begin", WorkUnit: "unbounded-unit", EvidenceGoal: "remain unbounded",
			MaxAttempts: 3, MaxChangedLines: 0,
		})
		if err != nil {
			t.Fatal(err)
		}
		failed, err := store.Finish(context.Background(), FinishAttemptRequest{
			ExpectedRevision: started.Revision, RequestID: "unbounded-finish", Outcome: AttemptFailed,
			EvidenceRevision: runtimeTestHash('d'), Diagnosis: "narrower work unit required",
			HarnessDisposition: HarnessReused, CleanupEvidence: "cleanup completed", ProcessEvidence: "no descendants",
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = store.Rescope(context.Background(), RescopeObjectiveRequest{
			ExpectedRevision: failed.Revision, RequestID: "unbounded-rescope-positive", WorkUnit: "narrower-unit",
			EvidenceGoal: "attempt to add a ceiling", MaxAttempts: 3, MaxChangedLines: 10,
			Reason: "attempted to change the limit kind", Actor: "maintainer",
		})
		if !errors.Is(err, ErrRuntimeRescopeWidened) {
			t.Fatalf("positive ceiling on an unbounded rescope = %v, want ErrRuntimeRescopeWidened", err)
		}

		rescoped, err := store.Rescope(context.Background(), RescopeObjectiveRequest{
			ExpectedRevision: failed.Revision, RequestID: "unbounded-rescope-zero", WorkUnit: "narrower-unit",
			EvidenceGoal: "remain unbounded", MaxAttempts: 3, MaxChangedLines: 0,
			Reason: "maintainer narrowed work while preserving no line ceiling", Actor: "maintainer",
		})
		if err != nil {
			t.Fatalf("unbounded rescope was refused: %v", err)
		}
		if rescoped.Objective == nil || rescoped.Objective.MaxChangedLines != 0 {
			t.Fatalf("unbounded rescope gained a line ceiling: %#v", rescoped.Objective)
		}

		_, err = store.Begin(context.Background(), BeginAttemptRequest{
			ExpectedRevision: rescoped.Revision, RequestID: "unbounded-positive-continuation", WorkUnit: "narrower-unit",
			EvidenceGoal: "remain unbounded", MaxAttempts: 3, MaxChangedLines: 10,
		})
		if !errors.Is(err, ErrRuntimeObjectiveChange) {
			t.Fatalf("positive continuation on unbounded objective = %v, want ErrRuntimeObjectiveChange", err)
		}
	})
}

func TestRuntimePositiveHistoricalLimitRemainsAdvisoryAndReplays(t *testing.T) {
	repo := initRuntimeLedgerRepo(t)
	store := mustRuntimeStore(t, repo, "positive-limit-replay")
	started, err := store.Begin(context.Background(), BeginAttemptRequest{
		RequestID: "positive-limit-begin", WorkUnit: "bounded-unit", EvidenceGoal: "enforce persisted line ceiling",
		MaxAttempts: 3, MaxChangedLines: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	appendRuntimeLedgerFile(t, repo, strings.Repeat("bounded line\n", 8))
	finished, err := store.Finish(context.Background(), FinishAttemptRequest{
		ExpectedRevision: started.Revision, RequestID: "positive-limit-finish", Outcome: AttemptPassed,
		EvidenceRevision: runtimeTestHash('b'), Diagnosis: "work exceeded its historical ceiling",
		HarnessDisposition: HarnessReused, CleanupEvidence: "cleanup completed", ProcessEvidence: "no descendants",
	})
	if err != nil {
		t.Fatal(err)
	}
	last := finished.Attempts[len(finished.Attempts)-1]
	if !last.ChangedLineBudgetExceeded || finished.DecisionRequired || !finished.Complete {
		t.Fatalf("positive historical telemetry blocked passing evidence: status=%#v attempt=%#v", finished, last)
	}
	replayed, err := store.Status()
	if err != nil {
		t.Fatal(err)
	}
	if replayed.DecisionRequired || !replayed.Complete || replayed.Objective == nil || replayed.Objective.MaxChangedLines != 5 {
		t.Fatalf("positive historical limit did not replay: %#v", replayed)
	}
}
