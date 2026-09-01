package system

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func TestScanConfigsUsesCanonicalPersonalClientSet(t *testing.T) {
	configs := ScanConfigs(t.TempDir())
	got := make([]model.AgentID, 0, len(configs))
	for _, config := range configs {
		got = append(got, model.AgentID(config.Agent))
	}
	if !reflect.DeepEqual(got, model.PersonalClientIDs()) {
		t.Fatalf("ScanConfigs() agents = %v, want canonical set %v", got, model.PersonalClientIDs())
	}
}

// TestScanConfigs_ReturnsAllKnownAgentsWithExistsFlag verifies the canonical
// ScanConfigs contract: ALL known registry agents are returned, with Exists=true
// for those whose config dir is present on disk and Exists=false for those absent.
//
// This is the TUI contract: the detection screen shows "present"/"missing" for
// every known agent. The shim must enumerate all adapters from the default
// registry, not just the ones that are installed.
func TestScanConfigs_ReturnsAllKnownAgentsWithExistsFlag(t *testing.T) {
	home := t.TempDir()

	// Create only claude-code config dir — others intentionally absent.
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	configs := ScanConfigs(home)

	// The scan is driven by the exact retained five-client registry.
	if len(configs) != 5 {
		t.Fatalf("ScanConfigs() returned %d entries, want 5; got %v", len(configs), configs)
	}

	// Find claude — must be Exists=true.
	var claudeState *ConfigState
	for i := range configs {
		if configs[i].Agent == "claude-code" {
			claudeState = &configs[i]
			break
		}
	}
	if claudeState == nil {
		t.Fatalf("ScanConfigs() missing claude-code entry; got %v", configs)
	}
	if !claudeState.Exists {
		t.Errorf("ScanConfigs() claude-code Exists = false, want true (dir was created)")
	}
	if !claudeState.IsDirectory {
		t.Errorf("ScanConfigs() claude-code IsDirectory = false, want true")
	}

	// All other agents must appear with Exists=false (their dirs weren't created).
	existsTrueCount := 0
	for _, c := range configs {
		if c.Exists {
			existsTrueCount++
		}
	}
	if existsTrueCount != 1 {
		t.Errorf("ScanConfigs() Exists=true count = %d, want 1 (only claude-code created); states: %v", existsTrueCount, configs)
	}
}

// TestScanConfigs_AgentFieldMatchesModelAgentID verifies that the Agent field
// in each ConfigState matches the canonical model.AgentID string values used
// by the TUI and validate.go switch statements.
func TestScanConfigs_AgentFieldMatchesModelAgentID(t *testing.T) {
	home := t.TempDir()
	configs := ScanConfigs(home)

	// These are the AgentID string values the TUI switch statements check.
	knownAgents := map[string]bool{
		"claude-code": false,
		"codex":       false,
		"cursor":      false,
		"antigravity": false,
		"pi":          false,
	}

	for _, c := range configs {
		if _, known := knownAgents[c.Agent]; known {
			knownAgents[c.Agent] = true
		}
	}

	// All known agents must appear.
	for agent, seen := range knownAgents {
		if !seen {
			t.Errorf("ScanConfigs() missing agent %q — TUI switch statements require it; got agents: %v", agent, agentNames(configs))
		}
	}
}

// TestScanConfigs_PathFieldIsNonEmpty verifies that every ConfigState has a
// non-empty Path — the TUI and validate.go use the path for display purposes.
func TestScanConfigs_PathFieldIsNonEmpty(t *testing.T) {
	home := t.TempDir()
	configs := ScanConfigs(home)

	for _, c := range configs {
		if c.Path == "" {
			t.Errorf("ScanConfigs() agent %q has empty Path — all entries must have non-empty paths", c.Agent)
		}
	}
}

// TestScanConfigs_ExistsFalseWhenDirAbsent verifies that agents whose
// GlobalConfigDir does not exist on disk have Exists=false.
func TestScanConfigs_ExistsFalseWhenDirAbsent(t *testing.T) {
	home := t.TempDir()
	// No dirs created — all agents should have Exists=false.

	configs := ScanConfigs(home)

	for _, c := range configs {
		if c.Exists {
			t.Errorf("ScanConfigs() agent %q Exists = true, want false (no dirs created)", c.Agent)
		}
	}
}

// TestScanConfigs_IsDirectorySetForExistingDirs verifies that IsDirectory is
// set correctly for existing directories.

// agentNames extracts agent name strings for error messages.
func agentNames(configs []ConfigState) []string {
	names := make([]string, len(configs))
	for i, c := range configs {
		names[i] = c.Agent
	}
	return names
}
