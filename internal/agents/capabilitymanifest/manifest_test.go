package capabilitymanifest

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/catalog"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func TestCanonicalImplementationRoutingBoundaries(t *testing.T) {
	t.Parallel()

	got := CanonicalImplementationRouting()
	want := ImplementationRoutingFacts{
		DirectInline: DirectInlineFacts{
			MinUnderstandingFiles:                    1,
			MaxUnderstandingFiles:                    3,
			MaxMechanicalWriteFiles:                  1,
			MechanicalWriteMustBeAlreadyUnderstood:   true,
			MechanicalWriteMustNotRequireResearch:    true,
			MechanicalWriteMustNotHaveOpenDesignWork: true,
		},
		DelegatedDirect: DelegatedDirectFacts{
			MappingMinUnderstandingFiles:  4,
			WriterMinNonTrivialFiles:      2,
			DelegateWhenReadPreparesWrite: true,
			DelegateWhenBroadResearch:     true,
		},
		SDD: SDDProposalFacts{
			ProposeWhenSubstantialOrAmbiguous:     true,
			DurableArtifactsMustReduceUncertainty: true,
			SelectionPolicy:                       SDDSelectionExplicitRequestOrAcceptedProposal,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CanonicalImplementationRouting() = %#v, want %#v", got, want)
	}
}

func TestManifestRejectsWeakenedRoutingFacts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		weaken func(*AgentCapabilityManifest)
	}{
		{
			name: "direct understanding starts below one file",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.DirectInline.MinUnderstandingFiles = 0
			},
		},
		{
			name: "direct understanding exceeds three files",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.DirectInline.MaxUnderstandingFiles = 4
			},
		},
		{
			name: "mapping starts after four files",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.DelegatedDirect.MappingMinUnderstandingFiles = 5
			},
		},
		{
			name: "writer starts after two non-trivial files",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.DelegatedDirect.WriterMinNonTrivialFiles = 3
			},
		},
		{
			name: "read preparing write no longer delegates",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.DelegatedDirect.DelegateWhenReadPreparesWrite = false
			},
		},
		{
			name: "broad research no longer delegates",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.DelegatedDirect.DelegateWhenBroadResearch = false
			},
		},
		{
			name: "substantial ambiguity no longer proposes SDD",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.SDD.ProposeWhenSubstantialOrAmbiguous = false
			},
		},
		{
			name: "SDD proposal need not reduce durable uncertainty",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.SDD.DurableArtifactsMustReduceUncertainty = false
			},
		},
		{
			name: "SDD selection bypasses explicit consent",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.SDD.SelectionPolicy = "automatic"
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			manifest := MustForAgent(model.AgentClaudeCode)
			test.weaken(&manifest)
			if err := manifest.Validate(); err == nil {
				t.Fatal("Validate() = nil, want non-canonical routing rejection")
			}
		})
	}
}

func TestEveryManifestKeepsWorkRoutingDormantAndHashesCanonically(t *testing.T) {
	t.Parallel()

	const wantRoutingDigest = "sha256:ed03b86f20c9449a6e4c018f51d1e05619e1070b1076287a0792a74c458762b2"
	// Digests pin the retained providers with an enforceable fresh-reviewer
	// boundary and provider-owned transport that reaches Go-owned admission.
	wantManifestDigests := map[model.AgentID]string{
		model.AgentAntigravity: "sha256:8e09945cd860b793c59f73db19827bcb4dcfd75c9ecc7f876167ab52fe77ccc2",
		model.AgentClaudeCode:  "sha256:132b9219b222d35b0e4eafce3dae965c56eb8d79f07dff6d45c42c137e36fd9b",
		model.AgentCodex:       "sha256:dbf94a3b7815cf68ccd6299c634f3e17be9abc305b3849adee382c65055c5ed9",
		model.AgentCursor:      "sha256:08e32b28b4cde7ffaf67210354fb95df2aaf424016ec6093190fb38c5f7226cb",
		model.AgentPi:          "sha256:0332851d2286a97ab824a1d656b94f02651bfbf85bdf0f6cc47fe8f7d09765ad",
	}

	for agent, wantDigest := range wantManifestDigests {
		agent := agent
		wantDigest := wantDigest
		t.Run(string(agent), func(t *testing.T) {
			t.Parallel()

			manifest := MustForAgent(agent)
			if err := manifest.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if manifest.Contracts.WorkRoutingV1.Exposure != ContractExposureDormant {
				t.Fatalf("work-routing exposure = %q, want %q", manifest.Contracts.WorkRoutingV1.Exposure, ContractExposureDormant)
			}
			if manifest.Advertises(ContractWorkRoutingV1) {
				t.Fatal("work-routing must remain unadvertised before final activation")
			}
			wantImmutableExecutor := agent == model.AgentClaudeCode || agent == model.AgentCodex || agent == model.AgentPi
			if got := manifest.Advertises(ContractImmutableReviewExecutorV1); got != wantImmutableExecutor {
				t.Fatalf("immutable reviewer execution advertised = %t, want %t", got, wantImmutableExecutor)
			}
			wantExposure := ContractExposureDormant
			if wantImmutableExecutor {
				wantExposure = ContractExposureAdvertised
			}
			if got := manifest.Contracts.ImmutableReviewExecutorV1.Exposure; got != wantExposure {
				t.Fatalf("immutable reviewer execution exposure = %q, want %q", got, wantExposure)
			}

			payload, err := manifest.CanonicalJSON()
			if err != nil {
				t.Fatalf("CanonicalJSON() error = %v", err)
			}
			var roundTrip AgentCapabilityManifest
			if err := json.Unmarshal(payload, &roundTrip); err != nil {
				t.Fatalf("Unmarshal(CanonicalJSON()) error = %v", err)
			}
			if roundTrip != manifest {
				t.Fatalf("canonical JSON round trip = %#v, want %#v", roundTrip, manifest)
			}

			gotDigest, err := roundTrip.Digest()
			if err != nil {
				t.Fatalf("Digest() error = %v", err)
			}
			if gotDigest != wantDigest {
				t.Fatalf("Digest() = %q, want %q", gotDigest, wantDigest)
			}

			gotRoutingDigest, err := manifest.RoutingDigest()
			if err != nil {
				t.Fatalf("RoutingDigest() error = %v", err)
			}
			if gotRoutingDigest != wantRoutingDigest {
				t.Fatalf("RoutingDigest() = %q, want %q", gotRoutingDigest, wantRoutingDigest)
			}
		})
	}
}

// TestEveryManifestDigestStaysByteStable pins every non-Pi row at the closed
// review-transport baseline for the four retained non-Pi providers.
func TestEveryManifestDigestStaysByteStable(t *testing.T) {
	t.Parallel()

	wantNonPiDigests := map[model.AgentID]string{
		model.AgentAntigravity: "sha256:8e09945cd860b793c59f73db19827bcb4dcfd75c9ecc7f876167ab52fe77ccc2",
		model.AgentClaudeCode:  "sha256:132b9219b222d35b0e4eafce3dae965c56eb8d79f07dff6d45c42c137e36fd9b",
		model.AgentCodex:       "sha256:dbf94a3b7815cf68ccd6299c634f3e17be9abc305b3849adee382c65055c5ed9",
		model.AgentCursor:      "sha256:08e32b28b4cde7ffaf67210354fb95df2aaf424016ec6093190fb38c5f7226cb",
	}

	nonPiAgents := make([]model.AgentID, 0, len(wantNonPiDigests))
	for agent := range wantNonPiDigests {
		nonPiAgents = append(nonPiAgents, agent)
	}

	if got := len(nonPiAgents); got != 4 {
		t.Fatalf("want 4 retained non-Pi agents, got %d", got)
	}

	for _, agent := range nonPiAgents {
		agent := agent
		wantDigest := wantNonPiDigests[agent]
		t.Run(string(agent), func(t *testing.T) {
			t.Parallel()

			manifest := MustForAgent(agent)
			gotDigest, err := manifest.Digest()
			if err != nil {
				t.Fatalf("Digest() error = %v", err)
			}
			if gotDigest != wantDigest {
				t.Fatalf("Digest() = %q, want %q (byte-stable contract)", gotDigest, wantDigest)
			}
		})
	}
}

func TestReviewTransportAdvertisementIsClosedCatalogSet(t *testing.T) {
	const wantExposed = 3

	exposed := 0
	for _, agent := range catalog.AllAgents() {
		t.Run(string(agent.ID), func(t *testing.T) {
			manifest := MustForAgent(agent.ID)
			want := agent.ID == model.AgentClaudeCode ||
				agent.ID == model.AgentCodex ||
				agent.ID == model.AgentPi
			if got := manifest.Advertises(ContractReviewTransportV1); got != want {
				t.Fatalf("review transport advertised = %t, want %t", got, want)
			}
			if want {
				exposed++
			}
		})
	}
	if exposed != wantExposed {
		t.Fatalf("advertised review transport runtimes = %d, want %d", exposed, wantExposed)
	}
}

func TestCanonicalFeatureClaimsCoverOnlyPersonalClients(t *testing.T) {
	want := map[model.AgentID]bool{
		model.AgentClaudeCode:  true,
		model.AgentCodex:       true,
		model.AgentCursor:      true,
		model.AgentAntigravity: true,
		model.AgentPi:          true,
	}
	if len(featureClaimsByAgent) != len(want) {
		t.Fatalf("canonical feature claim count = %d, want %d", len(featureClaimsByAgent), len(want))
	}
	for agent := range featureClaimsByAgent {
		if !want[agent] {
			t.Fatalf("retired agent %q entered canonical feature claims", agent)
		}
	}
}

func TestForAgentRejectsUnknownAgent(t *testing.T) {
	t.Parallel()

	_, err := ForAgent(model.AgentID("unknown"))
	if !errors.Is(err, ErrUnsupportedAgent) {
		t.Fatalf("ForAgent() error = %v, want ErrUnsupportedAgent", err)
	}
}
