package catalog

import "github.com/gentleman-programming/gentle-ai/v2/internal/model"

type Agent struct {
	ID         model.AgentID
	Name       string
	Tier       model.SupportTier
	ConfigPath string
	Binary     string
}

// AllAgents projects the canonical personal-client registry for display and
// selection. The model definitions are the single source of identity metadata;
// this package deliberately has no second runtime allowlist.
func AllAgents() []Agent {
	definitions := model.PersonalClientDefinitions()
	agents := make([]Agent, 0, len(definitions))
	for _, definition := range definitions {
		agents = append(agents, Agent{
			ID:         definition.ID,
			Name:       definition.Name,
			Tier:       definition.Tier,
			ConfigPath: definition.ConfigPath,
			Binary:     definition.Binary,
		})
	}
	return agents
}

// MVPAgents is retained as a source-compatible name for callers that used the
// former MVP catalog. The personal stack now has one canonical five-client
// selection surface, so it returns the same projection as AllAgents.
func MVPAgents() []Agent { return AllAgents() }

func IsMVPAgent(agent model.AgentID) bool { return IsSupportedAgent(agent) }

func IsSupportedAgent(agent model.AgentID) bool { return model.IsPersonalClient(agent) }
