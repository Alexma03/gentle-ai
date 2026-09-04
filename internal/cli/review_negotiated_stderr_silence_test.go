package cli

import (
	"bytes"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/reviewtransaction"
)

// captureReviewProcessStderr swaps the process stderr for a pipe so a test can
// assert the machine-readable (negotiated) surface stays byte-silent. It
// catches every writer that resolves os.Stderr at call time: the consent
// console (defaultReviewConsole) and any direct fmt.Fprint(os.Stderr, ...)
// site. The returned function restores stderr and yields the captured bytes.
func captureReviewProcessStderr(t *testing.T) func() string {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stderr
	os.Stderr = write
	var once sync.Once
	var captured string
	stop := func() string {
		once.Do(func() {
			os.Stderr = previous
			_ = write.Close()
			data, readErr := io.ReadAll(read)
			_ = read.Close()
			if readErr != nil {
				t.Fatal(readErr)
			}
			captured = string(data)
		})
		return captured
	}
	t.Cleanup(func() { _ = stop() })
	return stop
}

// TestNegotiatedStatusSuccessIsByteSilentOnStderr pins the pi compatibility
// contract: a successful negotiated STATUS (exit 0, valid JSON on stdout)
// writes zero bytes to stderr. The forecast is carried structurally in the
// JSON `forecast` field, never narrated (gentle-pi fails closed with
// UNEXPECTED_STDERR on any stderr byte a successful operation writes).
func TestNegotiatedStatusSuccessIsByteSilentOnStderr(t *testing.T) {
	reviewModeHome(t)
	repo := initReviewCLIRepo(t)
	writeReviewStartCandidate(t, repo, "docs/notes.md", "silent status\n", 0o644)
	stderr := captureReviewProcessStderr(t)

	var stdout bytes.Buffer
	if err := RunReview([]string{
		"status", "--cwd", repo, "--contract", ReviewIntegrationContractV2,
		"--agent", "claude-code", "--next-transition",
	}, &stdout); err != nil {
		t.Fatalf("negotiated STATUS: %v\n%s", err, stdout.String())
	}
	var status ReviewTargetStatusResult
	decodeStrictReviewJSON(t, stdout.Bytes(), &status)
	if status.NextTransition == nil || status.Forecast == nil || len(status.Forecast.Steps) == 0 {
		t.Fatalf("forecast must stay structural on stdout: %#v", status.Forecast)
	}
	if got := stderr(); got != "" {
		t.Fatalf("successful negotiated STATUS wrote stderr, want zero bytes:\n%q", got)
	}
}

// TestNegotiatedStatusStopIsByteSilentOnStderr covers the stop-shaped success:
// a negotiated STATUS answering `stop` (here rdd_disabled) still exits 0 with
// a valid envelope, so its Tier C stop narration and the rendered enable
// continuation must not reach stderr either. The reason code stays structural
// in next_transition.reason_code.
func TestNegotiatedStatusStopIsByteSilentOnStderr(t *testing.T) {
	reviewModeHome(t)
	repo := initReviewCLIRepo(t)
	writeReviewStartCandidate(t, repo, "docs/notes.md", "disabled stop\n", 0o644)
	if err := RunReviewMode([]string{"disable", "--cwd", repo, "--scope", "global"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	stderr := captureReviewProcessStderr(t)

	var stdout bytes.Buffer
	if err := RunReview([]string{
		"status", "--cwd", repo, "--contract", ReviewIntegrationContractV2,
		"--agent", "claude-code", "--next-transition",
	}, &stdout); err != nil {
		t.Fatalf("negotiated STATUS while disabled: %v\n%s", err, stdout.String())
	}
	var status ReviewTargetStatusResult
	decodeStrictReviewJSON(t, stdout.Bytes(), &status)
	if status.NextTransition == nil || status.NextTransition.Kind != reviewNextTransitionStop ||
		status.NextTransition.ReasonCode != "rdd_disabled" {
		t.Fatalf("disabled transition = %#v", status.NextTransition)
	}
	if got := stderr(); got != "" {
		t.Fatalf("stop-shaped negotiated STATUS wrote stderr, want zero bytes:\n%q", got)
	}
}

// TestNegotiatedStartUndeclaredConsentIsByteSilentOnStderr pins the START half
// of the same contract: a negotiated START without a --consent declaration
// reviews the candidate automatically, but announces nothing: RDD is already
// enabled for the user, so no candidate-scoped consent surface is involved.
func TestNegotiatedStartUndeclaredConsentIsByteSilentOnStderr(t *testing.T) {
	reviewEnabledHome(t)
	repo := initReviewCLIRepo(t)
	console := stubReviewConsole(t, false, "")
	writeReviewStartCandidate(t, repo, "scripts/deploy.sh", "echo deploy\n", 0o644)
	stderr := captureReviewProcessStderr(t)

	output := runConsentRelayStart(t, boundNegotiatedStartArgs(t, []string{
		"start", "--contract", ReviewIntegrationContractV1, "--cwd", repo,
		"--lineage", "review-negotiated-silent",
	}))
	started := decodeNegotiatedReviewStart(t, output.Bytes())
	if started.Action != "created" || len(started.SelectedLenses) != 4 {
		t.Fatalf("undeclared negotiated START stopped reviewing: %#v", started)
	}
	if console.Len() != 0 {
		t.Fatalf("undeclared negotiated START wrote to the console, want silence:\n%s", console.String())
	}
	if got := stderr(); got != "" {
		t.Fatalf("undeclared negotiated START wrote stderr, want zero bytes:\n%q", got)
	}
}

// TestNegotiatedLifecycleOperationsAreByteSilentOnStderr walks a complete
// low-risk negotiated lifecycle — STATUS, terminal START, unmanaged VALIDATE —
// and requires zero stderr bytes from every successful operation, the exact
// sequence gentle-pi drives fail-closed.
func TestNegotiatedLifecycleOperationsAreByteSilentOnStderr(t *testing.T) {
	reviewEnabledHome(t)
	repo := initReviewCLIRepo(t)
	if err := RunReviewMode([]string{"enable", "--scope", "global", "--cwd", repo}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	stubReviewConsole(t, false, "")
	writeReviewStartCandidate(t, repo, "docs/notes.md", "lifecycle\n", 0o644)
	stderr := captureReviewProcessStderr(t)

	status := negotiatedStartStatusForContract(t, repo, ReviewIntegrationContractV2, "--agent", "claude-code")
	started := executeStartTransition(t, repo, status)
	if started.RiskLevel != reviewtransaction.RiskLow || len(started.SelectedLenses) != 0 {
		t.Fatalf("low-risk negotiated START = %#v", started)
	}

	if started.State != reviewtransaction.StateApproved || started.Action != "closed" {
		t.Fatalf("low-risk terminal START = %#v", started)
	}

	runReviewCLIGit(t, repo, "add", "--", "docs/notes.md")
	var validated bytes.Buffer
	if err := RunReview([]string{
		"validate", "--contract", ReviewIntegrationContractV2, "--gate", "pre-commit", "--cwd", repo,
	}, &validated); err != nil {
		t.Fatalf("negotiated VALIDATE: %v\n%s", err, validated.String())
	}
	var envelope ReviewIntegrationOperationResult
	decodeStrictReviewJSON(t, validated.Bytes(), &envelope)
	var validation ReviewValidateResult
	decodeStrictReviewJSON(t, envelope.Result, &validation)
	if validation.Result != reviewtransaction.GateInvalidated || validation.Allowed ||
		validation.Delivery != reviewtransaction.RDDDeliveryUnmanaged {
		t.Fatalf("pre-commit gate was not successful unmanaged delivery: %#v", validation)
	}

	if got := stderr(); got != "" {
		t.Fatalf("negotiated lifecycle wrote stderr, want zero bytes:\n%q", got)
	}
}
