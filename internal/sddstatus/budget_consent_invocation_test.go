package sddstatus

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// The consent field remains decode-compatible for persisted or relayed status
// payloads written by older builds, even though current accounting never emits
// a new budget-consent envelope.
func TestLegacyBudgetConsentPayloadStillDecodes(t *testing.T) {
	payload := `{"change":"demo","consent":{"schema":"gentle-ai.sdd-integration.consent/v1","change":"demo","action":"consent_required","blocking":true,"headline":"legacy","choices":[{"answer":"granted","label":"continue","effect":"legacy","invocation":"legacy"}]}}`
	var status RuntimeStatus
	if err := json.Unmarshal([]byte(payload), &status); err != nil {
		t.Fatal(err)
	}
	if status.Consent == nil || status.Consent.Schema != "gentle-ai.sdd-integration.consent/v1" || !status.Consent.Blocking {
		t.Fatalf("legacy consent payload was not preserved: %#v", status.Consent)
	}
}

func TestCurrentRuntimeStatusOmitsLegacyBudgetConsent(t *testing.T) {
	payload, err := json.Marshal(RuntimeStatus{Change: "demo", NextAction: RuntimeActionBegin})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), `"consent"`) {
		t.Fatalf("current status unexpectedly emitted legacy consent: %s", payload)
	}
}

func TestAttemptLimitIsAdvisoryAtLedgerBoundary(t *testing.T) {
	ctx := context.Background()
	repo := initRuntimeLedgerRepo(t)
	store := mustRuntimeStore(t, repo, "advisory-limit")
	store.ReviewDisabled = true

	appendRuntimeLedgerFile(t, repo, "work\n")
	begun, err := store.Begin(ctx, BeginAttemptRequest{
		RequestID: "advisory-begin", WorkUnit: "u", EvidenceGoal: "g",
		MaxAttempts: 1, MaxChangedLines: 400,
	})
	if err != nil {
		t.Fatal(err)
	}
	failed, err := store.Finish(ctx, FinishAttemptRequest{
		ExpectedRevision: begun.Revision, RequestID: "advisory-finish", Outcome: AttemptFailed,
		EvidenceRevision: runtimeTestHash('a'), Diagnosis: "failed",
		HarnessDisposition: HarnessInvalidated, CleanupEvidence: "clean", ProcessEvidence: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	if failed.DecisionRequired || failed.NextAction != RuntimeActionBegin || failed.Consent != nil {
		t.Fatalf("accounting ceiling blocked ordinary recovery: %#v", failed)
	}
}

// Reset remains revision-bound for explicit maintenance use even though it is
// no longer required to recover from routine accounting exhaustion.
func TestStaleResetFailsCASWithoutMutation(t *testing.T) {
	ctx := context.Background()
	repo := initRuntimeLedgerRepo(t)
	store := mustRuntimeStore(t, repo, "stale-reset")
	store.ReviewDisabled = true

	appendRuntimeLedgerFile(t, repo, "work\n")
	begun, err := store.Begin(ctx, BeginAttemptRequest{
		RequestID: "s-begin", WorkUnit: "u", EvidenceGoal: "g",
		MaxAttempts: 1, MaxChangedLines: 400,
	})
	if err != nil {
		t.Fatal(err)
	}
	failed, err := store.Finish(ctx, FinishAttemptRequest{
		ExpectedRevision: begun.Revision, RequestID: "s-finish", Outcome: AttemptFailed,
		EvidenceRevision: runtimeTestHash('a'), Diagnosis: "failed",
		HarnessDisposition: HarnessReused, CleanupEvidence: "clean", ProcessEvidence: "none",
	})
	if err != nil {
		t.Fatal(err)
	}

	drifted, err := store.Reset(ctx, ResetObjectiveRequest{
		ExpectedRevision: failed.Revision, RequestID: "current-reset",
		Reason: "explicit maintenance", Actor: "maintainer",
	})
	if err != nil {
		t.Fatal(err)
	}
	if drifted.Revision == failed.Revision {
		t.Fatal("fixture did not drift the revision")
	}

	_, staleErr := store.Reset(ctx, ResetObjectiveRequest{
		ExpectedRevision: failed.Revision, RequestID: "stale-reset",
		Reason: "stale maintenance", Actor: "maintainer",
	})
	if staleErr == nil {
		t.Fatal("a reset captured before authority drift applied afterwards")
	}
	after, err := store.Status()
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != drifted.Revision {
		t.Fatalf("refused stale reset moved the ledger: %q -> %q", drifted.Revision, after.Revision)
	}
}
