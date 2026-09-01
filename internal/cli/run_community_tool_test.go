package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/components/codegraph"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/planner"
	"github.com/gentleman-programming/gentle-ai/v2/internal/system"
)

func TestInstallRuntimeStagePlanAddsCommunityToolStepsInSelectionOrder(t *testing.T) {
	runtime := &installRuntime{
		homeDir:      t.TempDir(),
		workspaceDir: "/work/project",
		selection: model.Selection{
			CommunityTools: []model.CommunityToolID{model.CommunityToolCodeGraph},
		},
		resolved: planner.ResolvedPlan{},
		profile:  system.PlatformProfile{},
		state:    &runtimeState{},
	}

	plan := runtime.stagePlan()
	var got []string
	for _, step := range plan.Apply {
		got = append(got, step.ID())
	}
	want := []string{"apply:rollback-restore", "community-tool:codegraph"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("apply step IDs = %#v, want %#v", got, want)
	}
}

func TestInstallRuntimeStagePlanDeselectionCleansOwnedPiIntegration(t *testing.T) {
	home := t.TempDir()
	writePiInstallFixture(t, home)
	if _, err := codegraph.ReconcilePiCodeGraph(codegraph.PiCodeGraphOptions{
		HomeDir:  home,
		Selected: true,
		EffectiveMCPProbe: func(string) (codegraph.PiCodeGraphMCPProbeResult, error) {
			return codegraph.PiCodeGraphMCPProbeResult{
				AdapterAvailable: true,
				Initialized:      true,
				Tools: []codegraph.PiCodeGraphMCPTool{{
					Name: "codegraph_explore",
					InputSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"query":       map[string]any{"type": "string"},
							"maxFiles":    map[string]any{"type": "integer"},
							"projectPath": map[string]any{"type": "string"},
						},
						"required": []any{"query"},
					},
				}},
			}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	runtime := &installRuntime{
		homeDir:      home,
		workspaceDir: filepath.Join(home, "project"),
		selection:    model.Selection{Agents: []model.AgentID{model.AgentPi}},
		resolved:     planner.ResolvedPlan{Agents: []model.AgentID{model.AgentPi}},
		profile:      system.PlatformProfile{},
		state:        &runtimeState{},
	}
	plan := runtime.stagePlan()
	step := plan.Apply[len(plan.Apply)-1]
	if step.ID() != "community-tool:pi-codegraph-deselect" {
		t.Fatalf("last apply step = %q, want Pi deselection cleanup", step.ID())
	}
	if err := step.Run(); err != nil {
		t.Fatalf("deselection pipeline step error = %v", err)
	}
	if runtime.state.piCodeGraph == nil || !runtime.state.piCodeGraph.Changed {
		t.Fatalf("pipeline Pi result = %#v, want reported cleanup", runtime.state.piCodeGraph)
	}
	if _, err := os.Stat(filepath.Join(home, ".gentle-ai", "pi-codegraph.json")); !os.IsNotExist(err) {
		t.Fatalf("manifest remains after pipeline deselection: %v", err)
	}
}

func TestBackupTargetsSnapshotPiManifestOverlayDuringDeselection(t *testing.T) {
	home := t.TempDir()
	overlay := filepath.Join(home, ".pi", "agent", "subagents", "package.md")
	manifest := filepath.Join(home, ".gentle-ai", "pi-codegraph.json")
	writePiInstallFixture(t, home)
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{"children": map[string]any{overlay: map[string]any{"after": "managed", "afterHash": "hash", "overlay": true}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	targets, err := backupTargets(home, "", ScopeGlobal, model.Selection{}, planner.ResolvedPlan{Agents: []model.AgentID{model.AgentPi}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(targets, manifest) || !slices.Contains(targets, overlay) {
		t.Fatalf("backup targets = %v, want manifest and discovered overlay during deselection", targets)
	}
}

func TestBackupTargetsSnapshotCrossAgentCodeGraphGuidance(t *testing.T) {
	home := t.TempDir()
	claudeConfig := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeConfig, 0o755); err != nil {
		t.Fatal(err)
	}
	selection := model.Selection{CommunityTools: []model.CommunityToolID{model.CommunityToolCodeGraph}}
	targets, err := backupTargets(home, "", ScopeGlobal, selection, planner.ResolvedPlan{})
	if err != nil {
		t.Fatal(err)
	}
	guidancePaths := codegraph.CodeGraphGuidancePaths(home)
	if len(guidancePaths) == 0 {
		t.Fatal("CodeGraphGuidancePaths() = empty; Claude fixture was not detected")
	}
	for _, path := range guidancePaths {
		if !slices.Contains(targets, path) {
			t.Fatalf("backup targets = %v, missing guidance path %q", targets, path)
		}
	}
}

func TestPiCodeGraphReconcileStepRollbackRemovesDynamicPackageOverlay(t *testing.T) {
	home := t.TempDir()
	overlay := filepath.Join(home, ".pi", "agent", "subagents", "package.md")
	manifest := filepath.Join(home, ".gentle-ai", "pi-codegraph.json")
	writePiInstallFixture(t, home)
	mustWriteFile(t, overlay, []byte("owned overlay\n"))
	if err := os.MkdirAll(filepath.Dir(manifest), 0o700); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{"children": map[string]any{overlay: map[string]any{"after": "owned overlay\n", "afterHash": "c7455d95571450daf45e091de82bf35230a8016c09c60b15b2b84cfde219669f", "overlay": true}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	step := piCodeGraphReconcileStep{homeDir: home}
	if err := step.Rollback(); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if _, err := os.Stat(overlay); !os.IsNotExist(err) {
		t.Fatalf("dynamic package overlay remains after later pipeline rollback: %v", err)
	}
}

func TestPiCodeGraphPendingClassification(t *testing.T) {
	realErr := errors.New("restore Pi CodeGraph journal")
	tests := []struct {
		name    string
		err     error
		pending bool
	}{
		{name: "bare", err: codegraph.ErrPiCodeGraphAdapterHealthUnavailable, pending: true},
		{name: "wrapped", err: fmt.Errorf("verify adapter: %w", codegraph.ErrPiCodeGraphAdapterHealthUnavailable), pending: true},
		{name: "all pending join", err: errors.Join(codegraph.ErrPiCodeGraphAdapterHealthUnavailable, fmt.Errorf("wrapped: %w", codegraph.ErrPiCodeGraphAdapterHealthUnavailable)), pending: true},
		{name: "pending plus rollback failure", err: errors.Join(codegraph.ErrPiCodeGraphAdapterHealthUnavailable, realErr)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := codegraph.PreservePiCodeGraphPending(codegraph.PiCodeGraphResult{}, tc.err)
			if tc.pending {
				if err != nil || len(result.ManualActions) != 1 || !strings.Contains(result.ManualActions[0], "pending") {
					t.Fatalf("result=%#v error=%v, want exactly one pending action", result, err)
				}
				return
			}
			if !errors.Is(err, realErr) || len(result.ManualActions) != 0 {
				t.Fatalf("result=%#v error=%v, want fatal rollback error without pending action", result, err)
			}
		})
	}
}

func TestRenderInstallManualActionsIncludesPiCodeGraphDrift(t *testing.T) {
	out := RenderInstallManualActions(InstallResult{PiCodeGraph: &codegraph.PiCodeGraphResult{ManualActions: []string{"Pi CodeGraph child drifted; preserved: /tmp/worker.md"}}})
	if !strings.Contains(out, "Manual actions required") || !strings.Contains(out, "child drifted") {
		t.Fatalf("CLI manual action missing: %q", out)
	}
}

func TestCommunityToolInstallStepUsesInjectableInstaller(t *testing.T) {
	previousInstall := installCommunityToolWithHome
	previousRunCommand := runCommand
	t.Cleanup(func() {
		installCommunityToolWithHome = previousInstall
		runCommand = previousRunCommand
	})

	runCommand = func(string, ...string) error {
		t.Fatal("communityToolInstallStep should not call real command runner when installer is injected")
		return nil
	}

	var gotTool model.CommunityToolID
	var gotWorkspace string
	var runner codegraph.Runner
	installCommunityToolWithHome = func(tool model.CommunityToolID, workspaceDir string, _ string, r codegraph.Runner, _ codegraph.Detector) (codegraph.Result, error) {
		gotTool = tool
		gotWorkspace = workspaceDir
		runner = r
		return codegraph.Result{Tool: tool}, nil
	}

	step := communityToolInstallStep{id: "community-tool:codegraph", tool: model.CommunityToolCodeGraph, workspaceDir: "/work/project"}
	if err := step.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotTool != model.CommunityToolCodeGraph || gotWorkspace != "/work/project" || runner == nil {
		t.Fatalf("installer args = (%q, %q, %#v), want CodeGraph, workspace, runner", gotTool, gotWorkspace, runner)
	}
}

func TestCommunityToolInstallStepPassesRuntimeHomeToPiReconciler(t *testing.T) {
	previous := installCommunityToolWithHome
	t.Cleanup(func() { installCommunityToolWithHome = previous })
	var gotHome string
	installCommunityToolWithHome = func(_ model.CommunityToolID, _ string, home string, _ codegraph.Runner, _ codegraph.Detector) (codegraph.Result, error) {
		gotHome = home
		return codegraph.Result{Tool: model.CommunityToolCodeGraph}, nil
	}
	step := communityToolInstallStep{id: "community-tool:codegraph", tool: model.CommunityToolCodeGraph, workspaceDir: "/work/project", homeDir: "/tmp/pi-home"}
	if err := step.Run(); err != nil {
		t.Fatal(err)
	}
	if gotHome != "/tmp/pi-home" {
		t.Fatalf("home = %q, want runtime home", gotHome)
	}
}

func TestInstallPipelinePropagatesInitialPiPendingWhenPiUnselected(t *testing.T) {
	previous := installCommunityToolWithHome
	t.Cleanup(func() { installCommunityToolWithHome = previous })
	pending := codegraph.PiCodeGraphResult{ManualActions: []string{"Pi CodeGraph runtime verification is pending."}}
	installCommunityToolWithHome = func(_ model.CommunityToolID, _ string, _ string, _ codegraph.Runner, _ codegraph.Detector) (codegraph.Result, error) {
		return codegraph.Result{Tool: model.CommunityToolCodeGraph, PiCodeGraph: &pending}, nil
	}
	runtime := &installRuntime{
		selection: model.Selection{CommunityTools: []model.CommunityToolID{model.CommunityToolCodeGraph}},
		state:     &runtimeState{},
	}

	if err := runtime.stagePlan().Apply[1].Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if runtime.state.piCodeGraph == nil || !reflect.DeepEqual(runtime.state.piCodeGraph.ManualActions, pending.ManualActions) {
		t.Fatalf("pipeline Pi result = %#v, want initial pending action", runtime.state.piCodeGraph)
	}
	if out := RenderInstallManualActions(InstallResult{PiCodeGraph: runtime.state.piCodeGraph}); !strings.Contains(out, pending.ManualActions[0]) {
		t.Fatalf("rendered install actions = %q, want pending action", out)
	}
}

func TestInstallPipelineDoesNotDuplicatePiPendingWhenSelected(t *testing.T) {
	previousInstall := installCommunityToolWithHome
	previousReconcile := reconcilePiCodeGraph
	t.Cleanup(func() {
		installCommunityToolWithHome = previousInstall
		reconcilePiCodeGraph = previousReconcile
	})
	pending := codegraph.PiCodeGraphResult{ManualActions: []string{"Pi CodeGraph integration remains pending: CodeGraph configuration was installed and preserved, and direct MCP capability was verified. Pi adapter activation health cannot be machine-verified on the detected Pi version."}}
	installCommunityToolWithHome = func(_ model.CommunityToolID, _ string, _ string, _ codegraph.Runner, _ codegraph.Detector) (codegraph.Result, error) {
		return codegraph.Result{Tool: model.CommunityToolCodeGraph, PiCodeGraph: &pending}, nil
	}
	reconcilePiCodeGraph = func(codegraph.PiCodeGraphOptions) (codegraph.PiCodeGraphResult, error) {
		return pending, codegraph.ErrPiCodeGraphAdapterHealthUnavailable
	}
	runtime := &installRuntime{
		// newInstallRuntime never yields an empty homeDir, and the routing
		// guidance step fails closed on one. Keep the fixture faithful to the
		// constructor rather than relaxing that check.
		homeDir:   t.TempDir(),
		selection: model.Selection{CommunityTools: []model.CommunityToolID{model.CommunityToolCodeGraph}},
		resolved:  planner.ResolvedPlan{Agents: []model.AgentID{model.AgentPi}},
		state:     &runtimeState{},
	}

	for _, step := range runtime.stagePlan().Apply[1:] {
		if step.ID() == "agent:pi" {
			continue
		}
		if err := step.Run(); err != nil {
			t.Fatalf("step %q error = %v", step.ID(), err)
		}
	}
	if runtime.state.piCodeGraph == nil || len(runtime.state.piCodeGraph.ManualActions) != 1 {
		t.Fatalf("pipeline Pi result = %#v, want exactly one pending action", runtime.state.piCodeGraph)
	}
}

func TestPiCodeGraphMCPRuntimeClassification(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CodeGraph runtime uses a POSIX script")
	}
	tests := []struct {
		name    string
		tools   string
		pending bool
	}{
		{name: "valid MCP capability", tools: `[{"name":"codegraph_explore","inputSchema":{"type":"object","properties":{"query":{"type":"string"},"maxFiles":{"type":"integer"},"projectPath":{"type":"string"}},"required":["query"]}}]`, pending: true},
		{name: "malformed MCP response", tools: `not-json`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			writePiInstallFixture(t, home)
			mustWriteFile(t, filepath.Join(home, ".pi", "agent", "npm", "node_modules", "pi-mcp-adapter", "index.ts"), []byte("export default {}\n"))
			installFakeCodeGraphMCP(t, tc.tools)

			result, err := codegraph.ReconcilePiCodeGraph(codegraph.PiCodeGraphOptions{HomeDir: home, Selected: true})
			result, err = codegraph.PreservePiCodeGraphPending(result, err)
			if tc.pending {
				if err != nil || len(result.ManualActions) != 1 {
					t.Fatalf("result=%#v error=%v, want exactly one pending action", result, err)
				}
				return
			}
			if err == nil || errors.Is(err, codegraph.ErrPiCodeGraphAdapterHealthUnavailable) {
				t.Fatalf("result=%#v error=%v, want concrete fatal runtime error", result, err)
			}
		})
	}
}

func TestSyncPlanIncludesPiCodeGraphReconciliationAfterComponentsWhenSelected(t *testing.T) {
	home := t.TempDir()
	runtime, err := newSyncRuntime(home, model.Selection{
		Agents:         []model.AgentID{model.AgentPi},
		CommunityTools: []model.CommunityToolID{model.CommunityToolCodeGraph},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan := runtime.stagePlan()
	if len(plan.Apply) == 0 || plan.Apply[len(plan.Apply)-1].ID() != "sync:community-tool:pi-codegraph" {
		t.Fatalf("sync apply steps do not end in Pi reconciliation")
	}
}

func withCodeGraphLookPath(t *testing.T, lookPath func(string) (string, error)) {
	t.Helper()
	previousLookPath := cmdLookPath
	t.Cleanup(func() { cmdLookPath = previousLookPath })
	cmdLookPath = func(name string) (string, error) {
		if name != "codegraph" {
			return "", errors.New("not found")
		}
		return lookPath(name)
	}
}

func assertOpenCodeSharedPromptCodeGraphGuidance(t *testing.T, home string, want bool) {
	t.Helper()
	promptPath := filepath.Join(home, ".config", "opencode", "prompts", "sdd", "sdd-apply.md")
	content, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", promptPath, err)
	}
	text := string(content)
	hasGuidance := strings.Contains(text, "<!-- gentle-ai:codegraph-guidance -->") && strings.Contains(text, "gentle-ai codegraph init --cwd <project-root>")
	if hasGuidance != want {
		t.Fatalf("CodeGraph guidance present = %v, want %v in %s", hasGuidance, want, promptPath)
	}
}

func writePiInstallFixture(t *testing.T, home string) {
	t.Helper()
	mustWriteFile(t, filepath.Join(home, ".pi", "agent", "settings.json"), []byte(`{}`))
	mustWriteFile(t, filepath.Join(home, ".pi", "agent", "subagents", "worker.md"), []byte("---\ntools: bash\n---\nwork\n"))
}

func installFakeCodeGraphMCP(t *testing.T, tools string) {
	t.Helper()
	binDir := t.TempDir()
	codeGraphPath := filepath.Join(binDir, "codegraph")
	script := "#!/bin/sh\n" +
		"[ \"$1\" = serve ] && [ \"$2\" = --mcp ] || exit 64\n" +
		"IFS= read -r initialize || exit 65\n" +
		"printf '%s\\n' '{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2025-03-26\",\"capabilities\":{},\"serverInfo\":{\"name\":\"fake-codegraph\",\"version\":\"1\"}}}'\n" +
		"IFS= read -r initialized || exit 66\n" +
		"IFS= read -r tools_list || exit 67\n" +
		"printf '%s\\n' '{\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{\"tools\":" + tools + "}}'\n"
	if err := os.WriteFile(codeGraphPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
