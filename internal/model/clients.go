package model

// PersonalClientDefinition is the canonical identity metadata shared by the
// runtime registry, catalog, and config discovery. Keeping path and display
// facts here prevents each consumer from growing a subtly different allowlist.
// The returned definitions are always copies; callers cannot mutate the
// canonical inventory.
type PersonalClientDefinition struct {
	ID         AgentID
	Name       string
	Tier       SupportTier
	ConfigPath string
	Binary     string
}

var personalClientDefinitions = []PersonalClientDefinition{
	{ID: AgentClaudeCode, Name: "Claude Code", Tier: TierFull, ConfigPath: "~/.claude", Binary: "claude"},
	{ID: AgentCodex, Name: "Codex", Tier: TierFull, ConfigPath: "~/.codex", Binary: "codex"},
	{ID: AgentCursor, Name: "Cursor", Tier: TierFull, ConfigPath: "~/.cursor"},
	{ID: AgentAntigravity, Name: "Google Antigravity", Tier: TierFull, ConfigPath: "~/.gemini/antigravity-cli"},
	{ID: AgentPi, Name: "Pi", Tier: TierFull, ConfigPath: "~/.pi", Binary: "pi"},
}

// PersonalClientDefinitions returns a defensive copy of the exact five
// retained clients in stable display order.
func PersonalClientDefinitions() []PersonalClientDefinition {
	return append([]PersonalClientDefinition(nil), personalClientDefinitions...)
}

// PersonalClientIDs returns the canonical runtime client inventory for the
// personal Gentle stack. Legacy AgentID constants remain declared so state
// migration and rollback can describe old selections, but they are not
// selectable through the current registry.
func PersonalClientIDs() []AgentID {
	ids := make([]AgentID, 0, len(personalClientDefinitions))
	for _, definition := range personalClientDefinitions {
		ids = append(ids, definition.ID)
	}
	return ids
}

func PersonalClientDefinitionFor(id AgentID) (PersonalClientDefinition, bool) {
	for _, definition := range personalClientDefinitions {
		if definition.ID == id {
			return definition, true
		}
	}
	return PersonalClientDefinition{}, false
}

func IsPersonalClient(id AgentID) bool {
	_, ok := PersonalClientDefinitionFor(id)
	return ok
}
