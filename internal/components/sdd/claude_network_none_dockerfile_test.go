package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeNetworkNoneDockerfileTargetsRetainedSDDProof(t *testing.T) {
	dockerfile := filepath.Join("..", "..", "..", "e2e", "Dockerfile.claude-network-none")
	content, err := os.ReadFile(dockerfile)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"go test -c -o /usr/local/bin/claude-review-e2e ./internal/components/sdd",
		"-test.run=^TestClaudeReviewerTransportInNetworkNone$",
	} {
		if !strings.Contains(string(content), want) {
			t.Errorf("Claude network-none Dockerfile missing retained proof target %q", want)
		}
	}
	for _, retired := range []string{
		"./e2e/organicruntime",
		"TestClaudeProviderAdapterUsesPinnedNetworkNoneRuntime",
		"GENTLE_AI_ORGANIC_TEST_BINARY",
	} {
		if strings.Contains(string(content), retired) {
			t.Errorf("Claude network-none Dockerfile retains retired target %q", retired)
		}
	}
}
