package reviewtransaction

import (
	"context"
	"testing"
)

// TestRDDModeResolvesPersonalDefaultAndPersistedOverrides proves the personal
// default is on while explicit persisted decisions remain authoritative.
func TestRDDModeResolvesPersonalDefaultAndPersistedOverrides(t *testing.T) {
	repo := initSnapshotRepo(t)
	ctx := context.Background()

	status, err := ResolveRDDMode(ctx, repo, RDDGlobalMode{})
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled() || status.Source != RDDModeSourceDefault {
		t.Fatalf("RDD mode did not resolve the personal default: %#v", status)
	}

	status, err = ResolveRDDMode(ctx, repo, RDDGlobalMode{Value: string(RDDModeOn)})
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled() {
		t.Fatal("explicit persisted global RDD enable did not enable review mode")
	}

	if _, err := SetCloneLocalRDDMode(ctx, repo, RDDModeOff, "", RDDGlobalMode{}); err != nil {
		t.Fatal(err)
	}
	status, err = ResolveRDDMode(ctx, repo, RDDGlobalMode{Value: string(RDDModeOn)})
	if err != nil {
		t.Fatal(err)
	}
	if status.Enabled() {
		t.Fatal("persisted clone-local RDD off did not override global opt-in")
	}
}
