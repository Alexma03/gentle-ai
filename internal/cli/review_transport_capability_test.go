package cli

import (
	"bytes"
	"slices"
	"sort"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/catalog"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/reviewerprovider"
)

// TestImmutableReviewRuntimeMatrix keeps runtime advertisement fail-closed for
// runtimes that do not own a native executor boundary.

func TestImmutableReviewRuntimeCapabilityIsClosedCatalogSet(t *testing.T) {
	t.Setenv(reviewPiHostRelayContractEnvironment, reviewPiHostRelayContract)

	const wantExposed = 3
	exposed := 0
	for _, agent := range catalog.AllAgents() {
		t.Run(string(agent.ID), func(t *testing.T) {
			capability := reviewImmutableRuntimeCapability(agent.ID)
			want := agent.ID == model.AgentClaudeCode ||
				agent.ID == model.AgentCodex ||
				agent.ID == model.AgentPi
			if capability.Eligible != want || capability.supportsImmutableReceiptReview() != want {
				t.Fatalf("runtime capability = %#v, supported = %t, want exposed = %t", capability, capability.supportsImmutableReceiptReview(), want)
			}
			if !want && capability.Transport != reviewImmutableTransportUnsupported {
				t.Fatalf("unsupported runtime transport = %q, want %q", capability.Transport, reviewImmutableTransportUnsupported)
			}
			if want {
				exposed++
			}
		})
	}
	if exposed != wantExposed {
		t.Fatalf("immutable review runtimes = %d, want %d", exposed, wantExposed)
	}
}

// TestSupportedImmutableReviewTransportReachesRepositoryValidation proves
// supported runtimes reach repository validation in an ordinary session:
// neither depends on OPENCODE_DISABLE_PROJECT_CONFIG or
// OPENCODE_DISABLE_EXTERNAL_SKILLS, which this test deliberately leaves unset.

func TestV21RejectsDuplicateRuntimeAgentsBeforeRepositoryAccess(t *testing.T) {
	var output bytes.Buffer
	err := RunReview([]string{
		"status", "--contract", ReviewIntegrationContractV2, "--cwd", t.TempDir() + "/missing",
		"--agent", string(model.AgentClaudeCode), "--agent", string(model.AgentClaudeCode),
	}, &output)
	if err == nil {
		t.Fatal("v2.1 STATUS accepted multiple runtime identities")
	}
	failure := decodeReviewIntegrationFailure(t, output.Bytes())
	if failure.Code != reviewImmutableTransportUnsupportedCode || failure.Operation != "review.status" ||
		failure.MutationOutcome != ReviewMutationNotStarted || failure.AuthorityApplicability != "not_evaluated" {
		t.Fatalf("duplicate runtime failure = %#v", failure)
	}
}

// TestRegisteredRuntimeIdentitiesMatchCompiledTransportBoundary pins the
// published provider-contract runtime inventory to the compiled capability:
// the bundle may only declare what the boundary actually admits.
func TestRegisteredRuntimeIdentitiesMatchCompiledTransportBoundary(t *testing.T) {
	t.Setenv(reviewPiHostRelayContractEnvironment, reviewPiHostRelayContract)
	registered := reviewerprovider.RegisteredRuntimeIdentities()
	supported := reviewTransportSupportedRuntimeIDs()
	sort.Strings(registered)
	sort.Strings(supported)
	if !slices.Equal(registered, supported) {
		t.Fatalf("RegisteredRuntimeIdentities() = %q, want the compiled supported runtimes %q", registered, supported)
	}
}

// TestPiHostRelayContractHandshakeGatesAdmission pins the version handshake
// for the externally-owned Pi launcher: without the exact declared relay
// contract, Pi is refused at admission before any authority work and never
// appears among the suggested supported runtimes.
func TestPiHostRelayContractHandshakeGatesAdmission(t *testing.T) {
	for _, declared := range []string{"", "gentle-pi.review-relay/v0", "GENTLE-PI.REVIEW-RELAY/V1"} {
		t.Setenv(reviewPiHostRelayContractEnvironment, declared)
		capability := reviewImmutableRuntimeCapability(model.AgentPi)
		if capability.Eligible || capability.supportsImmutableReceiptReview() {
			t.Fatalf("declared %q: capability = %#v, want fail-closed admission", declared, capability)
		}
		if slices.Contains(reviewTransportSupportedRuntimeIDs(), string(model.AgentPi)) {
			t.Fatalf("declared %q: refusal exits steer users toward an undeclared relay", declared)
		}
	}
	t.Setenv(reviewPiHostRelayContractEnvironment, reviewPiHostRelayContract)
	if capability := reviewImmutableRuntimeCapability(model.AgentPi); !capability.supportsImmutableReceiptReview() {
		t.Fatalf("declared handshake refused: %#v", capability)
	}
}
