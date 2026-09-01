package codegraph

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	piagent "github.com/gentleman-programming/gentle-ai/v2/internal/agents/pi"
)

// piCodeGraphPathSet is the component-owned set of paths CodeGraph may
// inspect or reconcile for Pi. The Pi adapter supplies only its client path
// projection; discovery and precedence remain CodeGraph policy.
type piCodeGraphPathSet struct {
	AgentDir  string
	MCPConfig string
	Manifest  string
}

type piCodeGraphDiscoveredChild struct {
	Name         string
	Source       string
	Target       string
	PackageOwned bool
}

var piWalkDir = filepath.WalkDir

func piCodeGraphPaths(homeDir string) piCodeGraphPathSet {
	agentDir := piagent.ConfiguredAgentDir(homeDir)
	return piCodeGraphPathSet{
		AgentDir:  agentDir,
		MCPConfig: piagent.ConfiguredMCPPath(agentDir),
		Manifest:  filepath.Join(homeDir, ".gentle-ai", "pi-codegraph.json"),
	}
}

// effectivePiCodeGraphMCPPath resolves the MCP configuration Pi will apply
// for a workspace. Later Pi discovery locations override an earlier CodeGraph
// server; malformed or unreadable participating configuration fails closed.
func effectivePiCodeGraphMCPPath(homeDir, workspaceDir string) (string, error) {
	paths := piCodeGraphPaths(homeDir)
	candidates := []string{
		filepath.Join(homeDir, ".config", "mcp", "mcp.json"),
		paths.MCPConfig,
	}
	if workspaceDir != "" {
		candidates = append(candidates,
			filepath.Join(workspaceDir, ".mcp.json"),
			filepath.Join(workspaceDir, ".pi", "mcp.json"),
		)
	}

	effective := paths.MCPConfig
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("read effective Pi MCP config %q: %w", path, err)
		}
		root := map[string]any{}
		if err := json.Unmarshal(data, &root); err != nil {
			return "", fmt.Errorf("parse effective Pi MCP config %q: %w", path, err)
		}
		servers, ok := root["mcpServers"].(map[string]any)
		if !ok && root["mcpServers"] != nil {
			return "", fmt.Errorf("parse effective Pi MCP config %q: mcpServers must be an object", path)
		}
		if _, configured := servers["codegraph"]; configured {
			effective = path
		}
	}
	return effective, nil
}

// discoverPiCodeGraphChildren resolves Pi's user then project child
// directories. Later directories override an earlier child with the same
// normalized name. Package children are copied to an overlay target instead
// of being mutated in place.
func discoverPiCodeGraphChildren(homeDir, workspaceDir string) ([]piCodeGraphDiscoveredChild, error) {
	paths := piCodeGraphPaths(homeDir)
	dirs := []string{
		filepath.Join(paths.AgentDir, "node_modules"),
		filepath.Join(paths.AgentDir, "agents"),
		filepath.Join(paths.AgentDir, "subagents"),
	}
	if workspaceDir != "" {
		dirs = append(dirs,
			filepath.Join(workspaceDir, ".pi", "agents"),
			filepath.Join(workspaceDir, ".pi", "subagents"),
		)
	}
	byName := map[string]piCodeGraphDiscoveredChild{}
	for _, dir := range dirs {
		candidates, err := piChildFiles(dir)
		if err != nil {
			return nil, err
		}
		for _, source := range candidates {
			name := normalizePiCodeGraphChildIdentity(strings.TrimSuffix(filepath.Base(source), ".md"))
			packageOwned := strings.Contains(filepath.ToSlash(source), "/node_modules/")
			target := source
			if packageOwned {
				target = filepath.Join(paths.AgentDir, "subagents", filepath.Base(source))
			}
			byName[name] = piCodeGraphDiscoveredChild{Name: name, Source: source, Target: target, PackageOwned: packageOwned}
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	slices.Sort(names)
	children := make([]piCodeGraphDiscoveredChild, 0, len(names))
	for _, name := range names {
		children = append(children, byName[name])
	}
	return children, nil
}

func normalizePiCodeGraphChildIdentity(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func piChildFiles(dir string) ([]string, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("read Pi child directory %q: %w", dir, err)
	}
	files := []string{}
	err := piWalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Ext(entry.Name()) != ".md" {
			return nil
		}
		parent := filepath.Base(filepath.Dir(path))
		if parent == "agents" || parent == "subagents" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read Pi child directory %q: %w", dir, err)
	}
	slices.Sort(files)
	return files, nil
}
