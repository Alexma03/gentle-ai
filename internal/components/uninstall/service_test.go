package uninstall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gentleman-programming/gentle-ai/v2/internal/agents"
	"github.com/gentleman-programming/gentle-ai/v2/internal/agents/claude"
	"github.com/gentleman-programming/gentle-ai/v2/internal/agents/codex"
	"github.com/gentleman-programming/gentle-ai/v2/internal/agents/pi"
	"github.com/gentleman-programming/gentle-ai/v2/internal/backup"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/codegraph"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/engram"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

type stubSnapshotter struct{}

func TestBuildPlanSnapshotsPiManifestAndOwnedOverlay(t *testing.T) {
	homeDir := t.TempDir()
	svc, err := NewService(homeDir, t.TempDir(), "dev")
	if err != nil {
		t.Fatal(err)
	}
	packageChild := filepath.Join(homeDir, ".pi", "agent", "node_modules", "gentle-pi", "subagents", "package.md")
	if err := os.MkdirAll(filepath.Dir(packageChild), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packageChild, []byte("---\ntools: bash\n---\npackage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := codegraph.ReconcilePiCodeGraph(codegraph.PiCodeGraphOptions{HomeDir: homeDir, Selected: true, EffectiveMCPProbe: piCodeGraphProbeForServiceTest}); err != nil {
		t.Fatal(err)
	}
	plan, err := svc.buildPlan([]model.AgentID{model.AgentPi}, nil)
	if err != nil {
		t.Fatal(err)
	}
	paths := codegraph.PiCodeGraphPaths(homeDir, "")
	for _, path := range paths {
		if !slices.Contains(plan.backupTargets, path) {
			t.Fatalf("backup targets = %v, missing Pi artifact %q", plan.backupTargets, path)
		}
	}
	promptPath := pi.NewAdapter().SystemPromptFile(homeDir)
	if !slices.Contains(plan.backupTargets, promptPath) {
		t.Fatalf("backup targets = %v, missing Pi system prompt file %q", plan.backupTargets, promptPath)
	}
}

// TestExecutePlanRetiresStalePiSystemPromptBlocks covers issue #4057: a Pi
// install made before SupportsSystemPrompt()==false for Pi left gentle-ai
// managed blocks in ~/.pi/agent/APPEND_SYSTEM.md. Since adapter.SupportsSystemPrompt()
// is false, componentOperations() never queues a rewrite op for that file, so
// uninstall must retire the stale blocks directly.
func TestExecutePlanRetiresStalePiSystemPromptBlocks(t *testing.T) {
	home := t.TempDir()
	svc, err := NewService(home, t.TempDir(), "dev")
	if err != nil {
		t.Fatal(err)
	}
	svc.snapshotter = stubSnapshotter{}

	promptPath := pi.NewAdapter().SystemPromptFile(home)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := "user text\n\n<!-- gentle-ai:sdd-orchestrator -->\nSDD body\n<!-- /gentle-ai:sdd-orchestrator -->\n"
	if err := os.WriteFile(promptPath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := svc.executePlan(plan{}, []model.AgentID{model.AgentPi})
	if err != nil {
		t.Fatal(err)
	}

	got := string(mustReadServiceFile(t, promptPath))
	want := "user text\n"
	if got != want {
		t.Fatalf("APPEND_SYSTEM.md = %q, want %q", got, want)
	}
	if !slices.Contains(result.ChangedFiles, promptPath) {
		t.Fatalf("ChangedFiles = %v, want to contain %q", result.ChangedFiles, promptPath)
	}
}

func TestExecutePlanCleansPiBeforeSharedMCPMutation(t *testing.T) {
	home := t.TempDir()
	svc, err := NewService(home, t.TempDir(), "dev")
	if err != nil {
		t.Fatal(err)
	}
	svc.snapshotter = stubSnapshotter{}
	mcpPath := filepath.Join(home, ".pi", "agent", "mcp.json")
	child := filepath.Join(home, ".pi", "agent", "subagents", "worker.md")
	if err := os.MkdirAll(filepath.Dir(child), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, []byte("---\ntools: bash\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := codegraph.ReconcilePiCodeGraph(codegraph.PiCodeGraphOptions{HomeDir: home, Selected: true, EffectiveMCPProbe: piCodeGraphProbeForServiceTest}); err != nil {
		t.Fatal(err)
	}
	result, err := svc.executePlan(plan{operations: []operation{{path: mcpPath, apply: func(path string) (bool, bool, error) {
		return true, false, os.WriteFile(path, []byte(`{"mcpServers":{"engram":{"command":"engram"}}}`), 0o600)
	}}}}, []model.AgentID{model.AgentPi})
	if err != nil {
		t.Fatal(err)
	}
	if body := string(mustReadServiceFile(t, mcpPath)); strings.Contains(body, `"codegraph"`) {
		t.Fatalf("false drift preserved CodeGraph entry: %s", body)
	}
	if slices.ContainsFunc(result.ManualActions, func(action string) bool { return strings.Contains(action, "CodeGraph MCP drifted") }) {
		t.Fatalf("manual actions = %v, want no false drift", result.ManualActions)
	}
}

func mustReadServiceFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestExecutePlanPiUninstallPreservesPreexistingMarkedUserChildAndUserMCP(t *testing.T) {
	homeDir := t.TempDir()
	svc, err := NewService(homeDir, t.TempDir(), "dev")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	svc.snapshotter = stubSnapshotter{}
	mcpPath := filepath.Join(homeDir, ".pi", "agent", "mcp.json")
	childPath := filepath.Join(homeDir, ".pi", "agent", "subagents", "worker.md")
	preexisting := "---\ntools: bash, mcp\n---\nuser instructions\n\n<!-- gentle-ai:pi-codegraph-tool -->\npreexisting tool guidance\n<!-- /gentle-ai:pi-codegraph -->\n\n<!-- gentle-ai:pi-codegraph-guidance -->\npreexisting lazy-init guidance\n<!-- /gentle-ai:pi-codegraph -->\n"
	if err := os.MkdirAll(filepath.Dir(mcpPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mcpPath, []byte(`{"mcpServers":{"user":{"command":"user-server"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(childPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(childPath, []byte(preexisting), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := codegraph.ReconcilePiCodeGraph(codegraph.PiCodeGraphOptions{HomeDir: homeDir, Selected: true, EffectiveMCPProbe: piCodeGraphProbeForServiceTest}); err != nil {
		t.Fatalf("ReconcilePiCodeGraph() error = %v", err)
	}

	result, err := svc.executePlan(plan{}, []model.AgentID{model.AgentPi})
	if err != nil {
		t.Fatalf("executePlan() error = %v", err)
	}
	if got, err := os.ReadFile(childPath); err != nil || string(got) != preexisting {
		t.Fatalf("service uninstall child = %q, err = %v; want exact preexisting content", got, err)
	}
	if got, err := os.ReadFile(mcpPath); err != nil || !strings.Contains(string(got), "user-server") || strings.Contains(string(got), `"codegraph"`) {
		t.Fatalf("service uninstall MCP = %q, err = %v", got, err)
	}
	if len(result.ManualActions) != 0 {
		t.Fatalf("service uninstall manual actions = %v, want none", result.ManualActions)
	}
	if _, err := svc.executePlan(plan{}, []model.AgentID{model.AgentPi}); err != nil {
		t.Fatalf("repeat service uninstall error = %v", err)
	}
}

func TestExecutePlanPiUninstallPreservesDriftedChildAndGentlePiSource(t *testing.T) {
	homeDir := t.TempDir()
	svc, err := NewService(homeDir, t.TempDir(), "dev")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	svc.snapshotter = stubSnapshotter{}
	childPath := filepath.Join(homeDir, ".pi", "agent", "subagents", "worker.md")
	packageChild := filepath.Join(homeDir, ".pi", "agent", "node_modules", "gentle-pi", "subagents", "package.md")
	packageBody := "---\ntools: bash\n---\npackage instructions\n"
	if err := os.MkdirAll(filepath.Dir(childPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(childPath, []byte("---\ntools: bash\n---\nuser instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(packageChild), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packageChild, []byte(packageBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := codegraph.ReconcilePiCodeGraph(codegraph.PiCodeGraphOptions{HomeDir: homeDir, Selected: true, EffectiveMCPProbe: piCodeGraphProbeForServiceTest}); err != nil {
		t.Fatalf("ReconcilePiCodeGraph() error = %v", err)
	}
	if err := os.WriteFile(childPath, append([]byte("user changed after provision\n"), []byte("keep this\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := svc.executePlan(plan{}, []model.AgentID{model.AgentPi})
	if err != nil {
		t.Fatalf("executePlan() error = %v", err)
	}
	if got, err := os.ReadFile(childPath); err != nil || !strings.Contains(string(got), "keep this") {
		t.Fatalf("drifted child was not preserved: %q, err = %v", got, err)
	}
	if got, err := os.ReadFile(packageChild); err != nil || string(got) != packageBody {
		t.Fatalf("gentle-pi package child changed: %q, err = %v", got, err)
	}
	if !slices.ContainsFunc(result.ManualActions, func(action string) bool { return strings.Contains(action, "child drifted") }) {
		t.Fatalf("manual actions = %v, want drift action", result.ManualActions)
	}
}

func piCodeGraphProbeForServiceTest(string) (codegraph.PiCodeGraphMCPProbeResult, error) {
	return codegraph.PiCodeGraphMCPProbeResult{
		AdapterAvailable: true,
		Initialized:      true,
		Tools: []codegraph.PiCodeGraphMCPTool{{
			Name: "codegraph_explore",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query":       map[string]any{"type": "string"},
					"maxFiles":    map[string]any{"type": "number"},
					"projectPath": map[string]any{"type": "string"},
				},
				"required": []any{"query"},
			},
		}},
	}, nil
}

func readJSONFileForTest(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", path, err)
	}
	return root
}

func (stubSnapshotter) Create(snapshotDir string, paths []string) (backup.Manifest, error) {
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		return backup.Manifest{}, err
	}
	return backup.Manifest{
		ID:        "snapshot-001",
		CreatedAt: time.Now().UTC(),
	}, nil
}

func TestComponentOperationsContext7ClaudeRemovesSettingsAndManagedLegacyFile(t *testing.T) {
	homeDir := t.TempDir()
	workspaceDir := t.TempDir()

	svc, err := NewService(homeDir, workspaceDir, "dev")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	adapter, ok := svc.registry.Get(model.AgentClaudeCode)
	if !ok {
		t.Fatal("Claude adapter not found in registry")
	}

	settingsPath := adapter.SettingsPath(homeDir)
	legacyPath := adapter.MCPConfigPath(homeDir, "context7")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(settings dir) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(legacy dir) error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"mcpServers":{"context7":{"command":"npx"},"engram":{"command":"engram"}},"theme":"dark"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(settings) error = %v", err)
	}
	legacyManaged := []byte(`{
  "command": "npx",
  "args": [
    "-y",
    "--package=@upstash/context7-mcp@1.0.0",
    "--",
    "context7-mcp"
  ]
}
`)
	if err := os.WriteFile(legacyPath, legacyManaged, 0o644); err != nil {
		t.Fatalf("WriteFile(legacy) error = %v", err)
	}

	ops, targets, err := svc.componentOperations(adapter, model.ComponentContext7)
	if err != nil {
		t.Fatalf("componentOperations() error = %v", err)
	}
	if !slices.Contains(targets, settingsPath) || !slices.Contains(targets, legacyPath) {
		t.Fatalf("targets = %#v, want settings and legacy paths", targets)
	}
	for _, op := range ops {
		if _, _, err := op.apply(op.path); err != nil {
			t.Fatalf("operation %v on %q error = %v", op.typeID, op.path, err)
		}
	}

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy managed context7 file should be removed; stat err = %v", err)
	}
	settings := readJSONFileForTest(t, settingsPath)
	mcpServers := settings["mcpServers"].(map[string]any)
	if _, ok := mcpServers["context7"]; ok {
		t.Fatalf("settings still contains mcpServers.context7: %#v", settings)
	}
	if _, ok := mcpServers["engram"]; !ok {
		t.Fatalf("settings lost unrelated mcpServers.engram: %#v", settings)
	}
}

func TestComponentOperationsClaudeNeverDeleteUserRegistry(t *testing.T) {
	homeDir := t.TempDir()
	workspaceDir := t.TempDir()

	svc, err := NewService(homeDir, workspaceDir, "dev")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	adapter, ok := svc.registry.Get(model.AgentClaudeCode)
	if !ok {
		t.Fatal("Claude adapter not found in registry")
	}

	// The registry holds ONLY the managed server, so removing it empties the
	// file; ~/.claude.json must survive because Claude Code owns it.
	registryPath := claude.UserConfigPath(homeDir)
	seed := []byte(`{"mcpServers":{"context7":{"command":"npx"}}}`)
	if err := os.WriteFile(registryPath, seed, 0o600); err != nil {
		t.Fatalf("WriteFile(registry) error = %v", err)
	}

	ops, _, err := svc.componentOperations(adapter, model.ComponentContext7)
	if err != nil {
		t.Fatalf("componentOperations(context7) error = %v", err)
	}
	for _, op := range ops {
		if _, _, err := op.apply(op.path); err != nil {
			t.Fatalf("operation %v on %q error = %v", op.typeID, op.path, err)
		}
	}

	info, err := os.Stat(registryPath)
	if err != nil {
		t.Fatalf("~/.claude.json must survive removing the last managed server: %v", err)
	}
	registry := readJSONFileForTest(t, registryPath)
	if servers, ok := registry["mcpServers"].(map[string]any); ok {
		if _, still := servers["context7"]; still {
			t.Fatalf("registry still contains mcpServers.context7: %#v", registry)
		}
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("registry mode widened to %v, want 0600", info.Mode().Perm())
	}
}

func TestComponentOperationsEngramClaudePreservesRegistryAndRemovesManagedLegacy(t *testing.T) {
	homeDir := t.TempDir()
	svc, err := NewService(homeDir, t.TempDir(), "dev")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	adapter, ok := svc.registry.Get(model.AgentClaudeCode)
	if !ok {
		t.Fatal("Claude adapter not found in registry")
	}

	registryPath := claude.UserConfigPath(homeDir)
	registry := []byte(`{"oauthAccount":{"emailAddress":"user@example.com"},"mcpServers":{"codegraph":{"command":"codegraph"},"engram":{"command":"/usr/local/bin/engram","args":["mcp","--tools=agent"]}}}`)
	if err := os.WriteFile(registryPath, registry, 0o600); err != nil {
		t.Fatalf("WriteFile(user registry) error = %v", err)
	}
	legacyPath := adapter.MCPConfigPath(homeDir, "engram")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(legacy dir) error = %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte(`{"command":"/usr/local/bin/engram","args":["mcp","--tools=agent"]}`), 0o644); err != nil {
		t.Fatalf("WriteFile(legacy config) error = %v", err)
	}

	ops, targets, err := svc.componentOperations(adapter, model.ComponentEngram)
	if err != nil {
		t.Fatalf("componentOperations(engram) error = %v", err)
	}
	for _, want := range []string{registryPath, legacyPath} {
		if !slices.Contains(targets, want) {
			t.Fatalf("uninstall targets missing %q: %v", want, targets)
		}
	}
	for _, op := range ops {
		if _, _, err := op.apply(op.path); err != nil {
			t.Fatalf("operation %v on %q error = %v", op.typeID, op.path, err)
		}
	}

	remaining := readJSONFileForTest(t, registryPath)
	servers, _ := remaining["mcpServers"].(map[string]any)
	if _, exists := servers["engram"]; exists {
		t.Fatalf("user registry still contains mcpServers.engram: %#v", remaining)
	}
	if got := remaining["oauthAccount"].(map[string]any)["emailAddress"]; got != "user@example.com" {
		t.Fatalf("OAuth data changed during uninstall: %#v", remaining)
	}
	if _, statErr := os.Stat(legacyPath); !os.IsNotExist(statErr) {
		t.Fatalf("managed legacy config must be removed; stat error = %v", statErr)
	}
	if info, statErr := os.Stat(registryPath); statErr != nil {
		t.Fatalf("Stat(user registry) error = %v", statErr)
	} else if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("user registry mode = %o; want 0600", info.Mode().Perm())
	}
}

func TestComponentOperationsContext7ClaudePreservesCustomLegacyFile(t *testing.T) {
	homeDir := t.TempDir()
	workspaceDir := t.TempDir()

	svc, err := NewService(homeDir, workspaceDir, "dev")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	adapter, ok := svc.registry.Get(model.AgentClaudeCode)
	if !ok {
		t.Fatal("Claude adapter not found in registry")
	}

	settingsPath := adapter.SettingsPath(homeDir)
	legacyPath := adapter.MCPConfigPath(homeDir, "context7")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(settings dir) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(legacy dir) error = %v", err)
	}
	custom := []byte(`{"command":"custom-context7"}`)
	if err := os.WriteFile(settingsPath, []byte(`{"mcpServers":{"context7":{"command":"npx"}}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(settings) error = %v", err)
	}
	if err := os.WriteFile(legacyPath, custom, 0o644); err != nil {
		t.Fatalf("WriteFile(legacy) error = %v", err)
	}

	ops, _, err := svc.componentOperations(adapter, model.ComponentContext7)
	if err != nil {
		t.Fatalf("componentOperations() error = %v", err)
	}
	for _, op := range ops {
		if _, _, err := op.apply(op.path); err != nil {
			t.Fatalf("operation %v on %q error = %v", op.typeID, op.path, err)
		}
	}

	got, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatalf("ReadFile(legacy) error = %v", err)
	}
	if string(got) != string(custom) {
		t.Fatalf("custom legacy file changed: %s", string(got))
	}
}

func TestComponentOperationsSDD_ClaudeRemovesManagedCommandFiles(t *testing.T) {
	homeDir := t.TempDir()
	workspaceDir := t.TempDir()

	svc, err := NewService(homeDir, workspaceDir, "dev")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	adapter, ok := svc.registry.Get(model.AgentClaudeCode)
	if !ok {
		t.Fatal("claude adapter not found in registry")
	}

	commandsDir := adapter.CommandsDir(homeDir)
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(commands dir) error = %v", err)
	}

	managed := []string{"sdd-init.md", "sdd-explore.md", "sdd-onboard.md"}
	for _, name := range managed {
		if err := os.WriteFile(filepath.Join(commandsDir, name), []byte(name), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}
	customPath := filepath.Join(commandsDir, "my-custom-command.md")
	if err := os.WriteFile(customPath, []byte("keep"), 0o644); err != nil {
		t.Fatalf("WriteFile(custom command) error = %v", err)
	}

	ops, _, err := svc.componentOperations(adapter, model.ComponentSDD)
	if err != nil {
		t.Fatalf("componentOperations() error = %v", err)
	}

	for _, op := range ops {
		if op.typeID == opRemoveFile {
			if _, _, err := op.apply(op.path); err != nil {
				t.Fatalf("remove file op.apply(%q) error = %v", op.path, err)
			}
		}
	}

	for _, name := range managed {
		if _, err := os.Stat(filepath.Join(commandsDir, name)); !os.IsNotExist(err) {
			t.Fatalf("managed command %q should be removed, stat err = %v", name, err)
		}
	}
	if _, err := os.Stat(customPath); err != nil {
		t.Fatalf("custom command should be preserved, stat err = %v", err)
	}
}

func applySDDOpenCodeOperations(t *testing.T, svc *Service, adapter agents.Adapter) {
	t.Helper()
	ops, _, err := svc.componentOperations(adapter, model.ComponentSDD)
	if err != nil {
		t.Fatalf("componentOperations() error = %v", err)
	}
	for _, op := range ops {
		if _, _, err := op.apply(op.path); err != nil {
			t.Fatalf("op.apply(%q) error = %v", op.path, err)
		}
	}
}

// TestComponentOperationsEngram_CodexRemovesConsolidatedProtocolAssetsWithNoOrphans
// is the task 2.9 regression assertion: the canonical-asset consolidation
// (design.md Decision 3) renamed/removed the SOURCE assets
// (internal/assets/claude/engram-protocol.md, codex/engram-instructions.md,
// codex/engram-compact-prompt.md -> internal/assets/engram/protocol.md), but
// the WRITTEN on-disk paths for Codex (~/.codex/engram-instructions.md,
// ~/.codex/engram-compact-prompt.md) MUST stay byte-identical so the
// uninstaller keeps covering them with no orphaned files left behind.
func TestComponentOperationsEngram_CodexRemovesConsolidatedProtocolAssetsWithNoOrphans(t *testing.T) {
	restore := codex.SetRuntimeVersionCommandForTest("codex-cli 0.144.0", nil)
	t.Cleanup(restore)
	homeDir := t.TempDir()
	workspaceDir := t.TempDir()

	svc, err := NewService(homeDir, workspaceDir, "dev")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	adapter, ok := svc.registry.Get(model.AgentCodex)
	if !ok {
		t.Fatal("codex adapter not found in registry")
	}

	// Actually write the files via the real (post-consolidation) engram
	// injector, instead of hand-crafting fixtures, so this test fails if the
	// renderer ever drifts from the on-disk paths the uninstaller expects.
	if _, err := engram.InjectWithOptions(homeDir, adapter, engram.InjectOptions{}); err != nil {
		t.Fatalf("engram.InjectWithOptions(codex) error = %v", err)
	}

	instructionsPath := filepath.Join(homeDir, ".codex", "engram-instructions.md")
	compactPath := filepath.Join(homeDir, ".codex", "engram-compact-prompt.md")
	for _, path := range []string{instructionsPath, compactPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected engram injection to create %q: %v", path, err)
		}
	}

	svc.SetEngramUninstallScope(model.EngramUninstallScopeGlobal)

	ops, targets, err := svc.componentOperations(adapter, model.ComponentEngram)
	if err != nil {
		t.Fatalf("componentOperations() error = %v", err)
	}

	for _, want := range []string{instructionsPath, compactPath} {
		if !slices.Contains(targets, want) {
			t.Fatalf("componentOperations() targets missing %q; got: %v", want, targets)
		}
	}

	for _, op := range ops {
		if _, _, err := op.apply(op.path); err != nil {
			t.Fatalf("op.apply(%q) error = %v", op.path, err)
		}
	}

	for _, path := range []string{instructionsPath, compactPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %q to be removed by uninstall, err = %v", path, err)
		}
	}

	// No orphaned directory left behind either.
	if entries, err := os.ReadDir(filepath.Join(homeDir, ".codex")); err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "engram-") {
				t.Fatalf("orphaned engram asset left behind after uninstall: %s", entry.Name())
			}
		}
	}
}

func TestComponentOperationsSDD_ClaudeRemovesSkillRegistryHook(t *testing.T) {
	homeDir := t.TempDir()
	workspaceDir := t.TempDir()

	svc, err := NewService(homeDir, workspaceDir, "dev")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	adapter, ok := svc.registry.Get(model.AgentClaudeCode)
	if !ok {
		t.Fatal("claude adapter not found in registry")
	}
	settingsPath := adapter.SettingsPath(homeDir)
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := `{
  "hooks": {
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [
          {"type": "command", "command": "gentle-ai skill-registry refresh --quiet --no-gitignore --cwd \"${CLAUDE_PROJECT_DIR:-$PWD}\" || true"},
          {"type": "command", "command": "echo keep"}
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"type": "command", "command": "echo pre"}]
      }
    ]
  }
}`
	if err := os.WriteFile(settingsPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	ops, _, err := svc.componentOperations(adapter, model.ComponentSDD)
	if err != nil {
		t.Fatalf("componentOperations() error = %v", err)
	}
	for _, op := range ops {
		if op.typeID == opRewriteFile && op.path == settingsPath {
			if _, _, err := op.apply(op.path); err != nil {
				t.Fatalf("settings rewrite op.apply() error = %v", err)
			}
		}
	}
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "gentle-ai skill-registry refresh") {
		t.Fatalf("managed hook should be removed:\n%s", text)
	}
	if !strings.Contains(text, "echo keep") || !strings.Contains(text, "echo pre") {
		t.Fatalf("unrelated hooks should be preserved:\n%s", text)
	}
}

func TestComponentOperationsSDD_CodexRemovesSkillRegistryHook(t *testing.T) {
	homeDir := t.TempDir()
	workspaceDir := t.TempDir()

	svc, err := NewService(homeDir, workspaceDir, "dev")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	adapter, ok := svc.registry.Get(model.AgentCodex)
	if !ok {
		t.Fatal("codex adapter not found in registry")
	}
	hooksPath := filepath.Join(adapter.GlobalConfigDir(homeDir), "hooks.json")
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := `{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup|resume|clear|compact",
        "hooks": [
          {"type": "command", "command": "gentle-ai skill-registry refresh --quiet --no-gitignore --cwd \"$PWD\" || true"},
          {"type": "command", "command": "echo keep"}
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"type": "command", "command": "echo pre"}]
      }
    ]
  }
}`
	if err := os.WriteFile(hooksPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	ops, _, err := svc.componentOperations(adapter, model.ComponentSDD)
	if err != nil {
		t.Fatalf("componentOperations() error = %v", err)
	}
	for _, op := range ops {
		if op.typeID == opRewriteFile && op.path == hooksPath {
			if _, _, err := op.apply(op.path); err != nil {
				t.Fatalf("Codex hooks rewrite op.apply() error = %v", err)
			}
		}
	}
	raw, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "gentle-ai skill-registry refresh") {
		t.Fatalf("managed hook should be removed:\n%s", text)
	}
	if !strings.Contains(text, "echo keep") || !strings.Contains(text, "echo pre") {
		t.Fatalf("unrelated hooks should be preserved:\n%s", text)
	}
}
