package assets

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// retiredWorkRunCeremonyTokens enumerates the managed-WorkRun control-plane
// vocabulary that organic routing retires. Prompt assets are the one place this
// ceremony can outlive its Go source, because nothing compiles them — so every
// token is checked against every orchestrator instead of a sample.
var retiredWorkRunCeremonyTokens = []string{
	"work-capabilities",
	"work-start",
	"work-advance",
	"work-route",
	"work-status",
	"work-transition",
	"work-reconcile",
	"work-verification-decide",
	"WorkRun",
	"authorizedTransition",
	"Capability stop rule",
	"connectorSessionRef",
	"GENTLE_AI_PRODUCTIVE_RUNTIME",
	"{{GENTLE_AI_RUNTIME_AGENT_ID}}",
	"--contract gentle-ai.work-",
}

func TestSDDOrchestratorsCarryNoRetiredWorkRunCeremony(t *testing.T) {
	paths := allSDDOrchestratorAssetPaths(t)
	if len(paths) != 5 {
		t.Fatalf("WorkRun-removal coverage sees %d orchestrators, want 5", len(paths))
	}

	for _, path := range paths {
		// Fold case so a re-cased reintroduction ("workrun", "WORK-START")
		// cannot slip past the guard.
		content := strings.ToLower(MustRead(path))
		for _, token := range retiredWorkRunCeremonyTokens {
			t.Run(path+"#"+token, func(t *testing.T) {
				if strings.Contains(content, strings.ToLower(token)) {
					t.Fatalf("%s retains retired WorkRun ceremony token %q", path, token)
				}
			})
		}
	}
}

func TestOrchestratorsProjectOrganicRouting(t *testing.T) {
	paths := allSDDOrchestratorAssetPaths(t)
	if len(paths) != 5 {
		t.Fatalf("organic routing coverage sees %d orchestrators, want 5", len(paths))
	}

	for _, path := range paths {
		content := MustRead(path)
		for _, required := range []string{
			"Mandatory Delegation Triggers",
			"Bounded read rule", "read 1–3 files inline",
			"4-file rule", "understanding requires 4+ files",
			"Write rule", "2+ non-trivial files",
			"Context rule", "reading that prepares a write", "broad research",
			"Per-action rule", "Optional SDD rule",
			"explicit request or accepted proposal", "risk alone never forces SDD",
			// The three implementation routes must stay nameable without any
			// control-plane handshake in front of them.
			"**direct inline**", "**delegated direct**", "**optional SDD**",
			"size, file count, or risk alone never selects SDD",
		} {
			if !strings.Contains(content, required) {
				t.Fatalf("%s missing organic routing/native authority contract %q", path, required)
			}
		}
		for _, retired := range []string{
			"#### Review Lens Selection", "run exactly ONE lens", "run the full 4R set",
			"review/start(target)", "gentle-ai.review-integration/v1 --next-transition",
		} {
			if strings.Contains(content, retired) {
				t.Fatalf("%s retained prompt-owned review ceremony %q", path, retired)
			}
		}

		delegationHeading := "### Delegation Rules"
		if path == "codex/sdd-orchestrator.md" {
			delegationHeading = "## General Delegation Rules (Always Active)"
		}
		start := strings.Index(content, delegationHeading)
		end := strings.Index(content, "#### Mandatory Delegation Triggers")
		if start < 0 || end <= start {
			t.Fatalf("%s missing bounded general delegation section", path)
		}
		delegation := content[start:end]
		for _, required := range []string{"delegated direct", "never selects SDD", "creates SDD state", "`sdd-*`"} {
			if !strings.Contains(delegation, required) {
				t.Fatalf("%s general delegation section missing route-neutral clause %q", path, required)
			}
		}
		for _, forbidden := range []string{
			"4+ files) | — | ✅ `sdd-explore`",
			"4+ files) | — | ✅ run as sdd-explore",
			"multiple files, new logic) | — | ✅ run as sdd-apply",
			"tests, builds, installs | — | ✅ `sdd-verify`",
			"Phase boundaries are not optional",
		} {
			if strings.Contains(delegation, forbidden) {
				t.Fatalf("%s general delegation section routes ordinary work through SDD %q", path, forbidden)
			}
		}
	}
}

func TestAllShippedOrchestratorsKeepDeliveryUnmanaged(t *testing.T) {
	const ordinaryDelivery = "Commit, push, PR, direct-main, emergency, and release gates are informational and unmanaged; ordinary repository policy decides delivery and they never reopen review for unchanged content."
	const receiptValidation = "Commit, push, PR, direct-main, emergency, and release gates validate the same exact owner-issued receipt/authorization"

	for _, path := range allSDDOrchestratorAssetPaths(t) {
		content := MustRead(path)
		if !strings.Contains(content, ordinaryDelivery) {
			t.Fatalf("%s does not leave delivery to ordinary repository policy", path)
		}
		if strings.Contains(content, receiptValidation) {
			t.Fatalf("%s retains receipt-gated delivery guidance", path)
		}
	}
}

func normalizedWords(s string) string {
	var b strings.Builder
	lastWasSpace := true
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastWasSpace = false
			continue
		}

		if !lastWasSpace {
			b.WriteByte(' ')
			lastWasSpace = true
		}
	}

	return strings.TrimSpace(b.String())
}

// TestAllEmbeddedAssetsAreReadable verifies that every expected embedded file
// can be loaded via Read() without error. This catches missing/misnamed files
// at test time rather than at runtime.

func TestSDDInitRequiresBoundedWorkspaceProjectDiscovery(t *testing.T) {
	skill := MustRead("skills/sdd-init/SKILL.md")
	for _, required := range []string{
		"authoritative workspace root",
		"Before classifying a stack or applying any no-runner fallback",
		"Aggregate those project-to-tool associations in the one workspace-level result",
		"non-empty discovered project set",
		"explicit workspace-level test command",
		"covers every in-scope project",
		"zero projects are discovered or no explicit workspace-level test command covers every in-scope project",
		"including missing or independent commands; those local facts do not override a workspace-level command that covers every in-scope project",
	} {
		if !strings.Contains(skill, required) {
			t.Fatalf("sdd-init skill missing workspace discovery contract %q", required)
		}
	}

	details := MustRead("skills/sdd-init/references/init-details.md")
	for _, required := range []string{
		"explicit workspace membership",
		"at most two directory levels",
		"`A/pyproject.toml` and `B/Cargo.toml`",
		"nested-repository boundaries",
		"`.git`, `node_modules`, `vendor`, `dist`, `build`, `out`, `target`, `.cache`, `__pycache__`, `.venv`, `venv`",
		"`projects:` list",
		"discovered project set is non-empty",
		"explicit workspace-level test command",
		"covers every in-scope project",
		"Do not synthesize or concatenate independent project commands",
		"zero projects are discovered or no explicit workspace-level command covers every in-scope project",
		"including missing or independent commands; those local facts do not override a workspace-level command that covers every in-scope project",
	} {
		if !strings.Contains(details, required) {
			t.Fatalf("sdd-init details missing bounded discovery contract %q", required)
		}
	}

	if discovery, fallback := strings.Index(details, "## Workspace Project Discovery"), strings.Index(details, "only then apply the no-runner fallback"); discovery < 0 || fallback < discovery {
		t.Fatal("sdd-init details must complete workspace discovery before the no-runner fallback")
	}
	if workspaceDiscovery, strictTDDResolution := strings.Index(skill, "1. Identify the authoritative workspace root."), strings.Index(skill, "4. Resolve Strict TDD from an agent marker or `openspec/config.yaml`"); workspaceDiscovery < 0 || strictTDDResolution < 0 || workspaceDiscovery >= strictTDDResolution {
		t.Fatal("sdd-init skill must place workspace discovery step 1 before Strict TDD resolution step 4")
	}
	for _, content := range []string{skill, details} {
		if strings.Contains(content, "a project has no test command") {
			t.Fatal("sdd-init fallback must not treat a missing project-local command as an independent Strict TDD disablement reason")
		}
	}
}

func TestSDDVerificationAndArchiveContractsIgnoreReviewContext(t *testing.T) {
	statusContract := MustRead("skills/_shared/sdd-status-contract.md")
	for _, want := range []string{
		"`verify` is `ready` only when every implementation task is complete and required planning/apply evidence is available.",
		"Review presence, absence, or non-allow state is informational: it never routes status to `review`, suppresses test/build execution, or blocks verification.",
		"`archive` is `ready` only when tasks are complete and strict SDD verification passes.",
	} {
		if !strings.Contains(statusContract, want) {
			t.Fatalf("sdd-status-contract missing independent SDD verification rule %q", want)
		}
	}
	for _, forbidden := range []string{
		"persisted bounded transaction reaches `ready_final_verification`",
		"Missing or active review state routes to `review`",
	} {
		if strings.Contains(statusContract, forbidden) {
			t.Fatalf("sdd-status-contract retains pre-verify review dependency %q", forbidden)
		}
	}

	for _, path := range []string{
		"skills/sdd-verify/SKILL.md",
		"skills/sdd-verify/references/report-format.md",
	} {
		content := MustRead(path)
		for _, want := range []string{
			"Review state is informational and never a verification prerequisite.",
			"A missing, pending, invalid, or non-allow review state never suppresses tests or builds.",
			"Exit `125` is reserved for an actual verification prerequisite or unavailable verification tooling, never missing review authority.",
		} {
			if !strings.Contains(content, want) {
				t.Fatalf("%s missing independent verification rule %q", path, want)
			}
		}
		for _, forbidden := range []string{"missing_review_authority", "authority_only_failure"} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s retains missing-review preflight denial %q", path, forbidden)
			}
		}
	}

	verifySkill := MustRead("skills/sdd-verify/SKILL.md")
	for _, want := range []string{
		"Review state is informational and never a verification prerequisite.",
		"A missing, pending, invalid, or non-allow review state never suppresses tests or builds.",
		"Exit `125` is reserved for an actual verification prerequisite or unavailable verification tooling, never missing review authority.",
	} {
		if got := strings.Count(verifySkill, want); got != 2 {
			t.Fatalf("sdd-verify must state independent verification in both model sections: %q occurs %d times", want, got)
		}
	}

	archiveSkill := MustRead("skills/sdd-archive/SKILL.md")
	for _, want := range []string{
		"CRITICAL issues in `verify-report` still block archive with no prompt override",
		"reviewOffer` is an invitation only and is never read as archive state",
		"The Task Completion Gate and strict independent verification decide whether archive can proceed",
	} {
		if !strings.Contains(archiveSkill, want) {
			t.Fatalf("sdd-archive missing independent archive prerequisite %q", want)
		}
	}
}

func TestSDDVerifyAdmissionPrecedesPersistence(t *testing.T) {
	for _, path := range []string{"skills/sdd-verify/SKILL.md", "skills/sdd-verify/references/report-format.md", "skills/_shared/sdd-phase-common.md", "skills/_shared/persistence-contract.md"} {
		content := MustRead(path)
		for _, want := range []string{"sdd-verify-validate", "exact candidate bytes", "before any OpenSpec or Engram write", "validator is unavailable", "valid `fail`"} {
			if !strings.Contains(content, want) {
				t.Fatalf("%s missing admission contract %q", path, want)
			}
		}
	}
	contract := MustRead("skills/_shared/persistence-contract.md")
	for _, want := range []string{"Do not create, truncate, delete, or overwrite any prior `verify-report`", "A valid `fail` report must be persisted", "validator is unavailable"} {
		if !strings.Contains(contract, want) {
			t.Fatalf("persistence contract missing %q", want)
		}
	}
	if count := strings.Count(MustRead("skills/sdd-verify/SKILL.md"), "sdd-verify-validate"); count < 2 {
		t.Fatalf("both sdd-verify model sections require admission, got %d occurrences", count)
	}
	for _, path := range []string{"claude/agents/sdd-verify.md", "claude/commands/sdd-verify.md", "cursor/agents/sdd-verify.md"} {
		content := MustRead(path)
		if skill, save := strings.Index(content, "sdd-verify/SKILL.md"), strings.LastIndex(content, "mem_save"); skill < 0 || save < 0 || skill > save {
			t.Fatalf("%s must load the shared verify contract before persistence", path)
		}
	}
}

// TestOpenCodeReviewTransportPluginContract pins the adapter-minimality
// boundary: the plugin correlates one host Task with one Go process, while Go
// owns all prompt, schema, admission, and capture semantics.

// TestModelVariantsPluginContract verifies the embedded model-variants.ts
// plugin keeps the contract enforced by PR #440 review: atomic write via
// tmp+rename, always-write semantics (no early return on empty variants),
// and visible error logging instead of silent failure.

func TestClaudeEmbeddedAssetLayout(t *testing.T) {
	entries, err := FS.ReadDir("claude")
	if err != nil {
		t.Fatalf("ReadDir(claude) error = %v", err)
	}

	seen := map[string]bool{}
	for _, entry := range entries {
		seen[entry.Name()] = true
	}

	for _, name := range []string{"agents", "commands", "persona-gentleman.md", "sdd-orchestrator.md"} {
		if !seen[name] {
			t.Fatalf("claude embedded assets missing %q", name)
		}
	}
	// engram-protocol.md moved to the canonical engram/protocol.md asset
	// (design.md Decision 3) — it MUST NOT ship a stale duplicate under claude/.
	if seen["engram-protocol.md"] {
		t.Fatal("claude embedded assets must not ship a stale engram-protocol.md — content now lives in engram/protocol.md")
	}

	commandEntries, err := FS.ReadDir("claude/commands")
	if err != nil {
		t.Fatalf("ReadDir(claude/commands) error = %v", err)
	}
	if len(commandEntries) != 11 {
		t.Fatalf("claude commands count = %d, want 11", len(commandEntries))
	}

	agentEntries, err := FS.ReadDir("claude/agents")
	if err != nil {
		t.Fatalf("ReadDir(claude/agents) error = %v", err)
	}
	if len(agentEntries) != 19 {
		t.Fatalf("claude agents count = %d, want 19", len(agentEntries))
	}
}

func TestSDDResearchRuntimeAssetsDeclareExactEvidenceGrants(t *testing.T) {
	tests := []struct {
		path        string
		declaration string
		toolLine    string
		toolsExact  string
		evidence    []string
		forbidden   []string
		required    []string
	}{
		{
			path: "claude/agents/sdd-research.md", declaration: "Evidence grants: documentation=[WebFetch]; open-web=[WebSearch,WebFetch].",
			toolLine: "tools:", toolsExact: "tools: WebFetch, WebSearch", evidence: []string{"WebFetch", "WebSearch"},
			forbidden: []string{"Read", "Edit", "Write", "mcp__plugin_engram_engram__"},
			required:  []string{"already-persisted intent", "Do not read or mutate repository or Engram state", "bounded evidence envelope", "The orchestrator validates and persists this envelope"},
		},
		{path: "cursor/agents/sdd-research.md", declaration: "Evidence grants: documentation=[]; open-web=[]."},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			content := MustRead(tt.path)
			for _, required := range []string{
				tt.declaration,
				"Persistence tools are not evidence grants.",
				"Unsupported or undeclared classes deny admission and emit no claims.",
			} {
				if !strings.Contains(content, required) {
					t.Fatalf("%s missing %q", tt.path, required)
				}
			}

			tools := ""
			if tt.toolLine != "" {
				for _, line := range strings.Split(content, "\n") {
					if strings.HasPrefix(line, tt.toolLine) {
						tools = line
						break
					}
				}
				if tools == "" {
					t.Fatalf("%s missing scoped tools", tt.path)
				}
				if tt.toolsExact != "" && tools != tt.toolsExact {
					t.Fatalf("%s tools = %q, want %q", tt.path, tools, tt.toolsExact)
				}
				for _, forbidden := range tt.forbidden {
					if strings.Contains(tools, forbidden) {
						t.Fatalf("%s collection-only tools retain %q", tt.path, forbidden)
					}
				}
				for _, required := range tt.required {
					if !strings.Contains(content, required) {
						t.Fatalf("%s missing collection-only contract %q", tt.path, required)
					}
				}
			}

			for _, known := range []string{"WebFetch", "WebSearch", "@context7"} {
				want := false
				for _, grant := range tt.evidence {
					want = want || grant == known
				}
				if got := strings.Contains(tools, known); got != want {
					t.Fatalf("%s evidence tool %q present = %v, want %v in %q", tt.path, known, got, want, tools)
				}
			}
		})
	}
}

// TestEngramEmbeddedAssetLayout verifies the canonical protocol asset
// directory introduced by the consolidation (design.md Decision 3).
func TestEngramEmbeddedAssetLayout(t *testing.T) {
	entries, err := FS.ReadDir("engram")
	if err != nil {
		t.Fatalf("ReadDir(engram) error = %v", err)
	}

	seen := map[string]bool{}
	for _, entry := range entries {
		seen[entry.Name()] = true
	}

	if !seen["protocol.md"] {
		t.Fatal("engram embedded assets missing \"protocol.md\"")
	}
}

// sddOrchestratorAutomaticDefaultRuntimes lists every runtime whose asset
// carries the flipped "default to Automatic" execution-mode sentence: retained
// runtimes with a standalone `sdd-orchestrator.md` plus Claude's separate
// workflow surface. Deliberately not "all 12 runtime dirs" — Claude ships two
// files and only its workflow file carries this sentence.
var sddOrchestratorAutomaticDefaultRuntimes = []string{
	"antigravity/sdd-orchestrator.md",
	"codex/sdd-orchestrator.md",
	"generic/sdd-orchestrator.md",
	"cursor/sdd-orchestrator.md",
	"claude/sdd-orchestrator-workflow.md",
}

const sddOrchestratorAutomaticDefaultSentence = "If the user doesn't specify, default to **Automatic**."

const sddOrchestratorPromptBudgetSentence = "After scope approval, expect zero further prompts on the happy path and at most one actionable prompt per recoverable failure; the gatekeeper summarizes phase progress instead of interrupting except on a second consecutive gate failure or a genuine scope/product decision."

// TestSDDOrchestratorAssetsDefaultToAutomatic pins that every SDD
// orchestrator asset defaults to Automatic execution mode when unspecified,
// with a byte-identical default sentence and prompt-budget sentence across
// all 5 retained runtimes, and that Interactive stays explicitly selectable (never
// removed as an option).
func TestSDDOrchestratorAssetsDefaultToAutomatic(t *testing.T) {
	for _, path := range sddOrchestratorAutomaticDefaultRuntimes {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if !strings.Contains(content, sddOrchestratorAutomaticDefaultSentence) {
				t.Fatalf("%s missing byte-identical default sentence %q", path, sddOrchestratorAutomaticDefaultSentence)
			}
			if !strings.Contains(content, sddOrchestratorPromptBudgetSentence) {
				t.Fatalf("%s missing byte-identical prompt-budget sentence %q", path, sddOrchestratorPromptBudgetSentence)
			}
			if strings.Contains(content, "default to **Interactive**") {
				t.Fatalf("%s still defaults to Interactive", path)
			}
			if !strings.Contains(content, "**Interactive**") {
				t.Fatalf("%s must keep Interactive explicitly selectable", path)
			}
		})
	}
}

func TestDelegatedSDDProvidersForwardApplyVerifyContext(t *testing.T) {
	tests := []struct {
		name               string
		path               string
		delegatedContext   string
		dependencyReadRows []string
	}{
		{
			name:             "Codex prompt",
			path:             "codex/sdd-orchestrator.md",
			delegatedContext: "Codex phase prompt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content := MustRead(tc.path)
			section := markdownSection(content, "### Apply/Verify Context Forwarding (MANDATORY)")
			if section == "" {
				t.Fatalf("%s missing apply/verify context forwarding section", tc.path)
			}

			required := []string{
				"`sdd-apply`",
				"`sdd-verify`",
				`mem_search(query: "sdd-init/{project}", project: "{project}")`,
				"mem_get_observation",
				"full project init",
				"Search previews are not sufficient",
				"`strict_tdd: true|false`",
				`mem_search(query: "sdd/{change-name}/apply-progress", project: "{project}")`,
				"full prior apply-progress",
				"`previous_apply_progress:",
				"READ-MERGE-WRITE",
				"Do NOT overwrite",
				"full combined apply-progress",
				tc.delegatedContext,
			}
			for _, required := range required {
				if !strings.Contains(section, required) {
					t.Fatalf("%s missing delegated apply/verify context contract %q", tc.path, required)
				}
			}
			if !hasApplyVerifyContextFlow(section, tc.delegatedContext) {
				t.Fatalf("%s does not relate retrieval, forwarding, and persistence", tc.path)
			}

			glossaryTokens := append(append([]string{}, required...), tc.dependencyReadRows...)
			glossaryOnly := "### Apply/Verify Context Forwarding (MANDATORY)\n" + strings.Join(glossaryTokens, "\n")
			if hasApplyVerifyContextFlow(glossaryOnly, tc.delegatedContext) {
				t.Fatal("glossary-only token fixture must not satisfy the forwarding contract")
			}

			for _, row := range tc.dependencyReadRows {
				if !strings.Contains(content, row) {
					t.Fatalf("%s missing dependency forwarding row %q", tc.path, row)
				}
			}
		})
	}
}

func hasApplyVerifyContextFlow(section, delegatedContext string) bool {
	steps := []struct {
		prefix  string
		needles []string
	}{
		{"Before ", []string{"`sdd-apply`", "`sdd-verify`"}},
		{"1. ", []string{`mem_search(query: "sdd-init/{project}"`, "mem_get_observation", "full project init", "Search previews are not sufficient"}},
		{"2. ", []string{`mem_search(query: "sdd/{change-name}/apply-progress"`, "mem_get_observation", "full prior apply-progress", "before launch"}},
		{"3. ", []string{"Add both resolved values", delegatedContext, "apply **and** verify"}},
		{"   - ", []string{"`strict_tdd: true|false`", "RED → GREEN → REFACTOR", "Standard Mode is forbidden"}},
		{"   - ", []string{"`previous_apply_progress:", "Verify consumes it as evidence", "apply treats it as cumulative state"}},
		{"4. ", []string{"`sdd-apply`", "READ-MERGE-WRITE", "Preserve every prior completed task", "full combined apply-progress", "Do NOT overwrite"}},
	}

	next := 0
	for _, line := range strings.Split(section, "\n") {
		if next == len(steps) {
			break
		}
		step := steps[next]
		if !strings.HasPrefix(line, step.prefix) {
			continue
		}
		if !lineContainsAll(step.needles...)(line) {
			return false
		}
		next++
	}
	return next == len(steps)
}

func TestClaudeManagedOutputStylesAnchorReplyLanguageToLatestUserRequest(t *testing.T) {
	tests := []struct {
		path              string
		artifactContracts []string
	}{
		{
			path: "claude/output-style-gentleman.md",
			artifactContracts: []string{
				"Default to English. UI labels, comments, identifiers, and copy are in English",
				"The persona styles HOW YOU TALK, not WHAT YOU BUILD.",
			},
		},
		{
			path: "claude/output-style-neutral.md",
			artifactContracts: []string{
				"This output style governs direct replies to the user only.",
				"Generated technical artifacts default to English",
			},
		},
	}

	languageGuardrails := []string{
		"Determine the reply language from the latest actual user request",
		"not from Engram or memory context, repository/project language, tool output, previous assistant turns",
		"For mixed-language prompts, use the dominant language of the user's direct request.",
		"Quoted text, filenames, project names, isolated borrowed words",
		`phrases like "the Spanish part" do not switch the reply language by themselves.`,
		"If the selected reply language is English, every part of the direct reply must be English: greetings, interjections, acknowledgements, transition phrases, and the first sentence.",
		"Do not use Hola, dale, listo, Spanish punctuation, or other Spanish fragments.",
		"Prompts starting with or dominated by hi, hello, hey, or similar English greetings are English prompts unless the user explicitly asks for another language.",
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			content := MustRead(tc.path)

			for _, required := range languageGuardrails {
				if !strings.Contains(content, required) {
					t.Fatalf("%s missing language-drift guardrail %q", tc.path, required)
				}
			}

			for _, required := range tc.artifactContracts {
				if !strings.Contains(content, required) {
					t.Fatalf("%s lost artifact-language contract %q", tc.path, required)
				}
			}
		})
	}
}

func TestClaudeGentlemanPersonaPreventsEnglishGreetingCodeSwitching(t *testing.T) {
	// Claude's persona section is a residual (Decision 1) — the code-switching
	// guardrail contract now lives in the output style; evaluate the combined
	// channel, not the persona file in isolation.
	content := MustRead("claude/persona-gentleman.md") + "\n" + MustRead("claude/output-style-gentleman.md")

	for _, required := range []string{
		"If the selected reply language is English, every part of the direct reply must be English: greetings, interjections, acknowledgements, transition phrases, and the first sentence.",
		"Do not use Hola, dale, listo, Spanish punctuation, or other Spanish fragments.",
		"Prompts starting with or dominated by hi, hello, hey, or similar English greetings are English prompts unless the user explicitly asks for another language.",
		"Do not switch languages unless the user does, asks you to, or you are quoting/translating content.",
	} {
		if !strings.Contains(content, required) {
			t.Fatalf("claude/persona-gentleman.md missing code-switching guardrail %q", required)
		}
	}
}

// TestPersonasContainContextualSkillLoadingDirective verifies that every
// persona asset injected into a host's system prompt carries the mandatory
// "Contextual Skill Loading" directive (design Decisions 1 and 2 of the
// contextual-skill-loading change). The hardcoded "Skills (Auto-load based
// on context)" table MUST be removed at the same time.
//
// Claude variant references the native `Skill` tool by name. Non-Claude
// variants instruct the model to read the matching SKILL.md using their
// agent's read mechanism, since they have no Skill tool.

// TestMustReadPanicsOnMissingFile verifies that MustRead panics for a
// nonexistent file, confirming the safety mechanism works.
func TestMustReadPanicsOnMissingFile(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("MustRead() did not panic for missing file")
		}
	}()

	MustRead("nonexistent/file.md")
}

// TestEmbeddedAssetCount verifies we have the expected number of embedded files.
// This catches accidental deletions of asset files.

func TestSDDPhaseCommonEnforcesExecutorBoundary(t *testing.T) {
	content := MustRead("skills/_shared/sdd-phase-common.md")

	// Must enforce executor boundary — no delegation allowed.
	for _, want := range []string{
		"EXECUTOR, not an orchestrator",
		"Do NOT launch sub-agents",
		"do NOT call `delegate`/`task`",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("sdd-phase-common missing executor boundary rule %q", want)
		}
	}

	// Must instruct phase agents to search the skill registry themselves
	// when no explicit skill path was provided — this is skill LOADING, not delegation.
	if !strings.Contains(content, `mem_search(query: "skill-registry"`) {
		t.Fatal("sdd-phase-common must instruct phase agents to search skill-registry themselves for skill loading")
	}

	// Must NOT tell agents to launch sub-agents or delegate tasks.
	for _, forbidden := range []string{
		"launch a sub-agent",
		"delegate this to",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("sdd-phase-common should not contain delegation instruction %q", forbidden)
		}
	}
}

func TestSDDStatusContractPreservesFrozenExternalV2Projection(t *testing.T) {
	content := MustRead("skills/_shared/sdd-status-contract.md")

	for _, want := range []string{
		"exact frozen external `StatusV2Projection`",
		"schemaName: gentle-ai.sdd-status",
		"schemaVersion: 2",
		"gentle-ai.sdd-status/v2",
		"changeName: <change-name-or-null>",
		// #3636: hybrid reaches the public v2 document; kept in lockstep with statusV2ArtifactStore.
		"artifactStore: openspec | engram | hybrid | none",
		"planningHome:",
		"mode: repo-local",
		"path: <absolute path to openspec>",
		"changeRoot: <absolute path to openspec/changes/<change> or null>",
		"artifactPaths:",
		"contextFiles:",
		"artifacts:",
		"proposal: [<absolute path>]",
		"verifyReport: [<absolute path>]",
		"proposal: [<absolute readable files>]",
		"verifyReport: [<absolute readable files>]",
		"proposal: missing | done | partial",
		"verifyReport: missing | done | partial",
		"taskProgress:",
		"total: 0",
		"completed: 0",
		"pending: 0",
		"allComplete: false",
		"dependencies:",
		"proposal: blocked | ready | all_done",
		"specs: blocked | ready | all_done",
		"design: blocked | ready | all_done",
		"tasks: blocked | ready | all_done",
		"apply: blocked | ready | all_done",
		"verify: blocked | ready | all_done",
		"archive: blocked | ready | all_done",
		"applyState: blocked | all_done | ready",
		"actionContext:",
		"relationships:",
		"dependsOn: []",
		"supersedes: []",
		"amends: []",
		"conflictsWith: []",
		"sameDomainActiveChanges: []",
		"remediationState:",
		"failedEvidenceRevision:",
		"reviewOffer:",
		"available: true",
		"invocation: <fresh review start command>",
		"phaseInstructions:",
		"apply: [<instruction strings>]",
		"verify: [<instruction strings>]",
		"remediate: [<instruction strings>]",
		"archive: [<instruction strings>]",
		"nextRecommended: propose | spec | design | tasks | apply | verify | remediate | archive | sdd-new | select-change | resolve-blockers",
		"blockedReasons: []",
		"Manual fallback status MUST stay shape-compatible with native `gentle-ai.sdd-status` JSON",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("sdd-status-contract missing frozen SDD v2 field or token %q", want)
		}
	}

	for _, forbidden := range []string{
		"runtimeStatus",
		"correctionBudget",
		"sdd-status/v1",
		"reviewGate",
		"reviewTransaction",
		"reVerify",
		"reviewPolicy",
		"reviewLedger",
		"reviewReceipt",
		"reviewBundle",
		"reviewContext",
		"reviewState",
		"lineageId:",
		"generation: 0",
		"fixBatch: 0",
		"routeDecision:",
		"implementationRoute:",
		"sddRunRef:",
		"publicState:",
		"verification:",
		"deliveryIntentRef:",
		"authorizedTransition:",
		"gentle-ai.work-status/v1",
		"gentle-ai.work-transition/v1",
		"schemaName: spec-driven",
		"root: <project-or-openspec-root>",
		"changesDir: <openspec/changes or engram topic prefix>",
		"complete: 0",
		"remaining: 0",
		"unchecked: []",
		"warnings: []",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("sdd-status-contract contains internal, work-routing, or retired field %q", forbidden)
		}
	}
}

// TestCommandsDoNotUseEchoNPwd guards against the nested-subshell pattern
// `echo -n "$(pwd)"` (and the basename variant) that causes Claude Code v2.1.113+
// to reject slash commands with "Unhandled node type: string". Use the plain pwd
// or basename command forms instead — both are accepted by old and new parsers.

// TestOpenCodeCommandsDetectWorkspaceAgentSide guards against parse-time shell
// interpolation for the working directory in OpenCode command files. In
// OpenCode Desktop (Electron), patterns like !pwd and !basename $(pwd) evaluate
// against the Electron app data directory rather than the project workspace
// (issue #74). Command files must instruct the agent to detect the workspace
// via its bash tool (e.g. git rev-parse --show-toplevel) and treat that
// returned path as authoritative.

// TestClaudeCommandsDetectWorkspaceAgentSide guards against parse-time shell
// interpolation for workspace/project context in Claude slash commands. Claude
// Code performs static permission validation before running commands, so forms
// like !`basename "$(pwd)"` can be rejected before the agent starts. Command
// files must instruct the agent to detect the workspace from inside the session.
func TestClaudeCommandsDetectWorkspaceAgentSide(t *testing.T) {
	forbiddenPatterns := []string{
		"!pwd",
		"!`pwd`",
		"!basename $(pwd)",
		"!basename \"$(pwd)\"",
		"!basename '$(pwd)'",
		"!`basename $(pwd)`",
		"!`basename \"$(pwd)\"`",
		"!`basename '$(pwd)'`",
		"!git rev-parse --show-toplevel",
		"!`git rev-parse --show-toplevel`",
	}
	const requiredHint = "git rev-parse --show-toplevel"

	entries, err := FS.ReadDir("claude/commands")
	if err != nil {
		t.Fatalf("ReadDir(claude/commands) error = %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := "claude/commands/" + entry.Name()
		content := MustRead(path)
		for _, pat := range forbiddenPatterns {
			if strings.Contains(content, pat) {
				t.Errorf("%s contains banned Claude parse-time shell interpolation %q — detect workspace/project context agent-side instead (see #837)", path, pat)
			}
		}
		for _, line := range strings.Split(content, "\n") {
			if (strings.Contains(line, "Working directory:") || strings.Contains(line, "Current project:")) && strings.Contains(line, "!") {
				t.Errorf("%s contains parse-time shell interpolation in workspace/project context line %q — detect it agent-side instead (see #837)", path, line)
			}
		}
		if strings.Contains(content, "Working directory:") && !strings.Contains(content, requiredHint) {
			t.Errorf("%s mentions \"Working directory:\" without the agent-side detection hint %q (see #837)", path, requiredHint)
		}
	}
}

// TestOrchestratorsRequireAutomaticGatekeeper asserts that every orchestrator
// template validates every phase boundary and keeps design/apply validation
// artifact-bound rather than silently opening an adversarial code review.

func TestSDDOrchestratorsProjectNativeCheckingWithoutPromptOwnedLenses(t *testing.T) {
	for _, path := range allSDDOrchestratorAssetPaths(t) {
		content := MustRead(path)
		section := markdownSection(content, "#### Native Checking Contract")
		if section == "" {
			t.Fatalf("%s missing Native Checking Contract", path)
		}
		for _, required := range []string{
			"Native RAR owns verification applicability",
			"bounded zero/one/four-lens plan",
			"never select lenses or author PASS",
			"passive ordinary document or image",
			"structural readback",
			"trivial passive documentation-only edit",
			"structural readback is the complete proportional check",
			"do not open a separate semantic-verification or heavy review ceremony",
			"applicable verifier is unavailable",
			"preserve the typed unavailable result",
			"never invent PASS, retry indefinitely, or escalate into extra ceremony",
			"quick check runs once",
			"Long or very-long work gets one cost/side-effect forecast",
			"Needs your decision",
			"Functional proof and adversarial review both project as **Checking**",
			"at most one scoped correction",
			"never reopen review for unchanged content",
		} {
			if !strings.Contains(section, required) {
				t.Fatalf("%s native checking contract missing %q", path, required)
			}
		}
		for _, retired := range []string{
			"Review Lens Selection", "review-risk", "review-readability",
			"review-reliability", "review-resilience", "loop-until-dry",
		} {
			if strings.Contains(content, retired) {
				t.Fatalf("%s retained prompt-owned review mechanism %q", path, retired)
			}
		}
	}
}

func markdownLineContaining(content, needle string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func lineContainsAll(needles ...string) func(string) bool {
	return func(line string) bool {
		for _, needle := range needles {
			if !strings.Contains(line, needle) {
				return false
			}
		}
		return true
	}
}

func lineContainsAny(needles ...string) func(string) bool {
	return func(line string) bool {
		for _, needle := range needles {
			if strings.Contains(line, needle) {
				return true
			}
		}
		return false
	}
}

func markdownSection(content, heading string) string {
	start := strings.Index(content, heading)
	if start == -1 {
		return ""
	}
	section := content[start:]
	end := len(section)
	for _, levelHeading := range []string{"\n#### ", "\n### ", "\n## "} {
		if next := strings.Index(section[len(heading):], levelHeading); next != -1 {
			end = min(end, len(heading)+next)
		}
	}
	return section[:end]
}

// TestSDDArchiveFinalStateAuthorityContract pins the instruction-layer fix for
// the community report that sdd-archive summarized intermediate artifacts
// (verify-report, apply-progress) instead of the final state of the work. The
// text must carry an explicit authority hierarchy, the intermediate-vs-final
// snapshot rule, and the contradiction-recording rule. This pins the words
// only — whether the model obeys them can be verified solely by community
// runtime behavior.

func TestSDDArchiveMoveTransactionPreservesFilesystemOnCollisions(t *testing.T) {
	shell := requireArchiveShell(t)
	t.Setenv("BASH_ENV", "repository-sentinel.txt")
	for _, tracked := range []bool{true, false} {
		sourceMode := "untracked"
		if tracked {
			sourceMode = "tracked"
		}
		t.Run(sourceMode+" source moves to absent destination", func(t *testing.T) {
			root, source, destination, sentinel := setupArchiveFixture(t, tracked)
			output, err := runArchiveMoveTransaction(shell, root)
			if err != nil {
				t.Fatalf("archive transaction failed: %v\n%s", err, output)
			}
			if _, err := os.Lstat(source); !os.IsNotExist(err) {
				t.Fatalf("%s remains after archive move: %v", source, err)
			}
			assertFileContents(t, filepath.Join(destination, "tasks.md"), "archive task bytes\n")
			assertFileContents(t, sentinel, "exit 99\nrepository sentinel\n")
			if tracked {
				assertGitCommandFails(t, root, "ls-files", "--error-unmatch", "openspec/changes/change/tasks.md")
				runGit(t, root, "ls-files", "--error-unmatch", "openspec/changes/archive/2030-01-02-change/tasks.md")
				staged := runGit(t, root, "diff", "--cached", "--name-status")
				if !strings.Contains(staged, "R100") {
					t.Fatalf("tracked archive move did not stage a rename:\n%s", staged)
				}
			} else {
				status := runGit(t, root, "status", "--porcelain", "--untracked-files=all")
				if strings.Contains(status, "openspec/changes/change/") || !strings.Contains(status, "openspec/changes/archive/") {
					t.Fatalf("untracked archive move has unexpected Git state:\n%s", status)
				}
			}
		})
		for _, collision := range []string{"directory", "regular file", "live symlink", "dangling symlink"} {
			t.Run(sourceMode+" source preserves "+collision+" collision", func(t *testing.T) {
				root, source, destination, sentinel := setupArchiveFixture(t, tracked)
				createArchiveCollision(t, root, destination, collision)
				beforeStatus := runGit(t, root, "status", "--porcelain")
				output, err := runArchiveMoveTransaction(shell, root)
				if err == nil {
					t.Fatalf("archive transaction unexpectedly succeeded for %s collision:\n%s", collision, output)
				}
				for _, required := range []string{
					"source openspec/changes/change and destination openspec/changes/archive/2030-01-02-change remain unchanged",
					"Resolve the destination collision, then rerun this archive step.",
				} {
					if !strings.Contains(output, required) {
						t.Fatalf("collision failure missing %q:\n%s", required, output)
					}
				}
				assertFileContents(t, filepath.Join(source, "tasks.md"), "archive task bytes\n")
				assertArchiveCollision(t, root, destination, collision)
				assertFileContents(t, sentinel, "exit 99\nrepository sentinel\n")
				if afterStatus := runGit(t, root, "status", "--porcelain"); afterStatus != beforeStatus {
					t.Fatalf("collision changed Git state:\nbefore:\n%safter:\n%s", beforeStatus, afterStatus)
				}
			})
		}
	}
}
func TestSDDArchiveHistoricalRecoveryRefusesDanglingActiveSourceSymlink(t *testing.T) {
	shell := requireArchiveShell(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "repository-sentinel.txt"), []byte("exit 99\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BASH_ENV", "repository-sentinel.txt")
	activeSource := filepath.Join(root, "openspec", "changes", "change")
	nestedSource := filepath.Join(root, "openspec", "changes", "archive", "2030-01-02-change", "change")
	if err := os.MkdirAll(nestedSource, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedSource, "tasks.md"), []byte("historical task bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing-active-source"), activeSource); err != nil {
		t.Skipf("dangling symlink fixture is unavailable: %v", err)
	}
	recovery := strings.ReplaceAll(archiveFencedShellBlock("### Historical Malformed Nesting Recovery (Manual Only)"), "{change-name}", "change")
	recovery = strings.ReplaceAll(recovery, "YYYY-MM-DD-change", "2030-01-02-change")
	command := exec.Command(shell, "-c", recovery)
	command.Dir = root
	command.Env = withoutBashEnv(isolatedGitEnvironment())
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "active source must be absent") {
		t.Fatalf("historical recovery did not fail closed for a dangling active-source symlink: %v\n%s", err, output)
	}
	if info, err := os.Lstat(activeSource); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("dangling active-source symlink was not preserved: %v, %v", info, err)
	}
	assertFileContents(t, filepath.Join(nestedSource, "tasks.md"), "historical task bytes\n")
}

func requireArchiveShell(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("archive shell integration is skipped in short mode")
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("archive shell integration requires git: %v", err)
	}
	// Prefer Git's POSIX shell over a possible WSL launcher and verify candidates.
	candidates := []string{filepath.Join(filepath.Dir(gitPath), "..", "bin", "bash.exe")}
	if bashPath, err := exec.LookPath("bash"); err == nil {
		candidates = append(candidates, bashPath)
	}
	for _, shell := range candidates {
		if _, err := os.Stat(shell); err != nil {
			continue
		}
		if err := exec.Command(shell, "-c", "exit 0").Run(); err == nil {
			return shell
		}
	}
	t.Skip("archive shell integration requires a usable POSIX shell")
	return ""
}
func setupArchiveFixture(t *testing.T, tracked bool) (root, source, destination, sentinel string) {
	t.Helper()
	root = t.TempDir()
	sentinel = filepath.Join(root, "repository-sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("exit 99\nrepository sentinel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source = filepath.Join(root, "openspec", "changes", "change")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "tasks.md"), []byte("archive task bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	destination = filepath.Join(root, "openspec", "changes", "archive", "2030-01-02-change")

	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.name", "Archive Test")
	runGit(t, root, "config", "user.email", "archive-test@example.invalid")
	if tracked {
		runGit(t, root, "add", "--", "repository-sentinel.txt", "openspec/changes/change/tasks.md")
	} else {
		runGit(t, root, "add", "--", "repository-sentinel.txt")
	}
	runGit(t, root, "commit", "-qm", "archive fixture")
	return root, source, destination, sentinel
}

func runArchiveMoveTransaction(shell, root string) (string, error) {
	const changeName = "change"
	transaction := archiveFencedShellBlock("### Step 3: Move to Archive")
	transaction = strings.ReplaceAll(transaction, "{change-name}", changeName)
	transaction = strings.ReplaceAll(transaction, "YYYY-MM-DD-"+changeName, "2030-01-02-"+changeName)
	command := exec.Command(shell, "-c", transaction)
	command.Dir = root
	command.Env = withoutBashEnv(isolatedGitEnvironment())
	output, err := command.CombinedOutput()
	return string(output), err
}

func archiveFencedShellBlock(heading string) string {
	skill := MustRead("skills/sdd-archive/SKILL.md")
	start := strings.Index(skill, heading)
	if start < 0 {
		panic("sdd-archive shell section is missing")
	}
	opening := strings.Index(skill[start:], "```bash\n")
	if opening < 0 {
		panic("sdd-archive shell section is missing its opening fence")
	}
	start += opening + len("```bash\n")
	end := strings.Index(skill[start:], "\n```")
	if end < 0 {
		panic("sdd-archive shell block is missing its closing fence")
	}
	return skill[start : start+end]
}

func createArchiveCollision(t *testing.T, root, destination, collision string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		t.Fatal(err)
	}
	switch collision {
	case "directory":
		if err := os.MkdirAll(destination, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(destination, "collision-sentinel"), []byte("directory sentinel\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	case "regular file":
		if err := os.WriteFile(destination, []byte("file sentinel\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	case "live symlink":
		target := filepath.Join(root, "live-symlink-target")
		if err := os.WriteFile(target, []byte("live symlink sentinel\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, destination); err != nil {
			t.Skipf("live symlink fixture is unavailable: %v", err)
		}
	case "dangling symlink":
		if err := os.Symlink(filepath.Join(root, "missing-symlink-target"), destination); err != nil {
			t.Skipf("dangling symlink fixture is unavailable: %v", err)
		}
	default:
		t.Fatalf("unknown collision type %q", collision)
	}
}

func assertArchiveCollision(t *testing.T, root, destination, collision string) {
	t.Helper()
	info, err := os.Lstat(destination)
	if err != nil {
		t.Fatalf("collision destination is missing: %v", err)
	}
	switch collision {
	case "directory":
		if !info.IsDir() {
			t.Fatalf("collision destination is %v, want directory", info.Mode())
		}
		assertFileContents(t, filepath.Join(destination, "collision-sentinel"), "directory sentinel\n")
	case "regular file":
		if !info.Mode().IsRegular() {
			t.Fatalf("collision destination is %v, want regular file", info.Mode())
		}
		assertFileContents(t, destination, "file sentinel\n")
	case "live symlink":
		if target, err := os.Readlink(destination); info.Mode()&os.ModeSymlink == 0 || err != nil || target != filepath.Join(root, "live-symlink-target") {
			t.Fatalf("collision destination is %v, want live symlink to %q: target %q, error %v", info.Mode(), filepath.Join(root, "live-symlink-target"), target, err)
		}
		assertFileContents(t, filepath.Join(root, "live-symlink-target"), "live symlink sentinel\n")
	case "dangling symlink":
		if target, err := os.Readlink(destination); info.Mode()&os.ModeSymlink == 0 || err != nil || target != filepath.Join(root, "missing-symlink-target") {
			t.Fatalf("collision destination is %v, want dangling symlink to %q: target %q, error %v", info.Mode(), filepath.Join(root, "missing-symlink-target"), target, err)
		}
		if _, err := os.Stat(destination); !os.IsNotExist(err) {
			t.Fatalf("dangling symlink target is unexpectedly available: %v", err)
		}
	}
}

func assertFileContents(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}

func withoutBashEnv(env []string) []string {
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.ToUpper(env[i]), "BASH_ENV=") {
			env = append(env[:i], env[i+1:]...)
		}
	}
	return env
}

func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	command.Env = isolatedGitEnvironment()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func assertGitCommandFails(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	command.Env = isolatedGitEnvironment()
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("git %s unexpectedly succeeded:\n%s", strings.Join(args, " "), output)
	}
}

func isolatedGitEnvironment() []string {
	env := os.Environ()
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.ToUpper(env[i]), "GIT_") {
			env = append(env[:i], env[i+1:]...)
		}
	}
	return append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_CONFIG_COUNT=0")
}
