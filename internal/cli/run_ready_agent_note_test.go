package cli

import (
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/planner"
	"github.com/gentleman-programming/gentle-ai/v2/internal/verify"
)

// TestWithPostInstallNotesNamesOnlyTheInstalledRunnableAgents closes fisidj
// finding 4 (organic-dx Phase 3f task 3f.4): an OpenCode-only install must not
// tell the user to "Run `claude` or `opencode`" -- it is the literal last
// line of the first-run experience and must name only what was installed.

// TestWithPostInstallNotesFallsBackWhenNoRunnableAgentSelected proves an
// install that selected neither of the two named runnable agents (e.g. an
// IDE-integrated agent only) gets the generic ready message instead of naming
// an agent that was never installed.

// TestWithPostInstallNotesDoesNotOverrideAnAlreadyCustomizedFinalNote proves
// the override is scoped to exactly the generic production ready message and
// never clobbers a FinalNote a test (or another note-builder) constructed by
// hand with different text.
func TestWithPostInstallNotesDoesNotOverrideAnAlreadyCustomizedFinalNote(t *testing.T) {
	t.Parallel()

	// AgentClaudeCode isolates the ready-agent-run override.
	report := verify.Report{Ready: true, FinalNote: "You're ready."}
	resolved := planner.ResolvedPlan{Agents: []model.AgentID{model.AgentClaudeCode}}

	updated := withPostInstallNotes(report, resolved)
	if updated.FinalNote != "You're ready." {
		t.Fatalf("FinalNote changed unexpectedly: %q", updated.FinalNote)
	}
}
