package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/sddstatus"
)

// Old clients may still send a status payload containing the retired consent
// field. Keeping it readable is compatibility; constructing a fresh blocking
// question from routine accounting is not.
func TestLegacyBudgetConsentStatusRemainsReadable(t *testing.T) {
	var status sddstatus.RuntimeStatus
	if err := json.Unmarshal([]byte(`{"change":"demo","consent":{"schema":"gentle-ai.sdd-integration.consent/v1","blocking":true}}`), &status); err != nil {
		t.Fatal(err)
	}
	if status.Consent == nil || !status.Consent.Blocking {
		t.Fatalf("legacy consent status was not decoded: %#v", status.Consent)
	}
}

func TestCurrentStatusDoesNotInventHarnessFailureConsent(t *testing.T) {
	status := sddstatus.RuntimeStatus{Change: "demo", NextAction: sddstatus.RuntimeActionBegin}
	payload, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), `"consent"`) {
		t.Fatalf("current status fabricated a blocking consent envelope: %s", payload)
	}
}

func TestCurrentStatusNeedsNoResetInvocation(t *testing.T) {
	status := sddstatus.RuntimeStatus{Change: "demo", NextAction: sddstatus.RuntimeActionBegin}
	if status.DecisionRequired || status.Consent != nil || status.NextAction != sddstatus.RuntimeActionBegin {
		t.Fatalf("retryable status still requires an accounting reset: %#v", status)
	}
}

// TestExhaustedBudgetSurfacesTheQuestionEndToEnd is the half a constructor
// cannot deliver. An envelope nobody renders is #2471's root 9, and it is
// exactly how #2588's reporter ended up answering the same implicit question
// four times: the ledger knew, and never asked.
func TestExhaustedBudgetRemainsRetryableEndToEnd(t *testing.T) {
	reviewModeHome(t)
	repo := initReviewCLIRepo(t)
	const change = "budget-consent-e2e"
	disableReviewForClone(t, repo)

	writeCLIAttemptFile(t, cliAttemptChangePath(repo, change, "proposal.md"), "# Proposal\n")
	writeCLIAttemptFile(t, cliAttemptChangePath(repo, change, "tasks.md"), "- [x] 1.1 Work\n")
	runReviewCLIGit(t, repo, "add", ".")
	runReviewCLIGit(t, repo, "commit", "-qm", "seed change")

	// One attempt, one budget, and the harness never runs the work: the actor
	// settles it invalidated, which is the settle contract's existing way of
	// saying the harness could not be used.
	writeCLIAttemptFile(t, cliAttemptChangePath(repo, change, "tasks.md"), "- [x] 1.1 Work\n# harness\n")
	acquired, _ := runCompactSDDAttempt(t, []string{
		"acquire", "--cwd", repo, "--change", change, "--request-id", "h1",
		"--work-unit", "acceptance", "--evidence-goal", "postgres acceptance",
		"--max-attempts", "1", "--max-changed-lines", "400",
	})
	if acquired.State != "proceed" {
		t.Fatalf("acquire = %#v", acquired)
	}
	runCompactSDDAttempt(t, []string{
		"settle", "--cwd", repo, "--change", change, "--token", acquired.Token, "--request-id", "h2",
		"--outcome", "failed", "--evidence-revision", cliAttemptHash('a'),
		"--diagnosis", "the harness could not be constructed; no consumer command ran",
		"--harness-disposition", "invalidated",
		"--cleanup-evidence", "container removed", "--process-evidence", "no descendants",
	})

	// Accounting is exhausted, but status remains retryable without consent.
	scoped := runSDDAttemptStatus(t, []string{
		"status", "--cwd", repo, "--change", change,
		"--work-unit", "acceptance", "--evidence-goal", "postgres acceptance",
		"--max-attempts", "1", "--max-changed-lines", "400",
	})
	if scoped.BlockedReason != "" || scoped.DecisionRequired || scoped.NextAction != sddstatus.RuntimeActionBegin {
		t.Fatalf("accounting telemetry blocked retry: %#v", scoped)
	}
	if scoped.Consent != nil {
		t.Fatalf("routine accounting unexpectedly requested consent: %#v", scoped.Consent)
	}
}

// TestSDDStatusContractRequiresAutomaticAccountingRecovery closes the loop a
// Go change cannot: generated agents must treat accounting as telemetry while
// preserving the separate edit-authority consent contract.
func TestSDDStatusContractRequiresAutomaticAccountingRecovery(t *testing.T) {
	contract := readSharedSDDStatusContract(t)
	for _, want := range []string{
		"advisory/no-progress telemetry",
		"must not demand human permission to reset bookkeeping",
		"change strategy",
	} {
		if !strings.Contains(contract, want) {
			t.Fatalf("the contract does not bind automatic accounting recovery; it must contain %q", want)
		}
	}
	for _, stale := range []string{"attempt budget asks", "attempt budget is asking", "attempt budget needs a maintainer decision"} {
		if strings.Contains(strings.ToLower(contract), stale) {
			t.Fatalf("routine contract still routes accounting exhaustion through consent: %q", stale)
		}
	}
}
