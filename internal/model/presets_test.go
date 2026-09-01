package model

import (
	"slices"
	"testing"
)

func TestComponentsForPresetRetainsPersonalStack(t *testing.T) {
	want := []ComponentID{
		ComponentEngram,
		ComponentSDD,
		ComponentSkills,
		ComponentContext7,
		ComponentPermission,
		ComponentPersona,
	}
	got := ComponentsForPreset(PresetFullGentleman, PersonaGentleman)
	if !slices.Equal(got, want) {
		t.Fatalf("ComponentsForPreset() = %v, want %v", got, want)
	}
}

func TestComponentsForPresetCustomPersonaOmitsPersona(t *testing.T) {
	got := ComponentsForPreset(PresetEcosystemOnly, PersonaCustom)
	want := []ComponentID{ComponentEngram, ComponentSDD, ComponentSkills, ComponentContext7}
	if !slices.Equal(got, want) {
		t.Fatalf("ComponentsForPreset() = %v, want %v", got, want)
	}
}
