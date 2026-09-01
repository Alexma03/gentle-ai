package model

import (
	"reflect"
	"testing"
)

func TestPersonalClientDefinitionsAreExactlyFive(t *testing.T) {
	want := []AgentID{AgentClaudeCode, AgentCodex, AgentCursor, AgentAntigravity, AgentPi}
	got := PersonalClientIDs()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PersonalClientIDs() = %v, want %v", got, want)
	}
	if len(PersonalClientDefinitions()) != len(want) {
		t.Fatalf("PersonalClientDefinitions() length = %d, want %d", len(PersonalClientDefinitions()), len(want))
	}
}

func TestPersonalClientDefinitionsAreDefensiveCopies(t *testing.T) {
	definitions := PersonalClientDefinitions()
	definitions[0].Name = "mutated"
	definitions[0].ConfigPath = "mutated"
	ids := PersonalClientIDs()
	ids[0] = AgentOpenCode

	definition, ok := PersonalClientDefinitionFor(AgentClaudeCode)
	if !ok || definition.Name != "Claude Code" || definition.ConfigPath != "~/.claude" {
		t.Fatalf("canonical Claude definition = %#v, found=%v", definition, ok)
	}
	if got := PersonalClientIDs()[0]; got != AgentClaudeCode {
		t.Fatalf("canonical first client = %q, want %q", got, AgentClaudeCode)
	}
}

func TestLegacyAgentIDsRemainMigrationAddressableButNotPersonal(t *testing.T) {
	if IsPersonalClient(AgentOpenCode) {
		t.Fatal("OpenCode must remain outside the retained personal-client set")
	}
}
