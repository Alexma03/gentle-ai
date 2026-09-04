package sdd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/assets"
	"github.com/gentleman-programming/gentle-ai/v2/internal/catalog"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/reviewerprovider"
)

// boundedReviewRequiredClausesFor is agent-parameterized because two of these
// clauses state the runtime identity the negotiated route must carry. Pinning
// them to a constant is what let issue #2440 ship: every runtime's generated
// instructions claimed to be claude-code, and the test suite agreed.
// captureTransportClausesFor returns the capture clauses that belong to the
// transport this runtime actually uses. A runtime whose compiled adapter
// captures in process is never told to assemble a reviewer prompt, because
// following that instruction would move the complete candidate onto the parent
// for every lens to reach a result one returned command already produces
// (issue #3825). Every other runtime keeps the relay wording: it is their only
// capture path.
func captureTransportClausesFor(agent model.AgentID) []string {
	if reviewerprovider.CapturesInProcess(agent) {
		return []string{
			"with its argument tokens exactly as returned",
			"This runtime captures in process",
			"Never assemble a reviewer prompt",
			"never add `--input` to a returned token list",
		}
	}
	return []string{
		"exact literal prefix `GENTLE_AI_REVIEW_BINDING `",
		"one-line JSON assembled only from that input",
		"`revision` from `expected-revision`",
		"`subject_hash` from `artifact_subject.subject_hash`",
	}
}

func boundedReviewRequiredClausesFor(agent model.AgentID) []string {
	if agent == model.AgentPi {
		return []string{
			"Native Compact Review Orchestration",
			"`gentle_review` with {\"operation\":\"inspect\"}",
			"`gentle_review` with operation `status`, the exact retained `lineageId`, and `workspaceRoot` only when needed",
			"`gentle_review_capture` for one current returned slot",
			"`gentle_review_capture_group` for the complete current reviewer group",
			"do not request or relay a second candidate-scoped consent",
			"Only the exact provider-issued acknowledgement continuation burns approved authority",
			"Commit, push, PR, and release remain separate human decisions under ordinary repository policy",
		}
	}
	return append(captureTransportClausesFor(agent), []string{
		"Native Compact Review Orchestration",
		"gentle-ai review status --cwd <repo> --contract gentle-ai.review-integration/v2 --agent " + string(agent) + " --next-transition",
		"Selectorless STATUS only preflights the current worktree candidate",
		"START freezes one compact atomic transaction",
		"run that provider-issued command verbatim",
		"exact tokens each returned transition names",
		"Route only from that transaction's returned `next_transition`",
		"Forecast is informational; route only from `next_transition`",
		"query the same exact-lineage STATUS",
		"reoffers the same bound slot",
		"repeated `--result-artifact-file <path>`",
		"Only candidate-caused severe findings block",
		"four-lens review is long work",
		"at-most-one bounded correction",
		"user-owned RDD switch is the complete review authorization",
		"provider-issued START runs automatically",
		"do not request or relay a second candidate-scoped consent",
		"validator that cannot inspect the immutable trees produced no verdict",
		"Claude Code, Codex, and Pi use the shared Go provider contract",
		"Never hand candidate bytes through `/tmp`",
		"### Authority-First Terminal Procedure",
		"Only that exact invocation burns authority and artifacts",
		"enabled gates return `invalidated/unmanaged`",
		"disabled gates return `disabled/unmanaged`",
		"The final reviewer, refuter, or targeted-validator capture owns closure.",
		"A malformed, incomplete, or unavailable capture never reaches acknowledgement: issue one retained target-bound read-only STATUS and relaunch only when it reoffers the same bound slot.",
		"Commit, push, PR, and release remain separate human decisions under ordinary repository policy.",
		"### Cross-repository lifecycle root",
		"explicit user authorization",
		"canonical B worktree root",
		"B as the lifecycle working root",
		"process cwd B",
		"Never append, remove, or rebuild provider-issued command tokens",
		"Opaque `repository_context` can capture or materialize from any process cwd",
		"Go owns repository binding; adapters never parse authorization or roots",
		"Approval awaits acknowledgement in B; exact acknowledgement burns B only, and A remains untouched",
		"review lifecycle stops",
		"Unsupported runtimes remain unavailable",
		"### Research and Pre-Proposal Gate (MANDATORY)",
		"immediately after `sdd-explore`",
		"selected research is `done` or research is unselected",
		"product decisions are `confirmed`",
		"evidence references are valid",
		"one lossless grouped prompt",
		"persist the pending state before prompting",
		"STOP without invoking `sdd-propose`",
	}...)
}

func TestReviewLifecycleContractRequiresAtomicBurnAndNonDecidingDelivery(t *testing.T) {
	content := boundedReviewContract()
	for _, want := range []string{
		"Selectorless STATUS only preflights the current worktree candidate",
		"START freezes one compact atomic transaction",
		"Only that exact invocation burns authority and artifacts",
		"enabled gates return `invalidated/unmanaged`",
		"disabled gates return `disabled/unmanaged`",
		"The final reviewer, refuter, or targeted-validator capture owns closure.",
		"A malformed, incomplete, or unavailable capture never reaches acknowledgement: issue one retained target-bound read-only STATUS and relaunch only when it reoffers the same bound slot.",
		"Commit, push, PR, and release remain separate human decisions under ordinary repository policy.",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("atomic lifecycle contract missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"reconcile-terminal-mirrors",
		"reviewGate.result: allow",
		"staged_delivery_candidate_required",
		"Reuse a valid receipt",
	} {
		if strings.Contains(content, forbidden) {
			t.Errorf("atomic lifecycle contract retains obsolete clause %q", forbidden)
		}
	}
}

func TestBoundedReviewStopInventoryIsCompleteWithoutRepeatingStatus(t *testing.T) {
	content := boundedReviewContract()
	const start = "### Continue after a stop reason code"
	const end = "## Delivery follows ordinary repository policy"
	startIndex := strings.Index(content, start)
	endIndex := strings.Index(content, end)
	if startIndex < 0 || endIndex < startIndex {
		t.Fatal("bounded review contract has no bounded stop inventory")
	}
	inventory := content[startIndex:endIndex]

	for _, code := range []string{
		"captured_artifacts_unverifiable",
		"captured_result_selection_unavailable",
		"missing_authority_binding",
		"corrupted_or_unverifiable_authority",
		"manual_intervention_required",
		"native_stop_required",
		"empty_base_diff_bootstrap_required",
		"lens_context_budget_exceeded",
		"staged_workspace_overlay_recovery_unavailable",
		"corrected_candidate_unavailable",
		"recovery_scope_unchanged",
		"rdd_disabled",
	} {
		if got := strings.Count(inventory, "`"+code+"`"); got != 1 {
			t.Errorf("stop inventory contains %d occurrences of %q, want exactly one", got, code)
		}
	}

	for _, group := range []string{
		"| `corrupted_or_unverifiable_authority`, `manual_intervention_required`, `native_stop_required` |",
	} {
		if strings.Count(inventory, group) != 1 {
			t.Errorf("stop inventory lost grouped continuation %q", group)
		}
	}

	for _, want := range []string{
		"`D` means `gentle-ai review mode disable --scope clone --cwd <B>`",
		"`S` means re-query the exact captured target-root STATUS command with lineage and target.",
		"then `S`; do not reuse the pre-correction target",
		"then `S`.",
	} {
		if !strings.Contains(inventory, want) {
			t.Errorf("stop inventory missing continuation alias rule %q", want)
		}
	}
	if strings.Contains(inventory, "gentle-ai review status --cwd") {
		t.Fatal("stop inventory repeats the canonical STATUS command instead of using S")
	}
	canonicalStatus := "gentle-ai review status --cwd <repo> --contract gentle-ai.review-integration/v2 --agent " + runtimeAgentIDPlaceholder + " --next-transition"
	if got := strings.Count(content, canonicalStatus); got != 1 {
		t.Fatalf("bounded review contract contains %d canonical STATUS commands, want exactly one", got)
	}
}

func TestBoundedReviewUsesModeAsCompleteAuthorization(t *testing.T) {
	content := boundedReviewContract()
	for _, want := range []string{
		"The user-owned RDD switch is the complete review authorization",
		"provider-issued START runs automatically for applicable candidates",
		"do not request or relay a second candidate-scoped consent",
		"global or clone-local disable command as the durable kill switch",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("orchestrator contract missing automatic review rule %q", want)
		}
	}
}

func TestPiRenderedReviewContractUsesOnlyTheClosedChoiceRoute(t *testing.T) {
	content := renderSDDOrchestratorAsset(model.AgentPi)
	for _, clause := range []string{
		"ask_user_choice",
		"2-4 ordered-option domain",
		"envelope-owned canonical option token as value",
		"returns exactly one value; map it to the exact envelope-owned choice once",
		"ask_user_question is the external open/free-text questionnaire and must not be used for a closed domain",
		"open/free-text questionnaires may use ask_user_question",
	} {
		if !strings.Contains(content, clause) {
			t.Errorf("Pi closed-choice route missing %q", clause)
		}
	}
	if strings.Contains(renderSDDOrchestratorAsset(model.AgentCursor), "ask_user_choice") {
		t.Fatal("generic fallback-only runtime received the Pi-only closed choice route")
	}
}

func TestBoundedReviewContractRendersForAdvertisedRuntimes(t *testing.T) {
	agents := catalog.AllAgents()
	rendered := 0
	for _, agent := range agents {
		if !expectedReviewLifecycleRuntime(agent.ID) {
			continue
		}
		rendered++
		t.Run(string(agent.ID), func(t *testing.T) {
			content := renderSDDOrchestratorAsset(agent.ID)
			assertTextContainsClauses(t, string(agent.ID), content, boundedReviewRequiredClausesFor(agent.ID))
			if strings.Count(content, researchLifecycleContract()) != 1 {
				t.Fatal("rendered orchestrator must contain one canonical research lifecycle")
			}
			// The retired WorkRun commands are gone from the assets, so nothing
			// here may require them. internal/assets/assets_test.go owns the
			// inverse assertion that they never come back.
			if strings.Contains(content, runtimeAgentIDPlaceholder) {
				t.Errorf("rendered %s retains runtime agent placeholder", agent.ID)
			}
			for _, forbidden := range []string{"review-start", "review-step", "review-resume", "review-validate", "review-bundle-export", "review-bundle-import"} {
				if strings.Contains(content, forbidden) {
					t.Errorf("rendered %s exposes lower-level compatibility command %q", agent.ID, forbidden)
				}
			}
			for _, forbidden := range []string{
				"exactly THREE refuters total",
				"3 total for full-4R",
				"run at most 2 sweeps per lens",
				"standard review or three lens passes sequentially",
				"verifies fix-touched lines",
				"may append fix-caused defects",
			} {
				if strings.Contains(content, forbidden) {
					t.Errorf("rendered %s retains obsolete review clause %q", agent.ID, forbidden)
				}
			}
		})
	}
	if rendered != 3 {
		t.Fatalf("review lifecycle runtime count = %d, want 3", rendered)
	}
	for _, forbidden := range []string{"review-start", "review-step", "review-resume", "review-validate", "review-bundle-export", "review-bundle-import"} {
		if strings.Contains(boundedReviewContract(), forbidden) {
			t.Errorf("orchestrator contract exposes lower-level compatibility command %q", forbidden)
		}
	}
	if got := sddOrchestratorAsset(model.AgentPi); got != "generic/sdd-orchestrator.md" {
		t.Fatalf("Pi orchestrator asset = %q, want generic adapter", got)
	}
}

func TestPersonalGentleStackRetainedTemplatesKeepOneFinalCandidateAuthority(t *testing.T) {
	wantAssets := map[model.AgentID]string{
		model.AgentClaudeCode:  "claude/sdd-orchestrator.md",
		model.AgentCodex:       "codex/sdd-orchestrator.md",
		model.AgentCursor:      "cursor/sdd-orchestrator.md",
		model.AgentAntigravity: "antigravity/sdd-orchestrator.md",
		model.AgentPi:          "generic/sdd-orchestrator.md",
	}

	agents := catalog.AllAgents()
	if len(agents) != len(wantAssets) {
		t.Fatalf("retained template count = %d, want %d", len(agents), len(wantAssets))
	}
	for _, agent := range agents {
		wantAsset, ok := wantAssets[agent.ID]
		if !ok {
			t.Errorf("retired client %q still has an orchestrator template", agent.ID)
			continue
		}
		if got := sddOrchestratorAsset(agent.ID); got != wantAsset {
			t.Errorf("orchestrator asset for %s = %q, want %q", agent.ID, got, wantAsset)
		}

		content := renderSDDOrchestratorAsset(agent.ID)
		for _, want := range []string{
			"Tasks and work units never create review authority or correction budgets.",
			"The SDD edit-authority consent relay only constrains filesystem edit roots; it never grants review authority.",
			"one immutable final-candidate correction transaction",
			"inferential blockers share one read-only refuter batch",
			"targeted validation",
			"Runtime and review selection remain user-owned",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("rendered %s template missing final-candidate clause %q", agent.ID, want)
			}
		}
		for _, forbidden := range []string{
			"tasks grant review authority",
			"work units grant correction budgets",
		} {
			if strings.Contains(content, forbidden) {
				t.Errorf("rendered %s template retains task authority clause %q", agent.ID, forbidden)
			}
		}
	}
}

func TestJudgmentDayReviewersUseNativeResultSchema(t *testing.T) {
	for name, content := range map[string]string{
		"rendered contract": judgmentDayReviewerContract(),
		"skill reference":   assets.MustRead("skills/judgment-day/references/prompts-and-formats.md"),
	} {
		for _, want := range []string{nativeReviewerResultSchema, "Never emit", "skill_resolution", "unknown field", "orchestration metadata outside the native result JSON", `{"findings":[],"evidence":["what was inspected"]}`} {
			if !strings.Contains(content, want) {
				t.Errorf("%s missing %q", name, want)
			}
		}
	}
}

func TestBoundedReviewContractDoesNotEnforceModelPolicy(t *testing.T) {
	content := boundedReviewContract()
	for _, forbidden := range []string{"MUST use model", "required provider", "enforced effort", "mandatory profile"} {
		if strings.Contains(content, forbidden) {
			t.Errorf("bounded review contract enforces model policy with %q", forbidden)
		}
	}
}

func TestBoundedReviewContractMakesCompatibilityGatesNonDeciding(t *testing.T) {
	content := boundedReviewContract()
	for _, clause := range []string{
		"Shipped `review validate` and gate commands are compatibility/informational only",
		"enabled gates return `invalidated/unmanaged`",
		"disabled gates return `disabled/unmanaged`",
		"They never allow, approve, block, commit, push, open a PR, or govern release",
	} {
		if !strings.Contains(content, clause) {
			t.Errorf("contract missing non-deciding gate clause %q", clause)
		}
	}
	for _, forbidden := range []string{"reviewGate.result: allow", "approved receipt", "reconcile-terminal-mirrors", "staged_delivery_candidate_required"} {
		if strings.Contains(content, forbidden) {
			t.Errorf("contract retains obsolete delivery gate clause %q", forbidden)
		}
	}
}

func TestAuthorityFirstTerminalProcedureIsStructuredAndAtomic(t *testing.T) {
	rows := parseAuthorityFirstRows(t, authorityFirstTerminalProcedure())
	want := []authorityFirstRow{
		{order: 1, operation: "canonical initial STATUS above", result: "exactly one current-worktree START preflight; no authority discovery"},
		{order: 2, operation: "exact returned START", result: "one compact lineage/worktree/target binding; retain lineage, revision, and target"},
		{order: 3, operation: "exact-lineage STATUS and collect", result: "only returned transaction actions; no ambient resume, reuse, or delivery gate"},
		{order: 4, operation: "final admitted capture", result: "native readback, approved authority, and one exact acknowledgement continuation"},
		{order: 5, operation: "STATUS restart + exact acknowledgement", result: "replayed operation/token/revision; only exact acknowledgement burns authority"},
		{order: 6, operation: "terminal lifecycle stop", result: "ordinary repository policy owns any later delivery decision"},
	}
	if len(rows) != len(want) {
		t.Fatalf("authority-first rows = %d, want %d", len(rows), len(want))
	}
	for index, expected := range want {
		if rows[index] != expected {
			t.Fatalf("authority-first row[%d] = %#v, want %#v", index, rows[index], expected)
		}
	}
}

func TestAuthorityFirstLifecycleRendersForAdvertisedRuntimes(t *testing.T) {
	rendered := 0
	for _, agent := range catalog.AllAgents() {
		if !expectedReviewLifecycleRuntime(agent.ID) {
			continue
		}
		rendered++
		t.Run(string(agent.ID), func(t *testing.T) {
			content := renderSDDOrchestratorAsset(agent.ID)
			if agent.ID == model.AgentPi {
				for _, want := range []string{"Inspect before START", "Stay bound", "Collect exactly", "Acknowledge exactly"} {
					if !strings.Contains(content, want) {
						t.Errorf("rendered Pi orchestrator missing facade lifecycle step %q", want)
					}
				}
				return
			}
			procedure := bindRuntimeAgentIdentity(authorityFirstTerminalProcedure(), agent.ID)
			if strings.Count(content, procedure) != 1 {
				t.Fatal("rendered orchestrator does not contain exactly one canonical terminal procedure")
			}
			for _, want := range []string{"Selectorless STATUS only preflights the current worktree candidate", "Route only from that transaction's returned `next_transition`", "Forecast is informational; route only from `next_transition`", "The final reviewer, refuter, or targeted-validator capture owns closure."} {
				if !strings.Contains(content, want) {
					t.Errorf("rendered orchestrator missing forecast contract %q", want)
				}
			}
		})
	}
	if rendered != 3 {
		t.Fatalf("authority-first lifecycle runtime count = %d, want 3", rendered)
	}
}

type authorityFirstRow struct {
	order     int
	operation string
	result    string
}

func parseAuthorityFirstRows(t *testing.T, content string) []authorityFirstRow {
	t.Helper()
	rows := make([]authorityFirstRow, 0, 15)
	for _, line := range strings.Split(content, "\n") {
		if len(line) < 4 || line[0] != '|' || line[2] < '0' || line[2] > '9' {
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) != 5 {
			t.Fatalf("malformed authority-first table row %q", line)
		}
		var order int
		if _, err := fmt.Sscanf(strings.TrimSpace(fields[1]), "%d", &order); err != nil {
			t.Fatalf("parse authority-first order: %v", err)
		}
		rows = append(rows, authorityFirstRow{
			order: order, operation: strings.Trim(strings.TrimSpace(fields[2]), "`"),
			result: strings.TrimSpace(fields[3]),
		})
	}
	return rows
}
