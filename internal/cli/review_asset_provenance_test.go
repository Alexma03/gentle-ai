package cli

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/state"
)

// TestManagedReviewerAssetProvenanceRefusesOnlyRecordedSkew pins the two
// shapes the refusal must NOT take. Only a recorded digest that disagrees is
// stale; a home that never installed anything has no managed assets to be
// stale, and refusing it would block every `go install` user from reviewing
// while telling them to run a sync that would fix nothing.

func TestManagedAssetsPreflightDoesNotClassifyUnrelatedRuntimeRefusal(t *testing.T) {
	failure := newReviewIntegrationFailure("review.start", nil, errors.New("unrelated runtime refusal"))
	if failure.Code != "operation_outcome_unknown" || failure.Phase != "native_running" ||
		failure.MutationOutcome != ReviewMutationUnknown || failure.NextAction != "review.status" ||
		!strings.Contains(failure.Cause, "unrelated runtime refusal") {
		t.Fatalf("unrelated runtime failure = %#v", failure)
	}
}

// TestManagedAssetDigestIsStableAndAssetBound proves the digest is a property
// of the embedded assets rather than of the build, which is the whole reason
// it replaced the capabilities build identity: a rebuild that changes no asset
// must not invalidate an installation, and a test binary must be able to agree
// with a released one.
func TestManagedAssetDigestIsStableAndAssetBound(t *testing.T) {
	first, err := managedAssetDigest()
	requireManagedAssetProvenanceNoError(t, err)
	second, err := managedAssetDigest()
	requireManagedAssetProvenanceNoError(t, err)
	if first != second || first == "" {
		t.Fatalf("digest is not stable: %q then %q", first, second)
	}
	build, err := reviewCapabilitiesBuildIdentity(AppVersion)
	requireManagedAssetProvenanceNoError(t, err)
	if first == build.ID {
		t.Fatal("digest equals the build identity, so it still carries build metadata")
	}
}

// staleManagedReviewerAssets records an asset digest that disagrees with this
// binary's, which is the only skew the provenance guard refuses on.
//
// It reads the existing user state and rewrites only that one field. A blind
// state.Write would also erase the explicit global "on" these fixtures depend
// on -- receipt-driven development is opt-in, so wiping it would turn every
// following gate into a disabled/unmanaged report and the provenance refusal
// under test would never be reached.
func staleManagedReviewerAssets(t *testing.T, home string) {
	t.Helper()
	recordManagedAssetDigest(t, home, "sha256:stale")
}

// recordManagedAssetDigest rewrites only the recorded managed-asset digest,
// preserving every other opinion already persisted in the user's state.
func recordManagedAssetDigest(t *testing.T, home, digest string) {
	t.Helper()
	// A home with nothing persisted yet is an ordinary starting point here, so
	// an absent state file seeds an empty one rather than failing the fixture.
	persisted, err := state.Read(home)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	persisted.ManagedAssetDigest = digest
	requireManagedAssetProvenanceNoError(t, state.Write(home, persisted))
}
func requireManagedAssetProvenanceNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func requireManagedAssetProvenanceError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte(want)) {
		t.Fatalf("delivery error = %v, want %q", err, want)
	}
}
