package assets

import (
	"fmt"
	"strings"
	"testing"
)

type blockingPromptRoute struct {
	nativeTool string
}

type providerDefectFixRoute string

const (
	providerDefectRecommendPublishedFix providerDefectFixRoute = "recommend_published_fix"
	providerDefectAddOccurrenceComment  providerDefectFixRoute = "add_occurrence_comment"
	providerDefectPossibleRegression    providerDefectFixRoute = "possible_regression"
)

// providerDefectEvidenceChannel models the prompt contract only. It deliberately
// has no production caller: this PR defines evidence relevance, not installation
// or updater-channel behavior.
func providerDefectEvidenceChannel(installedBuild string) string {
	if strings.Contains(installedBuild, "-rc.") || strings.Contains(installedBuild, "-main.") {
		return "prerelease"
	}
	return "stable"
}

func providerDefectRouteForPublishedFix(installedBuild, fixChannel string, published, predatesFix, containsFixAndReproduces bool) providerDefectFixRoute {
	if !published || providerDefectEvidenceChannel(installedBuild) != fixChannel {
		return providerDefectAddOccurrenceComment
	}
	if predatesFix {
		return providerDefectRecommendPublishedFix
	}
	if containsFixAndReproduces {
		return providerDefectPossibleRegression
	}
	return providerDefectAddOccurrenceComment
}

// generic is the provider-neutral source that every agent-specific handoff must project.
const providerDefectHandoffCanonicalPath = "generic/sdd-orchestrator.md"

var blockingPromptRoutes = map[string]blockingPromptRoute{
	"antigravity/sdd-orchestrator.md": {},
	"claude/sdd-orchestrator.md":      {nativeTool: "`AskUserQuestion`"},
	"codex/sdd-orchestrator.md":       {},
	"cursor/sdd-orchestrator.md":      {},
	"generic/sdd-orchestrator.md":     {},
}

func TestCoordinatorOrchestratorsCarryLosslessBlockingPromptRule(t *testing.T) {
	allPaths := allSDDOrchestratorAssetPaths(t)
	if len(allPaths) != len(blockingPromptRoutes) {
		t.Fatalf("discovered %d orchestrator variants, but %d have an explicit blocking-prompt route; classify every variant",
			len(allPaths), len(blockingPromptRoutes))
	}

	for _, path := range allPaths {
		route, classified := blockingPromptRoutes[path]
		if !classified {
			t.Fatalf("unclassified orchestrator variant %q; native/fallback routing must fail closed", path)
		}

		t.Run(path, func(t *testing.T) {
			contract := blockingPromptContractSection(t, path)
			for _, required := range []string{
				"complete user-facing choice envelope",
				"why input is required",
				"every group and question in original order",
				"including every group header",
				"every option label and description",
				"exact allowed-answer domain",
				"Never summarize, abbreviate, reorder, relabel, merge, or omit choices",
				"Never silently split an atomic business choice",
				"COMPLETE choice envelope as a plain chat or terminal response",
				"unavailable, denied, the runtime is noninteractive",
				"question-count, option-count, or text-length limits",
				"Then STOP",
				"Do not choose, default, infer, launch dependent work, or continue",
				"Accept an answer only when each response belongs to the exact allowed-answer domain",
				"free text or multi-select only when the original prompt allowed it",
				"request for information, not a candidate answer",
				"answer it directly from the envelope already held",
				"re-present the complete choice envelope and keep waiting",
				"invalid or ambiguous",
				"same blocked actor exactly once",
			} {
				if !strings.Contains(contract, required) {
					t.Errorf("lossless blocking-prompt contract missing %q", required)
				}
			}

			if route.nativeTool == "" {
				const fallbackOnly = "This variant has no classified native question UI for this contract; always use the plain chat or terminal fallback below."
				if !strings.Contains(contract, fallbackOnly) {
					t.Errorf("fallback-only variant missing explicit route %q", fallbackOnly)
				}
				return
			}

			for _, required := range []string{
				fmt.Sprintf("The classified native question UI is %s.", route.nativeTool),
				"Use it only when it is available in the current interactive runtime",
				"exactly representable in one grouped interaction",
			} {
				if !strings.Contains(contract, required) {
					t.Errorf("native route missing %q", required)
				}
			}
		})
	}
}

// TestCoordinatorOrchestratorsCarryClosedSingleSelectDomainContract guards
// issue #3070: the closed-domain rule lives inside the existing Native route
// and Answer validation clauses. The ordinal alias domain must be EXPLICIT
// (not "for example") per Matere413's review on 2026-08-14. The issue's
// reproduction (`la 1`) must be present.
func TestCoordinatorOrchestratorsCarryClosedSingleSelectDomainContract(t *testing.T) {
	for _, path := range allSDDOrchestratorAssetPaths(t) {
		t.Run(path, func(t *testing.T) {
			contract := blockingPromptContractSection(t, path)
			for _, required := range []string{
				"closed domain of a single-select envelope",
				"EXACTLY ONE presented option",
				"reject zero matches and reject multiple matches",
				"canonical internal token once",
				"Accepted ordinal aliases, for each presented option index N",
				"the bare numeral `N`",
				"`la N`",
				"`opción N`",
				"`first` is additionally accepted for index 1",
			} {
				if !strings.Contains(contract, required) {
					t.Errorf("%s missing closed-domain amendment %q", path, required)
				}
			}
			// Matere413: ordinal alias domain must be explicit, not "for example".
			if strings.Contains(contract, "Ordinal aliases") && strings.Contains(contract, "for example") {
				idx := strings.Index(contract, "Ordinal aliases")
				end := strings.Index(contract[idx:], "\n")
				if end < 0 {
					end = len(contract)
				} else {
					end += idx
				}
				if strings.Contains(contract[idx:end], "for example") {
					t.Errorf("%s uses 'for example' for ordinal aliases (Matere413: must be explicit list)", path)
				}
			}
		})
	}
}

func providerDefectHandoffLine(t *testing.T, contract, prefix string) string {
	t.Helper()
	start := strings.Index(contract, prefix)
	if start < 0 {
		t.Fatalf("provider-defect handoff missing invariant line starting %q", prefix)
	}
	end := strings.Index(contract[start:], "\n")
	if end < 0 {
		return contract[start:]
	}
	return contract[start : start+end]
}

// TestCoordinatorOrchestratorsCarrySDDEditAuthorityConsentRelay is #2570's
// (S6 of #2540) guard: every orchestrator variant teaches the lossless relay
// of the typed SDD edit-authority consent envelope that native status emits
// on blocked(edit_authority_missing) (#2563), byte-identical across variants
// like the provider-defect handoff above.
func TestCoordinatorOrchestratorsCarrySDDEditAuthorityConsentRelay(t *testing.T) {
	requirements := []string{
		"When native SDD status reports `blocked(edit_authority_missing)`",
		"typed `gentle-ai.sdd-integration.consent/v1` envelope",
		"optional `consent` block",
		"Treat that envelope as a Lossless Blocking Prompt under this contract",
		"same discipline as the review consent relay",
		"Present the complete envelope once in the active conversation language",
		"faithfully translate the headline, reason, `value`, the missing-root evidence, choice labels, every choice `effect`, and the off-path note",
		"preserving the original choices, order, selection mode, exact allowed-answer domain, and answer tokens",
		"Never translate or alter the machine answer tokens (`granted`, `declined`), commands, paths, or invocations",
		"Never summarize, reshape, reorder, merge, or omit any part",
		"never answer on the human's behalf and never run the grant unprompted",
		"Only after the human's explicit `granted` answer",
		"execute the envelope's exact grant invocation verbatim, exactly once",
		"then re-enter through native status",
		"granted roots project into `allowedEditRoots`",
		"per-change, audited, and dies with archive",
		"run the envelope's decline invocation",
		"nothing is persisted",
		"names both exits",
		"edit tasks.md so every work unit stays inside the authorized edit roots, or grant this change edit authority",
		"A blocked status without a `consent` block names the same two exits; relay them and stop.",
	}

	for _, path := range allSDDOrchestratorAssetPaths(t) {
		t.Run(path, func(t *testing.T) {
			contract := sddConsentRelaySection(t, path)
			if canonical := sddConsentRelaySection(t, providerDefectHandoffCanonicalPath); contract != canonical {
				t.Error("SDD edit-authority consent relay differs from the canonical cross-variant block")
			}
			for _, required := range requirements {
				if !strings.Contains(contract, required) {
					t.Errorf("SDD edit-authority consent relay missing %q", required)
				}
			}
		})
	}
}

func sddConsentRelaySection(t *testing.T, path string) string {
	t.Helper()
	const heading = "#### SDD Edit-Authority Consent Relay (MANDATORY)"
	content := MustRead(path)
	start := strings.Index(content, heading)
	if start == -1 {
		t.Fatalf("%s missing %q", path, heading)
	}
	contract := content[start:]
	const endMarker = "A blocked status without a `consent` block names the same two exits; relay them and stop."
	end := strings.Index(contract, endMarker)
	if end == -1 {
		t.Fatalf("%s SDD edit-authority consent relay missing terminal boundary", path)
	}
	return strings.TrimSpace(contract[:end+len(endMarker)])
}

func providerDefectHandoffSection(t *testing.T, path string) string {
	t.Helper()
	const heading = "#### Gentle AI Provider Defect Handoff (MANDATORY)"
	content := MustRead(path)
	start := strings.Index(content, heading)
	if start == -1 {
		t.Fatalf("%s missing %q", path, heading)
	}
	contract := content[start:]
	const endMarker = "Never resume against unpublished code: a source checkout, a local build, or an unmerged pull request."
	end := strings.Index(contract, endMarker)
	if end == -1 {
		t.Fatalf("%s provider-defect handoff missing terminal release boundary", path)
	}
	return strings.TrimSpace(contract[:end+len(endMarker)])
}

func blockingPromptContractSection(t *testing.T, path string) string {
	t.Helper()
	const heading = "### Lossless Blocking Prompts (MANDATORY)"
	content := MustRead(path)
	start := strings.Index(content, heading)
	if start == -1 {
		t.Fatalf("%s missing %q", path, heading)
	}
	contract := content[start:]
	if end := strings.Index(contract[len(heading):], "\n##"); end >= 0 {
		contract = contract[:len(heading)+end]
	}
	return contract
}
