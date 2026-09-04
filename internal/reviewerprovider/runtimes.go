package reviewerprovider

import "github.com/gentleman-programming/gentle-ai/v2/internal/model"

// registeredRuntimeIdentities is deliberately closed. A runtime appears here
// only after the compiled review boundary admits it: Claude's prompt-carried
// generated reviewer, Codex's advisory scratch process, and the host-mediated
// relays owned by the retained client runtimes. Consumers of the published contract
// bundle verify this list offline before trusting a runtime identity; prompt
// prose never expands it.
var registeredRuntimeIdentities = []string{
	"claude-code",
	"codex",
	"pi",
}

// RegisteredRuntimeIdentities returns a copy of every runtime identity the
// provider contract admits, in stable lexical order.
func RegisteredRuntimeIdentities() []string {
	return append([]string(nil), registeredRuntimeIdentities...)
}

// RegisteredRuntime reports whether agent belongs to the closed set admitted
// by the provider contract.
func RegisteredRuntime(agent model.AgentID) bool {
	for _, identity := range registeredRuntimeIdentities {
		if identity == string(agent) {
			return true
		}
	}
	return false
}
