package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/reviewtransaction"
)

func runConsentRelayStart(t *testing.T, args []string) *bytes.Buffer {
	t.Helper()
	var output bytes.Buffer
	if err := RunReview(args, &output); err != nil {
		t.Fatalf("negotiated START: %v\n%s", err, output.String())
	}
	return &output
}

func invocationArgs(t *testing.T, invocation string) []string {
	t.Helper()
	words, err := SplitPrintedCommandWords(invocation)
	if err != nil {
		t.Fatalf("parse review invocation: %v", err)
	}
	if len(words) < 3 || words[0] != "gentle-ai" || words[1] != "review" || words[2] != "start" {
		t.Fatalf("invocation is not a runnable gentle-ai review start command: %q", invocation)
	}
	return words[2:]
}

func TestEnabledRDDStartsHighRiskCandidateWithoutConsentPrompt(t *testing.T) {
	reviewEnabledHome(t)
	repo := initReviewCLIRepo(t)
	console := stubReviewConsole(t, false, "")
	writeReviewStartCandidate(t, repo, "scripts/deploy.sh", "echo deploy\n", 0o644)

	result := decodeNegotiatedReviewStart(t, runConsentRelayStart(t, boundNegotiatedStartArgs(t, []string{
		"start", "--contract", ReviewIntegrationContractV2, "--cwd", repo,
		"--lineage", "review-automatic-start", "--consent", "relay",
	})).Bytes())

	if result.Action != "created" || result.RiskLevel != reviewtransaction.RiskHigh || len(result.SelectedLenses) != 4 {
		t.Fatalf("enabled RDD did not start the review automatically: %#v", result)
	}
	if console.Len() != 0 {
		t.Fatalf("automatic review wrote a consent prompt or notice:\n%s", console.String())
	}
}

func TestV2StartTransitionDoesNotRequestCandidateConsent(t *testing.T) {
	reviewEnabledHome(t)
	repo := initReviewCLIRepo(t)
	writeReviewStartCandidate(t, repo, "scripts/deploy.sh", "echo deploy\n", 0o644)

	status := negotiatedStartStatusForContract(t, repo, ReviewIntegrationContractV2, "--lineage", "review-no-consent-token")
	command := strings.Join(transitionStartArgs(repo, status), " ")
	if strings.Contains(command, "--consent") {
		t.Fatalf("provider-issued START still requests per-candidate consent: %s", command)
	}
}
