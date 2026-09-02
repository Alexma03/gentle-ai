package sdd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gentleman-programming/gentle-ai/v2/internal/agents"
	"github.com/gentleman-programming/gentle-ai/v2/internal/assets"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/filemerge"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/skills"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

const legacyMandatoryWording = "TOTALMENTE " + "obligatorio"

type InjectionResult struct {
	Changed bool
	Files   []string
}

type InjectOptions struct {
	// ClaudeModelAssignments is the legacy model-only Claude assignment map.
	// Prefer ClaudePhaseAssignments for new callers that need per-phase effort.
	ClaudeModelAssignments      map[string]model.ClaudeModelAlias
	ClaudePhaseAssignments      map[string]model.ClaudePhaseAssignment
	CodexModelAssignments       map[string]model.CodexEffort
	CodexCarrilModelAssignments map[string]string // carril→model-id; nil = use defaults
	CodexPhaseModelAssignments  map[string]string // phase→model-id; non-empty = Custom per-phase mode; nil/empty = preset/carril mode

	// WorkspaceDir is retained for callers that share a sync options shape with
	// workspace-scoped components. SDD itself writes only global assets.
	WorkspaceDir string

	// StrictTDD enables Strict TDD mode. When true, a
	// <!-- gentle-ai:strict-tdd-mode --> marker section is injected into
	// the agent's system prompt so agents know Strict TDD is active.
	StrictTDD bool

	// Capability is the model capability ("capable" or "small") used to
	// extract the appropriate section from SDD skill files. If empty,
	// skills.InjectWithCapability will be called with empty capability
	// (no section extraction, full content written).
	Capability string

	// CodeGraphGuidanceMarkdown is the shared CodeGraph search-order guidance to
	// inject into SDD phase sub-agent prompts. Empty means disabled; normal SDD
	// installs must leave it empty unless the Community Tool path enabled CodeGraph.
	CodeGraphGuidanceMarkdown string
}

func (opts InjectOptions) orchestratorPolicyRenderOptions() OrchestratorRenderOptions {
	return OrchestratorRenderOptions{}
}

// claudeModelResolver is an optional adapter capability. When implemented,
// the subagent copy loop stamps the resolved ClaudeModelAlias into the agent
// frontmatter sentinel {{CLAUDE_MODEL}}. Claude Code accepts "fable", "opus",
// "sonnet", and "haiku" directly as model values, so the resolver is effectively an
// identity function on the alias string — but the interface keeps the opt-in
// shape consistent with retiredModelResolver.
type claudeModelResolver interface {
	ClaudeModelID(alias model.ClaudeModelAlias) string
}

// codexModelResolver is an optional adapter capability. When implemented,
// injectFileAppend will replace the {{CODEX_PHASE_EFFORTS}} placeholder in the
// Codex SDD orchestrator asset with a rendered per-phase effort+model table
// derived from CodexModelAssignments and CodexCarrilModelAssignments in InjectOptions.
//
// Adapters that do NOT implement this interface are completely unaffected —
// the substitution only fires when the adapter satisfies this interface.
type codexModelResolver interface {
	RenderCodexPhaseEfforts(assignments map[string]model.CodexEffort, carrilModels map[string]string) string
}

// overlayAssetPath returns the embedded asset path for the SDD agent overlay
// based on the selected SDD mode. Empty or SDDModeSingle uses the single
// orchestrator overlay; SDDModeMulti uses the multi-agent overlay.
var compatibilitySDDSkillIDs = []model.SkillID{
	"sdd-init", "sdd-explore", "sdd-research", "sdd-propose", "sdd-spec",
	"sdd-design", "sdd-tasks", "sdd-apply", "sdd-verify", "sdd-archive",
	"sdd-onboard", "judgment-day",
}

// SkillDirectoryPaths returns every file that InjectSkillDirectory may write or
// remove. The legacy marker remains a transaction target so a failed refresh can
// restore it from the compatibility backup.
func SkillDirectoryPaths(skillDir, capability string) ([]string, error) {
	sharedFiles, err := assets.SharedSkillFileNames()
	if err != nil {
		return nil, fmt.Errorf("resolve SDD shared files: %w", err)
	}
	if len(sharedFiles) == 0 {
		return nil, fmt.Errorf("resolve SDD shared files: embedded %s listing is empty", assets.SharedSkillDir)
	}
	paths := make([]string, 0, len(sharedFiles))
	for _, fileName := range sharedFiles {
		paths = append(paths, filepath.Join(skillDir, "_shared", fileName))
	}
	paths = append(paths, filepath.Join(skillDir, "_shared", "SKILL.md"))
	if capability == "" {
		capability = "capable"
	}
	skillPaths, err := skills.DirectoryPaths(skillDir, compatibilitySDDSkillIDs, capability)
	if err != nil {
		return nil, fmt.Errorf("enumerate SDD skills: %w", err)
	}
	return append(paths, skillPaths...), nil
}

// InjectSkillDirectory refreshes the SDD skills and their shared references in
// an already-selected skills directory. It is separate from adapter injection
// so compatibility paths can be refreshed once per operation.
func InjectSkillDirectory(skillDir, capability string) (InjectionResult, error) {
	return InjectSkillDirectoryWithWriter(skillDir, capability, filemerge.WriteFileAtomic)
}

// InjectSkillDirectoryWithWriter refreshes SDD skills with a caller-selected writer.
func InjectSkillDirectoryWithWriter(skillDir, capability string, writeFile func(string, []byte, fs.FileMode) (filemerge.WriteResult, error)) (InjectionResult, error) {
	return injectSkillDirectoryWithWriter(skillDir, capability, writeFile, removeLegacySharedSkillMarker)
}

// InjectSkillDirectoryWithCompatibilityWriter refreshes SDD skills through a
// compatibility-root writer that owns both writes and legacy-marker removal.
func InjectSkillDirectoryWithCompatibilityWriter(skillDir, capability string, writeFile func(string, []byte, fs.FileMode) (filemerge.WriteResult, error), removeLegacyMarker func(string) (bool, error)) (InjectionResult, error) {
	return injectSkillDirectoryWithWriter(skillDir, capability, writeFile, removeLegacyMarker)
}

func injectSkillDirectoryWithWriter(skillDir, capability string, writeFile func(string, []byte, fs.FileMode) (filemerge.WriteResult, error), removeLegacyMarker func(string) (bool, error)) (InjectionResult, error) {
	sharedFiles, err := assets.SharedSkillFileNames()
	if err != nil {
		return InjectionResult{}, fmt.Errorf("resolve SDD shared files: %w", err)
	}
	if len(sharedFiles) == 0 {
		return InjectionResult{}, fmt.Errorf("resolve SDD shared files: embedded %s listing is empty", assets.SharedSkillDir)
	}
	result := InjectionResult{}
	for _, fileName := range sharedFiles {
		assetPath := assets.SharedSkillDir + "/" + fileName
		content, err := assets.Read(assetPath)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("required SDD shared file %q: embedded asset not found: %w", fileName, err)
		}
		if len(content) == 0 {
			return InjectionResult{}, fmt.Errorf("required SDD shared file %q: embedded asset is empty", fileName)
		}

		path := filepath.Join(skillDir, "_shared", fileName)
		writeResult, err := writeFile(path, []byte(content), 0o644)
		if err != nil {
			return InjectionResult{}, err
		}
		result.Changed = result.Changed || writeResult.Changed
		result.Files = append(result.Files, path)
	}

	legacyMarkerPath := filepath.Join(skillDir, "_shared", "SKILL.md")
	removedLegacyMarker, err := removeLegacyMarker(legacyMarkerPath)
	if err != nil {
		return InjectionResult{}, err
	}
	if removedLegacyMarker {
		result.Changed = true
		result.Files = append(result.Files, legacyMarkerPath)
	}

	if capability == "" {
		capability = "capable"
	}
	sddResult, err := skills.InjectDirectoryWithCapabilityWithWriter(skillDir, compatibilitySDDSkillIDs, capability, writeFile)
	if err != nil {
		return InjectionResult{}, fmt.Errorf("inject SDD skills: %w", err)
	}
	result.Changed = result.Changed || sddResult.Changed
	result.Files = append(result.Files, sddResult.Files...)
	return result, nil
}

// removeLegacySharedSkillMarker removes the obsolete generated file that made
// the _shared support directory look like an invokable skill. It never touches
// README.md or shared reference files, and rejects non-regular paths rather than
// following or removing them.
func removeLegacySharedSkillMarker(markerPath string) (bool, error) {
	info, err := os.Lstat(markerPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat legacy shared skill marker %s: %w", markerPath, err)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("legacy shared skill marker %s is not a regular file", markerPath) // refusal:by-design world-action: replace or remove the non-regular legacy marker before refreshing shared SDD assets
	}
	if err := os.Remove(markerPath); err != nil {
		return false, fmt.Errorf("remove legacy shared skill marker %s: %w", markerPath, err)
	}
	return true, nil
}

// Engram registers under two valid shapes for Claude Code: `claude mcp add
// engram` exposes its tools as mcp__engram__*, and the plugin route namespaces
// them as mcp__plugin_engram_engram__*. #2698/#3778: the agent contracts
// hardcoded the plugin form, so on the direct route every declared Engram tool
// named a tool that does not exist and the phase actor returned an empty
// result.
//
// The assets carry {{ENGRAM_TOOL_PREFIX}}<tool> and injection expands it to
// BOTH shapes, which is what #2698 proposes. Probing the ambient config
// instead would be order-dependent: the Engram component registers the
// user-scope entry during the same sync that renders these agents, so a probe
// resolves one way on first render and the other way on the next.
var engramToolPlaceholder = regexp.MustCompile(`\{\{ENGRAM_TOOL_PREFIX\}\}([A-Za-z0-9_]+)`)

func expandEngramToolNames(content string) string {
	return engramToolPlaceholder.ReplaceAllString(content, "mcp__engram__$1, mcp__plugin_engram_engram__$1")
}

func Inject(homeDir string, adapter agents.Adapter, sddMode model.SDDModeID, options ...InjectOptions) (InjectionResult, error) {
	if !adapter.SupportsSystemPrompt() {
		return InjectionResult{}, nil
	}
	var opts InjectOptions
	if len(options) > 0 {
		opts = options[0]
	}

	files := make([]string, 0)
	changed := false

	// 1. Inject SDD orchestrator into the global system prompt.
	switch adapter.SystemPromptStrategy() {
	case model.StrategyMarkdownSections:
		result, err := injectMarkdownSections(homeDir, adapter, opts.ClaudeModelAssignments, opts.ClaudePhaseAssignments, opts.orchestratorPolicyRenderOptions())
		if err != nil {
			return InjectionResult{}, err
		}
		changed = changed || result.Changed
		files = append(files, result.Files...)

	case model.StrategyFileReplace, model.StrategyAppendToFile:
		// For FileReplace/AppendToFile agents, the SDD orchestrator is included
		// in the generic persona asset. However, if the user chose neutral or
		// custom persona, the SDD content must still be injected. We append the
		// SDD orchestrator section to the existing system prompt file so it is
		// always present regardless of persona choice.
		result, err := injectFileAppend(homeDir, adapter, opts)
		if err != nil {
			return InjectionResult{}, err
		}
		changed = changed || result.Changed
		files = append(files, result.Files...)

	}

	// 1b. If StrictTDD is enabled, inject the strict-tdd-mode marker section
	// into the system prompt file so agents know Strict TDD is active.
	if opts.StrictTDD {
		promptPath := adapter.SystemPromptFile(homeDir)
		strictTDDContent := "Strict TDD Mode: enabled"
		existing, readErr := readFileOrEmpty(promptPath)
		if readErr != nil {
			return InjectionResult{}, readErr
		}
		updated := filemerge.InjectMarkdownSection(existing, "strict-tdd-mode", strictTDDContent)
		writeResult, writeErr := filemerge.WriteFileAtomic(promptPath, []byte(updated), 0o644)
		if writeErr != nil {
			return InjectionResult{}, writeErr
		}
		changed = changed || writeResult.Changed
		// Only append path once (it may already be in files from step 1).
		alreadyInFiles := false
		for _, f := range files {
			if f == promptPath {
				alreadyInFiles = true
				break
			}
		}
		if !alreadyInFiles {
			files = append(files, promptPath)
		}
	}

	// 2. Write slash commands (if the agent supports them).
	if adapter.SupportsSlashCommands() {
		commandsDir := adapter.CommandsDir(homeDir)
		if commandsDir != "" {
			commandsAssetDir := assets.SDDCommandsAssetDir(adapter.Agent())
			commandEntries, err := fs.ReadDir(assets.FS, commandsAssetDir)
			if err != nil {
				return InjectionResult{}, fmt.Errorf("read embedded %s: %w", commandsAssetDir, err)
			}

			for _, entry := range commandEntries {
				if entry.IsDir() {
					continue
				}

				content := renderBoundedReviewAsset(adapter.Agent(), commandsAssetDir+"/"+entry.Name())
				path := filepath.Join(commandsDir, entry.Name())
				writeResult, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
				if err != nil {
					return InjectionResult{}, err
				}

				changed = changed || writeResult.Changed
				files = append(files, path)
			}
		}
	}

	// 3. Write SDD skill files (if the agent supports skills).
	if adapter.SupportsSkills() {
		skillDir := adapter.SkillsDir(homeDir)
		if skillDir != "" {
			skillResult, skillErr := InjectSkillDirectory(skillDir, opts.Capability)
			if skillErr != nil {
				return InjectionResult{}, skillErr
			}
			changed = changed || skillResult.Changed
			files = append(files, skillResult.Files...)
		}
	}

	// Claude Code keeps the always-on CLAUDE.md bootstrap thin. The heavy SDD
	// workflow procedure is installed as a lazy shared skill document and read
	// only when an SDD command or SDD/Judgment-Day delegation needs it.
	if adapter.Agent() == model.AgentClaudeCode {
		workflowResult, workflowErr := writeClaudeLazySDDWorkflow(homeDir, adapter, opts.ClaudeModelAssignments, opts.ClaudePhaseAssignments)
		if workflowErr != nil {
			return InjectionResult{}, workflowErr
		}
		changed = changed || workflowResult.Changed
		files = append(files, workflowResult.Files...)
	}

	// 3c. Write native sub-agent files for adapters that support them. Sub-agent files are
	// written to the user's home directory (e.g. ~/.cursor/agents/), not to the
	// workspace, so no project-root detection is needed here.
	var agentsDir string
	if adapter.SupportsSubAgents() {
		agentsDir = adapter.SubAgentsDir(homeDir)
		if err := os.MkdirAll(agentsDir, 0o755); err != nil {
			return InjectionResult{}, fmt.Errorf("create agents dir: %w", err)
		}

		embeddedDir := adapter.EmbeddedSubAgentsDir()
		entries, err := assets.FS.ReadDir(embeddedDir)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("read embedded agents dir: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			// Copy all files so adapters may use their native prompt extensions.
			contentStr := renderBoundedReviewAsset(adapter.Agent(), embeddedDir+"/"+entry.Name())

			// Resolve {{CLAUDE_MODEL}} placeholder for adapters that support it (e.g. Claude Code).
			// Non-Claude adapters don't implement claudeModelResolver and are unaffected.
			if cmr, ok := adapter.(claudeModelResolver); ok {
				phase := strings.TrimSuffix(entry.Name(), ".md")
				assignment := resolveClaudePhaseAssignment(opts.ClaudeModelAssignments, opts.ClaudePhaseAssignments, phase)
				contentStr = strings.ReplaceAll(contentStr, "{{CLAUDE_MODEL}}", cmr.ClaudeModelID(assignment.Model))
				contentStr = injectClaudeEffortFrontmatter(contentStr, assignment)
			}

			contentStr = expandEngramToolNames(contentStr)

			if isMarkdownSubAgentPromptFile(entry.Name()) {
				contentStr = injectCodeGraphToolGrantIntoPrompt(contentStr, adapter.Agent(), opts.CodeGraphGuidanceMarkdown)
				contentStr = injectCodeGraphGuidanceIntoPrompt(contentStr, opts.CodeGraphGuidanceMarkdown)
				contentStr = injectLanguageContractIntoPrompt(contentStr)
			}
			outPath := filepath.Join(agentsDir, entry.Name())
			writeResult, err := filemerge.WriteFileAtomic(outPath, []byte(contentStr), 0o644)
			if err != nil {
				return InjectionResult{}, fmt.Errorf("write agent %s: %w", entry.Name(), err)
			}
			changed = changed || writeResult.Changed
			if writeResult.Changed {
				files = append(files, outPath)
			}
		}

		// Post-check: verify critical agent files exist (either .md or .yaml)
		for _, phase := range []string{"sdd-apply", "sdd-verify"} {
			found := false
			for _, ext := range []string{".md", ".yaml"} {
				checkPath := filepath.Join(agentsDir, phase+ext)
				if info, err := os.Stat(checkPath); err == nil && info.Size() >= 10 {
					found = true
					break
				}
			}
			if !found {
				return InjectionResult{}, fmt.Errorf("post-check: sub-agent %q not written correctly (missing or truncated)", phase)
			}
		}
	}

	// 4. Install skill-registry startup automation for agents with runtime hooks.
	// This keeps `.atl/skill-registry.md` fresh without making the orchestrator
	// spend tokens rescanning skills on every session. The command itself is
	// fingerprint-cached, so normal startup is cheap.
	automationResult, err := installSkillRegistryAutomation(homeDir, adapter)
	if err != nil {
		return InjectionResult{}, err
	}
	changed = changed || automationResult.Changed
	files = append(files, automationResult.Files...)

	if adapter.SupportsSkills() {
		skillDir := adapter.SkillsDir(homeDir)
		if skillDir != "" {
			for _, skill := range []string{"sdd-init", "sdd-research", "sdd-apply", "sdd-verify"} {
				path := filepath.Join(skillDir, skill, "SKILL.md")
				info, err := os.Stat(path)
				if err != nil {
					return InjectionResult{}, fmt.Errorf("post-check: SDD skill %q not found on disk: %w", skill, err)
				}
				if info.Size() < 100 {
					return InjectionResult{}, fmt.Errorf("post-check: SDD skill %q is too small (%d bytes) — content may be empty or corrupt", skill, info.Size())
				}
			}
		}
	}

	return InjectionResult{Changed: changed, Files: files}, nil
}

func installSkillRegistryAutomation(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	if adapter.Agent() == model.AgentCodex {
		hooksPath := filepath.Join(adapter.GlobalConfigDir(homeDir), "hooks.json")
		changed, err := ensureCodexSkillRegistryHook(hooksPath)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("install Codex skill-registry hook: %w", err)
		}
		return InjectionResult{Changed: changed, Files: []string{hooksPath}}, nil
	}
	if adapter.Agent() != model.AgentClaudeCode {
		return InjectionResult{}, nil
	}
	settingsPath := adapter.SettingsPath(homeDir)
	if settingsPath == "" {
		return InjectionResult{}, nil
	}
	changed, err := ensureClaudeSkillRegistryHook(settingsPath)
	if err != nil {
		return InjectionResult{}, fmt.Errorf("install Claude skill-registry hook: %w", err)
	}
	return InjectionResult{Changed: changed, Files: []string{settingsPath}}, nil
}

func ensureCodexSkillRegistryHook(hooksPath string) (bool, error) {
	root := map[string]any{}
	if data, err := os.ReadFile(hooksPath); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return false, fmt.Errorf("parse Codex hooks %q: %w", hooksPath, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	const command = `gentle-ai skill-registry refresh --quiet --no-gitignore --cwd "$PWD" || true`
	if claudeHookExists(root, command) {
		return false, nil
	}

	hooksRaw, hasHooks := root["hooks"]
	hooksMap, _ := hooksRaw.(map[string]any)
	if hasHooks && hooksMap == nil {
		return false, fmt.Errorf("Codex hooks %q has unsupported hooks shape: want object", hooksPath)
	}
	if hooksMap == nil {
		hooksMap = map[string]any{}
	}

	sessionRaw, hasSessionStart := hooksMap["SessionStart"]
	sessionStart, _ := sessionRaw.([]any)
	if hasSessionStart && sessionStart == nil {
		return false, fmt.Errorf("Codex hooks %q has unsupported hooks.SessionStart shape: want array", hooksPath)
	}
	sessionStart = append(sessionStart, map[string]any{
		"matcher": "startup|resume|clear|compact",
		"hooks": []any{
			map[string]any{
				"type":          "command",
				"command":       command,
				"timeout":       30,
				"statusMessage": "Refreshing skill registry",
			},
		},
	})
	hooksMap["SessionStart"] = sessionStart
	root["hooks"] = hooksMap

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, err
	}
	out = append(out, '\n')
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		return false, err
	}
	wr, err := filemerge.WriteFileAtomic(hooksPath, out, 0o644)
	if err != nil {
		return false, err
	}
	return wr.Changed, nil
}

func ensureClaudeSkillRegistryHook(settingsPath string) (bool, error) {
	root := map[string]any{}
	if data, err := os.ReadFile(settingsPath); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return false, fmt.Errorf("parse Claude settings %q: %w", settingsPath, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	const command = `gentle-ai skill-registry refresh --quiet --no-gitignore --cwd "${CLAUDE_PROJECT_DIR:-$PWD}" || true`
	if claudeHookExists(root, command) {
		return false, nil
	}

	hooksRaw, hasHooks := root["hooks"]
	hooksMap, _ := hooksRaw.(map[string]any)
	if hasHooks && hooksMap == nil {
		return false, fmt.Errorf("Claude settings %q has unsupported hooks shape: want object", settingsPath)
	}
	if hooksMap == nil {
		hooksMap = map[string]any{}
	}
	promptRaw, hasUserPromptSubmit := hooksMap["UserPromptSubmit"]
	userPromptSubmit, _ := promptRaw.([]any)
	if hasUserPromptSubmit && userPromptSubmit == nil {
		return false, fmt.Errorf("Claude settings %q has unsupported hooks.UserPromptSubmit shape: want array", settingsPath)
	}
	userPromptSubmit = append(userPromptSubmit, map[string]any{
		"matcher": "",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": command,
			},
		},
	})
	hooksMap["UserPromptSubmit"] = userPromptSubmit
	root["hooks"] = hooksMap

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, err
	}
	out = append(out, '\n')
	wr, err := filemerge.WriteFileAtomic(settingsPath, out, 0o644)
	if err != nil {
		return false, err
	}
	return wr.Changed, nil
}

func claudeHookExists(root map[string]any, command string) bool {
	hooksMap, ok := root["hooks"].(map[string]any)
	if !ok {
		return false
	}
	for _, key := range []string{"UserPromptSubmit", "SessionStart"} {
		hookEntries, ok := hooksMap[key].([]any)
		if !ok {
			continue
		}
		if claudeHookListContains(hookEntries, command) {
			return true
		}
	}
	return false
}

func claudeHookListContains(hookEntries []any, command string) bool {
	for _, item := range hookEntries {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		hooks, ok := itemMap["hooks"].([]any)
		if !ok {
			continue
		}
		for _, hook := range hooks {
			hookMap, ok := hook.(map[string]any)
			if ok && hookMap["command"] == command {
				return true
			}
		}
	}
	return false
}

// sddOrchestratorMarkers identify legacy and current headings that may already
// exist in a retained agent prompt before the managed section is injected.
var sddOrchestratorMarkers = []string{
	"## Agent Teams Orchestrator",
	"## Spec-Driven Development (SDD) Orchestrator",
	"## Spec-Driven Development (SDD)",
	"# SDD Orchestrator for Cascade",
}

func hasSDDOrchestrator(content string) bool {
	for _, marker := range sddOrchestratorMarkers {
		if strings.Contains(content, marker) {
			return true
		}
	}
	return false
}

func injectFileAppend(homeDir string, adapter agents.Adapter, opts InjectOptions) (InjectionResult, error) {
	promptPath := adapter.SystemPromptFile(homeDir)

	existing, err := readFileOrEmpty(promptPath)
	if err != nil {
		return InjectionResult{}, err
	}

	// Use agent-specific SDD orchestrator content when available; fall back to generic.
	content := renderSDDOrchestratorAsset(adapter.Agent(), opts.orchestratorPolicyRenderOptions())

	// Codex-only: substitute {{CODEX_PHASE_EFFORTS}} with a rendered per-phase
	// effort table. Only fires when the adapter implements codexModelResolver.
	if cmr, ok := adapter.(codexModelResolver); ok {
		var rendered string
		if len(opts.CodexPhaseModelAssignments) > 0 {
			// Custom per-phase mode: render a per-phase table (phase | model | effort),
			// preserving the selected or explicitly saved carril models as fallbacks.
			rendered = model.RenderCodexPhaseEffortsByPhase(
				opts.CodexPhaseModelAssignments,
				opts.CodexModelAssignments,
				opts.CodexCarrilModelAssignments,
			)
		} else {
			// Preset / carril mode: render the standard per-carril table.
			rendered = cmr.RenderCodexPhaseEfforts(opts.CodexModelAssignments, opts.CodexCarrilModelAssignments)
		}
		content = strings.ReplaceAll(content, "{{CODEX_PHASE_EFFORTS}}", rendered)
		// Post-check: fail loudly if any placeholder token remains unresolved.
		if strings.Contains(content, "{{") {
			return InjectionResult{}, fmt.Errorf("inject(codex): unresolved placeholder token '{{' remains in AGENTS.md content after substitution")
		}
	}

	// If there is a bare (un-marked) legacy orchestrator block, strip it first
	// so InjectMarkdownSection can re-inject the current canonical content.
	if hasLegacyBareOrchestrator(existing) {
		existing = stripBareOrchestratorForFilePrompt(existing)
	}

	updated := filemerge.InjectMarkdownSection(existing, "sdd-orchestrator", content)

	writeResult, err := filemerge.WriteFileAtomic(promptPath, []byte(updated), 0o644)
	if err != nil {
		return InjectionResult{}, err
	}

	return InjectionResult{Changed: writeResult.Changed, Files: []string{promptPath}}, nil
}

func hasLegacyBareOrchestrator(content string) bool {
	markedIdx := strings.Index(content, "<!-- gentle-ai:sdd-orchestrator -->")
	if markedIdx >= 0 {
		prefix := content[:markedIdx]
		if strings.Contains(prefix, "# Agent Teams Lite — Orchestrator Instructions") {
			return true
		}
	}

	firstHeading := -1
	for _, marker := range sddOrchestratorMarkers {
		idx := strings.Index(content, marker)
		if idx >= 0 && (firstHeading == -1 || idx < firstHeading) {
			firstHeading = idx
		}
	}
	if firstHeading < 0 {
		return false
	}

	if markedIdx < 0 {
		return true
	}

	// Legacy bare content exists when an orchestrator heading appears before the
	// canonical marker-based section.
	return firstHeading < markedIdx
}

// stripBareOrchestratorForFilePrompt removes an un-marked SDD orchestrator
// block from file-replace/append/instructions prompt files.
//
// Unlike CLAUDE.md markdown-section files, these prompt files often carry the
// whole orchestrator as a contiguous block followed by other managed sections
// (for example engram-protocol markers). The legacy block also contains many
// "##" headings, so trimming until the next "##" is not enough.
//
// Strategy:
//   - start at the first known orchestrator heading
//   - end at the next managed marker ("<!-- gentle-ai:") if present, else EOF
//   - preserve content before/after and normalize surrounding blank lines
func stripBareOrchestratorForFilePrompt(content string) string {
	if markedIdx := strings.Index(content, "<!-- gentle-ai:sdd-orchestrator -->"); markedIdx >= 0 {
		prefix := content[:markedIdx]
		if start := strings.Index(prefix, "# Agent Teams Lite — Orchestrator Instructions"); start >= 0 {
			before := strings.TrimRight(content[:start], "\n")
			after := strings.TrimLeft(content[markedIdx:], "\n")
			if before == "" {
				if strings.HasSuffix(after, "\n") {
					return after
				}
				return after + "\n"
			}
			result := before + "\n\n" + after
			if !strings.HasSuffix(result, "\n") {
				result += "\n"
			}
			return result
		}
	}

	start := -1
	for _, marker := range sddOrchestratorMarkers {
		idx := strings.Index(content, marker)
		if idx >= 0 && (start == -1 || idx < start) {
			start = idx
		}
	}
	if start < 0 {
		return content
	}

	end := len(content)
	if rel := strings.Index(content[start:], "<!-- gentle-ai:"); rel >= 0 {
		end = start + rel
	}

	before := strings.TrimRight(content[:start], "\n")
	after := strings.TrimLeft(content[end:], "\n")

	if before == "" && after == "" {
		return ""
	}
	if before == "" {
		if strings.HasSuffix(after, "\n") {
			return after
		}
		return after + "\n"
	}
	if after == "" {
		return before + "\n"
	}

	result := before + "\n\n" + after
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return result
}

// legacyPiSystemPromptSectionIDs lists every gentle-ai:-marked section that a
// Pi install made before the capability manifest started reporting
// SupportsSystemPrompt()==false for Pi (see 965187e6) could have left behind
// in the Pi adapter's SystemPromptFile. Nothing manages this file anymore, so
// none of these blocks self-heal on install/sync/uninstall. The last entry
// mirrors communitytool.codeGraphGuidanceSectionID, which is unexported.
var legacyPiSystemPromptSectionIDs = []string{
	"sdd-orchestrator",
	"strict-tdd-mode",
	"persona",
	"codegraph-guidance",
}

// RetirePiSystemPromptBlocks removes any gentle-ai managed markdown sections
// (and a bare legacy SDD orchestrator block) that an older gentle-ai build
// wrote into the Pi adapter's SystemPromptFile. gentle-pi owns the Pi system
// prompt, so this file should carry no gentle-ai content going forward.
//
// It is safe to call unconditionally: a missing file is a no-op, and repeated
// calls are idempotent. Content outside the managed markers is preserved
// byte-for-byte. If nothing but whitespace remains after stripping, the file
// is rewritten with that whitespace-only remainder rather than deleted: a
// whitespace-only file is harmless (Pi appends nothing), the rewrite is
// recoverable and consistent with every other managed-section rewrite, and
// only the uninstall call site registers a backup target for this file.
//
// Only ever call this with the Pi adapter. Unlike the SupportsSystemPrompt()
// gated helpers elsewhere in this package, it strips these sections
// unconditionally regardless of what the adapter reports.
func RetirePiSystemPromptBlocks(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	promptPath := adapter.SystemPromptFile(homeDir)

	existing, err := readFileOrEmpty(promptPath)
	if err != nil {
		return InjectionResult{}, err
	}

	updated := existing
	if hasLegacyBareOrchestrator(updated) {
		updated = stripBareOrchestratorForFilePrompt(updated)
	}
	for _, sectionID := range legacyPiSystemPromptSectionIDs {
		updated = filemerge.InjectMarkdownSection(updated, sectionID, "")
	}

	if updated == existing {
		return InjectionResult{}, nil
	}

	writeResult, err := filemerge.WriteFileAtomic(promptPath, []byte(updated), 0o644)
	if err != nil {
		return InjectionResult{}, err
	}

	return InjectionResult{Changed: writeResult.Changed, Files: []string{promptPath}}, nil
}

const instructionsFrontmatter = "---\n" +
	"name: Gentle AI Persona\n" +
	"description: Gentleman persona with SDD orchestration and Engram protocol\n" +
	"applyTo: \"**\"\n" +
	"---\n"

const steeringFrontmatter = "---\n" +
	"inclusion: always\n" +
	"---\n"

// stripBareOrchestratorSection removes an un-marked "## Agent Teams Orchestrator"
// (or legacy equivalent) block from content. It finds the first matching heading
// and removes everything from that line to the next same-level (##) heading or
// the end of file. This is used to migrate files that contain bare orchestrator
// content (e.g. copied from docs) before injecting the canonical marker-based version.
func stripBareOrchestratorSection(content string) string {
	lines := strings.Split(content, "\n")

	startLine := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, marker := range sddOrchestratorMarkers {
			if trimmed == marker {
				startLine = i
				break
			}
		}
		if startLine >= 0 {
			break
		}
	}

	if startLine < 0 {
		return content
	}

	// Find end: next ## heading (same or higher level) after startLine, or EOF.
	endLine := len(lines)
	for i := startLine + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "## ") {
			endLine = i
			break
		}
	}

	// Rebuild: keep lines before startLine and lines from endLine onward.
	before := lines[:startLine]
	after := lines[endLine:]

	// Trim trailing blank lines from the before section to avoid double newlines.
	for len(before) > 0 && strings.TrimSpace(before[len(before)-1]) == "" {
		before = before[:len(before)-1]
	}

	var parts []string
	if len(before) > 0 {
		parts = append(parts, strings.Join(before, "\n"))
	}
	if len(after) > 0 {
		afterStr := strings.Join(after, "\n")
		// Trim leading blank lines from the after section.
		afterStr = strings.TrimLeft(afterStr, "\n")
		if afterStr != "" {
			parts = append(parts, afterStr)
		}
	}

	result := strings.Join(parts, "\n\n")
	if result != "" && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return result
}

func injectMarkdownSections(homeDir string, adapter agents.Adapter, legacyAssignments map[string]model.ClaudeModelAlias, phaseAssignments map[string]model.ClaudePhaseAssignment, renderOptions OrchestratorRenderOptions) (InjectionResult, error) {
	promptPath := adapter.SystemPromptFile(homeDir)
	content := renderSDDOrchestratorAsset(adapter.Agent(), renderOptions)

	existing, err := readFileOrEmpty(promptPath)
	if err != nil {
		return InjectionResult{}, err
	}

	// Strip legacy Agent Teams Lite block (from standalone ATL installer).
	existing = filemerge.StripLegacyATLBlock(existing)

	// If bare (un-marked) orchestrator content exists but the HTML markers are
	// not present, strip the bare block first. This migrates legacy files to the
	// canonical marker-based state without duplicating the section.
	if hasSDDOrchestrator(existing) && !strings.Contains(existing, "<!-- gentle-ai:sdd-orchestrator -->") {
		existing = stripBareOrchestratorSection(existing)
	}

	updated := filemerge.InjectMarkdownSection(existing, "sdd-orchestrator", content)

	writeResult, err := filemerge.WriteFileAtomic(promptPath, []byte(updated), 0o644)
	if err != nil {
		return InjectionResult{}, err
	}

	return InjectionResult{Changed: writeResult.Changed, Files: []string{promptPath}}, nil
}

func writeClaudeLazySDDWorkflow(homeDir string, adapter agents.Adapter, legacyAssignments map[string]model.ClaudeModelAlias, phaseAssignments map[string]model.ClaudePhaseAssignment) (InjectionResult, error) {
	if adapter.Agent() != model.AgentClaudeCode {
		return InjectionResult{}, nil
	}
	skillDir := adapter.SkillsDir(homeDir)
	if strings.TrimSpace(skillDir) == "" {
		return InjectionResult{}, nil
	}

	content := renderBoundedReviewAsset(model.AgentClaudeCode, "claude/sdd-orchestrator-workflow.md")
	if len(legacyAssignments) > 0 || len(phaseAssignments) > 0 {
		var err error
		content, err = injectClaudePhaseAssignments(content, legacyAssignments, phaseAssignments)
		if err != nil {
			return InjectionResult{}, err
		}
	}

	path := filepath.Join(skillDir, "_shared", "sdd-orchestrator-workflow.md")
	writeResult, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
	if err != nil {
		return InjectionResult{}, err
	}
	return InjectionResult{Changed: writeResult.Changed, Files: []string{path}}, nil
}

var claudeModelAssignmentRowOrder = []string{
	"sdd-explore",
	"sdd-research",
	"sdd-propose",
	"sdd-spec",
	"sdd-design",
	"sdd-tasks",
	"sdd-apply",
	"sdd-verify",
	"sdd-archive",
	"sdd-onboard",
	"jd-judge-a",
	"jd-judge-b",
	"jd-fix-agent",
	"default",
}

var claudeModelAssignmentReasons = map[string]string{
	"orchestrator": "Coordinates, makes decisions",
	"sdd-explore":  "Reads code, structural - not architectural",
	"sdd-research": "Collects source-backed evidence",
	"sdd-propose":  "Architectural decisions",
	"sdd-spec":     "Structured writing",
	"sdd-design":   "Architecture decisions",
	"sdd-tasks":    "Mechanical breakdown",
	"sdd-apply":    "Implementation",
	"sdd-verify":   "Validation against spec",
	"sdd-archive":  "Copy and close",
	"sdd-onboard":  "Guided walkthrough, pedagogical",
	"jd-judge-a":   "Adversarial review — blind judge A",
	"jd-judge-b":   "Adversarial review — blind judge B",
	"jd-fix-agent": "Surgical fixes from confirmed issues",
	"default":      "SDD/JD phase fallback",
}

func injectClaudeModelAssignments(content string, assignments map[string]model.ClaudeModelAlias) (string, error) {
	return injectClaudePhaseAssignments(content, assignments, nil)
}

func injectClaudePhaseAssignments(content string, legacyAssignments map[string]model.ClaudeModelAlias, phaseAssignments map[string]model.ClaudePhaseAssignment) (string, error) {
	const openMarker = "<!-- gentle-ai:sdd-model-assignments -->"
	const closeMarker = "<!-- /gentle-ai:sdd-model-assignments -->"

	start := strings.Index(content, openMarker)
	end := strings.Index(content, closeMarker)
	if start == -1 || end == -1 || end < start {
		return "", fmt.Errorf("sdd orchestrator asset missing model assignment markers")
	}

	merged := defaultClaudePhaseAssignments()
	for key, assignment := range model.ClaudePhaseAssignmentsFromLegacy(legacyAssignments) {
		merged[key] = assignment
	}
	for key, assignment := range phaseAssignments {
		if assignment.Valid() {
			merged[key] = assignment
		}
	}

	replacement := renderClaudeModelAssignmentsSection(merged)
	start += len(openMarker)
	return content[:start] + "\n" + replacement + content[end:], nil
}

func defaultClaudePhaseAssignments() map[string]model.ClaudePhaseAssignment {
	return model.ClaudePhaseAssignmentsFromLegacy(model.ClaudeModelPresetBalanced())
}

func resolveClaudeModelAlias(assignments map[string]model.ClaudeModelAlias, phase string) model.ClaudeModelAlias {
	return resolveClaudePhaseAssignment(assignments, nil, phase).Model
}

func resolveClaudePhaseAssignment(legacyAssignments map[string]model.ClaudeModelAlias, phaseAssignments map[string]model.ClaudePhaseAssignment, phase string) model.ClaudePhaseAssignment {
	merged := defaultClaudePhaseAssignments()
	for key, assignment := range model.ClaudePhaseAssignmentsFromLegacy(legacyAssignments) {
		merged[key] = assignment
	}
	for key, assignment := range phaseAssignments {
		if assignment.Valid() {
			merged[key] = assignment
		}
	}

	if assignment, ok := merged[phase]; ok && assignment.Valid() {
		return assignment
	}
	if assignment, ok := merged["default"]; ok && assignment.Valid() {
		return assignment
	}
	return model.ClaudePhaseAssignment{Model: model.ClaudeModelSonnet}
}

func injectClaudeEffortFrontmatter(content string, assignment model.ClaudePhaseAssignment) string {
	const placeholder = "{{CLAUDE_EFFORT_FRONTMATTER}}"
	line := renderClaudeEffortFrontmatter(assignment)
	if line == "" {
		content = strings.ReplaceAll(content, placeholder+"\r\n", "")
		content = strings.ReplaceAll(content, placeholder+"\n", "")
		return strings.ReplaceAll(content, placeholder, "")
	}
	return strings.ReplaceAll(content, placeholder, line)
}

func renderClaudeEffortFrontmatter(assignment model.ClaudePhaseAssignment) string {
	if assignment.Effort == model.ClaudeEffortDefault || !model.ClaudeEffortAllowedForModel(assignment.Model, assignment.Effort) {
		return ""
	}
	return "effort: " + string(assignment.Effort)
}

func renderClaudeModelAssignmentsSection(assignments map[string]model.ClaudePhaseAssignment) string {
	var b strings.Builder
	b.WriteString("## Model Assignments\n\n")
	b.WriteString("Read this table at session start (or before first SDD/Judgment-Day delegation), cache it for the session, and use the mapped alias only for SDD/Judgment-Day phase agents. If an SDD/Judgment-Day phase is missing, use the `default` fallback row. If you do not have access to the assigned model (for example, no Opus access), substitute `sonnet` and continue.\n\n")
	b.WriteString("The Claude Code session model is controlled by Claude Code itself; Gentle AI does not configure the main orchestrator model. This table applies only to Agent tool calls for SDD/Judgment-Day phase sub-agents, not generic delegation.\n\n")
	b.WriteString("**Mandatory phase model gate:** Agent tool calls for SDD/Judgment-Day phase agents MUST include `model`. Generic/non-SDD delegation MUST NOT use this table; omit `model` unless the user explicitly requested an override. Before each SDD/Judgment-Day Agent call, resolve the target phase to an alias from this table.\n\n")
	b.WriteString("| Phase | Default Model | Effort | Reason |\n")
	b.WriteString("|-------|---------------|--------|--------|\n")
	for _, key := range claudeModelAssignmentRowOrder {
		assignment := assignments[key]
		if !assignment.Valid() {
			assignment = model.ClaudePhaseAssignment{Model: model.ClaudeModelSonnet}
		}
		effort := string(assignment.Effort)
		if effort == "" {
			effort = "default"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", key, assignment.Model, effort, claudeModelAssignmentReasons[key]))
	}
	b.WriteString("\n")
	return b.String()
}

// jdAgentSet is a package-level set for O(1) JD agent membership checks,
// consistent with the sddPhaseSet pattern in read_assignments.go.
func readFileOrEmpty(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read file %q: %w", path, err)
	}
	return string(data), nil
}
