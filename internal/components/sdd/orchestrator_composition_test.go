package sdd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/assets"
	"github.com/gentleman-programming/gentle-ai/v2/internal/catalog"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

const testGenericFallbackOnlyNativeRoute = "- Native route: This variant has no classified native question UI for this contract; always use the plain chat or terminal fallback below. When the closed domain of a single-select envelope is unrepresentable here, fall through to the Fallback clause below."

const testPiClosedSingleSelectNativeRoute = "- Native route: For every strictly closed single-select envelope, use ask_user_choice only when the interactive Pi TUI can represent its complete one-question 2-4 ordered-option domain. Pass each user-facing label and description with the envelope-owned canonical option token as value. The selector returns exactly one value; map it to the exact envelope-owned choice once, then select any envelope-owned continuation or invocation once where present. It has no custom/free-text or multi-select path. If the native TUI is unavailable or the envelope is not exactly representable, use the complete chat fallback. ask_user_question is the external open/free-text questionnaire and must not be used for a closed domain; open/free-text questionnaires may use ask_user_question when exactly representable. For gentle-ai.review-integration.consent/v3, the chosen continuation is still the exact captured provider-owned choice invocation, used once without synthesis."

// TestCanonicalCompositionAddsOnlyItsKnownSteps proves that composition is
// bounded-review rendering plus the shared-section substitution plus the Pi
// route, and nothing else. It deliberately no longer claims to preserve
// historical bytes: #3817 moved five section bodies into a shared asset, so
// this test's expected side must apply the same substitution, which means it
// cannot detect a substitution defect. Byte preservation across that move is
// proved where it actually lives — the rendered goldens under testdata/golden,
// none of which changed when the bodies moved.
func TestCanonicalCompositionAddsOnlyItsKnownSteps(t *testing.T) {
	for _, agent := range catalog.AllAgents() {
		t.Run(string(agent.ID), func(t *testing.T) {
			path := sddOrchestratorAsset(agent.ID)
			// #3817 adds shared-section substitution to the composition. The
			// invariant is unchanged in spirit: composition is bounded review
			// plus the shared sections plus the Pi route, and nothing else.
			content := substituteSharedOrchestratorSections(assets.MustRead(path))
			if agent.ID == model.AgentPi {
				content = strings.Replace(content, testGenericFallbackOnlyNativeRoute, testPiClosedSingleSelectNativeRoute, 1)
			}
			before := bindRuntimeAgentIdentity(renderBoundedReviewAssetBodyFromContent(agent.ID, path, content), agent.ID)
			after := composeOrchestratorPrompt(agent.ID)
			if after != before {
				t.Fatalf("canonical composition changed %s orchestrator bytes", agent.ID)
			}
		})
	}
}

func TestPiClosedChoiceRouteFailsClosedWhenGenericSourceClauseIsNotUnique(t *testing.T) {
	for _, tt := range []struct {
		name    string
		content string
	}{
		{name: "absent", content: "- Native route: custom runtime route"},
		{name: "duplicated", content: testGenericFallbackOnlyNativeRoute + "\n" + testGenericFallbackOnlyNativeRoute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				recovered := recover()
				if recovered == nil || !strings.Contains(fmt.Sprint(recovered), "Pi native route source clause count") {
					t.Fatalf("replacePiClosedSingleSelectRoute() panic = %v, want source clause count failure", recovered)
				}
			}()
			replacePiClosedSingleSelectRoute(tt.content, model.AgentPi)
		})
	}
}
