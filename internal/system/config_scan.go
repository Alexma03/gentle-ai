package system

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

// ConfigState records the filesystem presence of an agent's global config directory.
// All known registry agents are always represented — Exists=false for absent dirs.
// This contract is consumed by the TUI detection screen and install/validate flows.
type ConfigState struct {
	Agent       string
	Path        string
	Exists      bool
	IsDirectory bool
}

// knownAgentConfigDirs projects the canonical personal-client definitions for
// presence scanning. Keeping this in model (rather than duplicating a 16-item
// switch here) means TUI discovery and runtime registry selection cannot drift.
func knownAgentConfigDirs(homeDir string) []ConfigState {
	definitions := model.PersonalClientDefinitions()
	states := make([]ConfigState, 0, len(definitions))
	for _, definition := range definitions {
		states = append(states, ConfigState{
			Agent: string(definition.ID),
			Path:  expandConfigPath(homeDir, definition.ConfigPath),
		})
	}
	return states
}

func expandConfigPath(homeDir, path string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, filepath.FromSlash(strings.TrimPrefix(path, "~/")))
	}
	return path
}

// ScanConfigs returns the presence state of every known managed agent's global
// This is a compatibility shim: it preserves the ConfigState contract for TUI
// and validation callers while the canonical discovery (agents.DiscoverInstalled)
// is used by sync and upgrade flows. Full delegation is deferred until the
// system ← agents import cycle is resolved (follow-up change).
func ScanConfigs(homeDir string) []ConfigState {
	states := knownAgentConfigDirs(homeDir)

	for idx := range states {
		info, err := os.Stat(states[idx].Path)
		if err != nil {
			continue
		}

		states[idx].Exists = true
		states[idx].IsDirectory = info.IsDir()
	}

	return states
}
