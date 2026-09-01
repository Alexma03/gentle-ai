package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/reviewtransaction"
)

func writeCrossRepositoryLifecycleCandidate(t *testing.T, repo, marker string) {
	t.Helper()
	writeReviewStartCandidate(t, repo, "internal/auth/session.go", "package auth\n\nfunc CheckToken(token string) bool {\n\treturn token == \""+marker+"\"\n}\n", 0o644)
}

func crossRepositoryLifecycleStatus(t *testing.T, cwd, lineage string, runtime model.AgentID) ReviewTargetStatusResult {
	t.Helper()
	var output bytes.Buffer
	if err := RunReview([]string{
		"status", "--cwd", cwd, "--contract", ReviewIntegrationContractV2,
		"--agent", string(runtime), "--next-transition", "--lineage", lineage,
	}, &output); err != nil {
		t.Fatalf("status for %s at %s: %v\n%s", runtime, cwd, err, output.String())
	}
	var status ReviewTargetStatusResult
	decodeStrictReviewJSON(t, output.Bytes(), &status)
	if status.NextTransition == nil {
		t.Fatalf("status for %s at %s omitted next transition: %#v", runtime, cwd, status)
	}
	return status
}

func assertCrossRepositoryStartTransition(t *testing.T, status ReviewTargetStatusResult, root, lineage string, runtime model.AgentID) {
	t.Helper()
	if status.NextTransition.Execute == nil || status.NextTransition.Execute.Operation != "review.start" || status.NextTransition.Execute.Binding.LineageID != lineage {
		t.Fatalf("fresh B transition = %#v", status.NextTransition)
	}
	rootFound, runtimeFound := false, false
	for _, argument := range status.NextTransition.Execute.Arguments {
		if argument.Token == "" {
			t.Fatalf("START emitted an untokenized argument: %#v", argument)
		}
		rootFound = rootFound || argument.Name == "cwd" && argument.Value == root
		runtimeFound = runtimeFound || argument.Name == "agent" && argument.Value == string(runtime)
	}
	if !rootFound || !runtimeFound {
		t.Fatalf("START did not preserve B root/runtime: %#v", status.NextTransition.Execute)
	}
}

func startCrossRepositoryLifecycleFromTransition(t *testing.T, status ReviewTargetStatusResult, runtime model.AgentID) ReviewIntegrationStartResult {
	t.Helper()
	if status.NextTransition == nil || status.NextTransition.Execute == nil || status.NextTransition.Execute.Operation != "review.start" {
		t.Fatalf("START transition unavailable: %#v", status.NextTransition)
	}
	args := []string{"start"}
	for _, argument := range status.NextTransition.Execute.Arguments {
		if argument.Token == "" {
			t.Fatalf("START argument has no exact token: %#v", argument)
		}
		args = append(args, argument.Token)
	}
	question := decodeConsentQuestion(t, runConsentRelayStart(t, args).Bytes())
	if question.Agent != string(runtime) {
		t.Fatalf("consent runtime = %q, want %q", question.Agent, runtime)
	}
	for _, choice := range question.Choices {
		if choice.Answer == "granted" {
			return decodeNegotiatedReviewStart(t, runConsentRelayStart(t, invocationArgs(t, choice.Invocation)).Bytes())
		}
	}
	t.Fatalf("consent envelope has no granted invocation: %#v", question.Choices)
	return ReviewIntegrationStartResult{}
}

func assertCrossRepositoryContinuation(t *testing.T, status ReviewTargetStatusResult, started ReviewIntegrationStartResult, lineage, label string) {
	t.Helper()
	if started.RepositoryContext == nil {
		t.Fatalf("%s START has no repository context", label)
	}
	if status.Authority == nil || status.Authority.LineageID != lineage || status.RepositoryContext == nil ||
		status.RepositoryContext.TargetIdentity != started.RepositoryContext.TargetIdentity {
		t.Fatalf("%s continuation = %#v, want %q bound to target %q", label, status, lineage, started.RepositoryContext.TargetIdentity)
	}
}

func assertUnsupportedCrossRepositoryLifecycleRoot(t *testing.T, targetNested string, runtime model.AgentID) {
	t.Helper()
	before, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(targetNested)), "tracked.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = RunReview([]string{
		"status", "--cwd", targetNested, "--contract", ReviewIntegrationContractV2,
		"--agent", string(runtime), "--next-transition",
	}, &output)
	if err == nil {
		t.Fatalf("unsupported runtime %q accepted foreign B status", runtime)
	}
	failure := decodeReviewIntegrationFailure(t, output.Bytes())
	if failure.Code != reviewImmutableTransportUnsupportedCode || failure.MutationOutcome != ReviewMutationNotStarted ||
		failure.AuthorityApplicability != "not_evaluated" {
		t.Fatalf("unsupported foreign-root failure = %#v", failure)
	}
	after, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(targetNested)), "tracked.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("unsupported runtime changed B bytes before transport refusal")
	}
	stores, err := reviewtransaction.DiscoverCompactStores(context.Background(), filepath.Dir(filepath.Dir(targetNested)))
	if err != nil {
		t.Fatal(err)
	}
	if len(stores) != 0 {
		t.Fatalf("unsupported runtime created B authority: %#v", stores)
	}
}
