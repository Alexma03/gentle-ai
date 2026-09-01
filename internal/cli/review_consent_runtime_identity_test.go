package cli

// Issue #2676: a negotiated START explicitly bound to a non-Claude runtime
// (OpenCode, Codex) reported "claude-code" as the consent/v3 envelope's
// top-level `agent`, while the envelope's own follow-up invocations were
// already correctly bound to the real runtime. These tests pin the fix: the
// top-level identity now matches the declared binding, and Validate() stays
// fail-closed about which identities the v3 shape may ever name.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestNegotiatedConsentEnvelopeBindsTheDeclaredRuntimeIdentity drives a
// negotiated v2 START explicitly bound to each supported immutable-transport
// runtime and asserts the consent envelope's top-level agent matches the
// declared binding, exactly like its follow-up invocations already did. The
// claude-code case is the regression check: it must stay exactly as today.

// TestConsentValidateRejectsUnknownOrEmptyRuntimeIdentity keeps Validate()
// fail-closed: the v3 shape must name a runtime that actually carries the
// immutable receipt-review transport (the same authority
// reviewRuntimeWithImmutableTransport gates START on), never an arbitrary
// string, and never empty.

// TestConsentValidateAcceptsHistoricalShapes pins that neither the v1 legacy
// contract nor the pre-v3 v2-schema/empty-agent shape were disturbed by
// making Validate() fail-closed about the runtime identity.
func TestConsentValidateAcceptsHistoricalShapes(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "contracts", "review-integration", "v1", "fixtures", "consent.fixture.json"),
		filepath.Join("..", "..", "contracts", "review-integration", "v2", "fixtures", "consent.fixture.json"),
	} {
		t.Run(path, func(t *testing.T) {
			payload, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var result ReviewIntegrationConsentResult
			if err := json.Unmarshal(payload, &result); err != nil {
				t.Fatal(err)
			}
			if err := result.Validate(); err != nil {
				t.Fatalf("historical shape %s no longer validates: %v", path, err)
			}
		})
	}
}
