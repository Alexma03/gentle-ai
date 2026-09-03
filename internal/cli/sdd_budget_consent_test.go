package cli

import (
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/sddstatus"
)

// #2588: the bounded attempt budget was spent entirely on the apply worker
// failing to CONSTRUCT a valid harness — four different shell defects across
// three executions, every one of them before a single consumer command ran.
// The reporter ended at blocked(maintainer_decision) knowing nothing about
// their own change, because their change was never exercised.
//
// Two things were wrong, and only one of them is the budget.
//
// The ledger recorded provider defects as evidence about the candidate. The
// immutable chain now says "this candidate failed four bounded attempts" for a
// candidate that was never tried, and that chain is what later remediation,
// reset and archive decisions read.
//
// And maintainer_decision was a dead end in prose: it named a reset the human
// had to know about and assemble by hand from six flags. There is no reason
// the exhausted state cannot simply ASK, the way a medium-risk review START
// asks before it spends anything.
//
// Deliberately NOT a fixed exemption count. An exemption an actor can claim is
// not verifiable, so any N large enough to help is large enough to stop the
// budget bounding anything. A grant a human issues is verifiable and audited,
// and it needs no calibration.

func TestExhaustedBudgetAsksInsteadOfDeadEnding(t *testing.T) {
	envelope, err := sddstatus.BudgetConsentEnvelope(sddstatus.BudgetConsentInput{
		Repo: "/repo", Change: "demo", Revision: "sha256:" + strings.Repeat("a", 64),
		MaxAttempts: 2, CumulativeAttempts: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !envelope.Blocking {
		t.Fatal("the envelope must mark itself blocking: nothing may proceed until the human answers")
	}
	if len(envelope.Choices) != 2 || envelope.Choices[0].Answer != "granted" || envelope.Choices[1].Answer != "declined" {
		t.Fatalf("choices = %#v, want exactly granted then declined", envelope.Choices)
	}
	// The grant has to be runnable verbatim, not described. The whole defect
	// being fixed is a human being told to assemble a six-flag reset by hand.
	grant := envelope.Choices[0].Invocation
	for _, want := range []string{"gentle-ai sdd-attempt reset", "--cwd", "--change", "--expected-revision", "sha256:" + strings.Repeat("a", 64)} {
		if !strings.Contains(grant, want) {
			t.Fatalf("the grant invocation is not runnable verbatim (missing %q):\n%s", want, grant)
		}
	}
	if envelope.Choices[1].Invocation == "" {
		t.Fatal("declining must also be a runnable answer, not an absence")
	}
	// The accounting the human is deciding about has to be in front of them.
	joined := strings.Join(envelope.Evidence, "\n")
	if !strings.Contains(joined, "2") {
		t.Fatalf("the evidence does not show the attempt accounting being decided:\n%s", joined)
	}
}

// TestHarnessFailureIsTypedSoTheQuestionCanBeAnswered is the half that makes
// the question decidable. A prompt that only says "retry?" reproduces #2588
// with a click in the middle: the reporter answered the equivalent question
// four times because nothing distinguished the tool failing from their code
// failing.
func TestHarnessFailureIsTypedSoTheQuestionCanBeAnswered(t *testing.T) {
	envelope, err := sddstatus.BudgetConsentEnvelope(sddstatus.BudgetConsentInput{
		Repo: "/repo", Change: "demo", Revision: "sha256:" + strings.Repeat("a", 64),
		MaxAttempts: 2, CumulativeAttempts: 2, HarnessFailures: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := envelope.Headline + "\n" + envelope.Reason + "\n" + strings.Join(envelope.Evidence, "\n")
	if !strings.Contains(text, "harness") {
		t.Fatalf("the envelope does not say the attempts were spent on harness failures:\n%s", text)
	}
	// Invalidated means the result cannot prove the change; it does not mean the
	// process never started or that partial evidence cannot exist.
	if !strings.Contains(text, "partial evidence may exist") || !strings.Contains(text, "cannot prove the change") {
		t.Fatalf("the envelope does not describe incomplete harness evidence honestly:\n%s", text)
	}

	clean, err := sddstatus.BudgetConsentEnvelope(sddstatus.BudgetConsentInput{
		Repo: "/repo", Change: "demo", Revision: "sha256:" + strings.Repeat("a", 64),
		MaxAttempts: 2, CumulativeAttempts: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(clean.Headline+clean.Reason+strings.Join(clean.Evidence, "\n"), "harness") {
		t.Fatal("an ordinary exhausted budget must not claim harness failures it did not have")
	}
}

// TestBudgetConsentRefusesAnIncompleteEnvelope keeps the producer honest
// through the shared core's own completeness rule, the same one the review
// envelope answers to.
func TestBudgetConsentRefusesAnIncompleteEnvelope(t *testing.T) {
	if _, err := sddstatus.BudgetConsentEnvelope(sddstatus.BudgetConsentInput{
		Repo: "/repo", Change: "demo", MaxAttempts: 2, CumulativeAttempts: 2,
	}); err == nil {
		t.Fatal("an envelope with no revision cannot name a runnable reset, so it must refuse to be built")
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
