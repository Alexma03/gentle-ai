package agents

import (
	"fmt"
	"slices"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

type Registry struct {
	adapters    map[model.AgentID]Adapter
	manifests   map[model.AgentID]AgentCapabilityManifest
	definitions map[model.AgentID]Definition
}

// Definition is the immutable identity projection exposed by a Registry.
// Adapter behavior remains behind Adapter(id); callers cannot register or
// mutate through a returned definition.
type Definition struct {
	ID         model.AgentID
	Name       string
	Tier       model.SupportTier
	ConfigPath string
	Binary     string
}

func NewRegistry(adapters ...Adapter) (*Registry, error) {
	r := &Registry{
		adapters:    map[model.AgentID]Adapter{},
		manifests:   map[model.AgentID]AgentCapabilityManifest{},
		definitions: map[model.AgentID]Definition{},
	}
	for _, adapter := range adapters {
		if err := r.Register(adapter); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func (r *Registry) Register(adapter Adapter) error {
	if adapter == nil {
		return fmt.Errorf("adapter is nil")
	}

	agent := adapter.Agent()
	if _, exists := r.adapters[agent]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateAdapter, agent)
	}

	manifest, err := ResolveCapabilityManifest(adapter)
	if err != nil {
		return fmt.Errorf("register adapter %q: %w", agent, err)
	}

	r.adapters[agent] = adapter
	r.manifests[agent] = manifest
	definition := Definition{ID: agent, Tier: adapter.Tier()}
	if canonical, ok := model.PersonalClientDefinitionFor(agent); ok {
		definition.Name = canonical.Name
		definition.ConfigPath = canonical.ConfigPath
		definition.Binary = canonical.Binary
	} else {
		// Legacy adapters remain constructible for state migration and rollback,
		// but are intentionally not part of NewDefaultRegistry's inventory.
		definition.Name = string(agent)
	}
	r.definitions[agent] = definition
	return nil
}

func (r *Registry) Get(agent model.AgentID) (Adapter, bool) {
	adapter, ok := r.adapters[agent]
	return adapter, ok
}

// Adapter returns the registered adapter for agent. It is the named registry
// lookup used by lifecycle consumers; Get remains as a compatibility alias.
func (r *Registry) Adapter(agent model.AgentID) (Adapter, bool) {
	return r.Get(agent)
}

// Definitions returns a stable, defensive snapshot of the registry's client
// identity metadata. The registry owns all mutation and definitions are sorted
// by ID for deterministic planning and diagnostics.
func (r *Registry) Definitions() []Definition {
	ids := r.SupportedAgents()
	definitions := make([]Definition, 0, len(ids))
	for _, id := range ids {
		definition := r.definitions[id]
		definitions = append(definitions, definition)
	}
	return definitions
}

// Validate rejects any selection that is not present in this registry. It
// never drops unknown IDs, which keeps migration decisions explicit.
func (r *Registry) Validate(ids []model.AgentID) error {
	for _, id := range ids {
		if _, ok := r.adapters[id]; !ok {
			return AgentNotSupportedError{Agent: id}
		}
	}
	return nil
}

func (r *Registry) CapabilityManifest(agent model.AgentID) (AgentCapabilityManifest, bool) {
	manifest, ok := r.manifests[agent]
	return manifest, ok
}

func (r *Registry) SupportedAgents() []model.AgentID {
	ids := make([]model.AgentID, 0, len(r.adapters))
	for id := range r.adapters {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}
