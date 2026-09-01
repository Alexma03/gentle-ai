package screens_test

import (
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/tui/screens"
)

// ─── WelcomeOptions ──────────────────────────────────────────────────────────

// TestWelcomeOptions_WithoutProfiles verifies that when showProfiles is false,
// the profile option is not present and retired marketplace entries stay gone.

// TestWelcomeOptions_WithProfiles_ZeroCount shows "OpenCode SDD Profiles" without a badge.

// TestWelcomeOptions_WithProfiles_CountTwo shows "OpenCode SDD Profiles (2)".

// TestWelcomeOptions_WithProfiles_CountOne shows "OpenCode SDD Profiles (1)".

// TestWelcomeOptions_OptionCount_WithoutProfiles verifies 12 options when showProfiles=false
// and hasEngines=true.
func TestWelcomeOptions_OptionCount_WithoutProfiles(t *testing.T) {
	opts := screens.WelcomeOptions(nil, true, false, 0, true)
	// Includes the Receipt-Driven Development entry.
	want := 12
	if len(opts) != want {
		t.Errorf("WelcomeOptions(showProfiles=false, hasEngines=true) = %d options, want %d; opts: %v", len(opts), want, opts)
	}
}

// TestWelcomeOptions_OptionCount_WithProfiles verifies 13 options when showProfiles=true
// and hasEngines=true.

// TestWelcomeOptions_NoEngines_ShowsDisabledLabel verifies that when hasEngines=false,
// the agent option is labelled "(no agents)" to signal unavailability.
func TestWelcomeOptions_NoEngines_ShowsDisabledLabel(t *testing.T) {
	opts := screens.WelcomeOptions(nil, true, false, 0, false)
	found := false
	for _, opt := range opts {
		if strings.Contains(opt, "no agents") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'no agents' label when hasEngines=false; got: %v", opts)
	}
}

// TestWelcomeOptions_ProfilesInsertedBeforeManageBackups verifies the ordering:
// the retained profiles option sits immediately before "Manage backups".

func containsOption(opts []string, want string) bool {
	for _, opt := range opts {
		if opt == want {
			return true
		}
	}
	return false
}

func TestWelcomeOptions_IncludesManagedUninstall(t *testing.T) {
	opts := screens.WelcomeOptions(nil, true, false, 0, true)

	found := false
	for _, opt := range opts {
		if opt == "Managed uninstall" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected 'Managed uninstall' option; got: %v", opts)
	}
}

// ─── RenderWelcome ────────────────────────────────────────────────────────────

// TestRenderWelcome_WithoutProfiles verifies no "OpenCode SDD Profiles" in output.

// TestRenderWelcome_WithProfiles_ZeroCount contains "OpenCode SDD Profiles" but no badge.

// TestRenderWelcome_WithProfiles_CountTwo contains "OpenCode SDD Profiles (2)".

// TestRenderWelcome_WithProfiles_CountOne contains "OpenCode SDD Profiles (1)".
