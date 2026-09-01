package agents

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func TestDefaultRegistryIsExactlyThePersonalFive(t *testing.T) {
	registry, err := NewDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	want := model.PersonalClientIDs()
	got := registry.SupportedAgents()
	if !reflect.DeepEqual(got, sortedIDs(want)) {
		t.Fatalf("SupportedAgents() = %v, want canonical five %v", got, sortedIDs(want))
	}
	definitions := registry.Definitions()
	if len(definitions) != len(want) {
		t.Fatalf("Definitions() length = %d, want %d", len(definitions), len(want))
	}
	for _, definition := range definitions {
		canonical, ok := model.PersonalClientDefinitionFor(definition.ID)
		if !ok || definition.Name != canonical.Name || definition.Tier != canonical.Tier || definition.ConfigPath != canonical.ConfigPath {
			t.Fatalf("definition = %#v, canonical=%#v found=%v", definition, canonical, ok)
		}
		if _, ok := registry.Adapter(definition.ID); !ok {
			t.Fatalf("Adapter(%q) missing", definition.ID)
		}
	}
}

func TestDefaultRegistryValidateRejectsRetiredSelection(t *testing.T) {
	registry, err := NewDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Validate([]model.AgentID{model.AgentClaudeCode, model.AgentOpenCode}); !errors.Is(err, ErrAgentNotSupported) {
		t.Fatalf("Validate() error = %v, want retired selection rejection", err)
	}
}

func sortedIDs(ids []model.AgentID) []model.AgentID {
	copyIDs := append([]model.AgentID(nil), ids...)
	for i := 1; i < len(copyIDs); i++ {
		for j := i; j > 0 && copyIDs[j] < copyIDs[j-1]; j-- {
			copyIDs[j], copyIDs[j-1] = copyIDs[j-1], copyIDs[j]
		}
	}
	return copyIDs
}
