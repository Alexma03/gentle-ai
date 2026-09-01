package agents

import (
	"reflect"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func TestFactoryResolvesPiAdapter(t *testing.T) {
	adapter, err := NewAdapter(model.AgentPi)
	if err != nil {
		t.Fatalf("NewAdapter(%q) returned error: %v", model.AgentPi, err)
	}

	if got := adapter.Agent(); got != model.AgentPi {
		t.Fatalf("adapter.Agent() = %q, want %q", got, model.AgentPi)
	}
}

func TestDefaultRegistryIncludesPi(t *testing.T) {
	registry, err := NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() returned error: %v", err)
	}

	adapter, ok := registry.Get(model.AgentPi)
	if !ok {
		t.Fatalf("registry missing %s adapter", model.AgentPi)
	}

	if got := adapter.Agent(); got != model.AgentPi {
		t.Fatalf("registry adapter.Agent() = %q, want %q", got, model.AgentPi)
	}
}

func TestFactoryResolvesAntigravityAdapter(t *testing.T) {
	adapter, err := NewAdapter(model.AgentAntigravity)
	if err != nil {
		t.Fatalf("NewAdapter(%q) returned error: %v", model.AgentAntigravity, err)
	}

	if got := adapter.Agent(); got != model.AgentAntigravity {
		t.Fatalf("adapter.Agent() = %q, want %q", got, model.AgentAntigravity)
	}
}

func TestDefaultRegistryIncludesAntigravity(t *testing.T) {
	registry, err := NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() returned error: %v", err)
	}

	adapter, ok := registry.Get(model.AgentAntigravity)
	if !ok {
		t.Fatalf("registry missing %s adapter", model.AgentAntigravity)
	}

	if got := adapter.Agent(); got != model.AgentAntigravity {
		t.Fatalf("registry adapter.Agent() = %q, want %q", got, model.AgentAntigravity)
	}
}

func TestDefaultRegistrySupportedAgentsMatchesFactoryAgents(t *testing.T) {
	registry, err := NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() returned error: %v", err)
	}

	want := []model.AgentID{model.AgentAntigravity, model.AgentClaudeCode, model.AgentCodex, model.AgentCursor, model.AgentPi}

	if got := registry.SupportedAgents(); !reflect.DeepEqual(got, want) {
		t.Fatalf("SupportedAgents() = %v, want %v", got, want)
	}
}
