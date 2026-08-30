package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestSDDAttemptMaxChangedLinesIsHiddenButParsingCompatible(t *testing.T) {
	for _, operation := range []string{"status", "begin", "rescope", "acquire"} {
		t.Run(operation+" help", func(t *testing.T) {
			var output bytes.Buffer
			if err := RunSDDAttempt([]string{operation, "--help"}, &output); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(output.String(), "--max-changed-lines") {
				t.Fatalf("deprecated compatibility flag is still advertised in %s help:\n%s", operation, output.String())
			}
		})
	}

	repo := initReviewCLIRepo(t)
	bounded := runSDDAttemptStatus(t, []string{
		"begin", "--cwd", repo, "--change", "compat-positive-lines", "--expected-revision=", "--request-id", "compat-begin",
		"--work-unit", "legacy-unit", "--evidence-goal", "preserve historical parsing", "--max-attempts", "2", "--max-changed-lines", "17",
	})
	if bounded.Objective == nil || bounded.Objective.MaxChangedLines != 17 {
		t.Fatalf("deprecated flag no longer preserves a positive historical limit: %#v", bounded.Objective)
	}
}

func TestSDDAttemptNewObjectiveDefaultsToUnboundedAndHistoricalContinuationInherits(t *testing.T) {
	repo := initReviewCLIRepo(t)
	unbounded := runSDDAttemptStatus(t, []string{
		"begin", "--cwd", repo, "--change", "cli-unbounded-default", "--expected-revision=", "--request-id", "unbounded-begin",
		"--work-unit", "new-unit", "--evidence-goal", "open without a line ceiling", "--max-attempts", "2",
	})
	if unbounded.Objective == nil || unbounded.Objective.MaxChangedLines != 0 {
		t.Fatalf("new CLI objective line limit = %#v, want unbounded zero", unbounded.Objective)
	}

	change := "cli-historical-inherit"
	started := runSDDAttemptStatus(t, []string{
		"begin", "--cwd", repo, "--change", change, "--expected-revision=", "--request-id", "historical-begin",
		"--work-unit", "legacy-unit", "--evidence-goal", "preserve historical limit", "--max-attempts", "3", "--max-changed-lines", "23",
	})
	failed := runSDDAttemptStatus(t, []string{
		"finish", "--cwd", repo, "--change", change, "--expected-revision", started.Revision, "--request-id", "historical-finish",
		"--outcome", "failed", "--evidence-revision", cliAttemptHash('a'), "--diagnosis", "retry required",
		"--harness-disposition", "reused", "--cleanup-evidence", "cleanup completed", "--process-evidence", "no descendants",
	})
	continued := runSDDAttemptStatus(t, []string{
		"begin", "--cwd", repo, "--change", change, "--expected-revision", failed.Revision, "--request-id", "historical-continue",
		"--work-unit", "legacy-unit", "--evidence-goal", "preserve historical limit", "--max-attempts", "3",
	})
	if continued.Objective == nil || continued.Objective.MaxChangedLines != 23 || continued.ActiveAttempt == nil {
		t.Fatalf("zero-valued CLI continuation laundered the historical limit: %#v", continued)
	}
}
