package reviewtransaction

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestGitOutputOverflowReportsStderrCollectorLimit(t *testing.T) {
	err := gitOutputOverflow([]string{"status"}, defaultGitOutputLimit, &boundedGitOutput{}, &boundedGitOutput{limit: 17, total: 18, exceeded: true}, true)
	var overflow *GitOutputLimitError
	if !errors.As(err, &overflow) || overflow.Limit != 17 || overflow.Actual != 18 {
		t.Fatalf("stderr overflow = %T %#v", err, overflow)
	}
}

func gitOutputLimitDetails(err error) (limits, actual []int) {
	switch typed := err.(type) {
	case *GitOutputLimitError:
		return []int{typed.Limit}, []int{typed.Actual}
	case interface{ Unwrap() []error }:
		for _, cause := range typed.Unwrap() {
			causeLimits, causeActual := gitOutputLimitDetails(cause)
			limits, actual = append(limits, causeLimits...), append(actual, causeActual...)
		}
	case interface{ Unwrap() error }:
		return gitOutputLimitDetails(typed.Unwrap())
	}
	return limits, actual
}

func TestRunGitTraceDoesNotContaminateMachineOutput(t *testing.T) {
	requireSnapshotGit(t)
	repo := initSnapshotRepo(t)
	t.Setenv("GIT_TRACE", "1")

	output, err := runGit(context.Background(), repo, nil, nil, "rev-parse", "--is-inside-work-tree")
	if err != nil || string(output) != "true\n" {
		t.Fatalf("traced rev-parse output = %q, error = %v", output, err)
	}
}

func TestRunGitOutputHelper(t *testing.T) {
	mode := os.Getenv("GENTLE_AI_GIT_OUTPUT_HELPER")
	if mode == "" {
		return
	}
	switch mode {
	case "stdout":
		fmt.Fprint(os.Stdout, "machine-output\n")
	case "stdout-stderr":
		fmt.Fprint(os.Stdout, "machine-output\n")
		fmt.Fprint(os.Stderr, "diagnostic output\n")
	case "stdout-overflow":
		fmt.Fprint(os.Stdout, strings.Repeat("o", defaultGitOutputLimit+1))
	case "stderr-overflow":
		fmt.Fprint(os.Stderr, strings.Repeat("e", defaultGitStderrLimit+1))
	case "failure":
		fmt.Fprint(os.Stderr, "meaningful diagnostic\n")
		os.Exit(7)
	case "failure-stdout-overflow":
		fmt.Fprint(os.Stdout, strings.Repeat("o", defaultGitOutputLimit+1))
		fmt.Fprint(os.Stderr, "actionable failure\n")
		os.Exit(7)
	case "failure-stderr-overflow":
		fmt.Fprint(os.Stdout, "machine-output\n")
		fmt.Fprint(os.Stderr, "actionable failure\n")
		fmt.Fprint(os.Stderr, strings.Repeat("e", defaultGitStderrLimit+1))
		os.Exit(7)
	case "failure-both-overflow":
		fmt.Fprint(os.Stdout, strings.Repeat("o", defaultGitOutputLimit+1))
		fmt.Fprint(os.Stderr, "actionable failure\n")
		fmt.Fprint(os.Stderr, strings.Repeat("e", defaultGitStderrLimit+1))
		os.Exit(7)
	}
	os.Exit(0)
}
