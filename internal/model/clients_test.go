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
