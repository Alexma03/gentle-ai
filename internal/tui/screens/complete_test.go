package screens

import (
	"strings"
	"testing"
)

func TestRenderCompleteSuccessShowsNextSteps(t *testing.T) {
	out := RenderComplete(CompletePayload{
		ConfiguredAgents:    1,
		InstalledComponents: 1,
	})

	if !strings.Contains(out, "Next steps") || !strings.Contains(out, "Run your selected agent") {
		t.Fatalf("missing completion next steps: %q", out)
	}
}

func TestRenderCompleteSuccessDoesNotMentionRetiredIntegrations(t *testing.T) {
	out := RenderComplete(CompletePayload{
		ConfiguredAgents:    1,
		InstalledComponents: 1,
	})

	for _, retired := range []string{"gga", "theme", "marketplace"} {
		if strings.Contains(strings.ToLower(out), retired) {
			t.Fatalf("unexpected retired integration %q: %q", retired, out)
		}
	}
}

func TestRenderCompleteShowsManualActions(t *testing.T) {
	out := RenderComplete(CompletePayload{ManualActions: []string{"Pi CodeGraph child drifted; preserved: /tmp/worker.md"}})
	if !strings.Contains(out, "Manual actions required") || !strings.Contains(out, "Pi CodeGraph child drifted") {
		t.Fatalf("manual action missing from completion output: %q", out)
	}
}

func TestRenderCompleteDistinguishesPartialRollbackAndKeepsManualActions(t *testing.T) {
	out := RenderComplete(CompletePayload{FailedSteps: []FailedStep{{ID: "install", Error: "failed"}}, RollbackPerformed: true, RollbackComplete: false, ManualActions: []string{"Pi drift requires manual repair"}})
	if !strings.Contains(out, "partially completed") || !strings.Contains(out, "Pi drift requires manual repair") {
		t.Fatalf("output = %q, want partial rollback and manual action", out)
	}
}
