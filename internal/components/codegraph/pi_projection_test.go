package codegraph

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPiCodeGraphPathsResolveConfiguredAgentDirectory(t *testing.T) {
	home := t.TempDir()
	configured := filepath.Join(home, "custom-pi")
	t.Setenv("PI_CODING_AGENT_DIR", configured)

	paths := piCodeGraphPaths(home)
	if paths.AgentDir != configured {
		t.Fatalf("AgentDir = %q, want %q", paths.AgentDir, configured)
	}
	if paths.MCPConfig != filepath.Join(configured, "mcp.json") {
		t.Fatalf("MCPConfig = %q", paths.MCPConfig)
	}
	if paths.Manifest != filepath.Join(home, ".gentle-ai", "pi-codegraph.json") {
		t.Fatalf("Manifest = %q", paths.Manifest)
	}
}

func TestPiCodeGraphPolicyKeepsAgentDirectoryWhenProjectMCPOverrides(t *testing.T) {
	home := t.TempDir()
	configured := filepath.Join(home, "custom-pi")
	workspace := filepath.Join(home, "project")
	t.Setenv("PI_CODING_AGENT_DIR", configured)
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".mcp.json"), []byte(`{"mcpServers":{"codegraph":{}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	paths := piCodeGraphPaths(home)
	effective, err := effectivePiCodeGraphMCPPath(home, workspace)
	if err != nil {
		t.Fatal(err)
	}
	if paths.AgentDir != configured || effective != filepath.Join(workspace, ".mcp.json") {
		t.Fatalf("agent=%q effective=%q, want configured agent and project config", paths.AgentDir, effective)
	}
}

func TestDiscoverPiCodeGraphChildrenUsesProjectOverrideAndPreservesPackageSource(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(home, "project")
	mustWrite := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(filepath.Join(home, ".pi", "agent", "subagents", "worker.md"), "---\ntools: bash\n---\npackage worker\n")
	mustWrite(filepath.Join(workspace, ".pi", "subagents", "worker.md"), "---\ntools: bash, mcp\n---\nproject worker\n")
	mustWrite(filepath.Join(home, ".pi", "agent", "agents", "reader.md"), "---\ntools: read\n---\nreader\n")
	mustWrite(filepath.Join(home, ".pi", "agent", "node_modules", "gentle-pi", "subagents", "package-worker.md"), "---\ntools: bash\n---\npackage worker\n")

	children, err := discoverPiCodeGraphChildren(home, workspace)
	if err != nil {
		t.Fatalf("discoverPiCodeGraphChildren() error = %v", err)
	}
	if len(children) != 3 {
		t.Fatalf("children = %#v, want three effective children", children)
	}
	if children[0].Name != "package-worker" || children[1].Name != "reader" || children[2].Name != "worker" {
		t.Fatalf("children = %#v, want sorted package-worker, reader, and worker", children)
	}
	if !children[0].PackageOwned || children[0].Target == children[0].Source {
		t.Fatalf("package worker = %#v, want owned overlay", children[0])
	}
	if children[2].Source != filepath.Join(workspace, ".pi", "subagents", "worker.md") || children[2].PackageOwned {
		t.Fatalf("worker = %#v, want project effective child", children[2])
	}
}

func TestDiscoverPiCodeGraphChildrenReturnsUnreadableDirectoryError(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".pi", "agent", "subagents"), 0o755); err != nil {
		t.Fatal(err)
	}
	previous := piWalkDir
	piWalkDir = func(path string, walkFn fs.WalkDirFunc) error {
		return &fs.PathError{Op: "readdir", Path: path, Err: fs.ErrPermission}
	}
	t.Cleanup(func() { piWalkDir = previous })

	_, err := discoverPiCodeGraphChildren(home, "")
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("discoverPiCodeGraphChildren() error = %v, want unreadable-directory error", err)
	}
}

func TestDiscoverPiCodeGraphChildrenUsesNormalizedRuntimeIdentity(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(home, "project")
	for path, body := range map[string]string{
		filepath.Join(home, ".pi", "agent", "subagents", "Worker.md"): "---\ntools: bash\n---\nuser\n",
		filepath.Join(workspace, ".pi", "subagents", "worker.md"):     "---\ntools: bash\n---\nproject\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	children, err := discoverPiCodeGraphChildren(home, workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 1 || children[0].Source != filepath.Join(workspace, ".pi", "subagents", "worker.md") {
		t.Fatalf("children = %#v, want one project runtime identity", children)
	}
}
