package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gentleman-programming/gentle-ai/v2/internal/backup"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/codegraph"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/pipeline"
	"github.com/gentleman-programming/gentle-ai/v2/internal/planner"
	"github.com/gentleman-programming/gentle-ai/v2/internal/state"
	"github.com/gentleman-programming/gentle-ai/v2/internal/system"
	"github.com/gentleman-programming/gentle-ai/v2/internal/tui/screens"
	"github.com/gentleman-programming/gentle-ai/v2/internal/tui/styles"
	"github.com/gentleman-programming/gentle-ai/v2/internal/update"
	"github.com/gentleman-programming/gentle-ai/v2/internal/update/upgrade"
	"github.com/muesli/termenv"
)

func TestNavigationWelcomeToDetection(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenDetection {
		t.Fatalf("screen = %v, want %v", state.Screen, ScreenDetection)
	}
	if !state.InstallFlowActive {
		t.Fatal("expected Start installation to activate the install flow")
	}
}

func TestCodexCustomDiscoveryStartsAsCommandWithFallback(t *testing.T) {
	originalDiscover := discoverCodexModels
	t.Cleanup(func() { discoverCodexModels = originalDiscover })

	called := false
	discoverCodexModels = func(context.Context) []string {
		called = true
		return []string{"discovered-model"}
	}

	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenCodexModelPicker
	m.CodexModelPicker = screens.NewCodexModelPickerState()
	m.Cursor = 3 // Custom

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)
	if called {
		t.Fatal("Custom entry ran Codex discovery synchronously")
	}
	if cmd == nil {
		t.Fatal("Custom entry command = nil")
	}
	if state.CodexModelPicker.CustomMode != screens.CodexCustomModePhaseList {
		t.Fatalf("CustomMode = %v, want phase list", state.CodexModelPicker.CustomMode)
	}
	if !slices.Equal(state.CodexModelPicker.AvailableModels, model.CodexAvailableModels()) {
		t.Fatalf("AvailableModels = %v, want curated fallback", state.CodexModelPicker.AvailableModels)
	}

	msg := cmd()
	if !called {
		t.Fatal("discovery command did not run discovery")
	}
	updated, _ = state.Update(msg)
	state = updated.(Model)
	if !slices.Equal(state.CodexModelPicker.AvailableModels, []string{"discovered-model"}) {
		t.Fatalf("AvailableModels = %v, want discovery result", state.CodexModelPicker.AvailableModels)
	}
}

func TestCodexCustomDiscoveryIgnoresStaleOrIrrelevantResults(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*Model)
		msg         CodexModelsDiscoveredMsg
		wantApplied bool
	}{
		{
			name: "after leaving Custom",
			setup: func(m *Model) {
				m.CodexModelPicker.CustomMode = screens.CodexCustomModeNone
			},
			msg: CodexModelsDiscoveredMsg{RequestID: 1, Models: []string{"late-model"}},
		},
		{
			name: "after leaving picker",
			setup: func(m *Model) {
				m.Screen = ScreenWelcome
			},
			msg: CodexModelsDiscoveredMsg{RequestID: 1, Models: []string{"late-model"}},
		},
		{
			name: "older request",
			setup: func(m *Model) {
				m.codexModelDiscoveryRequest = 2
			},
			msg: CodexModelsDiscoveredMsg{RequestID: 1, Models: []string{"old-model"}},
		},
		{
			name: "current request",
			setup: func(m *Model) {
				m.codexModelDiscoveryRequest = 2
			},
			msg:         CodexModelsDiscoveredMsg{RequestID: 2, Models: []string{"new-model"}},
			wantApplied: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(system.DetectionResult{}, "dev")
			m.Screen = ScreenCodexModelPicker
			m.CodexModelPicker = screens.NewCodexModelPickerState()
			m.CodexModelPicker.CustomMode = screens.CodexCustomModePhaseList
			m.codexModelDiscoveryRequest = 1
			fallback := append([]string(nil), m.CodexModelPicker.AvailableModels...)
			tt.setup(&m)

			updated, _ := m.Update(tt.msg)
			state := updated.(Model)
			if tt.wantApplied {
				if !slices.Equal(state.CodexModelPicker.AvailableModels, tt.msg.Models) {
					t.Fatalf("AvailableModels = %v, want %v", state.CodexModelPicker.AvailableModels, tt.msg.Models)
				}
				return
			}
			if !slices.Equal(state.CodexModelPicker.AvailableModels, fallback) {
				t.Fatalf("AvailableModels = %v, want unchanged %v", state.CodexModelPicker.AvailableModels, fallback)
			}
		})
	}
}

func TestCodexCustomDiscoveryClampsModelSelectCursorBeforeEnter(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenCodexModelPicker
	m.CodexModelPicker = screens.NewCodexModelPickerState()
	m.CodexModelPicker.CustomMode = screens.CodexCustomModeModelSelect
	m.CodexModelPicker.CustomModelCursor = len(m.CodexModelPicker.AvailableModels) - 1
	m.codexModelDiscoveryRequest = 1

	updated, _ := m.Update(CodexModelsDiscoveredMsg{
		RequestID: 1,
		Models:    []string{"discovered-model-1", "discovered-model-2"},
	})
	state := updated.(Model)

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state = updated.(Model)
	if state.CodexModelPicker.CustomMode != screens.CodexCustomModeEffortSelect {
		t.Fatalf("CustomMode = %v, want %v", state.CodexModelPicker.CustomMode, screens.CodexCustomModeEffortSelect)
	}
	if state.CodexModelPicker.CustomPendingModel != "discovered-model-2" {
		t.Fatalf("CustomPendingModel = %q, want %q", state.CodexModelPicker.CustomPendingModel, "discovered-model-2")
	}
}

func TestCodexCustomDiscoveryNewestRequestWins(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenCodexModelPicker
	m.CodexModelPicker = screens.NewCodexModelPickerState()
	m.Cursor = 3 // Custom

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)
	firstRequest := state.codexModelDiscoveryRequest

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyEsc})
	state = updated.(Model)
	state.Cursor = 3 // Custom
	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state = updated.(Model)
	secondRequest := state.codexModelDiscoveryRequest
	if secondRequest <= firstRequest {
		t.Fatalf("second request = %d, want greater than first request %d", secondRequest, firstRequest)
	}

	updated, _ = state.Update(CodexModelsDiscoveredMsg{RequestID: firstRequest, Models: []string{"old-model"}})
	state = updated.(Model)
	if slices.Equal(state.CodexModelPicker.AvailableModels, []string{"old-model"}) {
		t.Fatal("older discovery result replaced the current catalog")
	}
	updated, _ = state.Update(CodexModelsDiscoveredMsg{RequestID: secondRequest, Models: []string{"new-model"}})
	state = updated.(Model)
	if !slices.Equal(state.CodexModelPicker.AvailableModels, []string{"new-model"}) {
		t.Fatalf("AvailableModels = %v, want newest result", state.CodexModelPicker.AvailableModels)
	}
}

func profileModelStep(available bool) Model {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenProfileCreate
	m.ProfileCreateStep = 1
	m.ModelPicker = screens.ModelPickerState{Mode: screens.ModePhaseList, ForProfile: true}
	if available {
		m.ModelPicker.AvailableIDs = []string{"openai"}
	}
	return m
}

func TestProfileCreateEmptyProviderEnterContinuesAndBacksOut(t *testing.T) {
	keep := model.ModelAssignment{ProviderID: "anthropic", ModelID: "claude-sonnet-4", Effort: "high"}
	orch := model.ModelAssignment{ProviderID: "openai", ModelID: "gpt-5"}

	m := profileModelStep(false)
	m.ProfileDraft = model.Profile{
		Name:              "work",
		OrchestratorModel: orch,
		PhaseAssignments:  map[string]model.ModelAssignment{"sdd-apply": keep},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.ProfileCreateStep != 2 || state.Cursor != 0 {
		t.Fatalf("step/cursor = %d/%d, want 2/0", state.ProfileCreateStep, state.Cursor)
	}
	if state.ProfileDraft.OrchestratorModel != orch {
		t.Fatalf("orchestrator = %+v, want unchanged %+v", state.ProfileDraft.OrchestratorModel, orch)
	}
	if got := state.ProfileDraft.PhaseAssignments["sdd-apply"]; got != keep {
		t.Fatalf("sdd-apply assignment = %+v, want unchanged %+v", got, keep)
	}

	back := profileModelStep(false)
	back.Cursor = 1
	updated, _ = back.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state = updated.(Model)

	if state.Screen != ScreenProfileCreate || state.ProfileCreateStep != 0 || state.Cursor != 0 {
		t.Fatalf("screen/step/cursor = %v/%d/%d, want ScreenProfileCreate/0/0", state.Screen, state.ProfileCreateStep, state.Cursor)
	}
}

func TestProfileCreateSeparatorIsIgnoredAndSkipped(t *testing.T) {
	sepIdx := screens.SeparatorRowIdx()
	if sepIdx < 0 {
		t.Skip("no separator row defined")
	}

	m := profileModelStep(true)
	m.Cursor = sepIdx

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.ModelPicker.Mode != screens.ModePhaseList {
		t.Fatalf("ModelPicker.Mode = %v, want ModePhaseList", state.ModelPicker.Mode)
	}
	if state.ModelPicker.SelectedPhaseIdx == sepIdx {
		t.Fatalf("separator row should not become selected phase index %d", sepIdx)
	}

	state.Cursor = sepIdx - 1
	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	state = updated.(Model)

	if state.Cursor != sepIdx+1 {
		t.Fatalf("cursor after j from row before separator = %d, want %d", state.Cursor, sepIdx+1)
	}
}

func TestProfileCreateCustomAgentsDoNotChangeNavigation(t *testing.T) {
	m := profileModelStep(true)
	m.ModelPicker.CustomAgents = []string{"custom-profile-agent"}
	rows := screens.ModelPickerRowsForProfile()
	m.Cursor = len(rows) - 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	state := updated.(Model)
	if state.Cursor != len(rows) {
		t.Fatalf("cursor after last profile row = %d, want Continue at %d", state.Cursor, len(rows))
	}

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state = updated.(Model)
	if state.ProfileCreateStep != 2 || state.Cursor != 0 {
		t.Fatalf("step/cursor after profile Continue = %d/%d, want 2/0", state.ProfileCreateStep, state.Cursor)
	}
}

func TestModelPickerNavigationSkipsReviewSeparator(t *testing.T) {
	rows := screens.ModelPickerRows()
	separator := -1
	for i, row := range rows {
		if row == "--- Review agents ---" {
			separator = i
			break
		}
	}
	if separator < 1 {
		t.Fatalf("review separator missing from rows: %v", rows)
	}

	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenModelPicker
	m.ModelPicker = screens.ModelPickerState{Mode: screens.ModePhaseList, AvailableIDs: []string{"openai"}}
	m.Cursor = separator - 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if got := updated.(Model).Cursor; got != separator+1 {
		t.Fatalf("cursor after review separator = %d, want %d", got, separator+1)
	}
}

func TestNavigationBackWithEscape(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenPersona

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	state := updated.(Model)

	if state.Screen != ScreenAgents {
		t.Fatalf("screen = %v, want %v", state.Screen, ScreenAgents)
	}
}

func TestPiOnlyAgentContinueSkipsPromptsAndIncludesEngram(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenAgents
	m.Selection.Agents = []model.AgentID{model.AgentPi}
	m.Selection.Components = componentsForPreset(model.PresetFullGentleman, model.PersonaGentleman)
	m.Cursor = len(screensAgentOptions())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenDependencyTree {
		t.Fatalf("screen = %v, want %v", state.Screen, ScreenDependencyTree)
	}
	wantComponents := []model.ComponentID{model.ComponentEngram}
	if !reflect.DeepEqual(state.Selection.Components, wantComponents) {
		t.Fatalf("components = %v, want %v", state.Selection.Components, wantComponents)
	}
	if !reflect.DeepEqual(state.DependencyPlan.Agents, []model.AgentID{model.AgentPi}) {
		t.Fatalf("dependency agents = %v, want [pi]", state.DependencyPlan.Agents)
	}
	if !reflect.DeepEqual(state.DependencyPlan.OrderedComponents, wantComponents) {
		t.Fatalf("dependency components = %v, want %v", state.DependencyPlan.OrderedComponents, wantComponents)
	}
}

func TestNewModelPiOnlyDetectionDefaultsToEngramOnly(t *testing.T) {
	detection := system.DetectionResult{Configs: []system.ConfigState{{
		Agent:       string(model.AgentPi),
		Path:        "/tmp/fake/pi",
		Exists:      true,
		IsDirectory: true,
	}}}

	m := NewModel(detection, "dev")

	wantAgents := []model.AgentID{model.AgentPi}
	if !reflect.DeepEqual(m.Selection.Agents, wantAgents) {
		t.Fatalf("agents = %v, want %v", m.Selection.Agents, wantAgents)
	}
	wantComponents := []model.ComponentID{model.ComponentEngram}
	if !reflect.DeepEqual(m.Selection.Components, wantComponents) {
		t.Fatalf("components = %v, want %v", m.Selection.Components, wantComponents)
	}
}

func TestNewModelDefaultsToNeutralPersona(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	if m.Selection.Persona != model.PersonaNeutral {
		t.Fatalf("Persona = %q, want %q", m.Selection.Persona, model.PersonaNeutral)
	}
}

func updateModel(m Model, msg tea.Msg) Model { next, _ := m.Update(msg); return next.(Model) }
func installingModel(labels []string, runID uint64) Model {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen, m.pipelineRunning, m.installRunID = ScreenInstalling, runID != 0, runID
	m.progressRun, m.Progress = newInstallProgressRun(), NewProgressState(labels)
	return m
}

func succeededExecution(stepID string) pipeline.ExecutionResult {
	return pipeline.ExecutionResult{Apply: pipeline.StageResult{Success: true, Steps: []pipeline.StepResult{{StepID: stepID, Status: pipeline.StepStatusSucceeded}}}}
}

func TestStepProgressMsgAddsNestedPackageProgress(t *testing.T) {
	const packageStep = "agent:pi:pi install git:github.com/Alexma03/gentle-pi@custom/main"
	state := updateModel(installingModel([]string{"agent:pi"}, 0), StepProgressMsg{StepID: packageStep, Status: pipeline.StepStatusRunning})
	if len(state.Progress.Items) != 2 {
		t.Fatalf("progress items = %v, want the nested package item", state.Progress.Items)
	}
	if state.Progress.Items[1].Label != packageStep || state.Progress.Items[1].Status != ProgressStatusRunning {
		t.Fatalf("nested package item = %+v, want running %q", state.Progress.Items[1], packageStep)
	}

	state = updateModel(state, StepProgressMsg{StepID: packageStep, Status: pipeline.StepStatusSucceeded})
	if state.Progress.Items[1].Status != string(pipeline.StepStatusSucceeded) {
		t.Fatalf("nested package status = %q, want succeeded", state.Progress.Items[1].Status)
	}
}

func TestStepProgressMsgUpdatesProgressState(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenInstalling
	m.Progress = NewProgressState([]string{"step-a", "step-b"})

	// Send running event for step-a.
	updated, _ := m.Update(StepProgressMsg{StepID: "step-a", Status: pipeline.StepStatusRunning})
	state := updated.(Model)
	if state.Progress.Items[0].Status != ProgressStatusRunning {
		t.Fatalf("step-a status = %q, want running", state.Progress.Items[0].Status)
	}

	// Send succeeded event for step-a.
	updated, _ = state.Update(StepProgressMsg{StepID: "step-a", Status: pipeline.StepStatusSucceeded})
	state = updated.(Model)
	if state.Progress.Items[0].Status != string(pipeline.StepStatusSucceeded) {
		t.Fatalf("step-a status = %q, want succeeded", state.Progress.Items[0].Status)
	}

	// Send failed event for step-b.
	updated, _ = state.Update(StepProgressMsg{StepID: "step-b", Status: pipeline.StepStatusFailed, Err: fmt.Errorf("oops")})
	state = updated.(Model)
	if state.Progress.Items[1].Status != string(pipeline.StepStatusFailed) {
		t.Fatalf("step-b status = %q, want failed", state.Progress.Items[1].Status)
	}

	if !state.Progress.HasFailures() {
		t.Fatalf("expected HasFailures() = true")
	}
}

func TestPipelineDoneMsgRejectsStaleProgress(t *testing.T) {
	m := installingModel([]string{"step-x"}, 1)
	m.Progress.Start(0)
	state := updateModel(m, PipelineDoneMsg{RunID: 1, Result: succeededExecution("step-x")})
	state = updateModel(state, StepProgressMsg{RunID: 1, StepID: "step-x", Status: pipeline.StepStatusFailed, Err: errors.New("late progress")})

	if state.Progress.Items[0].Status != string(pipeline.StepStatusSucceeded) {
		t.Fatalf("stale progress changed completed status to %q", state.Progress.Items[0].Status)
	}
}

func TestPipelineDoneMsgMarksCompletion(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenInstalling
	m.pipelineRunning = true
	m.Progress = NewProgressState([]string{"step-x"})
	m.Progress.Start(0)

	// Simulate pipeline completion with a real step result.
	result := pipeline.ExecutionResult{
		Apply: pipeline.StageResult{
			Success: true,
			Steps: []pipeline.StepResult{
				{StepID: "step-x", Status: pipeline.StepStatusSucceeded},
			},
		},
	}
	updated, _ := m.Update(PipelineDoneMsg{Result: result})
	state := updated.(Model)

	if state.pipelineRunning {
		t.Fatalf("expected pipelineRunning = false")
	}

	if !state.Progress.Done() {
		t.Fatalf("expected progress to be done")
	}
}

func TestPipelineDoneMsgPreservesNestedPackageProgress(t *testing.T) {
	m := installingModel([]string{"agent:pi", "fallback:with:colon"}, 7)

	const packageStep = "agent:pi:pi install git:github.com/Alexma03/gentle-pi@custom/main"
	for _, msg := range []StepProgressMsg{
		{RunID: 7, StepID: packageStep, Status: pipeline.StepStatusRunning},
		{RunID: 7, StepID: packageStep, Status: pipeline.StepStatusSucceeded},
		{RunID: 7, StepID: "agent:pi", Status: pipeline.StepStatusRunning},
		{RunID: 7, StepID: "agent:pi", Status: pipeline.StepStatusSucceeded},
	} {
		m = updateModel(m, msg)
	}

	state := updateModel(m, PipelineDoneMsg{RunID: 7, Result: succeededExecution("agent:pi")})
	if state.pipelineRunning {
		t.Fatal("matching PipelineDoneMsg did not finish the active pipeline")
	}
	nestedIndex := state.findProgressItem(packageStep)
	if nestedIndex < 0 {
		t.Fatalf("nested package item was dropped: %v", state.Progress.Items)
	}
	if !state.Progress.Items[nestedIndex].Nested {
		t.Fatalf("nested package item = %+v, want explicit nested metadata", state.Progress.Items[nestedIndex])
	}
	if state.findProgressItem("fallback:with:colon") >= 0 {
		t.Fatalf("unmarked colon-containing item was preserved: %v", state.Progress.Items)
	}
	if !state.Progress.Done() {
		t.Fatalf("progress = %+v, want done", state.Progress)
	}
	if len(state.Progress.Logs) < 2 || state.Progress.Logs[1] != "done: "+packageStep {
		t.Fatalf("logs = %v, want nested package log preserved", state.Progress.Logs)
	}
}

func TestPipelineDoneMsgSurfacesFailedSteps(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenInstalling
	m.pipelineRunning = true
	m.Progress = NewProgressState([]string{"step-ok", "step-bad"})

	result := pipeline.ExecutionResult{
		Apply: pipeline.StageResult{
			Success: false,
			Err:     fmt.Errorf("step-bad failed"),
			Steps: []pipeline.StepResult{
				{StepID: "step-ok", Status: pipeline.StepStatusSucceeded},
				{StepID: "step-bad", Status: pipeline.StepStatusFailed, Err: fmt.Errorf("skill inject: write failed")},
			},
		},
		Err: fmt.Errorf("step-bad failed"),
	}
	updated, _ := m.Update(PipelineDoneMsg{Result: result})
	state := updated.(Model)

	if !state.Progress.HasFailures() {
		t.Fatalf("expected HasFailures() = true")
	}

	// Verify that the error message appears in the logs.
	found := false
	for _, log := range state.Progress.Logs {
		if contains(log, "skill inject: write failed") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected error detail in logs, got: %v", state.Progress.Logs)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestInstallingScreenManualFallbackWithoutExecuteFn(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenInstalling
	m.Progress = NewProgressState([]string{"step-1", "step-2"})
	m.Progress.Start(0)
	// ExecuteFn is nil — manual fallback should work.

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	// First enter advances step-1 to succeeded.
	if state.Progress.Items[0].Status != "succeeded" {
		t.Fatalf("step-1 status = %q, want succeeded", state.Progress.Items[0].Status)
	}
}

func TestEscBlockedWhilePipelineRunning(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenInstalling
	m.pipelineRunning = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	state := updated.(Model)

	if state.Screen != ScreenInstalling {
		t.Fatalf("screen = %v, want ScreenInstalling (esc should be blocked)", state.Screen)
	}
}

func TestEnterAtFullProgressWaitsForPipelineDone(t *testing.T) {
	m := installingModel([]string{"only-step"}, 11)
	m.Progress.Mark(0, string(pipeline.StepStatusSucceeded))

	state := updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if state.Screen != ScreenInstalling {
		t.Fatalf("screen = %v, want ScreenInstalling while pipeline is active", state.Screen)
	}

	state.progressRun.complete(succeededExecution("only-step"))
	doneValue := state.nextProgressCommand()()
	doneMsg, ok := doneValue.(PipelineDoneMsg)
	if !ok {
		t.Fatalf("progress command returned %T, want PipelineDoneMsg", doneValue)
	}
	state = updateModel(state, doneMsg)
	if state.pipelineRunning {
		t.Fatal("matching PipelineDoneMsg left pipelineRunning set")
	}
	if state.Screen != ScreenInstalling {
		t.Fatalf("screen after PipelineDoneMsg = %v, want ScreenInstalling until Enter", state.Screen)
	}

	state = updateModel(state, tea.KeyMsg{Type: tea.KeyEnter})
	if state.Screen != ScreenComplete {
		t.Fatalf("screen after completed pipeline Enter = %v, want ScreenComplete", state.Screen)
	}
}

func TestInstallingDoneToComplete(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenInstalling
	m.Progress = NewProgressState([]string{"only-step"})
	m.Progress.Mark(0, string(pipeline.StepStatusSucceeded))

	// Progress is at 100%, enter should go to complete.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenComplete {
		t.Fatalf("screen = %v, want ScreenComplete", state.Screen)
	}
}

func TestCompletionViewShowsExecutionErrorWhenStepsSucceeded(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenInstalling
	result := pipeline.ExecutionResult{
		Apply: pipeline.StageResult{
			Success: true,
			Steps:   []pipeline.StepResult{{StepID: "install", Status: pipeline.StepStatusSucceeded}},
		},
		Err: errors.New("persist install state: atomic replacement refused"),
	}

	updated, _ := m.Update(PipelineDoneMsg{Result: result})
	state := updated.(Model)
	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state = updated.(Model)
	out := state.View()

	if state.Screen != ScreenComplete {
		t.Fatalf("screen = %v, want ScreenComplete", state.Screen)
	}
	if strings.Contains(out, "Done! Your AI agents are ready.") || strings.Contains(out, "completed successfully") {
		t.Fatalf("completion output rendered success for execution error: %q", out)
	}
	if !strings.Contains(out, "Installation completed with errors.") || !strings.Contains(out, result.Err.Error()) {
		t.Fatalf("completion output missing execution error: %q", out)
	}
}

func TestBuildProgressLabelsFromResolvedPlan(t *testing.T) {
	resolved := planner.ResolvedPlan{
		Agents:            []model.AgentID{model.AgentClaudeCode},
		OrderedComponents: []model.ComponentID{model.ComponentEngram, model.ComponentSDD},
	}

	labels := buildProgressLabels(resolved, []model.CommunityToolID{model.CommunityToolCodeGraph})

	want := []string{
		"prepare:check-dependencies",
		"prepare:backup-snapshot",
		"apply:rollback-restore",
		"agent:claude-code",
		"community-tool:codegraph",
		"component:engram",
		"component:sdd",
	}

	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels = %v, want %v", labels, want)
	}
}

func TestBackupRestoreMsgHandledGracefully(t *testing.T) {
	// Error case: BackupRestoreMsg with error navigates to ScreenRestoreResult
	// and stores the error in RestoreErr.
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenRestoreConfirm

	updated, _ := m.Update(BackupRestoreMsg{Err: fmt.Errorf("restore-error")})
	state := updated.(Model)

	if state.Screen != ScreenRestoreResult {
		t.Fatalf("error case: expected ScreenRestoreResult, got %v", state.Screen)
	}
	if state.RestoreErr == nil {
		t.Fatalf("expected RestoreErr to be set on error")
	}

	// Success case: BackupRestoreMsg with no error navigates to ScreenRestoreResult
	// with nil RestoreErr.
	m2 := NewModel(system.DetectionResult{}, "dev")
	m2.Screen = ScreenRestoreConfirm
	updated2, _ := m2.Update(BackupRestoreMsg{})
	state2 := updated2.(Model)

	if state2.Screen != ScreenRestoreResult {
		t.Fatalf("success case: expected ScreenRestoreResult, got %v", state2.Screen)
	}
	if state2.RestoreErr != nil {
		t.Fatalf("unexpected RestoreErr on success: %v", state2.RestoreErr)
	}
}

func TestPresetFlowShowsClaudeModelPickerBeforeDependencyTree(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenPreset
	m.Selection.Agents = []model.AgentID{model.AgentClaudeCode}
	m.Selection.Components = []model.ComponentID{model.ComponentEngram, model.ComponentSDD}
	m.Cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenClaudeModelPicker {
		t.Fatalf("screen = %v, want %v", state.Screen, ScreenClaudeModelPicker)
	}
	if state.ClaudeModelPicker.Preset != screens.ClaudePresetBalanced {
		t.Fatalf("preset = %v, want %v", state.ClaudeModelPicker.Preset, screens.ClaudePresetBalanced)
	}
}

func TestClaudeModelPickerBalancedSelectionStoresAssignments(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenClaudeModelPicker
	m.Selection.Agents = []model.AgentID{model.AgentClaudeCode}
	m.Selection.Components = []model.ComponentID{model.ComponentEngram, model.ComponentSDD}
	m.ClaudeModelPicker = screens.NewClaudeModelPickerState()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	// With SDD selected, ClaudeCode flow now goes to ScreenStrictTDD before DependencyTree.
	if state.Screen != ScreenStrictTDD {
		t.Fatalf("screen = %v, want %v (ClaudeCode + SDD goes to StrictTDD first)", state.Screen, ScreenStrictTDD)
	}
	// Orchestrator is present in the balanced preset (injected as part of the model
	// assignment table). The Claude picker shows sub-agents and default; orchestrator
	// is carried through for injection but is not user-editable in the picker UI.
	if got := state.Selection.ClaudeModelAssignments["orchestrator"]; got != model.ClaudeModelOpus {
		t.Fatalf("orchestrator = %q, want %q", got, model.ClaudeModelOpus)
	}
	if got := state.Selection.ClaudeModelAssignments["default"]; got != model.ClaudeModelSonnet {
		t.Fatalf("default = %q, want %q", got, model.ClaudeModelSonnet)
	}
	if got := state.Selection.ClaudeModelAssignments["sdd-archive"]; got != model.ClaudeModelHaiku {
		t.Fatalf("sdd-archive = %q, want %q", got, model.ClaudeModelHaiku)
	}
}

// ─── SDDMode → ModelPicker / DependencyTree transition (issue #106 Bug 2) ──

// sddMultiCursor returns the cursor index for SDDModeMulti in SDDModeOptions.
func sddMultiCursor(t *testing.T) int {
	t.Helper()
	for i, opt := range screens.SDDModeOptions() {
		if opt == model.SDDModeMulti {
			return i
		}
	}
	t.Fatal("SDDModeMulti not found in SDDModeOptions()")
	return -1
}

func withModelPickerSettingsPath(t *testing.T, settingsPath string) {
	t.Helper()
	originalSettingsPath := modelPickerSettingsPath
	modelPickerSettingsPath = func() string { return settingsPath }
	t.Cleanup(func() {
		modelPickerSettingsPath = originalSettingsPath
	})
}

// TestSDDModeMultiShowsRuntimeModelPicker verifies that selecting SDDModeMulti
// opens the runtime model picker before catalog discovery completes.

func screensAgentOptions() []model.AgentID {
	return screens.AgentOptions()
}

// ─── OperationRunning guard: Enter blocked ──────────────────────────────────

// TestOperationRunningGuardBlocksEnterOnUpgrade verifies that pressing Enter on
// ScreenUpgrade while OperationRunning is true does nothing (no screen change,
// no command returned).
func TestOperationRunningGuardBlocksEnterOnUpgrade(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpgrade
	m.OperationRunning = true
	m.UpdateCheckDone = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenUpgrade {
		t.Fatalf("screen changed while OperationRunning=true: got %v, want ScreenUpgrade", state.Screen)
	}
	if cmd != nil {
		t.Fatalf("expected nil cmd while OperationRunning=true on ScreenUpgrade")
	}
}

// TestOperationRunningGuardBlocksEnterOnSync verifies that pressing Enter on
// ScreenSync while OperationRunning is true does nothing.
func TestOperationRunningGuardBlocksEnterOnSync(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenSync
	m.OperationRunning = true
	m.UpdateCheckDone = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenSync {
		t.Fatalf("screen changed while OperationRunning=true: got %v, want ScreenSync", state.Screen)
	}
	if cmd != nil {
		t.Fatalf("expected nil cmd while OperationRunning=true on ScreenSync")
	}
}

// TestOperationRunningGuardBlocksEnterOnUpgradeSync verifies that pressing Enter
// on ScreenUpgradeSync while OperationRunning is true does nothing.
func TestOperationRunningGuardBlocksEnterOnUpgradeSync(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpgradeSync
	m.OperationRunning = true
	m.UpdateCheckDone = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenUpgradeSync {
		t.Fatalf("screen changed while OperationRunning=true: got %v, want ScreenUpgradeSync", state.Screen)
	}
	if cmd != nil {
		t.Fatalf("expected nil cmd while OperationRunning=true on ScreenUpgradeSync")
	}
}

// ─── OperationRunning guard: Esc blocked ────────────────────────────────────

// TestEscBlockedDuringUpgrade verifies that Esc is blocked when OperationRunning
// is true on ScreenUpgrade.
func TestEscBlockedDuringUpgrade(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpgrade
	m.OperationRunning = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	state := updated.(Model)

	if state.Screen != ScreenUpgrade {
		t.Fatalf("screen changed on Esc while OperationRunning=true: got %v, want ScreenUpgrade", state.Screen)
	}
}

// TestEscBlockedDuringSync verifies that Esc is blocked when OperationRunning
// is true on ScreenSync.
func TestEscBlockedDuringSync(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenSync
	m.OperationRunning = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	state := updated.(Model)

	if state.Screen != ScreenSync {
		t.Fatalf("screen changed on Esc while OperationRunning=true: got %v, want ScreenSync", state.Screen)
	}
}

// TestEscBlockedDuringUpgradeSync verifies that Esc is blocked when OperationRunning
// is true on ScreenUpgradeSync.
func TestEscBlockedDuringUpgradeSync(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpgradeSync
	m.OperationRunning = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	state := updated.(Model)

	if state.Screen != ScreenUpgradeSync {
		t.Fatalf("screen changed on Esc while OperationRunning=true: got %v, want ScreenUpgradeSync", state.Screen)
	}
}

// ─── UpgradeDoneMsg error model ─────────────────────────────────────────────

// TestUpgradeDoneMsg_SetsUpgradeErr verifies that sending UpgradeDoneMsg with
// a non-nil error sets UpgradeErr, clears OperationRunning, and leaves
// UpgradeReport nil.
func TestUpgradeDoneMsg_SetsUpgradeErr(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpgrade
	m.OperationRunning = true

	updated, _ := m.Update(UpgradeDoneMsg{Err: fmt.Errorf("test error")})
	state := updated.(Model)

	if state.UpgradeErr == nil {
		t.Fatalf("expected UpgradeErr to be set, got nil")
	}
	if state.OperationRunning {
		t.Fatalf("expected OperationRunning=false after UpgradeDoneMsg with error")
	}
	if state.UpgradeReport != nil {
		t.Fatalf("expected UpgradeReport=nil when upgrade fails, got %+v", state.UpgradeReport)
	}
}

// ─── UpgradePhaseCompletedMsg (two-phase upgrade+sync) ─────────────────────

// TestUpgradePhaseCompletedMsg_SetsReport verifies that a successful upgrade
// phase sets UpgradeReport and keeps OperationRunning true (sync still pending).
func TestUpgradePhaseCompletedMsg_SetsReport(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpgradeSync
	m.OperationRunning = true

	report := upgrade.UpgradeReport{
		Results: []upgrade.ToolUpgradeResult{
			{ToolName: "engram", Status: upgrade.UpgradeSucceeded},
		},
	}
	updated, _ := m.Update(UpgradePhaseCompletedMsg{Report: report})
	state := updated.(Model)

	if state.UpgradeReport == nil {
		t.Fatal("expected UpgradeReport to be set after successful UpgradePhaseCompletedMsg")
	}
	if !state.OperationRunning {
		t.Fatal("expected OperationRunning to remain true (sync phase still pending)")
	}
	if state.UpgradeErr != nil {
		t.Fatalf("expected UpgradeErr=nil on success, got %v", state.UpgradeErr)
	}
}

// TestUpgradePhaseCompletedMsg_SetsErrAndKeepsRunning verifies that a failed
// upgrade phase sets UpgradeErr, keeps OperationRunning true (sync still runs).
func TestUpgradePhaseCompletedMsg_SetsErrAndKeepsRunning(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpgradeSync
	m.OperationRunning = true

	updated, _ := m.Update(UpgradePhaseCompletedMsg{Err: fmt.Errorf("upgrade failed")})
	state := updated.(Model)

	if state.UpgradeErr == nil {
		t.Fatal("expected UpgradeErr to be set after failed UpgradePhaseCompletedMsg")
	}
	if !state.OperationRunning {
		t.Fatal("expected OperationRunning to remain true (sync phase still pending)")
	}
	if state.UpgradeReport != nil {
		t.Fatal("expected UpgradeReport=nil when upgrade phase fails")
	}
}

// ─── UpgradeDoneMsg clears update state ─────────────────────────────────────

// TestUpgradeDoneClearsUpdateResults verifies that after upgrade completes,
// UpdateResults is cleared and UpdateCheckDone is reset so the welcome banner
// no longer shows "Updates available".
func TestUpgradeDoneClearsUpdateResults(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpgrade
	m.OperationRunning = true
	m.UpdateResults = []update.UpdateResult{
		{Tool: update.ToolInfo{Name: "engram"}, InstalledVersion: "1.0.0", LatestVersion: "1.1.0", Status: update.UpdateAvailable},
	}
	m.UpdateCheckDone = true

	report := upgrade.UpgradeReport{
		Results: []upgrade.ToolUpgradeResult{
			{ToolName: "engram", Status: upgrade.UpgradeSucceeded},
		},
	}
	updated, _ := m.Update(UpgradeDoneMsg{Report: report})
	state := updated.(Model)

	if state.UpdateResults != nil {
		t.Fatalf("expected UpdateResults=nil after UpgradeDoneMsg, got %v", state.UpdateResults)
	}
	if state.UpdateCheckDone {
		t.Fatalf("expected UpdateCheckDone=false after UpgradeDoneMsg, got true")
	}
}

// TestUpgradePhaseCompletedClearsUpdateResults verifies that after the upgrade
// phase completes (in Upgrade+Sync flow), UpdateResults is cleared and
// UpdateCheckDone is reset.
func TestUpgradePhaseCompletedClearsUpdateResults(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpgradeSync
	m.OperationRunning = true
	m.UpdateResults = []update.UpdateResult{
		{Tool: update.ToolInfo{Name: "engram"}, InstalledVersion: "1.0.0", LatestVersion: "1.1.0", Status: update.UpdateAvailable},
	}
	m.UpdateCheckDone = true

	report := upgrade.UpgradeReport{
		Results: []upgrade.ToolUpgradeResult{
			{ToolName: "engram", Status: upgrade.UpgradeSucceeded},
		},
	}
	updated, _ := m.Update(UpgradePhaseCompletedMsg{Report: report})
	state := updated.(Model)

	if state.UpdateResults != nil {
		t.Fatalf("expected UpdateResults=nil after UpgradePhaseCompletedMsg, got %v", state.UpdateResults)
	}
	if state.UpdateCheckDone {
		t.Fatalf("expected UpdateCheckDone=false after UpgradePhaseCompletedMsg, got true")
	}
}

func TestReportUpgradedGentleAI(t *testing.T) {
	report := upgrade.UpgradeReport{Results: []upgrade.ToolUpgradeResult{
		{ToolName: "engram", Status: upgrade.UpgradeSucceeded},
		{ToolName: "gentle-ai", Status: upgrade.UpgradeSucceeded},
	}}
	if !reportUpgradedGentleAI(report) {
		t.Fatal("reportUpgradedGentleAI() = false, want true")
	}

	report.Results[1].Status = upgrade.UpgradeFailed
	if reportUpgradedGentleAI(report) {
		t.Fatal("reportUpgradedGentleAI() = true for failed gentle-ai upgrade")
	}
}

// ─── T16: Welcome screen 7-item menu navigation ────────────────────────────

// TestWelcomeMenu_InstallNavigation verifies cursor 0 (Install) goes to ScreenDetection.
func TestWelcomeMenu_InstallNavigation(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenWelcome
	m.Cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenDetection {
		t.Fatalf("cursor=0 (Install): screen = %v, want %v", state.Screen, ScreenDetection)
	}
}

// TestWelcomeMenu_UpgradeNavigation verifies cursor 1 (Upgrade tools) goes to ScreenUpgrade.
func TestWelcomeMenu_UpgradeNavigation(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenWelcome
	m.UpdateCheckDone = true // Skip update-check-pending spinner.
	m.Cursor = 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenUpgrade {
		t.Fatalf("cursor=1 (Upgrade): screen = %v, want %v", state.Screen, ScreenUpgrade)
	}
}

// TestWelcomeMenu_SyncNavigation verifies cursor 2 (Sync configs) goes to ScreenSync.
func TestWelcomeMenu_SyncNavigation(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenWelcome
	m.Cursor = 2

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenSync {
		t.Fatalf("cursor=2 (Sync): screen = %v, want %v", state.Screen, ScreenSync)
	}
}

// TestWelcomeMenu_UpgradeSyncNavigation verifies cursor 3 (Upgrade+Sync) goes to ScreenUpgradeSync.
func TestWelcomeMenu_UpgradeSyncNavigation(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenWelcome
	m.UpdateCheckDone = true
	m.Cursor = 3

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenUpgradeSync {
		t.Fatalf("cursor=3 (Upgrade+Sync): screen = %v, want %v", state.Screen, ScreenUpgradeSync)
	}
}

// TestWelcomeMenu_ConfigureModelsNavigation verifies cursor 4 goes to ScreenModelConfig.
func TestWelcomeMenu_ConfigureModelsNavigation(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenWelcome
	m.Cursor = 4

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenModelConfig {
		t.Fatalf("cursor=4 (Configure Models): screen = %v, want %v", state.Screen, ScreenModelConfig)
	}
}

// TestWelcomeMenu_BackupsNavigation verifies cursor 6 (Manage backups) goes to ScreenBackups.
func TestWelcomeMenu_BackupsNavigation(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenWelcome
	m.Cursor = 6

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenBackups {
		t.Fatalf("cursor=6 (Backups): screen = %v, want %v", state.Screen, ScreenBackups)
	}
}

func TestWelcomeMenu_UninstallNavigation_WithoutProfiles(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenWelcome
	m.Cursor = 9

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenUninstallMode {
		t.Fatalf("cursor=9 (Managed uninstall): screen = %v, want %v", state.Screen, ScreenUninstallMode)
	}
}

// TestWelcomeMenu_OptionCount verifies the welcome menu has 12 items without OpenCode
// and 13 items when OpenCode is detected (adds "OpenCode SDD Profiles" option).

func TestCommunityToolsToggleSelectsCodeGraph(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenCommunityTools
	m.Cursor = 0

	updated, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	state := updated.(Model)

	if !state.Selection.HasCommunityTool(model.CommunityToolCodeGraph) {
		t.Fatalf("expected CodeGraph selected, got %v", state.Selection.CommunityTools)
	}
}

func TestStandaloneCommunityToolsContinueWithoutSelectionNoOps(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenCommunityTools
	m.CommunityToolsStandalone = true
	m.Cursor = len(communityToolDefinitions()) * 2

	updated, cmd := m.confirmSelection()
	state := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no command when no community tools are selected")
	}
	if state.Screen != ScreenCommunityToolResult {
		t.Fatalf("screen = %v, want %v", state.Screen, ScreenCommunityToolResult)
	}
	if state.OperationRunning {
		t.Fatal("OperationRunning should be false for no-op community tool selection")
	}
}

func TestStandaloneCommunityToolsShowsInstallingBeforeCompletion(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenCommunityTools
	m.CommunityToolsStandalone = true
	m.Selection.CommunityTools = []model.CommunityToolID{model.CommunityToolCodeGraph}
	m.Cursor = len(communityToolDefinitions()) * 2

	updated, cmd := m.confirmSelection()
	state := updated.(Model)

	if cmd == nil {
		t.Fatal("expected install command for selected community tools")
	}
	if state.Screen != ScreenCommunityToolInstalling {
		t.Fatalf("screen = %v, want %v", state.Screen, ScreenCommunityToolInstalling)
	}
	if !state.OperationRunning {
		t.Fatal("OperationRunning should be true while community tool installation is in flight")
	}

	out := state.View()
	for _, want := range []string{"Installing community tools…", "1 selected.", "CodeGraph"} {
		if !strings.Contains(out, want) {
			t.Fatalf("installing view missing %q; output:\n%s", want, out)
		}
	}
	for _, unexpected := range []string{"✓ Community tools configured", "> Return to menu"} {
		if strings.Contains(out, unexpected) {
			t.Fatalf("installing view should not show %q before completion; output:\n%s", unexpected, out)
		}
	}
}

func TestStandaloneCommunityToolsShowsResultAfterCompletion(t *testing.T) {
	tests := []struct {
		name     string
		msg      CommunityToolInstallationDoneMsg
		wantText string
	}{
		{
			name: "success",
			msg: CommunityToolInstallationDoneMsg{Results: []codegraph.Result{{
				Tool: model.CommunityToolCodeGraph,
			}}},
			wantText: "✓ Community tools configured",
		},
		{
			name: "error with partial result",
			msg: CommunityToolInstallationDoneMsg{
				Results: []codegraph.Result{{Tool: model.CommunityToolCodeGraph}},
				Err:     errors.New("install failed"),
			},
			wantText: "Community tool setup failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(system.DetectionResult{}, "dev")
			m.Screen = ScreenCommunityToolInstalling
			m.OperationRunning = true
			m.Selection.CommunityTools = []model.CommunityToolID{model.CommunityToolCodeGraph}

			updated, _ := m.Update(tt.msg)
			state := updated.(Model)

			if state.Screen != ScreenCommunityToolResult {
				t.Fatalf("screen = %v, want %v", state.Screen, ScreenCommunityToolResult)
			}
			if state.OperationRunning {
				t.Fatal("OperationRunning should be false after community tool completion")
			}
			out := state.View()
			if !strings.Contains(out, tt.wantText) {
				t.Fatalf("result view missing %q; output:\n%s", tt.wantText, out)
			}
			if strings.Contains(out, "Installing community tools…") {
				t.Fatalf("result view should not keep loading text; output:\n%s", out)
			}
		})
	}
}

func TestCommunityToolInstallationPreservesPartialResultOnError(t *testing.T) {
	originalInstall := communityToolInstallFn
	originalGetwd := osGetwdFn
	t.Cleanup(func() {
		communityToolInstallFn = originalInstall
		osGetwdFn = originalGetwd
	})

	osGetwdFn = func() (string, error) { return "/work/project", nil }
	communityToolInstallFn = func(id model.CommunityToolID, workspaceDir string, runner codegraph.Runner) (codegraph.Result, error) {
		if id != model.CommunityToolCodeGraph || workspaceDir != "/work/project" || runner == nil {
			t.Fatalf("install args = (%q, %q, %#v), want CodeGraph, workspace, runner", id, workspaceDir, runner)
		}
		return codegraph.Result{
			Tool:        id,
			CommandsRun: []string{"npm exec --yes --package @colbymchenry/codegraph@latest -- codegraph install --yes"},
		}, errors.New("install failed")
	}

	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenCommunityToolInstalling
	m.OperationRunning = true
	m.Selection.CommunityTools = []model.CommunityToolID{model.CommunityToolCodeGraph}
	cmd := m.startCommunityToolInstallation()
	if cmd == nil {
		t.Fatal("startCommunityToolInstallation() command = nil")
	}

	msg := cmd()
	done, ok := msg.(CommunityToolInstallationDoneMsg)
	if !ok {
		t.Fatalf("message = %T, want CommunityToolInstallationDoneMsg", msg)
	}
	if done.Err == nil {
		t.Fatal("expected install error")
	}
	if len(done.Results) != 1 || done.Results[0].Tool != model.CommunityToolCodeGraph || len(done.Results[0].CommandsRun) != 1 {
		t.Fatalf("results = %#v, want partial CodeGraph result", done.Results)
	}

	updated, _ := m.Update(done)
	state := updated.(Model)
	if state.CommunityToolErr == nil {
		t.Fatal("expected state to retain community tool error")
	}
	if len(state.CommunityToolResults) != 1 || len(state.CommunityToolResults[0].CommandsRun) != 1 {
		t.Fatalf("state results = %#v, want preserved partial result", state.CommunityToolResults)
	}
}

func TestUninstallModeScreen_PartialNavigatesToAgentSelection(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUninstallMode
	m.Cursor = 0 // Partial Uninstall option

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenUninstall {
		t.Fatalf("screen = %v, want %v", state.Screen, ScreenUninstall)
	}
	if state.UninstallMode != model.UninstallModePartial {
		t.Fatalf("UninstallMode = %v, want %v", state.UninstallMode, model.UninstallModePartial)
	}
}

func TestUninstallModeScreen_FullNavigatesToConfirm(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUninstallMode
	m.Cursor = 1 // Full Uninstall option

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenUninstallConfirm {
		t.Fatalf("screen = %v, want %v", state.Screen, ScreenUninstallConfirm)
	}
	if state.UninstallMode != model.UninstallModeFull {
		t.Fatalf("UninstallMode = %v, want %v", state.UninstallMode, model.UninstallModeFull)
	}
	// Verify all agents and components were populated
	if len(state.UninstallAgents) == 0 {
		t.Fatal("UninstallAgents should be populated for Full mode")
	}
	if len(state.UninstallComponents) == 0 {
		t.Fatal("UninstallComponents should be populated for Full mode")
	}
}

func TestUninstallModeScreen_FullRemoveNavigatesToConfirm(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUninstallMode
	m.Cursor = 2 // Full Uninstall & Remove Binary option

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenUninstallConfirm {
		t.Fatalf("screen = %v, want %v", state.Screen, ScreenUninstallConfirm)
	}
	if state.UninstallMode != model.UninstallModeFullRemove {
		t.Fatalf("UninstallMode = %v, want %v", state.UninstallMode, model.UninstallModeFullRemove)
	}
	if len(state.UninstallAgents) == 0 {
		t.Fatal("UninstallAgents should be populated for FullRemove mode")
	}
	if len(state.UninstallComponents) == 0 {
		t.Fatal("UninstallComponents should be populated for FullRemove mode")
	}
}

func TestUninstallModeScreen_CleanInstallNavigatesToConfirm(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUninstallMode
	m.Cursor = 3 // Full Uninstall + Clean Install option

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenUninstallConfirm {
		t.Fatalf("screen = %v, want %v", state.Screen, ScreenUninstallConfirm)
	}
	if state.UninstallMode != model.UninstallModeCleanInstall {
		t.Fatalf("UninstallMode = %v, want %v", state.UninstallMode, model.UninstallModeCleanInstall)
	}
	if len(state.UninstallAgents) == 0 {
		t.Fatal("UninstallAgents should be populated for CleanInstall mode")
	}
	if len(state.UninstallComponents) == 0 {
		t.Fatal("UninstallComponents should be populated for CleanInstall mode")
	}
}

func TestUninstallComponents_ContinueNavigatesToConfirm(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUninstallComponents
	m.UninstallMode = model.UninstallModePartial
	m.UninstallComponents = []model.ComponentID{model.ComponentSDD}
	m.Cursor = len(screens.UninstallComponentOptions())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenUninstallConfirm {
		t.Fatalf("screen = %v, want %v", state.Screen, ScreenUninstallConfirm)
	}
}

func TestUninstallProfiles_ContinueNavigatesToConfirm(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUninstallProfiles
	m.UninstallProfilesAvailable = []string{"cheap"}
	m.UninstallProfilesToRemove = []string{"cheap"}
	m.Cursor = len(m.UninstallProfilesAvailable)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenUninstallConfirm {
		t.Fatalf("screen = %v, want %v", state.Screen, ScreenUninstallConfirm)
	}
}

func TestUninstallConfirm_CancelCleanInstallReturnsToModeSelection(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUninstallConfirm
	m.UninstallMode = model.UninstallModeCleanInstall
	m.Cursor = 1 // Cancel

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenUninstallMode {
		t.Fatalf("screen = %v, want %v", state.Screen, ScreenUninstallMode)
	}
}

func TestCompleteViewShowsPipelineManualActions(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenComplete
	m.Execution.ManualActions = []string{"Pi CodeGraph child drifted; preserved: /tmp/worker.md"}
	if out := m.View(); !strings.Contains(out, "Manual actions required") || !strings.Contains(out, "child drifted") {
		t.Fatalf("completion output missing Pi manual action: %q", out)
	}
}

func TestOptionCount_UninstallModeMatchesRenderedOptions(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUninstallMode

	got := m.optionCount()
	want := len(screens.UninstallModeOptions()) + 1
	if got != want {
		t.Fatalf("optionCount() = %d, want %d", got, want)
	}
}

func TestUninstallResult_EnterReturnsToWelcome(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUninstallResult
	m.UninstallErr = nil

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenWelcome {
		t.Fatalf("screen = %v, want %v", state.Screen, ScreenWelcome)
	}
	if state.UninstallErr != nil {
		t.Fatalf("UninstallErr should be reset to nil: %v", state.UninstallErr)
	}
}

// ─── T19: Model config navigation ─────────────────────────────────────────

// TestModelConfig_ClaudePickerNavigation verifies that selecting cursor 0 from
// ScreenModelConfig transitions to ScreenClaudeModelPicker with ModelConfigMode set.
func TestModelConfig_ClaudePickerNavigation(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenModelConfig
	m.Cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenClaudeModelPicker {
		t.Fatalf("ModelConfig cursor=0 (Claude): screen = %v, want %v", state.Screen, ScreenClaudeModelPicker)
	}
	if !state.ModelConfigMode {
		t.Fatalf("ModelConfigMode should be true after entering Claude picker from ModelConfig")
	}
}

// TestModelConfig_OpenCodePickerNavigation verifies that selecting cursor 1
// from ScreenModelConfig transitions to ScreenModelPicker with ModelConfigMode set.

// TestModelConfig_BackNavigation verifies that selecting cursor 2 (Back) from
// ScreenModelConfig returns to ScreenWelcome.
func TestModelConfig_BackNavigation(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenModelConfig
	m.Cursor = 2 // Back is the last option

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenWelcome {
		t.Fatalf("ModelConfig cursor=2 (Back): screen = %v, want %v", state.Screen, ScreenWelcome)
	}
}

// TestModelConfig_EscReturnsToWelcome verifies that pressing Esc from
// ScreenModelConfig navigates back to ScreenWelcome.
func TestModelConfig_EscReturnsToWelcome(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenModelConfig

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	state := updated.(Model)

	if state.Screen != ScreenWelcome {
		t.Fatalf("ModelConfig esc: screen = %v, want %v", state.Screen, ScreenWelcome)
	}
}

// TestModelConfig_ClaudePickerBackReturnsToModelConfig verifies that pressing
// Esc from ScreenClaudeModelPicker when in ModelConfigMode returns to
// ScreenModelConfig (not the install flow).
func TestModelConfig_ClaudePickerBackReturnsToModelConfig(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenClaudeModelPicker
	m.ModelConfigMode = true
	m.ClaudeModelPicker = screens.NewClaudeModelPickerState()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	state := updated.(Model)

	if state.Screen != ScreenModelConfig {
		t.Fatalf("ClaudeModelPicker esc (ModelConfigMode): screen = %v, want %v", state.Screen, ScreenModelConfig)
	}
}

// TestModelConfig_KiroPickerBackReturnsToModelConfig verifies that pressing
// Esc from ScreenKiroModelPicker when in ModelConfigMode returns to ScreenModelConfig.

// TestCodexPickerBackRowEnterNavigates verifies that pressing Enter on the
// Codex picker "← Back" row actually navigates (regression: the back row used
// to be swallowed because HandleCodexModelPickerNav returned (true, nil) and
// model.go only navigates when assignments are non-nil). With Claude in the
// flow, Back must return to the Claude picker.
func TestCodexPickerBackRowEnterNavigates(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenCodexModelPicker
	m.ModelConfigMode = false
	m.Selection.Preset = model.PresetFullGentleman // non-custom
	m.Selection.Agents = []model.AgentID{model.AgentCodex, model.AgentClaudeCode}
	m.Selection.Components = []model.ComponentID{model.ComponentSDD}
	m.CodexModelPicker = screens.NewCodexModelPickerState()
	// Cursor on the "← Back" row.
	m.Cursor = screens.CodexModelPickerOptionCount(m.CodexModelPicker) - 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenClaudeModelPicker {
		t.Fatalf("CodexModelPicker enter on Back (Claude in flow): screen = %v, want %v",
			state.Screen, ScreenClaudeModelPicker)
	}
}

// TestSDDModeBackReturnsToCodexPicker verifies that going back from the OpenCode
// SDDMode screen returns to the Codex picker when Codex is in the flow
// (regression: SDDMode back skipped Codex and jumped straight to Claude).
// Forward order is Claude → Kiro → Codex → SDDMode, so back must hit Codex first.

// TestSDDModeEscReturnsToCodexPicker verifies the Esc path (goBack) is consistent
// with the Enter-on-Back path: it must also return to Codex when in the flow.

// TestPresetConfirmEntersFirstPickerInFlow verifies that confirming a preset on
// ScreenPreset enters the FIRST picker of the conditional chain and initializes
// its state — covering the Kiro-first and Codex-first entry paths (no Claude),
// which the previous round-trip cases only exercised with Claude first. This is
// the safety net for collapsing the ScreenPreset confirm ladder onto
// pickerNextScreen + applyPickerEntry.

func TestPresetConfirmCustomEntersDependencyTreeComponentPicker(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenPreset
	m.InstallFlowActive = true
	m.Cursor = presetCursor(t, model.PresetCustom)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenDependencyTree {
		t.Fatalf("screen = %v, want %v", state.Screen, ScreenDependencyTree)
	}
	if state.Cursor != 0 {
		t.Fatalf("cursor = %d, want 0", state.Cursor)
	}
	if state.Selection.Preset != model.PresetCustom {
		t.Fatalf("preset = %q, want %q", state.Selection.Preset, model.PresetCustom)
	}
	if len(state.Selection.Components) != 0 {
		t.Fatalf("components = %v, want empty custom selection", state.Selection.Components)
	}
}

// TestKiroPickerEscNonCustomWithClaudeGoesToClaudePicker verifies that Esc from
// ScreenKiroModelPicker in a non-custom preset returns to ScreenClaudeModelPicker
// when Claude is in the flow — keeping Esc consistent with Enter on "← Back".

// TestKiroPickerEscNonCustomWithoutClaudeGoesToPreset verifies that Esc from
// ScreenKiroModelPicker in a non-custom preset returns to ScreenPreset when
// Claude is NOT in the flow.

// TestModelConfig_OpenCodePickerBackReturnsToModelConfig verifies that pressing
// Esc from ScreenModelPicker when in ModelConfigMode returns to ScreenModelConfig.

// ─── Detection-default consumer regression tests ───────────────────────────

// makeDetectionWithAgents builds a DetectionResult with the specified agents
// marked as Exists=true. All other agents are absent.
func makeDetectionWithAgents(present ...string) system.DetectionResult {
	known := []string{"claude-code", "opencode", "cursor", "codex", "antigravity", "pi"}
	presentSet := make(map[string]bool, len(present))
	for _, p := range present {
		presentSet[p] = true
	}
	var configs []system.ConfigState
	for _, agent := range known {
		configs = append(configs, system.ConfigState{
			Agent:       agent,
			Path:        "/tmp/fake/" + agent,
			Exists:      presentSet[agent],
			IsDirectory: presentSet[agent],
		})
	}
	return system.DetectionResult{Configs: configs}
}

// ─── T_BACKUP_SCROLL: Backup scroll and new key navigation tests ──────────────

// makeBackupList creates a list of dummy backup manifests for testing.
func makeBackupList(count int) []backup.Manifest {
	manifests := make([]backup.Manifest, count)
	for i := range manifests {
		manifests[i] = backup.Manifest{
			ID:      fmt.Sprintf("backup-%02d", i),
			RootDir: fmt.Sprintf("/tmp/backups/backup-%02d", i),
			Source:  backup.BackupSourceInstall,
		}
	}
	return manifests
}

// TestBackupScroll_CursorDown verifies that scrolling down adjusts BackupScroll.
func TestBackupScroll_CursorDown(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenBackups
	m.Backups = makeBackupList(15)
	m.Cursor = 0
	m.BackupScroll = 0

	// Navigate down 10 times to go past BackupMaxVisible (10).
	for i := 0; i < 10; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = updated.(Model)
	}

	// After 10 downs, cursor is at 10. BackupScroll should have moved to keep cursor visible.
	if m.Cursor != 10 {
		t.Fatalf("cursor = %d, want 10", m.Cursor)
	}
	if m.BackupScroll < 1 {
		t.Errorf("BackupScroll = %d, want >= 1 (cursor at 10 needs scroll adjustment)", m.BackupScroll)
	}
}

// TestBackupScroll_CursorUp verifies that scrolling up adjusts BackupScroll.
func TestBackupScroll_CursorUp(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenBackups
	m.Backups = makeBackupList(15)
	m.Cursor = 12
	m.BackupScroll = 5

	// Navigate up — cursor should go down, scroll should follow.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = updated.(Model)

	if m.Cursor != 11 {
		t.Fatalf("cursor = %d, want 11", m.Cursor)
	}

	// Navigate up until cursor goes below BackupScroll.
	m.Cursor = 5
	m.BackupScroll = 5
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = updated.(Model)

	if m.Cursor != 4 {
		t.Fatalf("cursor = %d, want 4", m.Cursor)
	}
	// BackupScroll should have decreased to keep cursor visible.
	if m.BackupScroll > m.Cursor {
		t.Errorf("BackupScroll = %d should be <= cursor %d after scrolling up", m.BackupScroll, m.Cursor)
	}
}

// TestBackup_DeleteKeyNavigation verifies that pressing 'd' on a backup
// navigates to ScreenDeleteConfirm and sets SelectedBackup.
func TestBackup_DeleteKeyNavigation(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenBackups
	m.Backups = makeBackupList(3)
	m.Cursor = 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	state := updated.(Model)

	if state.Screen != ScreenDeleteConfirm {
		t.Fatalf("screen = %v, want ScreenDeleteConfirm", state.Screen)
	}
	if state.SelectedBackup.ID != "backup-01" {
		t.Fatalf("SelectedBackup.ID = %q, want %q", state.SelectedBackup.ID, "backup-01")
	}
}

// TestBackup_DeleteKeyOnBackItemIgnored verifies that pressing 'd' when cursor
// is on the "Back" item does nothing (no navigation to delete screen).
func TestBackup_DeleteKeyOnBackItemIgnored(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenBackups
	m.Backups = makeBackupList(3)
	m.Cursor = 3 // cursor on "Back" item (index = len(backups))

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	state := updated.(Model)

	if state.Screen != ScreenBackups {
		t.Fatalf("screen = %v, want ScreenBackups (d on Back item should do nothing)", state.Screen)
	}
}

// TestBackup_RenameKeyNavigation verifies that pressing 'r' on a backup
// navigates to ScreenRenameBackup and populates the rename text buffer.
func TestBackup_RenameKeyNavigation(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenBackups
	backups := makeBackupList(3)
	backups[0].Description = "my description"
	m.Backups = backups
	m.Cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	state := updated.(Model)

	if state.Screen != ScreenRenameBackup {
		t.Fatalf("screen = %v, want ScreenRenameBackup", state.Screen)
	}
	if state.BackupRenameText != "my description" {
		t.Fatalf("BackupRenameText = %q, want %q", state.BackupRenameText, "my description")
	}
	if state.BackupRenamePos != len([]rune("my description")) {
		t.Fatalf("BackupRenamePos = %d, want %d", state.BackupRenamePos, len("my description"))
	}
}

// TestRenameInput_TypeAndSubmit verifies that typing characters and pressing
// Enter in the rename screen calls RenameBackupFn and returns to ScreenBackups.
func TestRenameInput_TypeAndSubmit(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenRenameBackup
	m.SelectedBackup = backup.Manifest{
		ID:      "backup-00",
		RootDir: "/tmp/backup-00",
	}
	m.BackupRenameText = "old"
	m.BackupRenamePos = 3

	renameCalled := false
	var renameArg string
	m.RenameBackupFn = func(manifest backup.Manifest, newDesc string) error {
		renameCalled = true
		renameArg = newDesc
		return nil
	}
	refreshCalled := false
	m.ListBackupsFn = func() []backup.Manifest {
		refreshCalled = true
		return makeBackupList(1)
	}

	// Type " text" then press Enter.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" text")})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if !renameCalled {
		t.Fatalf("RenameBackupFn was not called")
	}
	if renameArg != "old text" {
		t.Fatalf("RenameBackupFn called with %q, want %q", renameArg, "old text")
	}
	if !refreshCalled {
		t.Fatalf("ListBackupsFn was not called after rename")
	}
	if state.Screen != ScreenBackups {
		t.Fatalf("screen = %v, want ScreenBackups after rename", state.Screen)
	}
}

// TestRenameInput_Escape verifies that pressing Esc in the rename screen
// cancels without calling RenameBackupFn and returns to ScreenBackups.
func TestRenameInput_Escape(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenRenameBackup
	m.SelectedBackup = backup.Manifest{ID: "backup-00"}
	m.BackupRenameText = "something"
	m.BackupRenamePos = 9

	renameCalled := false
	m.RenameBackupFn = func(manifest backup.Manifest, newDesc string) error {
		renameCalled = true
		return nil
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	state := updated.(Model)

	if renameCalled {
		t.Fatalf("RenameBackupFn should NOT be called on Esc")
	}
	if state.Screen != ScreenBackups {
		t.Fatalf("screen = %v, want ScreenBackups after Esc", state.Screen)
	}
}

// TestDeleteConfirm_DeleteOption verifies that pressing Enter on "Delete"
// calls DeleteBackupFn and navigates to ScreenDeleteResult.
func TestDeleteConfirm_DeleteOption(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenDeleteConfirm
	m.SelectedBackup = backup.Manifest{
		ID:      "backup-00",
		RootDir: "/tmp/backup-00",
	}
	m.Cursor = 0 // "Delete"

	deleteCalled := false
	m.DeleteBackupFn = func(manifest backup.Manifest) error {
		deleteCalled = true
		return nil
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if !deleteCalled {
		t.Fatalf("DeleteBackupFn was not called")
	}
	if state.Screen != ScreenDeleteResult {
		t.Fatalf("screen = %v, want ScreenDeleteResult", state.Screen)
	}
	if state.DeleteErr != nil {
		t.Fatalf("unexpected DeleteErr: %v", state.DeleteErr)
	}
}

// TestDeleteResult_EnterRefreshesAndReturnsToBackups verifies that pressing Enter
// on ScreenDeleteResult refreshes the backup list and returns to ScreenBackups.
func TestDeleteResult_EnterRefreshesAndReturnsToBackups(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenDeleteResult
	m.DeleteErr = nil

	refreshCalled := false
	m.ListBackupsFn = func() []backup.Manifest {
		refreshCalled = true
		return makeBackupList(2)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if !refreshCalled {
		t.Fatalf("ListBackupsFn was not called after delete result")
	}
	if state.Screen != ScreenBackups {
		t.Fatalf("screen = %v, want ScreenBackups", state.Screen)
	}
	if state.DeleteErr != nil {
		t.Fatalf("DeleteErr should be reset to nil: %v", state.DeleteErr)
	}
}

// TestPreselectedAgents_CodexIsIncludedWhenPresent is a regression guard:
// when the codex config dir is detected, preselectedAgents must include
// model.AgentCodex. Previously the switch statement omitted codex, so
// detection-driven TUI preselection silently dropped it.
func TestPreselectedAgents_CodexIsIncludedWhenPresent(t *testing.T) {
	detection := makeDetectionWithAgents("codex")
	selected := preselectedAgents(detection, state.InstallState{})

	found := false
	for _, id := range selected {
		if id == model.AgentCodex {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("preselectedAgents() did not include codex even though config dir is present; got %v", selected)
	}
}

// ─── T20: Model config → sync persistence (PendingSyncOverrides) ───────────

// TestModelConfig_ClaudePickerTriggersSyncScreen verifies the full path from
// ScreenModelConfig → ClaudeModelPicker (ModelConfigMode) → selecting a preset
// → ScreenSync with PendingSyncOverrides populated.
func TestModelConfig_ClaudePickerTriggersSyncScreen(t *testing.T) {
	// Step 1: from ScreenModelConfig, cursor=0 → goes to ClaudeModelPicker with ModelConfigMode=true.
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenModelConfig
	m.Cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenClaudeModelPicker {
		t.Fatalf("step1: screen = %v, want ScreenClaudeModelPicker", state.Screen)
	}
	if !state.ModelConfigMode {
		t.Fatalf("step1: ModelConfigMode should be true after entering Claude picker from ModelConfig")
	}

	// Step 2: from ClaudeModelPicker (ModelConfigMode=true), cursor=0 (balanced preset), enter
	// → should navigate to ScreenSync (NOT ScreenModelConfig) with PendingSyncOverrides set.
	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state = updated.(Model)

	if state.Screen != ScreenSync {
		t.Fatalf("step2: screen = %v, want ScreenSync (ModelConfigMode should redirect to sync)", state.Screen)
	}
	if state.ModelConfigMode {
		t.Fatalf("step2: ModelConfigMode should be cleared after routing to ScreenSync")
	}
	if state.PendingSyncOverrides == nil {
		t.Fatalf("step2: PendingSyncOverrides should be non-nil after Claude model selection")
	}
	if got := state.PendingSyncOverrides.TargetAgents; len(got) != 1 || got[0] != model.AgentClaudeCode {
		t.Fatalf("step2: TargetAgents = %v, want [%s]", got, model.AgentClaudeCode)
	}
	if len(state.PendingSyncOverrides.ClaudeModelAssignments) == 0 {
		t.Fatalf("step2: PendingSyncOverrides.ClaudeModelAssignments should be non-empty, got: %v",
			state.PendingSyncOverrides.ClaudeModelAssignments)
	}
	// Orchestrator is present in the balanced preset (injected as part of the model
	// assignment table). The Claude picker shows sub-agents and default; orchestrator
	// is carried through for injection but is not user-editable in the picker UI.
	if got := state.PendingSyncOverrides.ClaudeModelAssignments["orchestrator"]; got != model.ClaudeModelOpus {
		t.Errorf("step2: orchestrator = %q, want %q", got, model.ClaudeModelOpus)
	}
}

// TestModelConfig_OpenCodePickerContinueTriggersSyncScreen verifies that pressing
// "Continue" from ScreenModelPicker while in ModelConfigMode navigates to ScreenSync
// and populates PendingSyncOverrides with ModelAssignments and SDDMode=multi.

// TestModelConfig_SyncPassesOverridesToSyncFn verifies that when ScreenSync is
// entered with PendingSyncOverrides set, pressing enter launches the sync and the
// SyncFn receives the pending overrides (not nil).
func TestModelConfig_SyncPassesOverridesToSyncFn(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenSync

	testOverrides := &model.SyncOverrides{
		ClaudeModelAssignments: map[string]model.ClaudeModelAlias{
			"default": model.ClaudeModelSonnet,
		},
	}
	m.PendingSyncOverrides = testOverrides

	var capturedOverrides *model.SyncOverrides
	m.SyncFn = func(overrides *model.SyncOverrides) ([]string, error) {
		capturedOverrides = overrides
		return []string{"a", "b", "c"}, nil
	}

	// Press enter on ScreenSync to start the sync.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if !state.OperationRunning {
		t.Fatalf("OperationRunning should be true after triggering sync")
	}
	if state.OperationMode != "sync" {
		t.Fatalf("OperationMode = %q, want %q", state.OperationMode, "sync")
	}

	// Execute the returned command batch to find and run the sync cmd.
	// tea.Batch returns a tea.BatchMsg ([]tea.Cmd) — iterate to find the sync cmd.
	if cmd == nil {
		t.Fatalf("expected a non-nil cmd after triggering sync from ScreenSync")
	}

	syncMsg := findSyncDoneMsgInBatch(t, cmd)
	if syncMsg == nil {
		t.Fatalf("expected SyncDoneMsg from batch cmd, got nil")
	}
	if syncMsg.Err != nil {
		t.Fatalf("unexpected sync error: %v", syncMsg.Err)
	}
	if len(syncMsg.Files) != 3 {
		t.Fatalf("Files len = %d, want 3", len(syncMsg.Files))
	}

	if capturedOverrides == nil {
		t.Fatalf("SyncFn was not called with overrides — capturedOverrides is nil")
	}
	if got := capturedOverrides.ClaudeModelAssignments["default"]; got != model.ClaudeModelSonnet {
		t.Errorf("captured ClaudeModelAssignments[default] = %q, want %q", got, model.ClaudeModelSonnet)
	}

	// Feed SyncDoneMsg back through Update to verify end-to-end state cleanup.
	updated2, _ := state.Update(*syncMsg)
	final := updated2.(Model)
	if final.PendingSyncOverrides != nil {
		t.Errorf("PendingSyncOverrides should be nil after SyncDoneMsg, got %+v", final.PendingSyncOverrides)
	}
	if !final.HasSyncRun {
		t.Errorf("HasSyncRun should be true after SyncDoneMsg")
	}
	if final.OperationRunning {
		t.Errorf("OperationRunning should be false after SyncDoneMsg")
	}
}

// findUninstallDoneMsgInBatch executes all commands in a tea.Cmd (including BatchMsg)
// and returns the first UninstallDoneMsg found, or nil if none is produced.
func findUninstallDoneMsgInBatch(t *testing.T, cmd tea.Cmd) *UninstallDoneMsg {
	t.Helper()
	if cmd == nil {
		return nil
	}

	msg := cmd()

	if uninstallMsg, ok := msg.(UninstallDoneMsg); ok {
		return &uninstallMsg
	}

	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, innerCmd := range batch {
			if innerCmd == nil {
				continue
			}
			innerMsg := innerCmd()
			if uninstallMsg, ok := innerMsg.(UninstallDoneMsg); ok {
				return &uninstallMsg
			}
		}
	}

	return nil
}

// findSyncDoneMsgInBatch executes all commands in a tea.Cmd (including BatchMsg)
// and returns the first SyncDoneMsg found, or nil if none is produced.
func findSyncDoneMsgInBatch(t *testing.T, cmd tea.Cmd) *SyncDoneMsg {
	t.Helper()
	if cmd == nil {
		return nil
	}

	msg := cmd()

	// Direct SyncDoneMsg (non-batch case).
	if syncMsg, ok := msg.(SyncDoneMsg); ok {
		return &syncMsg
	}

	// tea.Batch returns tea.BatchMsg which is []tea.Cmd.
	// Execute each inner cmd and look for a SyncDoneMsg.
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, innerCmd := range batch {
			if innerCmd == nil {
				continue
			}
			innerMsg := innerCmd()
			if syncMsg, ok := innerMsg.(SyncDoneMsg); ok {
				return &syncMsg
			}
		}
	}

	return nil
}

// TestSyncDoneMsg_ClearsPendingOverrides verifies that receiving SyncDoneMsg
// clears PendingSyncOverrides regardless of the sync outcome.
func TestSyncDoneMsg_ClearsPendingOverrides(t *testing.T) {
	tests := []struct {
		name     string
		syncDone SyncDoneMsg
	}{
		{
			name:     "success clears overrides",
			syncDone: SyncDoneMsg{Files: []string{"a", "b", "c", "d", "e"}, Err: nil},
		},
		{
			name:     "error also clears overrides",
			syncDone: SyncDoneMsg{Files: nil, Err: fmt.Errorf("sync failed")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(system.DetectionResult{}, "dev")
			m.Screen = ScreenSync
			m.OperationRunning = true
			m.PendingSyncOverrides = &model.SyncOverrides{
				ClaudeModelAssignments: map[string]model.ClaudeModelAlias{
					"orchestrator": model.ClaudeModelOpus,
				},
			}

			updated, _ := m.Update(tt.syncDone)
			state := updated.(Model)

			if state.PendingSyncOverrides != nil {
				t.Errorf("PendingSyncOverrides should be nil after SyncDoneMsg, got: %+v",
					state.PendingSyncOverrides)
			}
			if state.OperationRunning {
				t.Errorf("OperationRunning should be false after SyncDoneMsg")
			}
		})
	}
}

// TestSyncDoneMsg_CursorClampedAfterProfileListRefresh verifies that when
// SyncDoneMsg causes the ProfileList to shrink, the cursor is clamped so it
// never points past the end of the new list.
func TestSyncDoneMsg_CursorClampedAfterProfileListRefresh(t *testing.T) {
	// Override readProfilesFn to return a shorter list.
	orig := readProfilesFn
	readProfilesFn = func(_ string) ([]model.Profile, error) {
		return []model.Profile{
			{Name: "cheap"},
			{Name: "premium"},
		}, nil
	}
	t.Cleanup(func() { readProfilesFn = orig })

	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenProfiles
	m.OperationRunning = true
	// Cursor was at 5 (pointing at a profile that no longer exists after sync).
	m.Cursor = 5

	updated, _ := m.Update(SyncDoneMsg{Files: []string{"a"}, Err: nil})
	state := updated.(Model)

	// After refresh, ProfileList has 2 items; cursor must be clamped to 1 (len-1).
	if state.Cursor >= len(state.ProfileList) {
		t.Fatalf("Cursor = %d is out of bounds (ProfileList len = %d); expected cursor to be clamped",
			state.Cursor, len(state.ProfileList))
	}
	if state.Cursor != len(state.ProfileList)-1 {
		t.Errorf("Cursor = %d, want %d (clamped to last profile index)",
			state.Cursor, len(state.ProfileList)-1)
	}
}

// TestSyncDoneMsg_ClearsPendingOverrides_WithReadProfilesStub is an extended
// version of TestSyncDoneMsg_ClearsPendingOverrides that also injects a
// readProfilesFn stub so the test does not depend on the filesystem.
func TestSyncDoneMsg_ClearsPendingOverrides_WithReadProfilesStub(t *testing.T) {
	stubProfiles := []model.Profile{{Name: "cheap"}, {Name: "premium"}}

	orig := readProfilesFn
	readProfilesFn = func(_ string) ([]model.Profile, error) {
		return stubProfiles, nil
	}
	t.Cleanup(func() { readProfilesFn = orig })

	tests := []struct {
		name     string
		syncDone SyncDoneMsg
	}{
		{
			name:     "success clears overrides",
			syncDone: SyncDoneMsg{Files: []string{"a", "b", "c", "d", "e"}, Err: nil},
		},
		{
			name:     "error also clears overrides",
			syncDone: SyncDoneMsg{Files: nil, Err: fmt.Errorf("sync failed")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(system.DetectionResult{}, "dev")
			m.Screen = ScreenSync
			m.OperationRunning = true
			m.PendingSyncOverrides = &model.SyncOverrides{
				ClaudeModelAssignments: map[string]model.ClaudeModelAlias{
					"orchestrator": model.ClaudeModelOpus,
				},
			}

			updated, _ := m.Update(tt.syncDone)
			state := updated.(Model)

			if state.PendingSyncOverrides != nil {
				t.Errorf("PendingSyncOverrides should be nil after SyncDoneMsg, got: %+v",
					state.PendingSyncOverrides)
			}
			if state.OperationRunning {
				t.Errorf("OperationRunning should be false after SyncDoneMsg")
			}
			// Verify profiles were refreshed from stub.
			if len(state.ProfileList) != len(stubProfiles) {
				t.Errorf("ProfileList len = %d, want %d (from stub)", len(state.ProfileList), len(stubProfiles))
			}
		})
	}
}

// TestModelConfig_EscFromPickersReturnsToModelConfig verifies that pressing Esc
// from either model picker in ModelConfigMode returns to ScreenModelConfig (the
// cancel path is not redirected to ScreenSync).
func TestModelConfig_EscFromPickersReturnsToModelConfig(t *testing.T) {
	tests := []struct {
		name   string
		screen Screen
		setup  func(m *Model)
	}{
		{
			name:   "Esc from ClaudeModelPicker in ModelConfigMode → ScreenModelConfig",
			screen: ScreenClaudeModelPicker,
			setup: func(m *Model) {
				m.ModelConfigMode = true
				m.ClaudeModelPicker = screens.NewClaudeModelPickerState()
			},
		},
		{
			name:   "Esc from ModelPicker in ModelConfigMode → ScreenModelConfig",
			screen: ScreenModelPicker,
			setup: func(m *Model) {
				m.ModelConfigMode = true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(system.DetectionResult{}, "dev")
			m.Screen = tt.screen
			tt.setup(&m)

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			state := updated.(Model)

			if state.Screen != ScreenModelConfig {
				t.Fatalf("esc from %v (ModelConfigMode): screen = %v, want ScreenModelConfig",
					tt.screen, state.Screen)
			}
			// Verify PendingSyncOverrides is NOT set by the cancel path.
			if state.PendingSyncOverrides != nil {
				t.Errorf("PendingSyncOverrides should remain nil after esc cancel, got: %+v",
					state.PendingSyncOverrides)
			}
		})
	}
}

// TestPreselectedAgents_AllKnownAgentsMappedCorrectly verifies every canonical
// agent string maps to its model.AgentID constant in preselectedAgents.
// This prevents silent drops when new agents are added to ScanConfigs without
// updating the TUI switch statement.

// ─── agentsToManage / preselectedAgents — state wins over detection ─────────

// TestAgentsToManage_StateTakesPriorityOverDetection verifies the core contract:
// when state.json is populated, it overrides filesystem detection for TUI pre-selection.

// TestPreselectedAgents_StateWinsOverDetection verifies that when a populated
// InstallState is passed to preselectedAgents, it returns only the persisted
// agents — not all detected config dirs.

// TestNewModel_StateAgentsArePreselected verifies that NewModel uses the
// supplied InstallState for pre-selection instead of detection.
func TestNewModel_StateAgentsArePreselected(t *testing.T) {
	// Filesystem: 3 agents detected.
	detection := makeDetectionWithAgents("claude-code", "cursor", "codex")

	// state.json: only 1 agent.
	installState := state.InstallState{
		InstalledAgents: []string{string(model.AgentCursor)},
	}

	m := NewModel(detection, "dev", installState)

	if len(m.Selection.Agents) != 1 {
		t.Fatalf("NewModel Selection.Agents = %v, want [%s]", m.Selection.Agents, model.AgentCursor)
	}
	if m.Selection.Agents[0] != model.AgentCursor {
		t.Errorf("Selection.Agents[0] = %q, want %q", m.Selection.Agents[0], model.AgentCursor)
	}
}

// ─── Task 4: StrictTDD screen navigation ────────────────────────────────────

// helper: returns cursor index for SDDModeSingle in SDDModeOptions.
func sddSingleCursor(t *testing.T) int {
	t.Helper()
	for i, opt := range screens.SDDModeOptions() {
		if opt == model.SDDModeSingle {
			return i
		}
	}
	t.Fatal("SDDModeSingle not found in SDDModeOptions()")
	return -1
}

// TestStrictTDDScreenAppearsAfterSDDMode verifies that from ScreenSDDMode,
// selecting single mode navigates to ScreenStrictTDD (not ScreenDependencyTree)
// when the SDD component and OpenCode agent are selected.

// TestStrictTDDScreenEnableSetsSelection verifies that selecting "Enable" on
// ScreenStrictTDD sets m.Selection.StrictTDD = true.

// TestStrictTDDScreenDisableSetsSelection verifies that selecting "Disable" on
// ScreenStrictTDD sets m.Selection.StrictTDD = false.

// TestStrictTDDScreenSkippedWhenNoSDD verifies that when the SDD component is
// NOT selected, the ScreenStrictTDD is not used in the navigation path.
// From ScreenSDDMode with single selection → should go directly to
// ScreenDependencyTree when SDD is not in components.
//
// NOTE: shouldShowSDDModeScreen() requires ComponentSDD, so in practice the
// SDDMode screen itself would not show when there is no SDD. This test
// validates that ScreenStrictTDD is never reached without SDD.

// TestStrictTDDBackNavigatesToSDDMode verifies that pressing Escape on
// ScreenStrictTDD returns to ScreenSDDMode.

// ─── Bug fixes: Enter-Back navigation must be consistent with ESC ────────────

// TestModelPickerEnterBackNavigatesToSDDMode verifies that pressing Enter on
// the "Back" option of ScreenModelPicker navigates to ScreenSDDMode (NOT
// StrictTDD). ModelPicker sits between SDDMode and StrictTDD in the forward
// flow: SDDMode → ModelPicker → StrictTDD. Back must go to SDDMode to avoid
// a loop between ModelPicker ↔ StrictTDD.

// TestModelPickerContinueMultiGoesToStrictTDD verifies that pressing Continue
// on ModelPicker (non-custom preset, multi mode) navigates to ScreenStrictTDD
// before going to DependencyTree. Previously it went directly to DependencyTree.

// ─── Bug fix: StrictTDD must appear for ANY agent when SDD is selected ───────

// TestStrictTDDScreenAppearsForClaudeCodeAgent verifies that when ClaudeCode
// (NOT OpenCode) is selected with SDD component, the flow goes to ScreenStrictTDD
// after the ClaudeModelPicker "confirmed" path instead of directly to DependencyTree.
// RED: currently fails because shouldShowStrictTDDScreen checks for AgentOpenCode.

// TestStrictTDDScreenAppearsForCursorAgent verifies that when Cursor agent
// (neither OpenCode nor ClaudeCode) is selected with SDD, the ScreenPreset flow
// goes to ScreenStrictTDD instead of ScreenDependencyTree.
// RED: currently fails because shouldShowStrictTDDScreen checks for AgentOpenCode.

// TestStrictTDDBackNavFromClaudeFlow verifies that pressing ESC on ScreenStrictTDD
// when ClaudeCode agent (no OpenCode) is selected goes back to ScreenClaudeModelPicker,
// not ScreenSDDMode (which is OpenCode-only).
// RED: currently fails because goBack() for ScreenStrictTDD always goes to SDDMode.

// TestStrictTDDBackNavFromPresetFlow verifies that pressing ESC on ScreenStrictTDD
// when only a non-OpenCode, non-Claude agent (e.g. Cursor) is selected goes back
// to ScreenPreset, not ScreenSDDMode.
// RED: currently fails because goBack() for ScreenStrictTDD always goes to SDDMode.

// ─── Custom preset StrictTDD navigation gaps ────────────────────────────────

// TestCustomPresetStrictTDDAppearsAfterComponentSelection verifies that in the
// custom preset flow, pressing Continue on DependencyTree (component selector)
// when SDD is selected but no OpenCode and no ClaudeCode agent goes to
// ScreenStrictTDD (not directly to SkillPicker or Review).
// RED: currently fails because the custom DependencyTree Continue has no StrictTDD check.
func TestCustomPresetStrictTDDAppearsAfterComponentSelection(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenDependencyTree
	m.Selection.Preset = model.PresetCustom
	// Cursor agent: no SDDMode, no ClaudeModelPicker.
	m.Selection.Agents = []model.AgentID{model.AgentCursor}
	// Select SDD component (and Skills so skill picker would show, but StrictTDD must come first).
	m.Selection.Components = []model.ComponentID{model.ComponentSDD, model.ComponentSkills}
	// cursor == len(allComps) → "Continue"
	allComps := screens.AllComponents()
	m.Cursor = len(allComps)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenStrictTDD {
		t.Fatalf("screen = %v, want ScreenStrictTDD (custom preset + SDD selected, Continue on DependencyTree)", state.Screen)
	}
}

// TestCustomPresetStrictTDDWithClaudeFlow verifies that in the custom preset,
// when ClaudeCode + SDD is selected, after ClaudeModelPicker confirms assignments,
// the flow goes to ScreenStrictTDD (not directly to SkillPicker or Review).
// RED: currently fails because the ClaudeModelPicker assignment path in custom preset
// goes straight to SkillPicker/Review without a StrictTDD check.

// TestCustomPresetStrictTDDContinueGoesToSkillPickerOrReview verifies that in the
// custom preset, when on ScreenStrictTDD, pressing Enter on the "Enable" option
// goes to ScreenSkillPicker (when Skills is selected) or ScreenReview (when not).
// This verifies Gap 4 — already fixed, this is a regression guard.
func TestCustomPresetStrictTDDContinueGoesToSkillPickerOrReview(t *testing.T) {
	// Case 1: Skills selected → should go to ScreenSkillPicker.
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenStrictTDD
	m.Selection.Preset = model.PresetCustom
	m.Selection.Agents = []model.AgentID{model.AgentCursor}
	m.Selection.Components = []model.ComponentID{model.ComponentSDD, model.ComponentSkills}
	m.Cursor = screens.StrictTDDOptionEnable

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenSkillPicker {
		t.Fatalf("case Skills selected: screen = %v, want ScreenSkillPicker after Enable in custom preset StrictTDD", state.Screen)
	}

	// Case 2: No Skills → should go to ScreenReview.
	m2 := NewModel(system.DetectionResult{}, "dev")
	m2.Screen = ScreenStrictTDD
	m2.Selection.Preset = model.PresetCustom
	m2.Selection.Agents = []model.AgentID{model.AgentCursor}
	m2.Selection.Components = []model.ComponentID{model.ComponentSDD} // no Skills
	m2.Cursor = screens.StrictTDDOptionDisable

	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state2 := updated2.(Model)

	if state2.Screen != ScreenReview {
		t.Fatalf("case no Skills: screen = %v, want ScreenReview after Disable in custom preset StrictTDD", state2.Screen)
	}
}

// TestCustomPresetStrictTDDBackGoesToDependencyTree verifies that in the custom
// preset, pressing ESC on ScreenStrictTDD when no SDDMode and no ClaudeModelPicker
// goes back to ScreenDependencyTree (the component selector).
// RED: currently fails because goBack() from ScreenStrictTDD has no custom-preset handling.

// TestCustomPresetStrictTDDBackGoesToSDDMode verifies that in the custom preset,
// pressing ESC on ScreenStrictTDD when SDDMode was shown (OpenCode + SDD) goes
// back to ScreenSDDMode.
// RED: currently fails because goBack() from ScreenStrictTDD has no custom-preset handling.

// TestCustomPresetSkillPickerBackGoesToStrictTDD verifies that in the custom preset,
// pressing ESC (or Enter on Back) on ScreenSkillPicker when StrictTDD should be shown
// (SDD selected) goes back to ScreenStrictTDD, not directly to SDDMode/DependencyTree.
// RED: currently fails because goBack() from SkillPicker in custom preset has no StrictTDD check.
func TestCustomPresetSkillPickerBackGoesToStrictTDD(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenSkillPicker
	m.Selection.Preset = model.PresetCustom
	// Cursor agent: no SDDMode, no ClaudeModelPicker.
	m.Selection.Agents = []model.AgentID{model.AgentCursor}
	m.Selection.Components = []model.ComponentID{model.ComponentSDD, model.ComponentSkills}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	state := updated.(Model)

	if state.Screen != ScreenStrictTDD {
		t.Fatalf("screen = %v, want ScreenStrictTDD after Esc on SkillPicker (custom preset + SDD)", state.Screen)
	}
}

func TestSkillPickerToggleUsesCanonicalIndex(t *testing.T) {
	all := screens.AllSkillsOrdered()
	for i, skill := range all {
		m := Model{Screen: ScreenSkillPicker, Cursor: i, SkillPicker: append([]model.SkillID(nil), all...)}
		m.toggleCurrentSkill()
		if slices.Contains(m.SkillPicker, skill) {
			t.Errorf("cursor %d did not toggle canonical skill %q", i, skill)
		}
	}
}

// TestCustomPresetReviewBackGoesToStrictTDD verifies that in the custom preset,
// pressing Back on ScreenReview when no Skills and StrictTDD should be shown
// (SDD selected) goes back to ScreenStrictTDD.
// RED: currently fails because Review Back in custom preset has no StrictTDD check.
func TestCustomPresetReviewBackGoesToStrictTDD(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenReview
	m.Selection.Preset = model.PresetCustom
	// Cursor agent: no SDDMode, no ClaudeModelPicker.
	m.Selection.Agents = []model.AgentID{model.AgentCursor}
	// No Skills component → shouldShowSkillPickerScreen() = false.
	// SDD selected → shouldShowStrictTDDScreen() = true.
	m.Selection.Components = []model.ComponentID{model.ComponentSDD}
	// cursor == 1 → "Back" option on ScreenReview.
	m.Cursor = 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenStrictTDD {
		t.Fatalf("screen = %v, want ScreenStrictTDD after Back on Review (custom preset + SDD, no Skills)", state.Screen)
	}
}

// TestCustomReviewBackGoesToStrictTDDNotSDDMode verifies that in the custom preset,
// with OpenCode + SDD (no Skills), pressing Back on ScreenReview goes to ScreenStrictTDD
// and NOT directly to ScreenSDDMode. StrictTDD must come before SDDMode in the back chain.

// TestCustomReviewBackGoesToStrictTDDNotModelPicker verifies that in the custom preset,
// with OpenCode + SDD Multi (no Skills), pressing Back on ScreenReview goes to
// ScreenStrictTDD and not ScreenModelPicker.

// ─── Issue #147: Cursor not reset after ClaudeModelPicker custom mode Back ───

// TestClaudeModelPickerCustomModeEscResetsCursor verifies that after entering
// custom mode and pressing Esc, the cursor is reset to 0.
//
// Closes #147.
func TestClaudeModelPickerCustomModeEscResetsCursor(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenClaudeModelPicker
	// Set custom mode active with cursor at some non-zero position (e.g. 7).
	m.ClaudeModelPicker = screens.NewClaudeModelPickerState()
	m.ClaudeModelPicker.InCustomMode = true
	m.Cursor = 7 // simulate user navigated down in custom phase list

	// Press Esc — should exit custom mode and reset cursor to 0.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	state := updated.(Model)

	// Custom mode must be off.
	if state.ClaudeModelPicker.InCustomMode {
		t.Fatalf("ClaudeModelPicker.InCustomMode = true, want false after Esc")
	}
	// Cursor must be reset to 0 (not remain at 7).
	if state.Cursor != 0 {
		t.Fatalf("Cursor = %d, want 0 after Esc from custom mode (bug: cursor not reset)", state.Cursor)
	}
}

// TestClaudeModelPickerBackRowExitCustomModeResetsCursor verifies that pressing
// Enter on the "Back" row (last option in custom mode list) also resets the cursor.
//
// Closes #147.
func TestClaudeModelPickerBackRowExitCustomModeResetsCursor(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenClaudeModelPicker
	m.ClaudeModelPicker = screens.NewClaudeModelPickerState()
	m.ClaudeModelPicker.InCustomMode = true
	// Back row = len(claudePhases) + 1 = 10 + 1 = 11 (Confirm is +0, Back is +1).
	// However cursor is controlled by m.Cursor (the global model cursor).
	m.Cursor = 9 // in custom mode, simulate cursor at some mid position

	// This test verifies the cursor is 0 after leaving custom mode, regardless of method.
	// Simulate ESC path (same code path as Back row for InCustomMode=false transition).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	state := updated.(Model)

	if state.Cursor != 0 {
		t.Fatalf("Cursor = %d, want 0 after exiting custom mode (bug: cursor not reset)", state.Cursor)
	}
}

// ─── Issue #150: Wrap-around navigation ─────────────────────────────────────

// TestWrapAroundDownAtLast verifies that pressing Down when at the last option
// wraps the cursor to 0.
//
// Closes #150.
func TestWrapAroundDownAtLast(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenPersona

	// optionCount() for ScreenPersona = len(PersonaOptions()) + 1 (Back).
	last := m.optionCount() - 1
	m.Cursor = last

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	state := updated.(Model)

	if state.Cursor != 0 {
		t.Fatalf("Down at last: Cursor = %d, want 0 (wrap-around)", state.Cursor)
	}
}

// TestWrapAroundUpAtFirst verifies that pressing Up when at cursor=0
// wraps the cursor to the last option.
//
// Closes #150.
func TestWrapAroundUpAtFirst(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenPersona
	m.Cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	state := updated.(Model)

	last := m.optionCount() - 1
	if state.Cursor != last {
		t.Fatalf("Up at first: Cursor = %d, want %d (wrap-around)", state.Cursor, last)
	}
}

// TestWrapAroundDownAtLastWithArrowKey verifies wrap also works with arrow Down key.
//
// Closes #150.
func TestWrapAroundDownAtLastWithArrowKey(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenPersona
	last := m.optionCount() - 1
	m.Cursor = last

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	state := updated.(Model)

	if state.Cursor != 0 {
		t.Fatalf("Down(arrow) at last: Cursor = %d, want 0 (wrap-around)", state.Cursor)
	}
}

// TestWrapAroundUpAtFirstWithArrowKey verifies wrap also works with arrow Up key.
//
// Closes #150.
func TestWrapAroundUpAtFirstWithArrowKey(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenPersona
	m.Cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	state := updated.(Model)

	last := m.optionCount() - 1
	if state.Cursor != last {
		t.Fatalf("Up(arrow) at first: Cursor = %d, want %d (wrap-around)", state.Cursor, last)
	}
}

// TestNoWrapAroundOnBackupScreen verifies that wrap-around does NOT happen on
// ScreenBackups (a scrollable screen). Down at last should stay at last.
//
// Closes #150.
func TestNoWrapAroundOnBackupScreen(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenBackups
	m.Backups = makeBackupList(3)
	last := m.optionCount() - 1 // 3 backups + 1 Back = 4
	m.Cursor = last

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	state := updated.(Model)

	// Must NOT wrap on scrollable screen.
	if state.Cursor != last {
		t.Fatalf("ScreenBackups: Down at last: Cursor = %d, want %d (no wrap on scrollable screen)",
			state.Cursor, last)
	}
}

// TestNoWrapAroundUpOnBackupScreen verifies that wrap-around does NOT happen on
// ScreenBackups when Up is pressed at cursor=0.
//
// Closes #150.
func TestNoWrapAroundUpOnBackupScreen(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenBackups
	m.Backups = makeBackupList(3)
	m.Cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	state := updated.(Model)

	// Must NOT wrap on scrollable screen.
	if state.Cursor != 0 {
		t.Fatalf("ScreenBackups: Up at 0: Cursor = %d, want 0 (no wrap on scrollable screen)",
			state.Cursor)
	}
}

// ─── Issue #130: ModelConfig pre-populate model assignments ────────────────

// TestModelConfigOpenCodePrePopulatesAssignments verifies that when the user
// opens the OpenCode model picker from ScreenModelConfig (ModelConfigMode),
// previously saved model assignments are pre-populated into
// m.Selection.ModelAssignments so the picker shows them instead of "(default)".

// TestModelConfigOpenCodeDoesNotOverwriteExistingSessionAssignments verifies that
// if m.Selection.ModelAssignments is already populated (user made changes in the
// current session), we do NOT overwrite them with the file contents.

// TestModelConfigOpenCodeNoPrePopulationWhenFileEmpty verifies that when
// ReadCurrentModelAssignments returns empty map, ModelAssignments stays nil.

// TestCustomSkillPickerBackGoesToStrictTDD verifies that in the custom preset,
// with OpenCode + SDD + Skills, pressing Back on ScreenSkillPicker goes to ScreenStrictTDD
// and NOT directly to ScreenSDDMode. StrictTDD must come before SDDMode in the back chain.

// ─── T_BACKUP_PIN: Pin key tests ───────────────────────────────────────────

// TestPinKeyTogglesPinnedBackup verifies that pressing "p" on a backup item
// calls TogglePinFn with the correct manifest.
func TestPinKeyTogglesPinnedBackup(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenBackups
	m.Backups = makeBackupList(3)
	m.Cursor = 1

	var pinnedManifest backup.Manifest
	m.TogglePinFn = func(manifest backup.Manifest) error {
		pinnedManifest = manifest
		return nil
	}
	m.ListBackupsFn = func() []backup.Manifest {
		return makeBackupList(3)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	state := updated.(Model)

	if pinnedManifest.ID != "backup-01" {
		t.Fatalf("TogglePinFn called with ID %q, want %q", pinnedManifest.ID, "backup-01")
	}
	// Must stay on ScreenBackups (no confirmation screen for pin).
	if state.Screen != ScreenBackups {
		t.Fatalf("screen = %v, want ScreenBackups after pin toggle", state.Screen)
	}
}

// TestPinKeyOnBackOption verifies that pressing "p" when the cursor is on the
// "Back" option does nothing (no TogglePinFn call, screen unchanged).
func TestPinKeyOnBackOption(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenBackups
	m.Backups = makeBackupList(3)
	m.Cursor = 3 // cursor on "Back" item (index == len(backups))

	toggleCalled := false
	m.TogglePinFn = func(manifest backup.Manifest) error {
		toggleCalled = true
		return nil
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	state := updated.(Model)

	if toggleCalled {
		t.Fatalf("TogglePinFn should NOT be called when cursor is on Back item")
	}
	if state.Screen != ScreenBackups {
		t.Fatalf("screen = %v, want ScreenBackups (unchanged)", state.Screen)
	}
}

// TestPinKeyNilFnIsNoop verifies that pressing "p" when TogglePinFn is nil
// does not panic and leaves the screen unchanged.
func TestPinKeyNilFnIsNoop(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenBackups
	m.Backups = makeBackupList(2)
	m.Cursor = 0
	// TogglePinFn intentionally left nil.

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	state := updated.(Model)

	if state.Screen != ScreenBackups {
		t.Fatalf("screen = %v, want ScreenBackups (nil TogglePinFn should be a no-op)", state.Screen)
	}
}

// TestPinKeyRefreshesBackupList verifies that after a successful pin toggle,
// the backup list is refreshed via ListBackupsFn.
func TestPinKeyRefreshesBackupList(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenBackups
	m.Backups = makeBackupList(3)
	m.Cursor = 0

	m.TogglePinFn = func(manifest backup.Manifest) error {
		return nil
	}

	refreshCalled := false
	refreshedList := makeBackupList(3)
	refreshedList[0].Pinned = true
	m.ListBackupsFn = func() []backup.Manifest {
		refreshCalled = true
		return refreshedList
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	state := updated.(Model)

	if !refreshCalled {
		t.Fatalf("ListBackupsFn was not called after pin toggle")
	}
	if !state.Backups[0].Pinned {
		t.Fatalf("Backups[0].Pinned = false after refresh, want true")
	}
}

// TestPinKeyError_ListNotRefreshed verifies that when TogglePinFn returns an
// error, ListBackupsFn is NOT called — the list stays unchanged and PinErr is set.
func TestPinKeyError_ListNotRefreshed(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenBackups
	originalList := makeBackupList(3)
	m.Backups = originalList
	m.Cursor = 0

	pinErr := fmt.Errorf("write failed: permission denied")
	m.TogglePinFn = func(manifest backup.Manifest) error {
		return pinErr
	}

	listRefreshCalled := false
	m.ListBackupsFn = func() []backup.Manifest {
		listRefreshCalled = true
		return makeBackupList(3)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	state := updated.(Model)

	if listRefreshCalled {
		t.Fatalf("ListBackupsFn should NOT be called when TogglePinFn returns an error")
	}
	if len(state.Backups) != len(originalList) {
		t.Fatalf("Backups list changed after pin error; got %d items, want %d", len(state.Backups), len(originalList))
	}
	if state.PinErr == nil {
		t.Fatalf("PinErr should be set after TogglePinFn error, got nil")
	}
}

// TestPinErrClearedOnScreenReentry verifies that PinErr is cleared when the user
// navigates away from ScreenBackups and then returns to it.
func TestPinErrClearedOnScreenReentry(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenBackups
	m.Backups = makeBackupList(3)
	m.Cursor = 0
	// Seed a stale PinErr from a previous attempt.
	m.PinErr = fmt.Errorf("write failed: permission denied")

	// Navigate away: Esc from ScreenBackups returns to ScreenWelcome.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	afterEsc := updated.(Model)
	if afterEsc.Screen != ScreenWelcome {
		t.Fatalf("Esc from ScreenBackups: screen = %v, want ScreenWelcome", afterEsc.Screen)
	}

	// Navigate back to ScreenBackups (cursor 6 on Welcome → enter).
	afterEsc.Cursor = 6
	updated2, _ := afterEsc.Update(tea.KeyMsg{Type: tea.KeyEnter})
	afterReturn := updated2.(Model)
	if afterReturn.Screen != ScreenBackups {
		t.Fatalf("Enter cursor=6 from ScreenWelcome: screen = %v, want ScreenBackups", afterReturn.Screen)
	}

	// PinErr must be cleared on re-entry.
	if afterReturn.PinErr != nil {
		t.Fatalf("PinErr should be nil after returning to ScreenBackups, got: %v", afterReturn.PinErr)
	}
}

// TestComponentsForPreset_PersonaMatrix verifies that componentsForPreset includes
// ComponentPersona when persona != PersonaCustom and excludes it for PersonaCustom.
func TestComponentsForPreset_PersonaMatrix(t *testing.T) {
	tests := []struct {
		name        string
		preset      model.PresetID
		persona     model.PersonaID
		wantPersona bool
		wantNil     bool
	}{
		{
			name:        "full-gentleman + gentleman includes persona",
			preset:      model.PresetFullGentleman,
			persona:     model.PersonaGentleman,
			wantPersona: true,
		},
		{
			name:        "full-gentleman + custom excludes persona",
			preset:      model.PresetFullGentleman,
			persona:     model.PersonaCustom,
			wantPersona: false,
		},
		{
			name:        "minimal + gentleman includes persona",
			preset:      model.PresetMinimal,
			persona:     model.PersonaGentleman,
			wantPersona: true,
		},
		{
			name:        "minimal + custom does not include persona",
			preset:      model.PresetMinimal,
			persona:     model.PersonaCustom,
			wantPersona: false,
		},
		{
			name:        "ecosystem-only + neutral includes persona",
			preset:      model.PresetEcosystemOnly,
			persona:     model.PersonaNeutral,
			wantPersona: true,
		},
		{
			name:        "ecosystem-only + custom does not include persona",
			preset:      model.PresetEcosystemOnly,
			persona:     model.PersonaCustom,
			wantPersona: false,
		},
		{
			name:    "custom preset returns nil regardless of persona (gentleman)",
			preset:  model.PresetCustom,
			persona: model.PersonaGentleman,
			wantNil: true,
		},
		{
			name:    "custom preset returns nil regardless of persona (custom)",
			preset:  model.PresetCustom,
			persona: model.PersonaCustom,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := componentsForPreset(tt.preset, tt.persona)

			if tt.wantNil {
				if got != nil {
					t.Fatalf("componentsForPreset(%v, %v) = %v, want nil", tt.preset, tt.persona, got)
				}
				return
			}

			hasPersona := false
			for _, c := range got {
				if c == model.ComponentPersona {
					hasPersona = true
				}
			}

			if tt.wantPersona && !hasPersona {
				t.Fatalf("componentsForPreset(%v, %v) missing ComponentPersona; got: %v", tt.preset, tt.persona, got)
			}
			if !tt.wantPersona && hasPersona {
				t.Fatalf("componentsForPreset(%v, %v) should not include ComponentPersona; got: %v", tt.preset, tt.persona, got)
			}
		})
	}
}

// TestPersonaScreenRecomputesComponentsWhenPresetAlreadySet verifies that changing
// the persona on the Persona screen recomputes the component list when a non-custom
// preset has already been selected.
func TestPersonaScreenRecomputesComponentsWhenPresetAlreadySet(t *testing.T) {
	// Start with a model that has already picked full-gentleman preset and
	// gentleman persona (the default), then go back to Persona screen and pick custom.
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenPersona
	m.Selection.Preset = model.PresetFullGentleman
	m.Selection.Persona = model.PersonaGentleman
	m.Selection.Components = componentsForPreset(model.PresetFullGentleman, model.PersonaGentleman)

	// Confirm that managed persona is initially included.
	hasPersonaBefore := false
	for _, c := range m.Selection.Components {
		if c == model.ComponentPersona {
			hasPersonaBefore = true
		}
	}
	if !hasPersonaBefore {
		t.Fatal("setup: expected ComponentPersona in initial components")
	}

	// Move cursor to PersonaCustom and confirm.
	m.Cursor = len(screens.PersonaOptions()) - 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Selection.Persona != model.PersonaCustom {
		t.Fatalf("Persona = %v, want %v", state.Selection.Persona, model.PersonaCustom)
	}

	// ComponentPersona must be removed after recompute.
	for _, c := range state.Selection.Components {
		if c == model.ComponentPersona {
			t.Fatalf("ComponentPersona must not be in components after switching to PersonaCustom; got: %v", state.Selection.Components)
		}
	}
}

// TestPersonaScreenDoesNotRecomputeForCustomPreset verifies that changing persona
// does NOT recompute (and wipe) the nil component list when preset is custom.
func TestPersonaScreenDoesNotRecomputeForCustomPreset(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenPersona
	m.Selection.Preset = model.PresetCustom
	m.Selection.Persona = model.PersonaGentleman
	m.Selection.Components = nil

	m.Cursor = 0 // PersonaNeutral
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	// Components must remain nil for custom preset.
	if state.Selection.Components != nil {
		t.Fatalf("components should stay nil for custom preset; got: %v", state.Selection.Components)
	}
}

func TestCustomPersonaCustomPresetCanSelectEngramWithoutPersonaOrPolish(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenPreset
	m.Selection.Persona = model.PersonaCustom
	m.Cursor = len(screens.PresetOptions()) - 1 // Custom preset

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Selection.Preset != model.PresetCustom {
		t.Fatalf("Preset = %v, want %v", state.Selection.Preset, model.PresetCustom)
	}
	if state.Screen != ScreenDependencyTree {
		t.Fatalf("Screen = %v, want ScreenDependencyTree for custom component selection", state.Screen)
	}
	if len(state.Selection.Components) != 0 {
		t.Fatalf("custom preset should start with no components selected; got %v", state.Selection.Components)
	}

	engramCursor := -1
	for idx, component := range screens.AllComponents() {
		if component.ID == model.ComponentEngram {
			engramCursor = idx
			break
		}
	}
	if engramCursor < 0 {
		t.Fatal("Engram component not found in custom component picker")
	}
	state.Cursor = engramCursor
	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeySpace})
	state = updated.(Model)

	if !slices.Equal(state.Selection.Components, []model.ComponentID{model.ComponentEngram}) {
		t.Fatalf("components = %v, want only Engram selected", state.Selection.Components)
	}
	if slices.Contains(state.Selection.Components, model.ComponentPersona) {
		t.Fatalf("custom preset should not auto-select ComponentPersona; components: %v", state.Selection.Components)
	}
}

func TestShouldShowCodexModelPickerScreen_TrueWhenCodexAndSDD(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Selection.Agents = []model.AgentID{model.AgentCodex}
	m.Selection.Components = []model.ComponentID{model.ComponentEngram, model.ComponentSDD}
	if !m.shouldShowCodexModelPickerScreen() {
		t.Fatal("shouldShowCodexModelPickerScreen() = false, want true when Codex+SDD selected")
	}
}

func TestShouldShowCodexModelPickerScreen_FalseWhenNoCodex(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Selection.Agents = []model.AgentID{model.AgentClaudeCode}
	m.Selection.Components = []model.ComponentID{model.ComponentEngram, model.ComponentSDD}
	if m.shouldShowCodexModelPickerScreen() {
		t.Fatal("shouldShowCodexModelPickerScreen() = true, want false when Codex not in agents")
	}
}

func TestShouldShowCodexModelPickerScreen_FalseWhenNoSDD(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Selection.Agents = []model.AgentID{model.AgentCodex}
	m.Selection.Components = []model.ComponentID{model.ComponentEngram}
	if m.shouldShowCodexModelPickerScreen() {
		t.Fatal("shouldShowCodexModelPickerScreen() = true, want false when SDD not in components")
	}
}

// ─── Codex picker install-flow routing tests ─────────────────────────────────
// These tests cover scenarios in which the Codex model picker MUST be reached
// during the install flow (non-ModelConfigMode, non-custom preset, SDD selected).

// TestCodexOnly_InstallFlowReachesCodexPicker verifies that selecting a preset
// when Codex is the only agent (no Claude, no Kiro) navigates to
// ScreenCodexModelPicker.
func TestCodexOnly_InstallFlowReachesCodexPicker(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenPreset
	m.Selection.Agents = []model.AgentID{model.AgentCodex}
	m.Cursor = 0 // PresetFullGentleman (includes SDD)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenCodexModelPicker {
		t.Fatalf("Codex-only install flow: screen = %v, want ScreenCodexModelPicker", state.Screen)
	}
}

// TestClaudeAndCodex_InstallFlowReachesCodexPickerAfterClaude verifies that
// after the Claude model picker is completed, the flow advances to
// ScreenCodexModelPicker when Codex is also selected (no Kiro).
// RED: currently goes to ScreenSDDMode instead of ScreenCodexModelPicker.
func TestClaudeAndCodex_InstallFlowReachesCodexPickerAfterClaude(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenClaudeModelPicker
	m.ModelConfigMode = false
	m.Selection.Preset = model.PresetFullGentleman
	m.Selection.Agents = []model.AgentID{model.AgentClaudeCode, model.AgentCodex}
	m.Selection.Components = componentsForPreset(model.PresetFullGentleman, model.PersonaGentleman)
	m.ClaudeModelPicker = screens.NewClaudeModelPickerState()

	// Press Enter to confirm the default preset option (cursor 0).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	if state.Screen != ScreenCodexModelPicker {
		t.Fatalf("Claude+Codex install flow (after Claude picker): screen = %v, want ScreenCodexModelPicker", state.Screen)
	}
}

// TestKiroAndCodex_InstallFlowReachesCodexPickerAfterKiro verifies that
// after the Kiro model picker is completed, the flow advances to
// ScreenCodexModelPicker when Codex is also selected (no Claude).
// RED: currently goes to ScreenSDDMode instead of ScreenCodexModelPicker.

// TestClaudeKiroCodex_InstallFlowSequence verifies that the full Claude→Kiro→Codex
// picker chain is traversed in order during an install flow where all three agents
// are selected.
// RED: currently Claude→Kiro→SDDMode (Codex is skipped).

// TestCodexPicker_EscBackNavToKiroWhenKiroSelected verifies that pressing Esc
// from ScreenCodexModelPicker goes back to ScreenKiroModelPicker when Kiro is
// also selected in the flow.

// TestCodexPicker_EscBackNavToClaudeWhenClaudeSelectedNoKiro verifies that
// pressing Esc from ScreenCodexModelPicker goes back to ScreenClaudeModelPicker
// when Claude is selected but Kiro is not.

// TestCodexPicker_EscBackNavToPresetWhenNeitherClaudeNorKiro verifies that
// pressing Esc from ScreenCodexModelPicker goes back to ScreenPreset when
// neither Claude nor Kiro is in the flow.

// TestCodexPresetSelection_PopulatesPendingSyncOverrides verifies that every
// preset persists its model matrix through the user-visible Model.Update path.
func TestCodexPresetSelection_PopulatesPendingSyncOverrides(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		want   map[string]string
	}{
		{
			name:   "low cost",
			cursor: 0,
			want: map[string]string{
				"sdd-strong": "gpt-5.6-sol",
				"sdd-mid":    "gpt-5.6-terra",
				"sdd-cheap":  "gpt-5.6-luna",
			},
		},
		{
			name:   "recommended",
			cursor: 1,
			want: map[string]string{
				"sdd-strong": "gpt-5.6-sol",
				"sdd-mid":    "gpt-5.6-terra",
				"sdd-cheap":  "gpt-5.6-luna",
			},
		},
		{
			name:   "powerful",
			cursor: 2,
			want: map[string]string{
				"sdd-strong": "gpt-5.6-sol",
				"sdd-mid":    "gpt-5.6-sol",
				"sdd-cheap":  "gpt-5.6-luna",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(system.DetectionResult{}, "dev")
			m.Screen = ScreenCodexModelPicker
			m.ModelConfigMode = true
			m.Selection.Agents = []model.AgentID{model.AgentCodex}
			m.Selection.Components = []model.ComponentID{model.ComponentEngram, model.ComponentSDD}
			m.CodexModelPicker = screens.NewCodexModelPickerState()
			m.Cursor = tt.cursor

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			state := updated.(Model)

			if state.ModelConfigMode {
				t.Fatal("ModelConfigMode should be false after Codex preset selection")
			}
			if state.PendingSyncOverrides == nil {
				t.Fatal("PendingSyncOverrides = nil, want non-nil after Codex preset selection")
			}
			if state.PendingSyncOverrides.CodexModelAssignments == nil {
				t.Fatal("PendingSyncOverrides.CodexModelAssignments = nil, want non-nil")
			}

			pendingCarrils := state.PendingSyncOverrides.CodexCarrilModelAssignments
			selectedCarrils := state.Selection.CodexCarrilModelAssignments
			for carril, wantModel := range tt.want {
				if got := pendingCarrils[carril]; got != wantModel {
					t.Errorf("PendingSyncOverrides.CodexCarrilModelAssignments[%q] = %q, want %q", carril, got, wantModel)
				}
				if got := selectedCarrils[carril]; got != wantModel {
					t.Errorf("Selection.CodexCarrilModelAssignments[%q] = %q, want %q", carril, got, wantModel)
				}
			}
		})
	}
}

// ─── FIX W-1: Codex custom sub-mode cursor reset ─────────────────────────────

// ─── FIX W-2: CustomConfirmed reset on preset selection ──────────────────────

// TestCodexModelPickerPresetClearsCustomState verifies that selecting a preset
// after a prior Custom confirm resets CustomConfirmed to false and clears
// CodexPhaseModelAssignments so the inject layer uses the carril table.
func TestCodexModelPickerPresetClearsCustomState(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenCodexModelPicker
	m.CodexModelPicker = screens.NewCodexModelPickerState()

	// Simulate a previously confirmed Custom flow.
	m.CodexModelPicker.CustomConfirmed = true
	m.Selection.CodexPhaseModelAssignments = map[string]string{
		"sdd-propose": "gpt-5.4",
	}

	// Select the Recommended preset (cursor index 1).
	m.Cursor = 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)

	// CustomConfirmed must be reset.
	if state.CodexModelPicker.CustomConfirmed {
		t.Error("CodexModelPicker.CustomConfirmed = true after preset selection, want false")
	}
	// CodexPhaseModelAssignments must be nil — inject layer should use carril table.
	if state.Selection.CodexPhaseModelAssignments != nil {
		t.Errorf("Selection.CodexPhaseModelAssignments = %v after preset selection, want nil",
			state.Selection.CodexPhaseModelAssignments)
	}
}

// ─── FIX W-1: Codex custom sub-mode cursor reset ─────────────────────────────

// TestCodexModelPickerCustomModeEscResetsCursor verifies that after entering
// the Codex custom sub-mode and pressing Esc, the outer cursor is reset to 0.
func TestCodexModelPickerCustomModeEscResetsCursor(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenCodexModelPicker
	m.CodexModelPicker = screens.NewCodexModelPickerState()
	// Enter the Custom sub-mode (index 3).
	m.CodexModelPicker.CustomMode = screens.CodexCustomModePhaseList
	m.Cursor = 7 // simulate user navigated down in custom phase list

	// Press Esc — should exit the Custom sub-mode and reset the outer cursor to 0.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	state := updated.(Model)

	// Custom mode must be off.
	if state.CodexModelPicker.CustomMode != screens.CodexCustomModeNone {
		t.Fatalf("CodexModelPicker.CustomMode = %v, want CodexCustomModeNone after Esc", state.CodexModelPicker.CustomMode)
	}
	// Outer cursor must be reset to 0.
	if state.Cursor != 0 {
		t.Fatalf("Cursor = %d, want 0 after Esc from Codex custom sub-mode (cursor not reset)", state.Cursor)
	}
}

func TestGentleAIUpgradeVersionDetectsSucceededGentleAI(t *testing.T) {
	report := upgrade.UpgradeReport{Results: []upgrade.ToolUpgradeResult{
		{ToolName: "engram", Status: upgrade.UpgradeSucceeded, NewVersion: "1.0.0"},
		{ToolName: "gentle-ai", Status: upgrade.UpgradeSucceeded, NewVersion: "v1.40.0"},
	}}
	m := Model{UpgradeReport: &report}
	got, ok := m.GentleAIUpgradeVersion()
	if !ok {
		t.Fatal("GentleAIUpgradeVersion() ok = false, want true")
	}
	if got != "1.40.0" {
		t.Fatalf("GentleAIUpgradeVersion() = %q, want %q", got, "1.40.0")
	}
}

func TestUpgradeResultEnterQuitsWhenGentleAIWasUpgraded(t *testing.T) {
	report := upgrade.UpgradeReport{Results: []upgrade.ToolUpgradeResult{
		{ToolName: "gentle-ai", Status: upgrade.UpgradeSucceeded, NewVersion: "v1.40.0"},
	}}
	m := Model{Screen: ScreenUpgrade, UpgradeReport: &report}
	_, cmd := m.confirmSelection()
	if cmd == nil {
		t.Fatal("confirmSelection() cmd = nil, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("confirmSelection() command returned %T, want tea.QuitMsg", cmd())
	}
}

func TestUpgradeSyncResultEscQuitsWhenGentleAIWasUpgraded(t *testing.T) {
	report := upgrade.UpgradeReport{Results: []upgrade.ToolUpgradeResult{
		{ToolName: "gentle-ai", Status: upgrade.UpgradeSucceeded, NewVersion: "v1.40.0"},
	}}
	m := Model{Screen: ScreenUpgradeSync, UpgradeReport: &report, HasSyncRun: true}
	_, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("handleKeyPress(esc) cmd = nil, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("handleKeyPress(esc) command returned %T, want tea.QuitMsg", cmd())
	}
}

// ─── TUI-path PendingSync (task 4.8) ────────────────────────────────────────

// executeUpgradeSyncSequence runs the tea.Sequence returned by startUpgradeSync
// and collects the messages produced by each command in order.
// tea.Sequence returns a Cmd whose result is an internal sequenceMsg (type []Cmd).
// Since sequenceMsg is unexported we iterate via reflect.
func executeUpgradeSyncSequence(t *testing.T, m Model) []tea.Msg {
	t.Helper()

	seqCmd := m.startUpgradeSync()
	if seqCmd == nil {
		t.Fatal("startUpgradeSync() returned nil cmd")
	}

	// Calling the outer cmd returns either:
	//   a) the only element directly (when compactCmds collapses a single-cmd slice), or
	//   b) a sequenceMsg (type []tea.Cmd) when there are 2+ cmds.
	outerMsg := seqCmd()

	// Try direct cast to known concrete types first.
	if _, ok := outerMsg.(UpgradePhaseCompletedMsg); ok {
		// Only one cmd was returned; no sequence wrapper.
		return []tea.Msg{outerMsg}
	}
	if _, ok := outerMsg.(SyncDoneMsg); ok {
		return []tea.Msg{outerMsg}
	}

	// sequenceMsg is type []tea.Cmd — use reflect to iterate without importing
	// the unexported type.
	v := reflect.ValueOf(outerMsg)
	if v.Kind() != reflect.Slice {
		t.Fatalf("startUpgradeSync outer msg kind = %v, want slice (sequenceMsg)", v.Kind())
	}

	var msgs []tea.Msg
	for i := range v.Len() {
		elem := v.Index(i).Interface()
		innerCmd, ok := elem.(tea.Cmd)
		if !ok || innerCmd == nil {
			continue
		}
		msgs = append(msgs, innerCmd())
	}
	return msgs
}

// TestStartUpgradeSync_SetsPendingSyncWhenGentleAIUpgraded verifies that when
// the UpgradeFn reports gentle-ai as upgraded, the syncCmd branch of
// startUpgradeSync writes PendingSync=true to state.json before returning
// SyncDoneMsg. This is the TUI-path equivalent of the selfupdate.go path tested
// in TestSelfUpdate_SetsPendingSyncOnSuccess.
func TestStartUpgradeSync_SetsPendingSyncWhenGentleAIUpgraded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpgradeSync
	m.OperationRunning = true

	// UpgradeFn reports gentle-ai as successfully upgraded.
	m.UpgradeFn = func(_ context.Context, _ []update.UpdateResult) upgrade.UpgradeReport {
		return upgrade.UpgradeReport{
			Results: []upgrade.ToolUpgradeResult{
				{ToolName: "gentle-ai", Status: upgrade.UpgradeSucceeded, NewVersion: "1.8.0"},
			},
		}
	}

	msgs := executeUpgradeSyncSequence(t, m)

	// Verify the sequence produced both expected messages.
	var gotUpgradePhase bool
	var gotSyncDone bool
	for _, msg := range msgs {
		if _, ok := msg.(UpgradePhaseCompletedMsg); ok {
			gotUpgradePhase = true
		}
		if _, ok := msg.(SyncDoneMsg); ok {
			gotSyncDone = true
		}
	}
	if !gotUpgradePhase {
		t.Errorf("sequence did not produce UpgradePhaseCompletedMsg; msgs = %v", msgs)
	}
	if !gotSyncDone {
		t.Errorf("sequence did not produce SyncDoneMsg; msgs = %v", msgs)
	}

	// The key assertion: PendingSync=true must be written to state.json on disk.
	s, err := state.Read(home)
	if err != nil {
		t.Fatalf("state.Read(%q) error = %v (PendingSync was not written)", home, err)
	}
	if !s.PendingSync {
		t.Errorf("PendingSync = false after gentle-ai self-upgrade in TUI flow, want true")
	}
}

// TestStartUpgradeSync_DoesNotSetPendingSyncWhenGentleAINotUpgraded verifies
// that when gentle-ai was NOT upgraded (e.g. only engram was upgraded), the
// syncCmd branch does NOT set PendingSync, and sync proceeds normally via SyncFn.
func TestStartUpgradeSync_DoesNotSetPendingSyncWhenGentleAINotUpgraded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpgradeSync
	m.OperationRunning = true

	// UpgradeFn reports only engram upgraded, not gentle-ai.
	m.UpgradeFn = func(_ context.Context, _ []update.UpdateResult) upgrade.UpgradeReport {
		return upgrade.UpgradeReport{
			Results: []upgrade.ToolUpgradeResult{
				{ToolName: "engram", Status: upgrade.UpgradeSucceeded, NewVersion: "1.16.4"},
			},
		}
	}

	var syncCalled bool
	m.SyncFn = func(_ *model.SyncOverrides) ([]string, error) {
		syncCalled = true
		return []string{"file.json"}, nil
	}

	msgs := executeUpgradeSyncSequence(t, m)

	// SyncFn must have been called (not the deferred-PendingSync path).
	if !syncCalled {
		t.Errorf("SyncFn was not called — expected normal sync when gentle-ai was not upgraded")
	}

	// PendingSync must NOT be set when gentle-ai was not upgraded.
	// state.json may not exist at all if nothing wrote it; that is expected and
	// means PendingSync was never set (correct). Any other read error is
	// unexpected and should fail the test loudly.
	s, readErr := state.Read(home)
	if readErr != nil {
		if !errors.Is(readErr, os.ErrNotExist) {
			t.Fatalf("unexpected state.Read error: %v", readErr)
		}
		// File absent → PendingSync was never set — correct.
	} else if s.PendingSync {
		t.Errorf("PendingSync = true after non-gentle-ai upgrade, want false")
	}

	// Verify SyncDoneMsg arrived.
	var gotSyncDone bool
	for _, msg := range msgs {
		if sd, ok := msg.(SyncDoneMsg); ok {
			gotSyncDone = true
			if sd.Err != nil {
				t.Errorf("SyncDoneMsg.Err = %v, want nil", sd.Err)
			}
		}
	}
	if !gotSyncDone {
		t.Errorf("sequence did not produce SyncDoneMsg; msgs = %v", msgs)
	}
}

// TestStartUpgradeSync_NoClobberOnCorruptStateFile verifies that when the HOME
// directory has a corrupt (non-missing) state.json, the TUI syncCmd branch does
// NOT overwrite it when setting PendingSync=true — matching the no-clobber
// pattern in internal/update/cooldown.go.
func TestStartUpgradeSync_NoClobberOnCorruptStateFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Write a corrupt state file so state.Read returns a non-ErrNotExist error.
	stateDir := filepath.Join(home, ".gentle-ai")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	corruptPayload := []byte("this is not valid JSON {{{")
	stateFilePath := filepath.Join(stateDir, "state.json")
	if err := os.WriteFile(stateFilePath, corruptPayload, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpgradeSync
	m.OperationRunning = true

	// UpgradeFn reports gentle-ai as successfully upgraded.
	m.UpgradeFn = func(_ context.Context, _ []update.UpdateResult) upgrade.UpgradeReport {
		return upgrade.UpgradeReport{
			Results: []upgrade.ToolUpgradeResult{
				{ToolName: "gentle-ai", Status: upgrade.UpgradeSucceeded, NewVersion: "1.8.0"},
			},
		}
	}

	executeUpgradeSyncSequence(t, m)

	// The corrupt state file must NOT have been overwritten.
	got, readErr := os.ReadFile(stateFilePath)
	if readErr != nil {
		t.Fatalf("os.ReadFile after startUpgradeSync: %v", readErr)
	}
	if string(got) != string(corruptPayload) {
		t.Errorf("state file was overwritten on corrupt-read error\ngot:  %q\nwant: %q", got, corruptPayload)
	}
}

// ─── AdvisoryMsg TUI layer tests ─────────────────────────────────────────────

// TestAdvisoryMsg_SetsAdvisoryMessage verifies that dispatching AdvisoryMsg
// into model.Update stores the advisory text in m.AdvisoryMessage.
func TestAdvisoryMsg_SetsAdvisoryMessage(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")

	updated, _ := m.Update(AdvisoryMsg{Advisory: update.Advisory{Message: "test advisory"}})
	state := updated.(Model)

	if state.AdvisoryMessage != "test advisory" {
		t.Fatalf("AdvisoryMessage = %q, want %q", state.AdvisoryMessage, "test advisory")
	}
}

// TestAdvisoryMsg_EmptyAdvisoryNoChange verifies that dispatching an AdvisoryMsg
// with an empty message leaves AdvisoryMessage as the empty string.
func TestAdvisoryMsg_EmptyAdvisoryNoChange(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")

	updated, _ := m.Update(AdvisoryMsg{})
	state := updated.(Model)

	if state.AdvisoryMessage != "" {
		t.Fatalf("AdvisoryMessage = %q, want empty for zero-value AdvisoryMsg", state.AdvisoryMessage)
	}
}

// TestWelcomeView_ContainsAdvisoryMessage verifies that View() on ScreenWelcome
// renders the advisory message when AdvisoryMessage is set.
func TestWelcomeView_ContainsAdvisoryMessage(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenWelcome
	m.AdvisoryMessage = "security notice"

	view := m.View()

	if !strings.Contains(view, "security notice") {
		t.Fatalf("View() does not contain advisory message %q\nView output:\n%s", "security notice", view)
	}
}

// TestWelcomeView_AdvisoryPrefixed verifies that the advisory message is
// rendered with the "Advisory: " prefix on the Welcome screen.
func TestWelcomeView_AdvisoryPrefixed(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenWelcome
	m.AdvisoryMessage = "critical update"

	view := m.View()

	if !strings.Contains(view, "Advisory: critical update") {
		t.Fatalf("View() does not contain %q\nView output:\n%s", "Advisory: critical update", view)
	}
}

// TestWelcomeView_NewlineSeparatorBetweenUpdateAndAdvisory verifies that when
// both an update banner and an advisory message are present, they are rendered
// on separate lines (the banner string uses "\n" as separator so RenderWelcome
// outputs them as distinct visual lines).
func TestWelcomeView_NewlineSeparatorBetweenUpdateAndAdvisory(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenWelcome
	m.UpdateCheckDone = true
	m.UpdateResults = []update.UpdateResult{
		{
			Tool:             update.ToolInfo{Name: "engram"},
			InstalledVersion: "1.0.0",
			LatestVersion:    "1.1.0",
			Status:           update.UpdateAvailable,
		},
	}
	m.AdvisoryMessage = "advisory here"

	view := m.View()

	// Both pieces must appear in the view.
	if !strings.Contains(view, "Updates available") {
		t.Fatalf("View() does not contain update banner\nView output:\n%s", view)
	}
	if !strings.Contains(view, "Advisory: advisory here") {
		t.Fatalf("View() does not contain advisory message\nView output:\n%s", view)
	}
	// The box renderer wraps the banner string into per-line box rows, so the
	// update line and the advisory line must appear on distinct lines. Verify
	// that no single rendered line contains both substrings at once.
	lines := strings.Split(view, "\n")
	updateLineIdx, advisoryLineIdx := -1, -1
	for i, line := range lines {
		if strings.Contains(line, "Updates available") {
			updateLineIdx = i
		}
		if strings.Contains(line, "Advisory: advisory here") {
			advisoryLineIdx = i
		}
	}
	if updateLineIdx < 0 {
		t.Fatalf("no line contains 'Updates available'\nView output:\n%s", view)
	}
	if advisoryLineIdx < 0 {
		t.Fatalf("no line contains 'Advisory: advisory here'\nView output:\n%s", view)
	}
	if updateLineIdx == advisoryLineIdx {
		t.Fatalf("update banner and advisory appear on the same line (%d); expected separate lines\nView output:\n%s", updateLineIdx, view)
	}
}

func TestWelcomeView_LongAdvisoryStaysWithinWindowWidth(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenWelcome
	m.Width = 50
	m.AdvisoryMessage = "🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀 advisory must stay within the visible frame width"

	view := m.View()

	foundAdvisory := false
	for i, line := range strings.Split(view, "\n") {
		if !strings.Contains(line, "Advisory:") && !strings.Contains(line, "visible frame") {
			continue
		}
		foundAdvisory = true
		if width := lipgloss.Width(line); width > m.Width {
			t.Fatalf("advisory line %d width = %d, want <= %d\nline: %q\nview:\n%s", i, width, m.Width, line, view)
		}
	}
	if !foundAdvisory {
		t.Fatalf("advisory text was not rendered\nview:\n%s", view)
	}
}

func TestWelcomeAdvisory_BoundsAndScrollsOverflow(t *testing.T) {
	const releaseURL = "https://github.com/Gentleman-Programming/gentle-ai/releases/tag/v1.49.0"
	m := NewModel(system.DetectionResult{}, "dev")
	m.Width = 60
	m.Height = 50
	updated, _ := m.Update(AdvisoryMsg{Advisory: update.Advisory{
		Message: strings.Repeat("release detail ", 80),
		URL:     releaseURL,
	}})
	state := updated.(Model)
	initial := state.View()

	if got := lipgloss.Height(initial); got > state.Height {
		t.Fatalf("welcome height = %d, want <= terminal height %d", got, state.Height)
	}
	if !strings.Contains(initial, "PgUp/PgDn: scroll") {
		t.Fatalf("overflowing advisory missing scroll affordance\nview:\n%s", initial)
	}
	if !strings.Contains(initial, "Latest release:") || !strings.Contains(initial, "v1.49.0") {
		t.Fatalf("overflowing advisory did not keep release link visible\nview:\n%s", initial)
	}
	if !strings.Contains(initial, "Start installation") {
		t.Fatalf("overflowing advisory crowded out primary action\nview:\n%s", initial)
	}

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	state = updated.(Model)
	if state.AdvisoryScroll == 0 {
		t.Fatal("Page Down did not advance advisory scroll")
	}
	if state.Cursor != 0 {
		t.Fatalf("Page Down changed menu cursor to %d", state.Cursor)
	}
	if got := state.View(); got == initial {
		t.Fatal("Page Down did not change visible advisory content")
	}

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if got := updated.(Model).AdvisoryScroll; got != 0 {
		t.Fatalf("Page Up scroll = %d, want 0", got)
	}
}

func TestWelcomeAdvisory_FittingContentShowsLatestReleaseWithoutScrollHint(t *testing.T) {
	const releaseURL = "https://github.com/Gentleman-Programming/gentle-ai/releases/tag/v1.49.0"
	m := NewModel(system.DetectionResult{}, "dev")
	m.Width = 100
	m.Height = 60
	updated, _ := m.Update(AdvisoryMsg{Advisory: update.Advisory{
		Message: "Maintenance improvements are available.",
		URL:     releaseURL,
	}})
	state := updated.(Model)
	view := state.View()

	if state.AdvisoryURL != releaseURL || !strings.Contains(view, "Latest release: "+releaseURL) {
		t.Fatalf("latest release link not carried through\nview:\n%s", view)
	}
	if strings.Contains(view, "PgUp/PgDn: scroll") {
		t.Fatalf("fitting advisory unexpectedly shows scroll affordance\nview:\n%s", view)
	}
}

func TestWelcomeAdvisory_SmallTerminalPreservesMenu(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Width = 60
	m.Height = 35
	baselineHeight := lipgloss.Height(m.View())
	updated, _ := m.Update(AdvisoryMsg{Advisory: update.Advisory{
		Message: strings.Repeat("long advisory ", 80),
		URL:     "https://github.com/Gentleman-Programming/gentle-ai/releases/tag/v1.49.0",
	}})
	view := updated.(Model).View()

	if got := lipgloss.Height(view); got != baselineHeight {
		t.Fatalf("small-terminal advisory added %d lines, want none", got-baselineHeight)
	}
	if !strings.Contains(view, "Start installation") {
		t.Fatalf("small-terminal welcome lost primary action\nview:\n%s", view)
	}
}

func TestWelcomeAdvisory_ResizeAndContentChangesClampScroll(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Width = 45
	m.Height = 60
	updated, _ := m.Update(AdvisoryMsg{Advisory: update.Advisory{
		Message: strings.Repeat("release detail ", 12),
		URL:     "https://github.com/Gentleman-Programming/gentle-ai/releases/tag/v1.49.0",
	}})
	state := updated.(Model)
	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	state = updated.(Model)
	if state.AdvisoryScroll == 0 {
		t.Fatal("test setup did not produce advisory overflow")
	}

	updated, _ = state.Update(tea.WindowSizeMsg{Width: 120, Height: 80})
	state = updated.(Model)
	if state.AdvisoryScroll != 0 {
		t.Fatalf("scroll after fitting resize = %d, want 0", state.AdvisoryScroll)
	}
	if strings.Contains(state.View(), "PgUp/PgDn: scroll") {
		t.Fatal("scroll affordance remained after content fit")
	}

	updated, _ = state.Update(tea.WindowSizeMsg{Width: 45, Height: 60})
	state = updated.(Model)
	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	state = updated.(Model)
	updated, _ = state.Update(AdvisoryMsg{Advisory: update.Advisory{Message: "Short notice."}})
	if got := updated.(Model).AdvisoryScroll; got != 0 {
		t.Fatalf("scroll after advisory content change = %d, want 0", got)
	}
}

func TestWelcomeView_WindowResizeFitsMeasuredViewport(t *testing.T) {
	cases := []struct {
		name         string
		width        int
		height       int
		withOptional bool
		minimum      bool
		wantPrimary  string
		wantControl  string
	}{
		{name: "startup viewport", width: 120, height: 30},
		{name: "narrow resize", width: 80, height: 24},
		{name: "short viewport", width: 120, height: 19},
		{name: "below compact height", width: 120, height: 2, minimum: true},
		{name: "below compact width", width: 18, height: 20, minimum: true},
		{name: "below frame border width", width: 2, height: 20, minimum: true, wantPrimary: "Go"},
		{name: "tiny viewport uses atomic labels", width: 2, height: 2, minimum: true, wantPrimary: "Go", wantControl: "q"},
		{name: "single column tiny viewport uses atomic labels", width: 1, height: 2, minimum: true, wantPrimary: ">", wantControl: "q"},
		{name: "compact viewport with optional content", width: 120, height: 17, withOptional: true},
		{name: "wide resize", width: 160, height: 50},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel(system.DetectionResult{}, "dev")
			if tc.withOptional {
				m.UpdateCheckDone = true
				m.UpdateResults = []update.UpdateResult{{
					Tool:             update.ToolInfo{Name: "engram"},
					InstalledVersion: "1.0.0",
					LatestVersion:    "1.1.0",
					Status:           update.UpdateAvailable,
				}}
				m.AdvisoryMessage = "Optional advisory content"
			}
			updated, _ := m.Update(tea.WindowSizeMsg{Width: tc.width, Height: tc.height})
			state := updated.(Model)
			view := state.View()

			if state.Width != tc.width || state.Height != tc.height {
				t.Fatalf("window size = %dx%d, want %dx%d", state.Width, state.Height, tc.width, tc.height)
			}
			if got := lipgloss.Width(view); got > tc.width {
				t.Fatalf("welcome width = %d, want <= %d\nview:\n%s", got, tc.width, view)
			}
			if got := lipgloss.Height(view); got > tc.height {
				t.Fatalf("welcome height = %d, want <= %d\nview:\n%s", got, tc.height, view)
			}
			content := view
			wrappedContent := view
			if tc.minimum {
				content = strings.ReplaceAll(view, "\n", "")
				wrappedContent = strings.ReplaceAll(view, "\n", " ")
			}
			primary := tc.wantPrimary
			if primary == "" {
				primary = "Start installation"
			}
			if !strings.Contains(content, primary) && !strings.Contains(wrappedContent, primary) {
				t.Fatalf("welcome lost primary action %q after resize\nview:\n%s", primary, view)
			}
			if tc.minimum {
				control := tc.wantControl
				if control == "" {
					control = "j/k"
				}
				for _, want := range []string{primary, control} {
					if !strings.Contains(content, want) && !strings.Contains(wrappedContent, want) {
						t.Fatalf("minimum welcome state lost %q\nview:\n%s", want, view)
					}
				}
			} else {
				for _, want := range []string{"Quit", "j/k: navigate • enter: select • q: quit"} {
					if !strings.Contains(view, want) {
						t.Fatalf("welcome lost %q after resize\nview:\n%s", want, view)
					}
				}
			}
		})
	}
}

func TestWelcomeView_MinimumResizePreservesNonzeroCursor(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	const (
		cursor = 1
		width  = 18
		height = 20
	)
	m := NewModel(system.DetectionResult{}, "dev")
	m.Cursor = cursor

	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	state := updated.(Model)
	view := state.View()

	if state.Cursor != cursor {
		t.Fatalf("cursor after resize = %d, want %d", state.Cursor, cursor)
	}
	if got := lipgloss.Width(view); got > width {
		t.Fatalf("welcome width = %d, want <= %d\nview:\n%s", got, width, view)
	}
	if got := lipgloss.Height(view); got > height {
		t.Fatalf("welcome height = %d, want <= %d\nview:\n%s", got, height, view)
	}
	if !strings.Contains(view, styles.UnselectedStyle.Render("Start installation")) {
		t.Fatalf("minimum welcome state did not preserve the non-selected style\nview:\n%s", view)
	}
	if strings.Contains(view, styles.SelectedStyle.Render("Start installation")) {
		t.Fatalf("minimum welcome state marked Start installation selected for cursor %d\nview:\n%s", cursor, view)
	}
}

// ─── Advisory message sanitization tests ─────────────────────────────────────

// TestSanitizeAdvisoryMessage_StripControlChars verifies that ASCII control
// characters (including carriage return, bell, backspace, etc.) are removed
// from the advisory message, keeping only printable characters and normal spaces.
func TestSanitizeAdvisoryMessage_StripControlChars(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "carriage return stripped",
			input: "hello\rworld",
			want:  "helloworld",
		},
		{
			name:  "bell stripped",
			input: "ring\x07bell",
			want:  "ringbell",
		},
		{
			name:  "backspace stripped",
			input: "a\x08b",
			want:  "ab",
		},
		{
			name:  "null byte stripped",
			input: "null\x00byte",
			want:  "nullbyte",
		},
		{
			name:  "tab stripped",
			input: "ta\tb",
			want:  "tab",
		},
		{
			name:  "newline stripped",
			input: "line\nbreak",
			want:  "linebreak",
		},
		{
			name:  "clean message unchanged",
			input: "security notice: update now",
			want:  "security notice: update now",
		},
		{
			name:  "empty string unchanged",
			input: "",
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeAdvisoryMessage(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeAdvisoryMessage(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestSanitizeAdvisoryMessage_StripANSIEscapes verifies that ANSI escape
// sequences (e.g. color codes, cursor movement) are stripped from the message
// so they cannot corrupt the TUI layout.
func TestSanitizeAdvisoryMessage_StripANSIEscapes(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "color reset stripped",
			input: "\x1b[0mhello",
			want:  "hello",
		},
		{
			name:  "bold red color stripped",
			input: "\x1b[1;31mwarn\x1b[0m",
			want:  "warn",
		},
		{
			name:  "cursor movement stripped",
			input: "a\x1b[2Jb",
			want:  "ab",
		},
		{
			name:  "mixed text and escapes",
			input: "normal \x1b[32mgreen\x1b[0m text",
			want:  "normal green text",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeAdvisoryMessage(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeAdvisoryMessage(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestAdvisoryMsg_SanitizesOnStore verifies that control characters in an
// advisory message dispatched via AdvisoryMsg are sanitized before being stored
// in m.AdvisoryMessage, so they can never reach the rendered View.
func TestAdvisoryMsg_SanitizesOnStore(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")

	dirty := "notice\x1b[1;31m URGENT\x1b[0m\r\nupdate now"
	updated, _ := m.Update(AdvisoryMsg{Advisory: update.Advisory{Message: dirty}})
	state := updated.(Model)

	// Must not contain any ESC character or control character.
	for i, ch := range state.AdvisoryMessage {
		if ch < 0x20 || ch == 0x7f {
			t.Errorf("AdvisoryMessage[%d] = %U (%q) — control character not stripped; full value: %q",
				i, ch, ch, state.AdvisoryMessage)
		}
	}
	// Printable parts of the original message must be preserved.
	if !strings.Contains(state.AdvisoryMessage, "notice") {
		t.Errorf("AdvisoryMessage = %q — expected printable word %q to survive sanitization", state.AdvisoryMessage, "notice")
	}
	if !strings.Contains(state.AdvisoryMessage, "update now") {
		t.Errorf("AdvisoryMessage = %q — expected printable phrase %q to survive sanitization", state.AdvisoryMessage, "update now")
	}
}

// ---------------------------------------------------------------------------
// Slice 6 — TUI Pre-Welcome Update Prompt Screen
// ---------------------------------------------------------------------------

// makeUpdateResult returns a minimal UpdateResult with the given status and release URL.
func makeUpdateResult(status update.UpdateStatus, releaseURL string) update.UpdateResult {
	return update.UpdateResult{
		Tool:             update.ToolInfo{Name: "gentle-ai"},
		Status:           status,
		InstalledVersion: "1.0.0",
		LatestVersion:    "2.0.0",
		ReleaseURL:       releaseURL,
	}
}

// TestUpdatePromptScreen_ShownWhenUpdateAvailable verifies that receiving
// UpdateCheckResultMsg with HasUpdates=true transitions to ScreenUpdatePrompt.
func TestUpdatePromptScreen_ShownWhenUpdateAvailable(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")

	result := makeUpdateResult(update.UpdateAvailable, "https://github.com/releases/v2.0.0")
	updated, _ := m.Update(UpdateCheckResultMsg{Results: []update.UpdateResult{result}})
	got := updated.(Model)

	if got.Screen != ScreenUpdatePrompt {
		t.Fatalf("Screen = %v, want ScreenUpdatePrompt when update is available", got.Screen)
	}
	if !got.UpdateCheckDone {
		t.Fatal("UpdateCheckDone should be true after UpdateCheckResultMsg")
	}
}

// TestUpdatePromptScreen_SkippedWhenNoUpdate verifies that when no update is
// available, UpdateCheckResultMsg does NOT transition to ScreenUpdatePrompt.
func TestUpdatePromptScreen_SkippedWhenNoUpdate(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")

	result := makeUpdateResult(update.UpToDate, "")
	updated, _ := m.Update(UpdateCheckResultMsg{Results: []update.UpdateResult{result}})
	got := updated.(Model)

	if got.Screen == ScreenUpdatePrompt {
		t.Fatal("Screen should NOT be ScreenUpdatePrompt when no update is available")
	}
	// Should stay on Welcome (the initial screen).
	if got.Screen != ScreenWelcome {
		t.Fatalf("Screen = %v, want ScreenWelcome when no update", got.Screen)
	}
}

// TestUpdatePromptScreen_SkippedWhenCheckFailed verifies that an empty results
// slice (check failed / offline) does NOT trigger ScreenUpdatePrompt.
func TestUpdatePromptScreen_SkippedWhenCheckFailed(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")

	updated, _ := m.Update(UpdateCheckResultMsg{Results: nil})
	got := updated.(Model)

	if got.Screen == ScreenUpdatePrompt {
		t.Fatal("Screen should NOT be ScreenUpdatePrompt when update check returned nil results")
	}
	if got.Screen != ScreenWelcome {
		t.Fatalf("Screen = %v, want ScreenWelcome when check failed", got.Screen)
	}
}

// TestUpdatePromptScreen_KeyU_RunsUpgradeThenQuits verifies that pressing "u"
// on ScreenUpdatePrompt invokes UpgradeFn and on success (ExitRequested=true)
// eventually produces a tea.QuitMsg via the UpgradeDoneMsg two-step flow.
func TestUpdatePromptScreen_KeyU_RunsUpgradeThenQuits(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpdatePrompt
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "https://example.com/releases")}
	m.UpdateCheckDone = true

	upgraded := false
	m.UpgradeFn = func(_ context.Context, results []update.UpdateResult) upgrade.UpgradeReport {
		upgraded = true
		return upgrade.UpgradeReport{ExitRequested: true}
	}

	m2Raw, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if cmd == nil {
		t.Fatal("cmd should not be nil after pressing 'u' on ScreenUpdatePrompt")
	}
	m2 := m2Raw.(Model)

	// Step 1: execute the goroutine cmd → should produce UpgradeDoneMsg.
	// The cmd may be a BatchMsg (tickCmd + upgrade goroutine); search all items
	// in the batch to find the UpgradeDoneMsg rather than stopping at the first
	// non-nil result (which could be a TickMsg from the spinner).
	var msg tea.Msg
	raw := cmd()
	if batch, ok := raw.(tea.BatchMsg); ok {
		for _, fn := range batch {
			if inner := fn(); inner != nil {
				if _, isDone := inner.(UpgradeDoneMsg); isDone {
					msg = inner
					break
				}
			}
		}
		if msg == nil {
			msg = raw // fallback: use the batch result itself
		}
	} else {
		msg = raw
	}

	if !upgraded {
		t.Error("UpgradeFn should have been called when pressing 'u'")
	}

	// Step 2: feed UpgradeDoneMsg into the model returned by the keypress
	// Update (m2), not the pre-keypress model, to avoid masking false positives.
	doneMsg, ok := msg.(UpgradeDoneMsg)
	if !ok {
		t.Fatalf("expected UpgradeDoneMsg from upgrade goroutine, got %T", msg)
	}
	_, quitCmd := m2.Update(doneMsg)
	if quitCmd == nil {
		t.Fatal("cmd must not be nil after UpgradeDoneMsg with ExitRequested=true")
	}
	gotQuit := false
	if _, ok := quitCmd().(tea.QuitMsg); ok {
		gotQuit = true
	}
	if !gotQuit {
		t.Error("expected QuitMsg after UpgradeDoneMsg with ExitRequested=true")
	}
}

// TestUpdatePromptScreen_KeyC_TransitionsToWelcome verifies that pressing "c"
// on ScreenUpdatePrompt transitions to ScreenWelcome.
func TestUpdatePromptScreen_KeyC_TransitionsToWelcome(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpdatePrompt
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "")}
	m.UpdateCheckDone = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	got := updated.(Model)

	if got.Screen != ScreenWelcome {
		t.Fatalf("Screen = %v, want ScreenWelcome after pressing 'c'", got.Screen)
	}
}

// TestUpdatePromptScreen_KeyEnter_TransitionsToWelcome verifies that pressing
// Enter on ScreenUpdatePrompt with cursor on "Keep current version" (cursor=2,
// the default when entering via setScreen) transitions to ScreenWelcome.
func TestUpdatePromptScreen_KeyEnter_TransitionsToWelcome(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.setScreen(ScreenUpdatePrompt) // cursor is set to 2 (Keep current) by setScreen
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "")}
	m.UpdateCheckDone = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if got.Screen != ScreenWelcome {
		t.Fatalf("Screen = %v, want ScreenWelcome after Enter with default cursor (Keep current) on ScreenUpdatePrompt", got.Screen)
	}
}

// TestUpdatePromptScreen_KeyV_CallsOpenBrowser verifies that pressing "v" on
// ScreenUpdatePrompt calls the open-browser function with the release URL and
// the screen remains on ScreenUpdatePrompt.
func TestUpdatePromptScreen_KeyV_CallsOpenBrowser(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpdatePrompt
	releaseURL := "https://github.com/releases/v2.0.0"
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, releaseURL)}
	m.UpdateCheckDone = true

	var openedURL string
	origFn := tuiOpenBrowserFn
	tuiOpenBrowserFn = func(url string) error {
		openedURL = url
		return nil
	}
	defer func() { tuiOpenBrowserFn = origFn }()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	got := updated.(Model)

	if got.Screen != ScreenUpdatePrompt {
		t.Fatalf("Screen = %v, want ScreenUpdatePrompt to remain after 'v'", got.Screen)
	}
	if openedURL != releaseURL {
		t.Fatalf("openedURL = %q, want %q", openedURL, releaseURL)
	}
}

// TestUpdatePromptScreen_KeyV_FallsBackWhenBrowserFails verifies that when the
// open-browser function returns an error, the screen stays on ScreenUpdatePrompt
// (the URL is printed as fallback — tested by ensuring no panic and correct screen).
func TestUpdatePromptScreen_KeyV_FallsBackWhenBrowserFails(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpdatePrompt
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "https://example.com")}
	m.UpdateCheckDone = true

	origFn := tuiOpenBrowserFn
	tuiOpenBrowserFn = func(_ string) error {
		return fmt.Errorf("browser not found")
	}
	defer func() { tuiOpenBrowserFn = origFn }()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	got := updated.(Model)

	// Screen must remain on ScreenUpdatePrompt even when browser fails.
	if got.Screen != ScreenUpdatePrompt {
		t.Fatalf("Screen = %v, want ScreenUpdatePrompt after browser failure", got.Screen)
	}
}

// TestUpdatePromptScreen_OptionCount verifies that optionCount() returns 3
// for ScreenUpdatePrompt (Update / View changes / Keep current).
func TestUpdatePromptScreen_OptionCount(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpdatePrompt

	if got := m.optionCount(); got != 3 {
		t.Fatalf("optionCount() = %d, want 3 for ScreenUpdatePrompt", got)
	}
}

// TestUpdatePromptScreen_View_NonEmpty verifies that View() returns a non-empty
// string when the screen is ScreenUpdatePrompt (smoke test for the render function).
func TestUpdatePromptScreen_View_NonEmpty(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpdatePrompt
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "https://example.com")}
	m.UpdateCheckDone = true

	rendered := m.View()
	if strings.TrimSpace(rendered) == "" {
		t.Fatal("View() should return non-empty string for ScreenUpdatePrompt")
	}
}

// TestUpdatePromptScreen_ConfirmSelection_EnterEquivalent verifies that
// confirmSelection() on ScreenUpdatePrompt (cursor 2 = Keep current) navigates
// to Welcome, mirroring the "Enter" behavior exercised via handleKeyPress.
func TestUpdatePromptScreen_ConfirmSelection_EnterEquivalent(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpdatePrompt
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "")}
	m.UpdateCheckDone = true
	m.Cursor = 2 // "Keep current version"

	updated, _ := m.confirmSelection()
	got := updated.(Model)

	if got.Screen != ScreenWelcome {
		t.Fatalf("Screen = %v, want ScreenWelcome after confirmSelection cursor=2 on ScreenUpdatePrompt", got.Screen)
	}
}

// ─── Enter confirms highlighted cursor option ─────────────────────────────────

// TestUpdatePromptScreen_EnterWithCursorOnUpdate_RunsUpgrade verifies that when
// the cursor is on "Update now" (0) and Enter is pressed, the upgrade is started
// (not silently ignored or treated as keep-current).
func TestUpdatePromptScreen_EnterWithCursorOnUpdate_RunsUpgrade(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpdatePrompt
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "https://example.com/releases")}
	m.UpdateCheckDone = true
	m.Cursor = 0 // Update now

	upgraded := false
	m.UpgradeFn = func(_ context.Context, results []update.UpdateResult) upgrade.UpgradeReport {
		upgraded = true
		return upgrade.UpgradeReport{ExitRequested: true}
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("cmd should not be nil when Enter is pressed with cursor on Update now")
	}

	// Execute the command to trigger the upgrade goroutine.
	msg := cmd()
	// Accept BatchMsg: unwrap one level.
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, fn := range batch {
			fn()
		}
	}

	if !upgraded {
		t.Error("UpgradeFn should have been called when Enter is pressed with cursor on Update now (cursor=0)")
	}
}

func TestUpdatePromptScreen_UpdateNowTransitionsToVisibleUpgradeProgress(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpdatePrompt
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "https://example.com/releases")}
	m.UpdateCheckDone = true
	m.Cursor = 0
	m.UpgradeFn = func(_ context.Context, _ []update.UpdateResult) upgrade.UpgradeReport {
		return upgrade.UpgradeReport{}
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if cmd == nil {
		t.Fatal("cmd should not be nil when Update now is confirmed")
	}
	if got.Screen != ScreenUpgrade {
		t.Fatalf("Screen = %v, want ScreenUpgrade for visible upgrade progress", got.Screen)
	}
	if !got.OperationRunning {
		t.Fatal("OperationRunning must be true after confirming Update now")
	}
	view := got.View()
	if !strings.Contains(view, "Upgrading") && !strings.Contains(view, "Running") {
		t.Fatalf("upgrade progress view should show an in-progress state\nview:\n%s", view)
	}
}

// TestUpdatePromptScreen_EnterWithDefaultCursor_GoesToWelcome verifies that the
// default cursor position on ScreenUpdatePrompt is "Keep current" (2), so an
// accidental Enter press does NOT trigger an upgrade.
func TestUpdatePromptScreen_EnterWithDefaultCursor_GoesToWelcome(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	// Simulate entering ScreenUpdatePrompt via setScreen (which sets cursor=2).
	m.setScreen(ScreenUpdatePrompt)
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "")}
	m.UpdateCheckDone = true

	// Cursor should be at 2 (Keep current) after setScreen.
	if m.Cursor != 2 {
		t.Fatalf("Cursor = %d after setScreen(ScreenUpdatePrompt), want 2 (Keep current)", m.Cursor)
	}

	upgraded := false
	m.UpgradeFn = func(_ context.Context, _ []update.UpdateResult) upgrade.UpgradeReport {
		upgraded = true
		return upgrade.UpgradeReport{}
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if upgraded {
		t.Error("UpgradeFn must NOT be called when Enter is pressed on the default cursor (Keep current)")
	}
	if got.Screen != ScreenWelcome {
		t.Fatalf("Screen = %v, want ScreenWelcome after Enter with default cursor (Keep current)", got.Screen)
	}
}

// TestUpdatePromptScreen_ShortcutU_WorksRegardlessOfCursor verifies that the
// "u" shortcut triggers an upgrade even when the cursor is on a different option.
func TestUpdatePromptScreen_ShortcutU_WorksRegardlessOfCursor(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpdatePrompt
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "https://example.com/releases")}
	m.UpdateCheckDone = true
	m.Cursor = 2 // Keep current

	upgraded := false
	m.UpgradeFn = func(_ context.Context, _ []update.UpdateResult) upgrade.UpgradeReport {
		upgraded = true
		return upgrade.UpgradeReport{ExitRequested: true}
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if cmd == nil {
		t.Fatal("cmd should not be nil after pressing 'u'")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, fn := range batch {
			fn()
		}
	}

	if !upgraded {
		t.Error("UpgradeFn should have been called via 'u' shortcut regardless of cursor position")
	}
}

// TestUpdatePromptScreen_ShortcutC_WorksRegardlessOfCursor verifies that the
// "c" shortcut transitions to Welcome even when the cursor is on Update now.
func TestUpdatePromptScreen_ShortcutC_WorksRegardlessOfCursor(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpdatePrompt
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "")}
	m.UpdateCheckDone = true
	m.Cursor = 0 // Update now

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	got := updated.(Model)

	if got.Screen != ScreenWelcome {
		t.Fatalf("Screen = %v, want ScreenWelcome after pressing 'c' regardless of cursor", got.Screen)
	}
}

// ─── Upgrade error surfacing ──────────────────────────────────────────────────

// TestUpdatePromptScreen_UpgradeError_IsSurfaced verifies that when UpgradeFn
// is nil (infrastructure failure), the "u" key produces UpgradeDoneMsg with a
// non-nil Err rather than a silent QuitMsg — the error is routed through the
// existing UpgradeDoneMsg handler so it can be surfaced to the user.
func TestUpdatePromptScreen_UpgradeError_IsSurfaced(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpdatePrompt
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "https://example.com/releases")}
	m.UpdateCheckDone = true
	m.Cursor = 0
	m.UpgradeFn = nil // nil fn → startUpgrade returns UpgradeDoneMsg{Err: ...}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if cmd == nil {
		t.Fatal("cmd must not be nil after pressing 'u'")
	}

	// Execute the command: expect UpgradeDoneMsg (not a silent QuitMsg).
	// The cmd may be a BatchMsg (tickCmd + upgrade goroutine); search all items
	// to find the UpgradeDoneMsg rather than stopping at the first non-nil result.
	var msg tea.Msg
	raw := cmd()
	if batch, ok := raw.(tea.BatchMsg); ok {
		for _, fn := range batch {
			if inner := fn(); inner != nil {
				if _, isDone := inner.(UpgradeDoneMsg); isDone {
					msg = inner
					break
				}
			}
		}
		if msg == nil {
			msg = raw
		}
	} else {
		msg = raw
	}

	doneMsg, ok := msg.(UpgradeDoneMsg)
	if !ok {
		t.Fatalf("pressing 'u' must produce UpgradeDoneMsg (not %T) so errors are surfaced", msg)
	}
	if doneMsg.Err == nil {
		t.Fatal("UpgradeDoneMsg.Err must be non-nil when UpgradeFn is nil")
	}

	// Feed the UpgradeDoneMsg into the model — the error must be stored.
	updated, _ := m.Update(doneMsg)
	got := updated.(Model)

	if got.UpgradeErr == nil {
		t.Fatal("UpgradeErr must be set after UpgradeDoneMsg with non-nil Err")
	}
}

// TestUpdatePromptScreen_UpgradeSuccess_EmitsQuit verifies that when UpgradeFn
// succeeds with ExitRequested=true, a QuitMsg is eventually produced.
func TestUpdatePromptScreen_UpgradeSuccess_EmitsQuit(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpdatePrompt
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "https://example.com/releases")}
	m.UpdateCheckDone = true
	m.Cursor = 0

	m.UpgradeFn = func(_ context.Context, _ []update.UpdateResult) upgrade.UpgradeReport {
		return upgrade.UpgradeReport{ExitRequested: true}
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if cmd == nil {
		t.Fatal("cmd must not be nil after pressing 'u'")
	}

	// Execute the command to get UpgradeDoneMsg.
	// The cmd may be a BatchMsg (tickCmd + upgrade goroutine); search all items
	// to find the UpgradeDoneMsg rather than stopping at the first non-nil result.
	var msg tea.Msg
	raw := cmd()
	if batch, ok := raw.(tea.BatchMsg); ok {
		for _, fn := range batch {
			if inner := fn(); inner != nil {
				if _, isDone := inner.(UpgradeDoneMsg); isDone {
					msg = inner
					break
				}
			}
		}
		if msg == nil {
			msg = raw
		}
	} else {
		msg = raw
	}

	doneMsg, ok := msg.(UpgradeDoneMsg)
	if !ok {
		t.Fatalf("expected UpgradeDoneMsg from upgrade goroutine, got %T", msg)
	}

	// Feed the UpgradeDoneMsg into the model — should trigger tea.Quit.
	_, quitCmd := m.Update(doneMsg)
	if quitCmd == nil {
		t.Fatal("cmd must not be nil after UpgradeDoneMsg with ExitRequested=true")
	}
	gotQuit := false
	quitMsg := quitCmd()
	if _, ok := quitMsg.(tea.QuitMsg); ok {
		gotQuit = true
	}
	if !gotQuit {
		t.Error("expected QuitMsg after UpgradeDoneMsg with ExitRequested=true")
	}
}

// ─── UpdateCheckResultMsg guard: only switch from Welcome ────────────────────

// TestUpdateCheckResult_DoesNotInterruptNonWelcomeScreen verifies that when an
// update result arrives while the user is already on a screen other than Welcome,
// the TUI does NOT jump back to ScreenUpdatePrompt.
func TestUpdateCheckResult_DoesNotInterruptNonWelcomeScreen(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	// User has already navigated away from Welcome.
	m.Screen = ScreenDetection
	m.UpdateCheckDone = false

	result := makeUpdateResult(update.UpdateAvailable, "https://example.com/releases")
	updated, _ := m.Update(UpdateCheckResultMsg{Results: []update.UpdateResult{result}})
	got := updated.(Model)

	if got.Screen == ScreenUpdatePrompt {
		t.Fatal("Screen must NOT jump to ScreenUpdatePrompt when update arrives while user is not on ScreenWelcome")
	}
	if got.Screen != ScreenDetection {
		t.Fatalf("Screen = %v, want ScreenDetection (should not change when not on Welcome)", got.Screen)
	}
	if !got.UpdateCheckDone {
		t.Fatal("UpdateCheckDone should still be set to true")
	}
}

// ─── UpgradeFn nil guard ─────────────────────────────────────────────────────

// TestUpdatePromptScreen_KeyU_NilUpgradeFn_NoPanic verifies the contract when
// UpgradeFn is nil: pressing "u" must NOT panic, must NOT silently quit, and
// must produce an UpgradeDoneMsg carrying a non-nil error (so the error is
// surfaced via the normal upgrade-done path rather than lost).
func TestUpdatePromptScreen_KeyU_NilUpgradeFn_NoPanic(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpdatePrompt
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "")}
	m.UpdateCheckDone = true
	m.UpgradeFn = nil

	// Must not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic when UpgradeFn is nil: %v", r)
		}
	}()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if cmd == nil {
		t.Fatal("cmd must not be nil when UpgradeFn is nil: the contract requires an UpgradeDoneMsg to surface the error")
	}

	// The cmd may be a BatchMsg (tickCmd + upgrade goroutine); search all items
	// in the batch to find the UpgradeDoneMsg rather than stopping at the first
	// non-nil result (which could be a TickMsg from the spinner).
	var msg tea.Msg
	raw := cmd()
	if batch, ok := raw.(tea.BatchMsg); ok {
		for _, fn := range batch {
			if inner := fn(); inner != nil {
				if _, isDone := inner.(UpgradeDoneMsg); isDone {
					msg = inner
					break
				}
			}
		}
		if msg == nil {
			msg = raw // fallback: use the batch result itself
		}
	} else {
		msg = raw
	}

	// The ONLY acceptable outcome is UpgradeDoneMsg with a non-nil error.
	// A silent quit or an untyped result means the error was swallowed.
	doneMsgResult, ok := msg.(UpgradeDoneMsg)
	if !ok {
		t.Fatalf("expected UpgradeDoneMsg when UpgradeFn is nil, got %T — error must not be swallowed", msg)
	}
	if doneMsgResult.Err == nil {
		t.Error("UpgradeDoneMsg.Err must be non-nil when UpgradeFn is nil")
	}
}

// TestUpdatePromptScreen_UpdateNow_NoDuplicateUpgrade verifies that triggering
// the "Update now" action twice (or while an upgrade is already in progress)
// starts the upgrade only ONCE. The operation-in-progress guard on
// ScreenUpdatePrompt must mirror the guard on ScreenUpgrade.
func TestUpdatePromptScreen_UpdateNow_NoDuplicateUpgrade(t *testing.T) {
	callCount := 0
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenUpdatePrompt
	m.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "")}
	m.UpdateCheckDone = true
	m.UpgradeFn = func(_ context.Context, _ []update.UpdateResult) upgrade.UpgradeReport {
		callCount++
		return upgrade.UpgradeReport{}
	}

	// First trigger via "u" key — should start the upgrade and set OperationRunning.
	m1Raw, cmd1 := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if cmd1 == nil {
		t.Fatal("cmd should not be nil after first 'u' press")
	}
	m1 := m1Raw.(Model)

	if !m1.OperationRunning {
		t.Error("OperationRunning must be true after triggering update-now on ScreenUpdatePrompt")
	}

	// Second trigger while OperationRunning=true — must be a no-op (no new cmd, no second goroutine).
	m2Raw, cmd2 := m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m2 := m2Raw.(Model)

	if cmd2 != nil {
		// Execute to check whether it would invoke UpgradeFn a second time.
		msg := cmd2()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, fn := range batch {
				fn()
			}
		}
	}
	_ = m2

	// Execute the first cmd so UpgradeFn runs (exactly once across all batch items).
	raw1 := cmd1()
	if batch, ok := raw1.(tea.BatchMsg); ok {
		for _, fn := range batch {
			fn()
		}
	}

	if callCount != 1 {
		t.Errorf("UpgradeFn call count = %d, want exactly 1 (duplicate upgrade guard failed)", callCount)
	}

	// Also verify via Enter key (cursor=0) on the original model — same guard must apply.
	m3 := NewModel(system.DetectionResult{}, "dev")
	m3.setScreen(ScreenUpdatePrompt)
	m3.Cursor = 0 // "Update now"
	m3.UpdateResults = []update.UpdateResult{makeUpdateResult(update.UpdateAvailable, "")}
	m3.UpdateCheckDone = true
	enterCallCount := 0
	m3.UpgradeFn = func(_ context.Context, _ []update.UpdateResult) upgrade.UpgradeReport {
		enterCallCount++
		return upgrade.UpgradeReport{}
	}

	m3aRaw, cmd3a := m3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3a := m3aRaw.(Model)
	if !m3a.OperationRunning {
		t.Error("OperationRunning must be true after Enter on cursor=0 (Update now) on ScreenUpdatePrompt")
	}
	if cmd3a == nil {
		t.Fatal("first Enter on Update now should return a command")
	}
	if batch, ok := cmd3a().(tea.BatchMsg); ok {
		for _, fn := range batch {
			if fn != nil {
				fn()
			}
		}
	}

	// Second Enter while in progress — must be no-op.
	_, cmd3b := m3a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd3b != nil {
		msg := cmd3b()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, fn := range batch {
				fn()
			}
		}
	}

	if enterCallCount != 1 {
		t.Errorf("UpgradeFn call count via Enter = %d, want exactly 1 (Enter must start exactly one upgrade)", enterCallCount)
	}
}

// ─── Unit 1+2: pickerFlowSlice, pickerNextScreen, pickerPreviousScreen ──────

// ─── Unit 3: applyPickerEntry ─────────────────────────────────────────────

// ─── Unit 4: TestPickerBackRowRegression ─────────────────────────────────────
//
// These tests are the RED gate for Unit 5 (forward call-site rewrites) and
// Unit 6 (back call-site rewrites). They cover the 4 pre-existing
// inconsistencies between goBack (Esc) and confirmSelection (Enter on Back row).
// Cases 3, 4, 5, 6 MUST FAIL before Units 5/6 are implemented.
// Cases 1, 2 may already pass; they are included as regression guards.

// TestStrictTDDForward verifies the StrictTDD Continue path for retained flow variants.
// Custom flows go to SkillPicker or Review; non-custom flows advance to the
// dependency tree.
func TestStrictTDDForward(t *testing.T) {
	tests := []struct {
		name       string
		setup      func() Model
		wantScreen Screen
	}{
		{
			name: "non-custom StrictTDD Enable goes to DependencyTree",
			setup: func() Model {
				m := NewModel(system.DetectionResult{}, "dev")
				m.Screen = ScreenStrictTDD
				m.Selection.Preset = model.PresetFullGentleman
				m.Selection.Agents = []model.AgentID{model.AgentCursor}
				m.Selection.Components = []model.ComponentID{model.ComponentSDD}
				m.Cursor = screens.StrictTDDOptionEnable
				return m
			},
			wantScreen: ScreenDependencyTree,
		},
		{
			name: "custom without Skills StrictTDD Enable goes to Review",
			setup: func() Model {
				m := NewModel(system.DetectionResult{}, "dev")
				m.Screen = ScreenStrictTDD
				m.Selection.Preset = model.PresetCustom
				m.Selection.Agents = []model.AgentID{model.AgentCursor}
				m.Selection.Components = []model.ComponentID{model.ComponentSDD}
				m.Cursor = screens.StrictTDDOptionEnable
				return m
			},
			wantScreen: ScreenReview,
		},
		{
			name: "custom with Skills StrictTDD Enable goes to SkillPicker",
			setup: func() Model {
				m := NewModel(system.DetectionResult{}, "dev")
				m.Screen = ScreenStrictTDD
				m.Selection.Preset = model.PresetCustom
				m.Selection.Agents = []model.AgentID{model.AgentCursor}
				m.Selection.Components = []model.ComponentID{model.ComponentSDD, model.ComponentSkills}
				m.Cursor = screens.StrictTDDOptionEnable
				return m
			},
			wantScreen: ScreenSkillPicker,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated, _ := tt.setup().Update(tea.KeyMsg{Type: tea.KeyEnter})
			if got := updated.(Model).Screen; got != tt.wantScreen {
				t.Fatalf("screen = %v, want %v", got, tt.wantScreen)
			}
		})
	}
}

func TestCodexModelPickerCustomConfirmSignalsOrchestratorClear(t *testing.T) {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenCodexModelPicker
	m.ModelConfigMode = true
	m.CodexModelPicker = screens.NewCodexModelPickerState()
	m.CodexModelPicker.CustomMode = screens.CodexCustomModePhaseList
	m.Selection.CodexOrchestratorAssignment = model.CodexPresetOrchestratorAssignment(string(model.CodexPresetRecommended))
	m.Cursor = 14 // Confirm row after the 14 phases.

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)
	if state.Selection.CodexOrchestratorAssignment != nil {
		t.Fatalf("custom confirmation retained curated orchestrator: %#v", state.Selection.CodexOrchestratorAssignment)
	}
	if !state.Selection.ClearCodexOrchestratorAssignment {
		t.Fatal("custom confirmation did not signal persisted orchestrator clear")
	}
	if state.PendingSyncOverrides == nil || !state.PendingSyncOverrides.ClearCodexOrchestratorAssignment {
		t.Fatal("custom confirmation did not propagate clear signal to sync overrides")
	}
}

// screenName renders a Screen value as a stable string for subtest names.
func screenName(s Screen) string {
	return fmt.Sprintf("Screen#%d", int(s))
}

func setNoAnimationEnv(t *testing.T, value *string) {
	t.Helper()
	const name = "GENTLE_AI_NO_ANIMATION"
	previous, wasSet := os.LookupEnv(name)
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv(name, previous)
		} else {
			_ = os.Unsetenv(name)
		}
	})

	var err error
	if value == nil {
		err = os.Unsetenv(name)
	} else {
		err = os.Setenv(name, *value)
	}
	if err != nil {
		t.Fatalf("set %s: %v", name, err)
	}
}

func executeSingleNoAnimationCommand(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected operation command")
	}

	raw := cmd()
	if batch, ok := raw.(tea.BatchMsg); ok {
		if len(batch) != 1 {
			t.Fatalf("no-animation operation batch contains %d commands, want 1", len(batch))
		}
		if batch[0] == nil {
			t.Fatal("no-animation operation batch contains a nil command")
		}
		return batch[0]()
	}
	return raw
}

func TestTickMsg_NoAnimationRequiresExactOne(t *testing.T) {
	one := "1"
	zero := "0"
	empty := ""
	truthy := "true"
	tests := []struct {
		name     string
		value    *string
		wantStep int
		wantCmd  bool
	}{
		{name: "exact one disables animation", value: &one, wantStep: 3, wantCmd: false},
		{name: "unset preserves animation", value: nil, wantStep: 4, wantCmd: true},
		{name: "empty preserves animation", value: &empty, wantStep: 4, wantCmd: true},
		{name: "zero preserves animation", value: &zero, wantStep: 4, wantCmd: true},
		{name: "other value preserves animation", value: &truthy, wantStep: 4, wantCmd: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setNoAnimationEnv(t, tt.value)
			m := NewModel(system.DetectionResult{}, "dev")
			m.Screen = ScreenUpgrade
			m.OperationRunning = true
			m.SpinnerFrame = 3

			updated, cmd := m.Update(TickMsg{})
			state := updated.(Model)
			if state.SpinnerFrame != tt.wantStep {
				t.Fatalf("spinner frame = %d, want %d", state.SpinnerFrame, tt.wantStep)
			}
			if (cmd != nil) != tt.wantCmd {
				t.Fatalf("tick command present = %t, want %t", cmd != nil, tt.wantCmd)
			}
		})
	}
}

func TestNoAnimationPreservesSyncOperationCommand(t *testing.T) {
	one := "1"
	setNoAnimationEnv(t, &one)

	called := false
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenSync
	m.SyncFn = func(_ *model.SyncOverrides) ([]string, error) {
		called = true
		return []string{"changed"}, nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)
	if !state.OperationRunning {
		t.Fatal("sync operation should start with animation disabled")
	}

	msg := executeSingleNoAnimationCommand(t, cmd)
	done, ok := msg.(SyncDoneMsg)
	if !ok {
		t.Fatalf("operation message = %T, want SyncDoneMsg", msg)
	}
	if done.Err != nil {
		t.Fatalf("sync returned unexpected error: %v", done.Err)
	}
	if !called {
		t.Fatal("sync operation command was not executed")
	}
}
