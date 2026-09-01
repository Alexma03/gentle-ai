package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gentleman-programming/gentle-ai/v2/internal/cli"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/system"
	"github.com/gentleman-programming/gentle-ai/v2/internal/tui/screens"
)

func piSDDReviewModel(background model.PiBackgroundIntent) Model {
	m := NewModel(system.DetectionResult{}, "dev")
	m.Screen = ScreenReview
	m.Cursor = 0
	m.Selection.Agents = []model.AgentID{model.AgentPi}
	m.Selection.Components = []model.ComponentID{model.ComponentEngram}
	m.PiBackgroundIntent = background
	return m
}

func TestPiBackgroundPromptVisibility(t *testing.T) {
	t.Setenv(cli.PiBackgroundSubagentsEnv, "")
	m := piSDDReviewModel("")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model)
	if state.Screen != ScreenPiBackground {
		t.Fatalf("screen = %v, want ScreenPiBackground", state.Screen)
	}
	if !strings.Contains(state.View(), "Enable managed background subagents") {
		t.Fatalf("pi background prompt missing enable choice:\n%s", state.View())
	}
}

func TestPiBackgroundPriorStateSkipsPrompt(t *testing.T) {
	t.Setenv(cli.PiBackgroundSubagentsEnv, "")
	for _, want := range []model.PiBackgroundIntent{model.PiBackgroundOn, model.PiBackgroundOff} {
		t.Run(string(want), func(t *testing.T) {
			m := piSDDReviewModel(want)
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			state := updated.(Model)
			if state.Screen != ScreenInstalling {
				t.Fatalf("screen = %v, want ScreenInstalling", state.Screen)
			}
			if state.PiBackgroundIntent != want || state.PiBackgroundPersist != "" {
				t.Fatalf("pi background intent/persist = %q/%q, want %q/empty", state.PiBackgroundIntent, state.PiBackgroundPersist, want)
			}
			if cmd == nil {
				t.Fatal("install command = nil")
			}
		})
	}
}

func TestPiBackgroundCancellationLeavesChoiceUnchanged(t *testing.T) {
	t.Setenv(cli.PiBackgroundSubagentsEnv, "")
	for _, tt := range []struct {
		name string
		key  tea.KeyMsg
	}{
		{name: "escape", key: tea.KeyMsg{Type: tea.KeyEsc}},
		{name: "back option", key: tea.KeyMsg{Type: tea.KeyEnter}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := piSDDReviewModel("")
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			state := updated.(Model)
			if state.Screen != ScreenPiBackground {
				t.Fatalf("screen = %v, want ScreenPiBackground", state.Screen)
			}
			if tt.name == "back option" {
				state.Cursor = len(screens.PiBackgroundOptions())
			}

			updated, _ = state.Update(tt.key)
			state = updated.(Model)
			if state.Screen != ScreenReview || state.PiBackgroundIntent != "" || state.PiBackgroundPersist != "" {
				t.Fatalf("cancelled state = %v/%q/%q, want review/empty/empty", state.Screen, state.PiBackgroundIntent, state.PiBackgroundPersist)
			}
		})
	}
}
