package main

import "testing"

func TestReviewModeTTYSelectionKeys(t *testing.T) {
	t.Parallel()

	const want = "\x1b[A\x1b[A\x1b[A\r"
	if got := reviewModeTTYSelectionKeys(); got != want {
		t.Fatalf("review mode selection keys = %q, want %q", got, want)
	}
}
