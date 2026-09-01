package screens

import (
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/runtimecatalog"
)

// makeTestState builds a minimal ModelPickerState with one provider and models
// so that handleModelNav can reach the "enter" branch.
func makeTestState(phaseIdx int) *ModelPickerState {
	const providerID = "test-provider"
	testModels := []runtimecatalog.Model{
		{ID: "model-alpha", Name: "Alpha Model"},
		{ID: "model-beta", Name: "Beta Model"},
	}
	return &ModelPickerState{
		Mode:             ModeModelSelect,
		SelectedPhaseIdx: phaseIdx,
		SelectedProvider: providerID,
		SDDModels:        map[string][]runtimecatalog.Model{providerID: testModels},
		ModelCursor:      0, // always pick the first model for simplicity
	}
}

// ─── ModelPickerRows ───────────────────────────────────────────────────────

func TestRenderModelPickerScrollsToReviewAgents(t *testing.T) {
	rows := ModelPickerRows()
	cursor := len(rows) - 1
	state := ModelPickerState{AvailableIDs: []string{"openai"}}

	output := RenderModelPicker(nil, state, cursor)
	if !strings.Contains(output, "review-refuter") || !strings.Contains(output, "↑ more assignments") {
		t.Fatalf("review rows are not visible at cursor %d:\n%s", cursor, output)
	}
	if strings.Contains(output, "gentle-orchestrator") {
		t.Fatalf("picker did not window rows around review cursor:\n%s", output)
	}
}

func TestModelPickerRows_OrchestratorIsFirst(t *testing.T) {
	rows := ModelPickerRows()
	if rows[0] != "gentle-orchestrator" {
		t.Fatalf("ModelPickerRows()[0] = %q, want %q", rows[0], "gentle-orchestrator")
	}
}

func TestModelPickerRows_SetAllIsSecond(t *testing.T) {
	rows := ModelPickerRows()
	if rows[1] != "Set all SDD phases" {
		t.Fatalf("ModelPickerRows()[1] = %q, want %q", rows[1], "Set all SDD phases")
	}
}

// ─── handleModelNav: orchestrator row (idx 0) ──────────────────────────────

func TestHandleModelNav_OrchestratorRow_ModelValues(t *testing.T) {
	state := makeTestState(0)
	assignments := make(map[string]model.ModelAssignment)

	_, updated := handleModelNav("enter", state, assignments)

	orch := updated[SDDOrchestratorPhase]
	if orch.ProviderID != "test-provider" {
		t.Errorf("ProviderID = %q, want %q", orch.ProviderID, "test-provider")
	}
	if orch.ModelID != "model-alpha" {
		t.Errorf("ModelID = %q, want %q", orch.ModelID, "model-alpha")
	}
}

// ─── handleModelNav: "Set all phases" row (idx 1) ──────────────────────────

func TestHandleModelNav_SetAllPhasesRow_DoesNotOverwriteExistingOrchestrator(t *testing.T) {
	state := makeTestState(1) // row 1 = "Set all phases"

	// Pre-set orchestrator with a different assignment
	existing := model.ModelAssignment{ProviderID: "existing-provider", ModelID: "existing-model"}
	assignments := map[string]model.ModelAssignment{
		SDDOrchestratorPhase: existing,
	}

	_, updated := handleModelNav("enter", state, assignments)

	// The orchestrator assignment must remain untouched
	orch := updated[SDDOrchestratorPhase]
	if orch.ProviderID != "existing-provider" || orch.ModelID != "existing-model" {
		t.Errorf("orchestrator assignment should be unchanged; got: %v", orch)
	}
}

// ─── handleModelNav: sub-agent rows (idx 2+) ───────────────────────────────

// ─── SDDOrchestratorPhase constant ────────────────────────────────────────

func TestSDDOrchestratorPhaseConstant(t *testing.T) {
	if SDDOrchestratorPhase != "gentle-orchestrator" {
		t.Fatalf("SDDOrchestratorPhase = %q, want %q", SDDOrchestratorPhase, "gentle-orchestrator")
	}
}

// ─── Issue #146: "Set all phases" label must not change when individual phase selected ─

// TestSetAllPhasesLabelSeparateFromIndividualPhases verifies that the ModelPickerState
// has a dedicated AllPhasesModel field that only gets updated when "Set all phases"
// is selected (row idx 1), NOT when an individual sub-agent phase (idx >= 2) is selected.
//
// The "Set all phases" row label should show AllPhasesModel, not phases[0].
//
// Closes #146.

// TestSetAllPhasesSetsAllPhasesModelField verifies that selecting "Set all phases"
// sets AllPhasesModel on the state to the chosen model assignment.
//
// Closes #146.

// ─── ModeEffortSelect constant ────────────────────────────────────────────

func TestModeEffortSelectConstantValue(t *testing.T) {
	// ModeEffortSelect must be 3 (the 4th constant after 0, 1, 2).
	if ModeEffortSelect != 3 {
		t.Fatalf("ModeEffortSelect = %d, want 3", ModeEffortSelect)
	}
}

// ─── makeTestStateReasoning helper ────────────────────────────────────────

// makeTestStateReasoning is like makeTestState but includes a reasoning model.
func makeTestStateReasoning(phaseIdx int) *ModelPickerState {
	const providerID = "test-provider"
	testModels := []runtimecatalog.Model{
		{ID: "model-reason", Name: "Reasoning Model", Reasoning: true, ToolCall: true, Variants: []string{"high", "low", "medium"}},
		{ID: "model-plain", Name: "Plain Model", Reasoning: false, ToolCall: true},
	}
	return &ModelPickerState{
		Mode:             ModeModelSelect,
		SelectedPhaseIdx: phaseIdx,
		SelectedProvider: providerID,
		SDDModels:        map[string][]runtimecatalog.Model{providerID: testModels},
		ModelCursor:      0, // reasoning model is first
	}
}

// ─── handleModelNav: reasoning model triggers ModeEffortSelect ────────────

func TestHandleModelNav_ReasoningModelSetsModeEffortSelect(t *testing.T) {
	state := makeTestStateReasoning(2) // any sub-agent row
	assignments := make(map[string]model.ModelAssignment)

	handled, _ := handleModelNav("enter", state, assignments)

	if !handled {
		t.Fatal("handleModelNav should return handled=true on enter")
	}
	if state.Mode != ModeEffortSelect {
		t.Errorf("Mode after selecting reasoning model = %v, want ModeEffortSelect (%d)", state.Mode, ModeEffortSelect)
	}
	// PendingAssignment must be populated with provider + model
	if state.PendingAssignment.ProviderID == "" {
		t.Error("PendingAssignment.ProviderID should be set after selecting reasoning model")
	}
	if state.PendingAssignment.ModelID != "model-reason" {
		t.Errorf("PendingAssignment.ModelID = %q, want %q", state.PendingAssignment.ModelID, "model-reason")
	}
}

// TestHandleModelNav_ReasoningModelWithoutVariantsSkipsEffortPicker covers the
// runtime scenario where a reasoning-capable model has no reported variants.
// The picker must skip ModeEffortSelect instead of presenting an empty list.

// ─── applyAssignment helper ──────────────────────────────────────────────

// ─── handleEffortNav ──────────────────────────────────────────────────────

func TestHandleEffortNav_EscReturnsModeModelSelect(t *testing.T) {
	state := ModelPickerState{
		Mode:                      ModeEffortSelect,
		SelectedPhaseIdx:          2,
		PendingAssignment:         model.ModelAssignment{ProviderID: "anthropic", ModelID: "claude-opus-4"},
		SelectedModelEffortLevels: []string{"low", "medium", "high"},
	}
	assignments := make(map[string]model.ModelAssignment)

	newState, _ := handleEffortNav("esc", state, assignments)

	if newState.Mode != ModeModelSelect {
		t.Errorf("Mode after esc = %v, want ModeModelSelect", newState.Mode)
	}
	if newState.PendingAssignment != (model.ModelAssignment{}) {
		t.Errorf("PendingAssignment after esc = %+v, want zero value", newState.PendingAssignment)
	}
	if newState.SelectedModelEffortLevels != nil {
		t.Errorf("SelectedModelEffortLevels after esc = %v, want nil", newState.SelectedModelEffortLevels)
	}
}

func TestHandleEffortNav_NavigationUpdatesEffortCursor(t *testing.T) {
	// options: ["default", "low", "medium", "high"] — 4 items
	state := ModelPickerState{
		Mode:                      ModeEffortSelect,
		EffortCursor:              0,
		SelectedModelEffortLevels: []string{"low", "medium", "high"},
	}
	assignments := make(map[string]model.ModelAssignment)

	newState, _ := handleEffortNav("j", state, assignments)
	if newState.EffortCursor != 1 {
		t.Errorf("after j: EffortCursor = %d, want 1", newState.EffortCursor)
	}

	newState, _ = handleEffortNav("k", newState, assignments)
	if newState.EffortCursor != 0 {
		t.Errorf("after k: EffortCursor = %d, want 0", newState.EffortCursor)
	}
}

// ─── HandleModelPickerNav dispatches ModeEffortSelect ─────────────────────

// ─── handleEffortNav: "Set all phases" row (SelectedPhaseIdx==1) ──────────────

// TestHandleEffortNav_SetAllPhasesUpdatesAllPhasesModelAndAllSubAgents verifies
// that when the effort picker is confirmed via the "Set all phases" row
// (SelectedPhaseIdx==1), ALL 10 SDD sub-agent phases receive the effort assignment
// AND state.AllPhasesModel is updated to reflect the chosen effort.
//
// This covers the interaction between the effort picker and the "Set all phases"
// special row — a path not exercised by the single-phase tests above.

// ─── TestIndividualPhaseSelectionDoesNotSetAllPhasesModel (unchanged) ──────

// ─── Phase list display — effort annotation ───────────────────────────────

// ─── TestIndividualPhaseSelectionDoesNotSetAllPhasesModel (unchanged) ──────

// TestIndividualPhaseSelectionDoesNotSetAllPhasesModel verifies that selecting
// a model for any individual sub-agent phase does NOT update AllPhasesModel.
//
// Closes #146.

// ─── Separator row (non-selectable) ────────────────────────────────────────

func TestHandleModelNav_SeparatorRow_NoAssignment(t *testing.T) {
	sepIdx := SeparatorRowIdx()
	state := makeTestState(sepIdx)
	assignments := make(map[string]model.ModelAssignment)

	handled, updated := handleModelNav("enter", state, assignments)

	if !handled {
		t.Fatal("handleModelNav should return handled=true on enter for separator")
	}

	// Separator should produce NO assignments at all.
	if len(updated) != 0 {
		t.Fatalf("separator row should produce no assignments; got: %v", updated)
	}

	// State should return to phase list.
	if state.Mode != ModePhaseList {
		t.Fatalf("expected ModePhaseList after separator enter, got %d", state.Mode)
	}
}

// ─── JD agent rows ─────────────────────────────────────────────────────────

// ─── ModelPickerRowsForProfile ──────────────────────────────────────────

func TestModelPickerRowsForState_WithCustomAgents(t *testing.T) {
	state := ModelPickerState{
		CustomAgents: []string{"my-custom-agent-1", "my-custom-agent-2"},
	}
	rows := ModelPickerRowsForState(state)

	var hasCustomSep, hasSetAllCustom, hasAgent1, hasAgent2 bool
	for _, r := range rows {
		if r == "--- Custom / Native agents ---" {
			hasCustomSep = true
		}
		if r == "Set all custom agents" {
			hasSetAllCustom = true
		}
		if r == "my-custom-agent-1" {
			hasAgent1 = true
		}
		if r == "my-custom-agent-2" {
			hasAgent2 = true
		}
	}

	if !hasCustomSep || !hasSetAllCustom || !hasAgent1 || !hasAgent2 {
		t.Fatalf("ModelPickerRowsForState() missing custom agent rows: got %v", rows)
	}
}

func TestModelPickerRowsForState_ProfileExcludesCustomAgents(t *testing.T) {
	state := ModelPickerState{
		ForProfile:   true,
		CustomAgents: []string{"custom-profile-agent"},
	}
	want := ModelPickerRowsForProfile()
	got := ModelPickerRowsForState(state)

	if len(got) != len(want) {
		t.Fatalf("ModelPickerRowsForState(profile) len = %d, want %d; got %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ModelPickerRowsForState(profile)[%d] = %q, want %q; got %v", i, got[i], want[i], got)
		}
	}
}

func TestApplyAssignment_SetAllCustomAgents(t *testing.T) {
	state := ModelPickerState{
		CustomAgents:     []string{"custom-1", "custom-2"},
		SelectedPhaseIdx: 0,
	}
	rows := ModelPickerRowsForState(state)
	setAllIdx := -1
	for i, r := range rows {
		if r == "Set all custom agents" {
			setAllIdx = i
			break
		}
	}
	if setAllIdx < 0 {
		t.Fatalf("Set all custom agents row not found in %v", rows)
	}

	state.SelectedPhaseIdx = setAllIdx
	assignments := make(map[string]model.ModelAssignment)
	assigned := model.ModelAssignment{ProviderID: "anthropic", ModelID: "claude-3-5-haiku"}

	res := applyAssignment(state, assignments, assigned)
	if res["custom-1"] != assigned || res["custom-2"] != assigned {
		t.Fatalf("applyAssignment Set all custom agents = %v, want assigned %v for custom-1 and custom-2", res, assigned)
	}
}

func TestHandleModelPickerNav_SetAllCustomAgentsWithoutEffortAssignsEachAgent(t *testing.T) {
	state := makeTestState(0)
	state.CustomAgents = []string{"custom-1", "custom-2"}
	for i, row := range ModelPickerRowsForState(*state) {
		if row == "Set all custom agents" {
			state.SelectedPhaseIdx = i
			break
		}
	}

	handled, assignments := HandleModelPickerNav("enter", state, nil)
	want := model.ModelAssignment{ProviderID: "test-provider", ModelID: "model-alpha"}
	if !handled || assignments["custom-1"] != want || assignments["custom-2"] != want {
		t.Fatalf("HandleModelPickerNav() handled=%v assignments=%v, want each custom agent assigned %v", handled, assignments, want)
	}
	if state.AllCustomAgentsModel != want {
		t.Fatalf("AllCustomAgentsModel = %v, want %v", state.AllCustomAgentsModel, want)
	}
}

func TestClearModelPickerAssignment_SetAllCustomAgents(t *testing.T) {
	state := ModelPickerState{
		CustomAgents:         []string{"custom-1", "custom-2"},
		AllCustomAgentsModel: model.ModelAssignment{ProviderID: "openai", ModelID: "gpt-4o-mini"},
	}
	rows := ModelPickerRowsForState(state)
	setAllIdx := -1
	for i, r := range rows {
		if r == "Set all custom agents" {
			setAllIdx = i
			break
		}
	}
	if setAllIdx < 0 {
		t.Fatalf("Set all custom agents row not found in %v", rows)
	}

	state.SelectedPhaseIdx = setAllIdx
	assignments := map[string]model.ModelAssignment{
		"custom-1": {ProviderID: "openai", ModelID: "gpt-4o-mini"},
		"custom-2": {ProviderID: "openai", ModelID: "gpt-4o-mini"},
	}

	res := ClearModelPickerAssignment(&state, assignments)
	if _, ok := res["custom-1"]; ok {
		t.Error("custom-1 should be cleared")
	}
	if _, ok := res["custom-2"]; ok {
		t.Error("custom-2 should be cleared")
	}
	if state.AllCustomAgentsModel.ProviderID != "" {
		t.Errorf("AllCustomAgentsModel = %v, want empty after clear", state.AllCustomAgentsModel)
	}
}

func TestHandleModelPickerNav_CustomAgentLabelsDoNotChangeRowIdentity(t *testing.T) {
	for _, tt := range []struct {
		name      string
		agentName string
	}{
		{name: "bulk label", agentName: "Set all custom agents"},
		{name: "custom separator label", agentName: "--- Custom / Native agents ---"},
		{name: "review separator label", agentName: "--- Review agents ---"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state := makeTestState(0)
			state.CustomAgents = []string{tt.agentName, "other-custom-agent"}
			selectedIdx := -1
			for index, row := range ModelPickerRowsForStateWithIdentity(*state) {
				if row.Kind == ModelPickerRowKindAgent && row.AgentID == tt.agentName {
					selectedIdx = index
					break
				}
			}
			if selectedIdx < 0 {
				t.Fatalf("custom agent row %q not found", tt.agentName)
			}
			state.SelectedPhaseIdx = selectedIdx

			old := model.ModelAssignment{ProviderID: "old-provider", ModelID: "old-model", Effort: "high"}
			assignments := map[string]model.ModelAssignment{"other-custom-agent": old}
			handled, got := HandleModelPickerNav("enter", state, assignments)
			want := model.ModelAssignment{ProviderID: "test-provider", ModelID: "model-alpha"}
			if !handled || got[tt.agentName] != want {
				t.Fatalf("HandleModelPickerNav() handled=%v assignment=%v, want %v for %q", handled, got[tt.agentName], want, tt.agentName)
			}
			if got["other-custom-agent"] != old {
				t.Fatalf("collision row changed another custom agent: got %v, want %v", got["other-custom-agent"], old)
			}
			if state.AllCustomAgentsModel != (model.ModelAssignment{}) {
				t.Fatalf("custom agent row changed bulk state: %v", state.AllCustomAgentsModel)
			}
		})
	}
}

func TestHandleModelPickerNav_CustomAgentBulkLabelPreservesEffortPath(t *testing.T) {
	state := makeTestState(0)
	state.CustomAgents = []string{"Set all custom agents", "other-custom-agent"}
	state.SDDModels["test-provider"][0].Variants = []string{"low", "high"}
	for index, row := range ModelPickerRowsForStateWithIdentity(*state) {
		if row.Kind == ModelPickerRowKindAgent && row.AgentID == "Set all custom agents" {
			state.SelectedPhaseIdx = index
			break
		}
	}
	old := model.ModelAssignment{ProviderID: "old-provider", ModelID: "old-model", Effort: "high"}
	assignments := map[string]model.ModelAssignment{"other-custom-agent": old}

	handled, assignments := HandleModelPickerNav("enter", state, assignments)
	if !handled || state.Mode != ModeEffortSelect {
		t.Fatalf("enter should open effort selection: handled=%v mode=%v", handled, state.Mode)
	}
	handled, assignments = HandleModelPickerNav("enter", state, assignments)
	want := model.ModelAssignment{ProviderID: "test-provider", ModelID: "model-alpha"}
	if !handled || assignments["Set all custom agents"] != want {
		t.Fatalf("effort selection assignment = %v, want %v", assignments["Set all custom agents"], want)
	}
	if assignments["other-custom-agent"] != old {
		t.Fatalf("effort selection changed another custom agent: got %v, want %v", assignments["other-custom-agent"], old)
	}
	if state.AllCustomAgentsModel != (model.ModelAssignment{}) {
		t.Fatalf("effort selection changed bulk state: %v", state.AllCustomAgentsModel)
	}
}

func TestClearModelPickerAssignment_CustomAgentBulkLabelPreservesOtherRows(t *testing.T) {
	state := ModelPickerState{
		CustomAgents:         []string{"Set all custom agents", "other-custom-agent"},
		AllCustomAgentsModel: model.ModelAssignment{ProviderID: "openai", ModelID: "gpt-4o-mini"},
	}
	for index, row := range ModelPickerRowsForStateWithIdentity(state) {
		if row.Kind == ModelPickerRowKindAgent && row.AgentID == "Set all custom agents" {
			state.SelectedPhaseIdx = index
			break
		}
	}
	bulk := state.AllCustomAgentsModel
	assignments := map[string]model.ModelAssignment{
		"Set all custom agents": bulk,
		"other-custom-agent":    bulk,
	}

	result := ClearModelPickerAssignment(&state, assignments)
	if _, ok := result["Set all custom agents"]; ok {
		t.Fatal("custom agent named like the bulk row should be cleared")
	}
	if result["other-custom-agent"] != bulk {
		t.Fatalf("clear changed another custom agent: got %v, want %v", result["other-custom-agent"], bulk)
	}
	if state.AllCustomAgentsModel != bulk {
		t.Fatalf("custom agent clear changed bulk state: got %v, want %v", state.AllCustomAgentsModel, bulk)
	}
}
