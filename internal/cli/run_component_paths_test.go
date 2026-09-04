package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/agents"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/agentguidance"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/filemerge"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/planner"
	"github.com/gentleman-programming/gentle-ai/v2/internal/system"
)

func TestComponentPathsSDDIncludesClaudeLazyWorkflow(t *testing.T) {
	home := t.TempDir()
	adapters := resolveAdapters([]model.AgentID{model.AgentClaudeCode})

	paths := componentPaths(home, model.Selection{}, adapters, model.ComponentSDD)

	workflow := filepath.Join(home, ".claude", "skills", "_shared", "sdd-orchestrator-workflow.md")
	if !containsPath(paths, workflow) {
		t.Fatalf("componentPaths(sdd) missing Claude lazy workflow path %q\npaths=%v", workflow, paths)
	}
}

func TestComponentPersonaPiUsesResolvedScopePath(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	adapters := resolveAdapters([]model.AgentID{model.AgentPi})
	selection := model.Selection{Persona: model.PersonaNeutral}

	global := componentPathsWithWorkspaceScoped(home, workspace, ScopeGlobal, selection, adapters, model.ComponentPersona)
	if !containsPath(global, filepath.Join(home, ".pi", "gentle-ai", "persona.json")) {
		t.Fatalf("global Pi persona paths = %v, missing home-scoped config", global)
	}
	if !containsPath(global, filepath.Join(workspace, ".pi", "gentle-ai", "persona.json")) {
		t.Fatalf("global Pi persona paths = %v, missing active workspace config", global)
	}

	workspacePaths := componentPathsWithWorkspaceScoped(home, workspace, ScopeWorkspace, selection, adapters, model.ComponentPersona)
	if !containsPath(workspacePaths, filepath.Join(workspace, ".pi", "gentle-ai", "persona.json")) {
		t.Fatalf("workspace Pi persona paths = %v, missing workspace-scoped config", workspacePaths)
	}
	if containsPath(workspacePaths, filepath.Join(home, ".pi", "gentle-ai", "persona.json")) {
		t.Fatalf("workspace Pi persona paths = %v, unexpectedly contains home config", workspacePaths)
	}

	custom := componentPathsWithWorkspaceScoped(home, workspace, ScopeWorkspace, model.Selection{Persona: model.PersonaCustom}, adapters, model.ComponentPersona)
	if len(custom) != 0 {
		t.Fatalf("custom Pi persona paths = %v, want none", custom)
	}
}

func TestInstallPiPersonaWritesManagedScopePaths(t *testing.T) {
	for _, tt := range []struct {
		name  string
		scope InstallScope
	}{
		{name: "global", scope: ScopeGlobal},
		{name: "workspace", scope: ScopeWorkspace},
	} {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			workspace := t.TempDir()
			root, other := home, workspace
			if tt.scope == ScopeWorkspace {
				root, other = workspace, home
			}

			step := componentApplyStep{
				component:    model.ComponentPersona,
				homeDir:      home,
				workspaceDir: workspace,
				scope:        tt.scope,
				agents:       []model.AgentID{model.AgentPi},
				selection:    model.Selection{Persona: model.PersonaNeutral},
			}
			if err := step.Run(); err != nil {
				t.Fatalf("componentApplyStep.Run() error = %v", err)
			}

			want := filepath.Join(root, ".pi", "gentle-ai", "persona.json")
			if _, err := os.Stat(want); err != nil {
				t.Fatalf("Pi persona config %q was not written: %v", want, err)
			}
			if tt.scope == ScopeGlobal {
				workspacePath := filepath.Join(workspace, ".pi", "gentle-ai", "persona.json")
				if _, err := os.Stat(workspacePath); err != nil {
					t.Fatalf("global Pi persona config %q was not seeded: %v", workspacePath, err)
				}
				return
			}
			unwanted := filepath.Join(other, ".pi", "gentle-ai", "persona.json")
			if _, err := os.Stat(unwanted); !os.IsNotExist(err) {
				t.Fatalf("workspace-scoped Pi persona config %q was written outside scope; stat err = %v", unwanted, err)
			}
		})
	}
}

func TestComponentPathsSDDIncludesAntigravitySkillsAndSharedConventions(t *testing.T) {
	home := t.TempDir()
	adapters := resolveAdapters([]model.AgentID{model.AgentAntigravity})

	paths := componentPaths(home, model.Selection{}, adapters, model.ComponentSDD)

	// Verify all four shared convention files are reported.
	for _, sharedFile := range []string{
		"persistence-contract.md",
		"engram-convention.md",
		"openspec-convention.md",
		"sdd-phase-common.md",
		"skill-resolver.md",
	} {
		shared := filepath.Join(adapters[0].SkillsDir(home), "_shared", sharedFile)
		if !containsPath(paths, shared) {
			t.Fatalf("componentPaths(sdd) missing shared convention path %q\npaths=%v", shared, paths)
		}
	}

	skill := filepath.Join(adapters[0].SkillsDir(home), "sdd-verify", "SKILL.md")
	if !containsPath(paths, skill) {
		t.Fatalf("componentPaths(sdd) missing SDD skill path %q\npaths=%v", skill, paths)
	}
}

func TestComponentPathsWorkspaceScopedSkillsUsesWorkspaceDir(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	adapters := resolveAdapters([]model.AgentID{model.AgentClaudeCode})
	selection := model.Selection{
		Skills: []model.SkillID{
			model.SkillGoTesting,
			model.SkillBranchPR,
		},
	}

	paths := componentPathsWithWorkspaceScoped(home, workspace, ScopeWorkspace, selection, adapters, model.ComponentSkills)

	for _, want := range []string{
		filepath.Join(workspace, ".claude", "skills", "go-testing", "SKILL.md"),
		filepath.Join(workspace, ".claude", "skills", "branch-pr", "SKILL.md"),
	} {
		if !containsPath(paths, want) {
			t.Fatalf("componentPathsWithWorkspaceScoped(skills,claude-code,workspace) missing workspace-scoped path %q\npaths=%v", want, paths)
		}
	}

	for _, unwanted := range []string{
		filepath.Join(home, ".claude", "skills", "go-testing", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "branch-pr", "SKILL.md"),
	} {
		if containsPath(paths, unwanted) {
			t.Fatalf("componentPathsWithWorkspaceScoped(skills,claude-code,workspace) must not include home-scoped path %q\npaths=%v", unwanted, paths)
		}
	}
}

// TestInstallWorkspaceScopeVerificationWithNoGlobalSkills verifies that
// post-apply verification succeeds when --scope=workspace is used and no
// global skill files exist. This is a regression test for issue #785:
// the verifier used to check home-scoped paths even when workspace scope
// was active, causing false failures when only workspace skills existed.
func TestInstallWorkspaceScopeVerificationWithNoGlobalSkills(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	adapters := resolveAdapters([]model.AgentID{model.AgentClaudeCode})
	selection := model.Selection{
		Skills: []model.SkillID{
			model.SkillGoTesting,
			model.SkillBranchPR,
		},
	}

	// Simulate workspace-scoped install: skills are written to workspace only.
	// The verification should check workspace paths, not home paths.
	paths := componentPathsWithWorkspaceScoped(home, workspace, ScopeWorkspace, selection, adapters, model.ComponentSkills)

	// Verify that workspace paths are included (these should exist after install).
	for _, want := range []string{
		filepath.Join(workspace, ".claude", "skills", "go-testing", "SKILL.md"),
		filepath.Join(workspace, ".claude", "skills", "branch-pr", "SKILL.md"),
	} {
		if !containsPath(paths, want) {
			t.Fatalf("workspace-scoped verification missing workspace path %q\npaths=%v", want, paths)
		}
	}

	// Verify that home paths are NOT included (these would cause false failures
	// if checked when only workspace skills exist).
	for _, unwanted := range []string{
		filepath.Join(home, ".claude", "skills", "go-testing", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "branch-pr", "SKILL.md"),
	} {
		if containsPath(paths, unwanted) {
			t.Fatalf("workspace-scoped verification must not check home path %q when scope=workspace\npaths=%v", unwanted, paths)
		}
	}
}

// TestComponentPathsContext7ClaudeUsesUserRegistry pins Claude Context7 to
// the file injection actually writes: ~/.claude.json (issue #1868).
// settings.json is only mutated best-effort and may not exist, and the legacy
// managed ~/.claude/mcp/context7.json is removed by injection, so verifying
// either would fail on a healthy install.
func TestComponentPathsContext7ClaudeUsesUserRegistry(t *testing.T) {
	home := t.TempDir()
	adapters := resolveAdapters([]model.AgentID{model.AgentClaudeCode})

	paths := componentPaths(home, model.Selection{}, adapters, model.ComponentContext7)

	registry := filepath.Join(home, ".claude.json")
	if !containsPath(paths, registry) {
		t.Fatalf("componentPaths(context7,claude) missing %q\npaths=%v", registry, paths)
	}
	for _, absent := range []string{
		filepath.Join(home, ".claude", "mcp", "context7.json"),
		filepath.Join(home, ".claude", "settings.json"),
	} {
		if containsPath(paths, absent) {
			t.Fatalf("componentPaths(context7,claude) must not require %q\npaths=%v", absent, paths)
		}
	}
}

func TestComponentPathsContext7ClaudeRespectsWorkspaceScope(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	adapters := resolveAdapters([]model.AgentID{model.AgentClaudeCode})

	paths := componentPathsWithWorkspaceScoped(home, workspace, ScopeWorkspace, model.Selection{}, adapters, model.ComponentContext7)

	// Workspace scope writes <project-root>/.mcp.json, the file Claude Code
	// loads project-scoped MCP servers from (issue #2213). The legacy
	// .claude/settings.json key is inert for MCP discovery and is not declared.
	want := filepath.Join(workspace, ".mcp.json")
	if !containsPath(paths, want) {
		t.Fatalf("componentPathsWithWorkspaceScoped(context7,claude) with ScopeWorkspace missing %q\npaths=%v", want, paths)
	}
	for _, absent := range []string{
		filepath.Join(workspace, ".claude", "settings.json"),
		filepath.Join(home, ".claude.json"),
	} {
		if containsPath(paths, absent) {
			t.Fatalf("componentPathsWithWorkspaceScoped(context7,claude) with ScopeWorkspace must not require %q\npaths=%v", absent, paths)
		}
	}
}

func TestComponentPathsEngramClaudeUsesUserRegistryAndPreservesWorkspaceScope(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	adapters := resolveAdapters([]model.AgentID{model.AgentClaudeCode})

	global := componentPaths(home, model.Selection{}, adapters, model.ComponentEngram)
	registry := filepath.Join(home, ".claude.json")
	legacy := filepath.Join(home, ".claude", "mcp", "engram.json")
	if !containsPath(global, registry) || containsPath(global, legacy) {
		t.Fatalf("global Engram paths must use only Claude's user registry; paths=%v", global)
	}

	workspacePaths := componentPathsWithWorkspaceScoped(home, workspace, ScopeWorkspace, model.Selection{}, adapters, model.ComponentEngram)
	workspaceLegacy := filepath.Join(workspace, ".claude", "mcp", "engram.json")
	if !containsPath(workspacePaths, workspaceLegacy) || containsPath(workspacePaths, filepath.Join(workspace, ".claude.json")) {
		t.Fatalf("workspace Engram paths must remain unchanged; paths=%v", workspacePaths)
	}
}

// TestComponentPathsEngramCodexIncludesConfigTOML verifies that componentPaths
// for ComponentEngram + Codex reports ~/.codex/config.toml as a backup target.
func TestComponentPathsEngramCodexIncludesConfigTOML(t *testing.T) {
	home := t.TempDir()
	adapters := resolveAdapters([]model.AgentID{model.AgentCodex})

	paths := componentPaths(home, model.Selection{}, adapters, model.ComponentEngram)

	want := filepath.Join(home, ".codex", "config.toml")
	if !containsPath(paths, want) {
		t.Fatalf("componentPaths(engram,codex) missing %q\npaths=%v", want, paths)
	}
}

func TestComponentPathsSDDCodexIncludesHooksJSONOnlyForCodex(t *testing.T) {
	home := t.TempDir()
	adapters := resolveAdapters([]model.AgentID{model.AgentCodex, model.AgentClaudeCode})

	paths := componentPaths(home, model.Selection{}, adapters, model.ComponentSDD)
	codexHooks := filepath.Join(home, ".codex", "hooks.json")
	if !containsPath(paths, codexHooks) {
		t.Fatalf("componentPaths(sdd,codex) missing skill-registry hook %q\npaths=%v", codexHooks, paths)
	}
	claudeHooks := filepath.Join(home, ".claude", "hooks.json")
	if containsPath(paths, claudeHooks) {
		t.Fatalf("componentPaths(sdd,claude) declared unsupported hooks path %q\npaths=%v", claudeHooks, paths)
	}
}

// TestComponentPathsPermissionsCodexContributesNoPaths pins that the
// Permission component claims nothing under ~/.codex. gentle-ai does not write
// Codex's permissions config — not a profile, and not the legacy cleanup that
// used to strip one — so there is no injection target to verify and nothing to
// snapshot for rollback. A path reappearing here would mean something started
// writing that file again (#1794).
func TestComponentPathsPermissionsCodexContributesNoPaths(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(configPath, []byte("model = \"gpt-5.5\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	adapters := resolveAdapters([]model.AgentID{model.AgentCodex})

	paths := componentPaths(home, model.Selection{}, adapters, model.ComponentPermission)

	if len(paths) != 0 {
		t.Fatalf("componentPaths(permissions,codex) = %v, want none", paths)
	}
}

func TestComponentPathsPermissionsSkipsAgentsWithoutInjectionTarget(t *testing.T) {
	home := t.TempDir()
	adapters := resolveAdapters([]model.AgentID{
		model.AgentCursor,
		model.AgentAntigravity,
		model.AgentPi,
	})

	paths := componentPaths(home, model.Selection{}, adapters, model.ComponentPermission)

	for _, adapter := range adapters {
		unwanted := adapter.SettingsPath(home)
		if unwanted == "" {
			continue
		}
		if containsPath(paths, unwanted) {
			t.Fatalf("componentPaths(permissions) must not include unsupported injection path %q\npaths=%v", unwanted, paths)
		}
	}
}

func containsPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

// ─── Organic routing guidance is installed for every configured agent ──────
//
// Routing guidance decides how an agent picks between direct, delegated, and
// proposed work. It is therefore unconditional: an install that did not select
// the optional SDD component must still receive it (issue #1794).

const (
	routingOpenMarker  = "<!-- gentle-ai:" + agentguidance.RoutingSectionID + " -->"
	routingCloseMarker = "<!-- /gentle-ai:" + agentguidance.RoutingSectionID + " -->"

	legacyTriggerRulesOpenMarker = "<!-- gentle-ai:trigger-rules -->"
)

// newTestInstallRuntime builds an install runtime whose resolved plan mirrors
// the selection, which is what the real planner produces for these inputs.
func newTestInstallRuntime(t *testing.T, home string, selection model.Selection) *installRuntime {
	t.Helper()

	resolved := planner.ResolvedPlan{Agents: selection.Agents, OrderedComponents: selection.Components}
	rt, err := newInstallRuntime(home, ScopeGlobal, ChannelStable, selection, resolved, system.PlatformProfile{PackageManager: "brew"})
	if err != nil {
		t.Fatalf("newInstallRuntime() error = %v", err)
	}
	return rt
}

// runInstallInjectionSteps executes the staged apply steps that write managed
// assets. Agent installation steps are skipped because they shell out to real
// package managers, which is unrelated to what these tests assert.
func runInstallInjectionSteps(t *testing.T, rt *installRuntime) {
	t.Helper()

	for _, step := range rt.stagePlan().Apply {
		if _, isAgentInstall := step.(agentInstallStep); isAgentInstall {
			continue
		}
		if err := step.Run(); err != nil {
			t.Fatalf("Run(%s) error = %v", step.ID(), err)
		}
	}
}

// runInstallComponentSteps executes only the component steps, which is how a
// later run reaches an already-guided installation.
func runInstallComponentSteps(t *testing.T, rt *installRuntime) {
	t.Helper()

	for _, step := range rt.stagePlan().Apply {
		if _, isComponent := step.(componentApplyStep); !isComponent {
			continue
		}
		if err := step.Run(); err != nil {
			t.Fatalf("Run(%s) error = %v", step.ID(), err)
		}
	}
}

func systemPromptFileFor(t *testing.T, home string, agent model.AgentID) string {
	t.Helper()

	adapter, err := agents.NewAdapter(agent)
	if err != nil {
		t.Fatalf("NewAdapter(%q) error = %v", agent, err)
	}
	return adapter.SystemPromptFile(home)
}

func readTextFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return string(content)
}

// openCodeOrchestratorPrompt returns the decoded managed orchestrator prompt.
// Reading the raw settings bytes would not do: Go's JSON encoder escapes "<" to
// "<", so the managed markers only exist as such in the decoded string the
// agent actually loads.

func TestInstallDeliversRoutingGuidanceWithoutSDDComponent(t *testing.T) {
	home := t.TempDir()

	rt := newTestInstallRuntime(t, home, model.Selection{
		Agents:     []model.AgentID{model.AgentClaudeCode},
		Components: []model.ComponentID{model.ComponentPersona},
		Persona:    model.PersonaGentleman,
	})
	runInstallInjectionSteps(t, rt)

	prompt := readTextFile(t, systemPromptFileFor(t, home, model.AgentClaudeCode))
	if !strings.Contains(prompt, routingOpenMarker) || !strings.Contains(prompt, routingCloseMarker) {
		t.Fatalf("install without the SDD component left the agent unrouted:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Implementation Routing") {
		t.Fatalf("routing section is present but carries no routing guidance:\n%s", prompt)
	}
}

func TestInstallRoutingGuidanceIsIndependentOfSDDSelection(t *testing.T) {
	const sddMarker = "<!-- gentle-ai:sdd-orchestrator -->"

	withoutSDD := t.TempDir()
	runInstallInjectionSteps(t, newTestInstallRuntime(t, withoutSDD, model.Selection{
		Agents: []model.AgentID{model.AgentClaudeCode},
	}))

	withSDD := t.TempDir()
	runInstallInjectionSteps(t, newTestInstallRuntime(t, withSDD, model.Selection{
		Agents:     []model.AgentID{model.AgentClaudeCode},
		Components: []model.ComponentID{model.ComponentSDD},
		SDDMode:    model.SDDModeSingle,
	}))

	plain := readTextFile(t, systemPromptFileFor(t, withoutSDD, model.AgentClaudeCode))
	sdd := readTextFile(t, systemPromptFileFor(t, withSDD, model.AgentClaudeCode))

	for label, prompt := range map[string]string{"without sdd": plain, "with sdd": sdd} {
		if !strings.Contains(prompt, routingOpenMarker) {
			t.Fatalf("%s: routing guidance missing:\n%s", label, prompt)
		}
	}

	if strings.Contains(plain, sddMarker) {
		t.Fatalf("install without the SDD component gained SDD orchestration assets:\n%s", plain)
	}
	if !strings.Contains(sdd, sddMarker) {
		t.Fatalf("install with the SDD component lost SDD orchestration assets:\n%s", sdd)
	}
}

// TestInstallRoutingGuidanceSurvivesOpenCodeSDDInjection pins the ordering
// hazard: the OpenCode SDD injector assigns the orchestrator prompt wholesale,
// so guidance that is not preserved across that assignment disappears from the
// only always-loaded scope OpenCode reads.
//
// The SDD component step is replayed on its own after a complete install. That
// isolates the hazard from the staged step order: a fix that merely schedules
// guidance last would still pass a full-plan run and still destroy guidance
// here, which is the sequence a later sync actually performs.

func TestInstallStripsLegacyTriggerRulesSection(t *testing.T) {
	home := t.TempDir()

	promptPath := systemPromptFileFor(t, home, model.AgentClaudeCode)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(promptPath), err)
	}
	seeded := filemerge.InjectMarkdownSection("# My own notes\n", "trigger-rules", "Retired WorkRun ceremony\n")
	if err := os.WriteFile(promptPath, []byte(seeded), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", promptPath, err)
	}

	runInstallInjectionSteps(t, newTestInstallRuntime(t, home, model.Selection{
		Agents: []model.AgentID{model.AgentClaudeCode},
	}))

	prompt := readTextFile(t, promptPath)
	if strings.Contains(prompt, legacyTriggerRulesOpenMarker) {
		t.Fatalf("legacy trigger-rules section survived the install:\n%s", prompt)
	}
	if strings.Contains(prompt, "Retired WorkRun ceremony") {
		t.Fatalf("legacy trigger-rules content survived the install:\n%s", prompt)
	}
	if !strings.Contains(prompt, "# My own notes") {
		t.Fatalf("stripping the legacy section destroyed unmanaged user content:\n%s", prompt)
	}
	if !strings.Contains(prompt, routingOpenMarker) {
		t.Fatalf("routing guidance missing after the legacy strip:\n%s", prompt)
	}
}

// TestAgentRoutingGuidanceStepSkipsAgentsWithoutSystemPrompt covers issue
// #4063: Pi reports SupportsSystemPrompt()==false because gentle-pi owns its
// system prompt, so the routing guidance step must leave Pi's
// APPEND_SYSTEM.md untouched instead of writing an agent-routing block into a
// file gentle-ai does not own.
func TestAgentRoutingGuidanceStepSkipsAgentsWithoutSystemPrompt(t *testing.T) {
	home := t.TempDir()
	promptPath := systemPromptFileFor(t, home, model.AgentPi)
	existing := "user text before\n" +
		"\n" +
		"<!-- gentle-ai:agent-routing -->\n" +
		"stale routing body\n" +
		"<!-- /gentle-ai:agent-routing -->\n" +
		"\n" +
		"user text after\n"
	mustWriteFile(t, promptPath, []byte(existing))

	step := agentRoutingGuidanceStep{
		id:      "agent-guidance:" + string(model.AgentPi),
		agent:   model.AgentPi,
		homeDir: home,
		scope:   ScopeGlobal,
	}
	if err := step.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := readTextFile(t, promptPath)
	if got != existing {
		t.Fatalf("agentRoutingGuidanceStep rewrote Pi's system prompt file, want a no-op:\ngot  = %q\nwant = %q", got, existing)
	}
}

// TestRoutingGuidancePathsExcludesAgentsWithoutSystemPrompt covers issue
// #4063: Pi's APPEND_SYSTEM.md must never be listed as a routing guidance
// target, because the step that would write it is now a no-op for Pi and
// declaring the path would only add a backup target nothing ever writes.
func TestRoutingGuidancePathsExcludesAgentsWithoutSystemPrompt(t *testing.T) {
	home := t.TempDir()
	adapters := resolveAdapters([]model.AgentID{model.AgentPi, model.AgentClaudeCode})

	paths := routingGuidancePaths(home, "", ScopeGlobal, adapters)

	piPrompt := systemPromptFileFor(t, home, model.AgentPi)
	if containsPath(paths, piPrompt) {
		t.Fatalf("routingGuidancePaths() listed Pi's system prompt %q, a file the step no longer writes\npaths=%v", piPrompt, paths)
	}
	claudePrompt := systemPromptFileFor(t, home, model.AgentClaudeCode)
	if !containsPath(paths, claudePrompt) {
		t.Fatalf("routingGuidancePaths() lost Claude Code's prompt path %q\npaths=%v", claudePrompt, paths)
	}
}

// ─── Routing guidance is part of the rollback contract ─────────────────────
//
// Routing guidance is installed for every configured agent, independently of
// the selected components. A selection whose components happen to cover the
// same file hid the gap; a selection with no components at all exposes it:
// without the routing path in the snapshot, install rewrites a file that was
// never backed up and a rollback cannot restore it.

func TestBackupTargetsIncludeRoutingGuidancePathsWithoutAnyComponent(t *testing.T) {
	home := t.TempDir()
	agent := model.AgentClaudeCode
	selection := model.Selection{Agents: []model.AgentID{agent}}
	resolved := planner.ResolvedPlan{Agents: selection.Agents}

	targets, err := backupTargets(home, "", ScopeGlobal, selection, resolved)
	if err != nil {
		t.Fatalf("backupTargets() error = %v", err)
	}

	routing, err := agentguidance.RoutingPaths(home, agent)
	if err != nil {
		t.Fatalf("RoutingPaths(%q) error = %v", agent, err)
	}
	if len(routing) == 0 {
		t.Fatalf("RoutingPaths(%q) returned no path; the test proves nothing", agent)
	}
	for _, path := range routing {
		if !containsPath(targets, path) {
			t.Fatalf("backupTargets missing routing guidance path %q\ntargets = %v", path, targets)
		}
	}
}

func TestBackupTargetsEngramClaudeIncludeRegistryAndLegacyMigrationSource(t *testing.T) {
	home := t.TempDir()
	selection := model.Selection{Agents: []model.AgentID{model.AgentClaudeCode}, Components: []model.ComponentID{model.ComponentEngram}}
	resolved := planner.ResolvedPlan{Agents: selection.Agents, OrderedComponents: selection.Components}

	targets, err := backupTargets(home, "", ScopeGlobal, selection, resolved)
	if err != nil {
		t.Fatalf("backupTargets() error = %v", err)
	}
	for _, want := range []string{
		filepath.Join(home, ".claude.json"),
		filepath.Join(home, ".claude", "mcp", "engram.json"),
	} {
		if !containsPath(targets, want) {
			t.Fatalf("backupTargets missing Claude Engram path %q; targets=%v", want, targets)
		}
	}
}

func TestBackupTargetsClaudeContext7IncludeCleanupWithoutVerificationRequirement(t *testing.T) {
	for _, tc := range []struct {
		name          string
		scope         InstallScope
		sameWorkspace bool
		wantRoot      string
	}{
		{name: "user scope", scope: ScopeGlobal, wantRoot: "home"},
		{name: "workspace scope", scope: ScopeWorkspace, wantRoot: "workspace"},
		{name: "workspace is home", scope: ScopeWorkspace, sameWorkspace: true, wantRoot: "home"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			workspace := t.TempDir()
			if tc.sameWorkspace {
				workspace = home
			}
			selection := model.Selection{
				Agents:     []model.AgentID{model.AgentClaudeCode},
				Components: []model.ComponentID{model.ComponentContext7},
			}
			resolved := planner.ResolvedPlan{Agents: selection.Agents, OrderedComponents: selection.Components}
			adapters := resolveAdapters(selection.Agents)

			targets, err := backupTargets(home, workspace, tc.scope, selection, resolved)
			if err != nil {
				t.Fatalf("backupTargets() error = %v", err)
			}
			root := home
			if tc.wantRoot == "workspace" {
				root = workspace
			}
			wantSettings := adapters[0].SettingsPath(root)
			if !containsPath(targets, wantSettings) {
				t.Fatalf("backupTargets missing cleanup path %q; targets=%v", wantSettings, targets)
			}

			verificationPaths := componentPathsWithWorkspaceScoped(home, workspace, tc.scope, selection, adapters, model.ComponentContext7)
			if containsPath(verificationPaths, wantSettings) {
				t.Fatalf("component verification must not require best-effort cleanup path %q; paths=%v", wantSettings, verificationPaths)
			}

			otherRoot := workspace
			if root == workspace {
				otherRoot = home
			}
			if !tc.sameWorkspace && containsPath(targets, adapters[0].SettingsPath(otherRoot)) {
				t.Fatalf("backupTargets selected the wrong scope's cleanup path; targets=%v", targets)
			}
		})
	}
}

func assertNoDuplicatePaths(t *testing.T, label string, paths []string) {
	t.Helper()

	seen := map[string]struct{}{}
	for _, path := range paths {
		if _, duplicate := seen[path]; duplicate {
			t.Fatalf("%s returned duplicate path %q\npaths = %v", label, path, paths)
		}
		seen[path] = struct{}{}
	}
}
