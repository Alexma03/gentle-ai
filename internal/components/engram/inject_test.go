package engram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/agents"
	"github.com/gentleman-programming/gentle-ai/v2/internal/agents/antigravity"
	"github.com/gentleman-programming/gentle-ai/v2/internal/agents/claude"
	"github.com/gentleman-programming/gentle-ai/v2/internal/agents/codex"
	"github.com/gentleman-programming/gentle-ai/v2/internal/agents/pi"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func claudeAdapter() agents.Adapter { return claude.NewAdapter() }

func codexAdapter() agents.Adapter { return codex.NewAdapter() }

func validCodexRuntime(t *testing.T) {
	t.Helper()
	t.Cleanup(codex.SetRuntimeVersionCommandForTest("codex-cli 0.144.0", nil))
}

func antigravityAdapter() agents.Adapter {
	return antigravity.NewAdapter()
}

func piAdapter() agents.Adapter { return pi.NewAdapter() }

// assertArgsHaveToolsAgent is a shared helper that validates a JSON file
// contains the MCP "engram" entry with --tools=agent in args.
func assertArgsHaveToolsAgent(t *testing.T, path string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	text := string(content)
	if !strings.Contains(text, `"--tools=agent"`) {
		t.Fatalf("file %q missing --tools=agent in args; got:\n%s", path, text)
	}
}

func TestInjectClaudeWritesUserRegistryPreservingUnrelatedData(t *testing.T) {
	home := t.TempDir()
	mockEngramLookPath(t, "/opt/homebrew/bin/engram", "")
	registryPath := claude.UserConfigPath(home)
	seed := []byte(`{"oauthAccount":{"emailAddress":"user@example.com"},"projects":{"/repo":{"allowedTools":[]}},"mcpServers":{"codegraph":{"command":"codegraph"}}}`)
	if err := os.WriteFile(registryPath, seed, 0o644); err != nil {
		t.Fatalf("WriteFile(user registry) error = %v", err)
	}

	result, err := Inject(home, claudeAdapter())
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("Inject() changed = false")
	}

	registry := readJSONFile(t, registryPath)
	assertNestedString(t, registry, "user@example.com", "oauthAccount", "emailAddress")
	assertNestedString(t, registry, "codegraph", "mcpServers", "codegraph", "command")
	assertNestedString(t, registry, "/opt/homebrew/bin/engram", "mcpServers", "engram", "command")
	assertNestedStrings(t, registry, []string{"mcp", "--tools=agent"}, "mcpServers", "engram", "args")
	if info, statErr := os.Stat(registryPath); statErr != nil {
		t.Fatalf("Stat(user registry) error = %v", statErr)
	} else if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("user registry mode = %o; want 0600", info.Mode().Perm())
	}
	legacyPath := filepath.Join(home, ".claude", "mcp", "engram.json")
	if _, statErr := os.Stat(legacyPath); !os.IsNotExist(statErr) {
		t.Fatalf("fresh injection must not create legacy config %q; stat error = %v", legacyPath, statErr)
	}
}

func TestInjectClaudeRefusesCorruptUserRegistryWithoutMutation(t *testing.T) {
	home := t.TempDir()
	registryPath := claude.UserConfigPath(home)
	corrupt := []byte("{ not json")
	if err := os.WriteFile(registryPath, corrupt, 0o600); err != nil {
		t.Fatalf("WriteFile(corrupt registry) error = %v", err)
	}

	if _, err := Inject(home, claudeAdapter()); err == nil {
		t.Fatal("Inject() error = nil; want corrupt registry refusal")
	}
	after, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("ReadFile(corrupt registry) error = %v", err)
	}
	if !bytes.Equal(after, corrupt) {
		t.Fatalf("corrupt registry mutated: got %q, want %q", after, corrupt)
	}
}

func TestInjectClaudeWritesProtocolSection(t *testing.T) {
	home := t.TempDir()

	_, err := Inject(home, claudeAdapter())
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	claudeMDPath := filepath.Join(home, ".claude", "CLAUDE.md")
	content, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("ReadFile(CLAUDE.md) error = %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "<!-- gentle-ai:engram-protocol -->") {
		t.Fatal("CLAUDE.md missing open marker for engram-protocol")
	}
	if !strings.Contains(text, "<!-- /gentle-ai:engram-protocol -->") {
		t.Fatal("CLAUDE.md missing close marker for engram-protocol")
	}
	// Real content check.
	if !strings.Contains(text, "mem_save") {
		t.Fatal("CLAUDE.md missing real engram protocol content (expected 'mem_save')")
	}
	if !strings.Contains(text, "needs_review") {
		t.Fatal("CLAUDE.md missing memory lifecycle stale-context rule (expected 'needs_review')")
	}
}

func TestInjectClaudeIsIdempotent(t *testing.T) {
	home := t.TempDir()

	first, err := Inject(home, claudeAdapter())
	if err != nil {
		t.Fatalf("Inject() first error = %v", err)
	}
	if !first.Changed {
		t.Fatalf("Inject() first changed = false")
	}
	registryPath := claude.UserConfigPath(home)
	afterFirst, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("ReadFile(user registry after first injection) error = %v", err)
	}

	second, err := Inject(home, claudeAdapter())
	if err != nil {
		t.Fatalf("Inject() second error = %v", err)
	}
	if second.Changed {
		t.Fatalf("Inject() second changed = true")
	}
	afterSecond, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("ReadFile(user registry after second injection) error = %v", err)
	}
	if !bytes.Equal(afterFirst, afterSecond) {
		t.Fatal("second injection must leave the user registry byte-identical")
	}
}

func TestInjectPiProvisioningCreatesMissingMCPAdapterFiles(t *testing.T) {
	home := t.TempDir()

	result, err := Inject(home, piAdapter())
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("Inject() changed = false")
	}

	settings := readJSONFile(t, filepath.Join(home, ".pi", "agent", "settings.json"))
	assertNestedStrings(t, settings, []string{"npm:pi-subagents@0.65.0", "npm:pi-mcp-adapter"}, "packages")

	npmPackage := readJSONFile(t, filepath.Join(home, ".pi", "npm", "package.json"))
	assertNestedString(t, npmPackage, "^2.6.0", "dependencies", "pi-mcp-adapter")
}

func TestInjectPiProvisioningPreservesUnrelatedContent(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".pi", "agent", "settings.json"), `{"theme":"kanagawa","packages":["npm:other@1.0.0"]}`)
	writeFile(t, filepath.Join(home, ".pi", "npm", "package.json"), `{"name":"pi-user","dependencies":{"left-pad":"^1.0.0"},"devDependencies":{"vitest":"^1.0.0"}}`)

	_, err := Inject(home, piAdapter())
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	settings := readJSONFile(t, filepath.Join(home, ".pi", "agent", "settings.json"))
	assertNestedString(t, settings, "kanagawa", "theme")
	assertNestedStringsUnordered(t, settings, []string{"npm:other@1.0.0", "npm:pi-subagents@0.65.0", "npm:pi-mcp-adapter"}, "packages")

	npmPackage := readJSONFile(t, filepath.Join(home, ".pi", "npm", "package.json"))
	assertNestedString(t, npmPackage, "pi-user", "name")
	assertNestedString(t, npmPackage, "^1.0.0", "dependencies", "left-pad")
	assertNestedString(t, npmPackage, "^2.6.0", "dependencies", "pi-mcp-adapter")
	assertNestedString(t, npmPackage, "^1.0.0", "devDependencies", "vitest")
}

func TestInjectPiProvisioningCanonicalizesExistingEntriesAndIsIdempotent(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".pi", "agent", "settings.json"), `{"packages":["npm:pi-mcp-adapter@2.0.0"]}`)
	writeFile(t, filepath.Join(home, ".pi", "npm", "package.json"), `{"dependencies":{"pi-mcp-adapter":"^2.0.0"}}`)

	first, err := Inject(home, piAdapter())
	if err != nil {
		t.Fatalf("Inject() first error = %v", err)
	}
	if !first.Changed {
		t.Fatalf("Inject() first changed = false")
	}

	settings := readJSONFile(t, filepath.Join(home, ".pi", "agent", "settings.json"))
	assertNestedStrings(t, settings, []string{"npm:pi-subagents@0.65.0", "npm:pi-mcp-adapter"}, "packages")
	npmPackage := readJSONFile(t, filepath.Join(home, ".pi", "npm", "package.json"))
	assertNestedString(t, npmPackage, "^2.6.0", "dependencies", "pi-mcp-adapter")

	second, err := Inject(home, piAdapter())
	if err != nil {
		t.Fatalf("Inject() second error = %v", err)
	}
	if second.Changed {
		t.Fatalf("Inject() second changed = true")
	}
}

func TestInjectPiProvisioningMigratesLegacyObjectPackages(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".pi", "agent", "settings.json"), `{"theme":"kanagawa","packages":{"npm:other":"1.0.0","npm:pi-mcp-adapter":"2.0.0"}}`)

	_, err := Inject(home, piAdapter())
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	settings := readJSONFile(t, filepath.Join(home, ".pi", "agent", "settings.json"))
	assertNestedString(t, settings, "kanagawa", "theme")
	assertNestedStringsUnordered(t, settings, []string{"npm:other@1.0.0", "npm:pi-subagents@0.65.0", "npm:pi-mcp-adapter"}, "packages")
}

// TestInjectOpenCodeMigratesFromOldFormat verifies that when a user's
// opencode.json contains the old v1.11.3 format (separate "args" key),
// Inject() replaces mcp.engram atomically so that "args" is absent and
// "command" is an array — the format required by OpenCode 1.3.3+.

func TestInjectCursorMergesEngramToSettings(t *testing.T) {
	home := t.TempDir()

	cursorAdapter, err := agents.NewAdapter("cursor")
	if err != nil {
		t.Fatalf("NewAdapter(cursor) error = %v", err)
	}

	result, injectErr := Inject(home, cursorAdapter)
	if injectErr != nil {
		t.Fatalf("Inject(cursor) error = %v", injectErr)
	}

	// Cursor uses MCPConfigFile strategy — engram gets merged into mcp.json.
	if !result.Changed {
		t.Fatalf("Inject(cursor) changed = false")
	}
}

func TestInjectCursorWithMalformedMCPJsonRecovery(t *testing.T) {
	// Real Windows users may have a ~/.cursor/mcp.json that starts with non-JSON
	// content (e.g. "allow: all" or just "a"). The installer should recover by
	// treating the broken file as {} and proceeding with the overlay merge.
	home := t.TempDir()

	cursorAdapter, err := agents.NewAdapter("cursor")
	if err != nil {
		t.Fatalf("NewAdapter(cursor) error = %v", err)
	}

	// Pre-create ~/.cursor/mcp.json with invalid (non-JSON) content.
	mcpPath := cursorAdapter.MCPConfigPath(home, "engram")
	if err := os.MkdirAll(filepath.Dir(mcpPath), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(mcpPath, []byte("allow: all"), 0o644); err != nil {
		t.Fatalf("WriteFile(malformed mcp.json) error = %v", err)
	}

	result, injectErr := Inject(home, cursorAdapter)
	if injectErr != nil {
		t.Fatalf("Inject(cursor) with malformed mcp.json error = %v; want nil (should recover)", injectErr)
	}
	if !result.Changed {
		t.Fatalf("Inject(cursor) changed = false; want true")
	}

	content, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("ReadFile(mcp.json) error = %v", err)
	}

	text := string(content)
	if !strings.Contains(text, `"mcpServers"`) {
		t.Fatalf("mcp.json missing mcpServers key after recovery; got:\n%s", text)
	}
	if !strings.Contains(text, `"engram"`) {
		t.Fatalf("mcp.json missing engram server after recovery; got:\n%s", text)
	}
}

// ─── Gemini tests ─────────────────────────────────────────────────────────────

func TestInjectAntigravityWritesMCPToCLIConfig(t *testing.T) {
	home := t.TempDir()

	result, err := Inject(home, antigravityAdapter())
	if err != nil {
		t.Fatalf("Inject(antigravity) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("Inject(antigravity) changed = false")
	}

	cliMCPPath := filepath.Join(home, ".gemini", "antigravity-cli", "mcp_config.json")
	content, err := os.ReadFile(cliMCPPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", cliMCPPath, err)
	}
	text := string(content)
	if !strings.Contains(text, `"args": [`) || !strings.Contains(text, `"mcp"`) {
		t.Fatalf("Antigravity MCP config must launch Engram MCP; got:\n%s", text)
	}
	if strings.Contains(text, `--tools=`) {
		t.Fatalf("Antigravity should use Engram's default MCP invocation without tool-profile flags; got:\n%s", text)
	}

	pluginPath := filepath.Join(home, ".gemini", "antigravity-cli", "plugins", "gentle-ai-engram", "plugin.json")
	if _, err := os.Stat(pluginPath); err != nil {
		t.Fatalf("Antigravity Engram plugin manifest missing: %v", err)
	}

	pluginMCPPath := filepath.Join(home, ".gemini", "antigravity-cli", "plugins", "gentle-ai-engram", "mcp_config.json")
	pluginMCPContent, err := os.ReadFile(pluginMCPPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", pluginMCPPath, err)
	}
	pluginMCPText := string(pluginMCPContent)
	if !strings.Contains(pluginMCPText, `"mcp"`) || strings.Contains(pluginMCPText, `--tools=`) {
		t.Fatalf("Antigravity Engram plugin MCP config should expose default Engram MCP tools; got:\n%s", pluginMCPText)
	}

	hooksPath := filepath.Join(home, ".gemini", "antigravity-cli", "plugins", "gentle-ai-engram", "hooks.json")
	hooksContent, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", hooksPath, err)
	}
	hooksText := string(hooksContent)
	for _, want := range []string{
		"PreInvocation",
		"injectSteps",
		"mem_save",
		"mem_search",
		"mem_context",
		"mem_session_summary",
		"mem_get_observation",
		"mem_current_project",
		"mem_judge",
		"optional mem_review",
		"if mem_review is unavailable",
	} {
		if !strings.Contains(hooksText, want) {
			t.Fatalf("Antigravity Engram hook missing %q; got:\n%s", want, hooksText)
		}
	}

	desktopMCPPath := filepath.Join(home, ".gemini", "antigravity", "mcp_config.json")
	if _, err := os.Stat(desktopMCPPath); !os.IsNotExist(err) {
		t.Fatalf("legacy desktop MCP path %q should not be written for antigravity; stat err = %v", desktopMCPPath, err)
	}
}

func TestInjectAntigravityInitializesEmptySettingsWhenGeminiMissing(t *testing.T) {
	home := t.TempDir()

	first, err := Inject(home, antigravityAdapter())
	if err != nil {
		t.Fatalf("Inject(antigravity) first error = %v", err)
	}
	if !first.Changed {
		t.Fatalf("Inject(antigravity) first changed = false")
	}

	settingsPath := filepath.Join(home, ".gemini", "antigravity-cli", "settings.json")
	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", settingsPath, err)
	}
	if strings.TrimSpace(string(got)) != "{}" {
		t.Fatalf("antigravity settings = %q, want empty JSON object", got)
	}

	second, err := Inject(home, antigravityAdapter())
	if err != nil {
		t.Fatalf("Inject(antigravity) second error = %v", err)
	}
	if second.Changed {
		t.Fatalf("Inject(antigravity) second changed = true; want false")
	}
}

// ─── Codex tests ──────────────────────────────────────────────────────────────

func TestInjectCodexWritesTOMLMCP(t *testing.T) {
	validCodexRuntime(t)
	home := t.TempDir()

	result, err := Inject(home, codexAdapter())
	if err != nil {
		t.Fatalf("Inject(codex) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("Inject(codex) changed = false")
	}

	configPath := filepath.Join(home, ".codex", "config.toml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(config.toml) error = %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "[mcp_servers.engram]") {
		t.Fatalf("config.toml missing [mcp_servers.engram] block; got:\n%s", text)
	}
	// command must reference the engram binary — either relative ("engram") or an
	// absolute path (when engram is on PATH). Both are valid.
	if !strings.Contains(text, "command = ") {
		t.Fatalf("config.toml missing command field; got:\n%s", text)
	}
	cmdLine := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "command = ") {
			cmdLine = strings.TrimSpace(line)
			break
		}
	}
	if cmdLine == "" {
		t.Fatalf("config.toml missing command line; got:\n%s", text)
	}
	// The command value must end with "engram" or "engram.exe".
	cmdVal := strings.TrimPrefix(cmdLine, "command = ")
	cmdVal = strings.Trim(cmdVal, `"`)
	base := filepath.Base(cmdVal)
	if base != "engram" && base != "engram.exe" {
		t.Fatalf("config.toml command %q does not reference engram binary; got:\n%s", cmdVal, text)
	}
	if !strings.Contains(text, `"--tools=agent"`) {
		t.Fatalf("config.toml missing --tools=agent; got:\n%s", text)
	}
}

func TestInjectCodexWritesInstructionFiles(t *testing.T) {
	validCodexRuntime(t)
	home := t.TempDir()

	_, err := Inject(home, codexAdapter())
	if err != nil {
		t.Fatalf("Inject(codex) error = %v", err)
	}

	instructionsPath := filepath.Join(home, ".codex", "engram-instructions.md")
	content, err := os.ReadFile(instructionsPath)
	if err != nil {
		t.Fatalf("ReadFile(engram-instructions.md) error = %v", err)
	}
	if !strings.Contains(string(content), "mem_save") {
		t.Fatal("engram-instructions.md missing expected content (mem_save)")
	}
	if !strings.Contains(string(content), "needs_review") {
		t.Fatal("engram-instructions.md missing memory lifecycle stale-context rule (needs_review)")
	}

	compactPath := filepath.Join(home, ".codex", "engram-compact-prompt.md")
	compactContent, err := os.ReadFile(compactPath)
	if err != nil {
		t.Fatalf("ReadFile(engram-compact-prompt.md) error = %v", err)
	}
	if !strings.Contains(string(compactContent), "FIRST ACTION REQUIRED") {
		t.Fatal("engram-compact-prompt.md missing expected content (FIRST ACTION REQUIRED)")
	}
}

func TestInjectCodexInjectsTOMLKeys(t *testing.T) {
	validCodexRuntime(t)
	home := t.TempDir()

	_, err := Inject(home, codexAdapter())
	if err != nil {
		t.Fatalf("Inject(codex) error = %v", err)
	}

	configPath := filepath.Join(home, ".codex", "config.toml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(config.toml) error = %v", err)
	}
	text := string(content)

	instructionsPath := filepath.Join(home, ".codex", "engram-instructions.md")
	if !strings.Contains(text, `model_instructions_file`) {
		t.Fatalf("config.toml missing model_instructions_file key; got:\n%s", text)
	}
	normText := strings.ReplaceAll(strings.ReplaceAll(text, "\\\\", "/"), "\\", "/")
	normInstrPath := filepath.ToSlash(instructionsPath)
	if !strings.Contains(normText, normInstrPath) {
		t.Fatalf("config.toml model_instructions_file does not reference %q; got:\n%s", instructionsPath, text)
	}

	compactPath := filepath.Join(home, ".codex", "engram-compact-prompt.md")
	if !strings.Contains(text, `experimental_compact_prompt_file`) {
		t.Fatalf("config.toml missing experimental_compact_prompt_file key; got:\n%s", text)
	}
	normCompactPath := filepath.ToSlash(compactPath)
	if !strings.Contains(normText, normCompactPath) {
		t.Fatalf("config.toml experimental_compact_prompt_file does not reference %q; got:\n%s", compactPath, text)
	}
}

// ─── Engram setup absolute path preservation tests ────────────────────────────

// TestInjectClaudePreservesAbsoluteCommandFromEngramSetup verifies that when
// `engram setup claude-code` has already written an absolute-path command to
// the managed legacy file, Inject() migrates it into Claude's user registry.
func TestInjectClaudePreservesAbsoluteCommandFromEngramSetup(t *testing.T) {
	home := t.TempDir()

	// Simulate what `engram setup claude-code` writes on v1.10.3+:
	// an absolute path as the command value.
	absPath := "/opt/homebrew/bin/engram"
	mcpPath := filepath.Join(home, ".claude", "mcp", "engram.json")
	if err := os.MkdirAll(filepath.Dir(mcpPath), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	setupContent := []byte(`{
  "command": "/opt/homebrew/bin/engram",
  "args": ["mcp", "--tools=agent"]
}
`)
	if err := os.WriteFile(mcpPath, setupContent, 0o644); err != nil {
		t.Fatalf("WriteFile(engram.json) error = %v", err)
	}

	// Now run Inject — should NOT overwrite the absolute command.
	_, err := Inject(home, claudeAdapter())
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	registryPath := claude.UserConfigPath(home)
	content, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("ReadFile(user registry) error = %v", err)
	}

	text := string(content)
	if !strings.Contains(text, absPath) {
		t.Fatalf("Inject() overwrote absolute command path; want %q preserved, got:\n%s", absPath, text)
	}
	assertArgsHaveToolsAgent(t, registryPath)
	if _, statErr := os.Stat(mcpPath); !os.IsNotExist(statErr) {
		t.Fatalf("managed legacy file must be removed after migration; stat error = %v", statErr)
	}
	if _, statErr := os.Lstat(filepath.Dir(mcpPath)); !os.IsNotExist(statErr) {
		t.Fatalf("empty managed legacy directory must be removed; stat error = %v", statErr)
	}
}

func TestInjectClaudePreservesManagedLegacyParentLayouts(t *testing.T) {
	for _, symlinkParent := range []bool{true, false} {
		name := "non-empty real parent"
		if symlinkParent {
			name = "symlink parent"
		}
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			parent := filepath.Join(home, ".claude", "mcp")
			actualParent := parent
			if symlinkParent {
				actualParent = t.TempDir()
				if err := os.MkdirAll(filepath.Dir(parent), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(actualParent, parent); err != nil {
					t.Fatal(err)
				}
			} else if err := os.MkdirAll(parent, 0o755); err != nil {
				t.Fatal(err)
			}
			legacy := filepath.Join(actualParent, "engram.json")
			custom := filepath.Join(actualParent, "custom.json")
			customBytes := []byte(`{"command":"custom"}`)
			if err := os.WriteFile(legacy, []byte(`{"command":"/usr/local/bin/engram","args":["mcp","--tools=agent"]}`), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(custom, customBytes, 0o640); err != nil {
				t.Fatal(err)
			}

			if _, err := Inject(home, claudeAdapter()); err != nil {
				t.Fatalf("Inject() error = %v", err)
			}
			info, err := os.Lstat(parent)
			if err != nil || symlinkParent != (info.Mode()&os.ModeSymlink != 0) {
				t.Fatalf("legacy parent changed unsafely: info=%v error=%v", info, err)
			}
			if got, err := os.ReadFile(custom); err != nil || !bytes.Equal(got, customBytes) {
				t.Fatalf("custom sibling changed: got=%q error=%v", got, err)
			}
		})
	}
}

// TestInjectClaudePreservesAbsoluteCommandIsIdempotent verifies that calling
// Inject() twice when an absolute-path engram.json already exists does not
// cause repeated writes (idempotency).
func TestInjectClaudePreservesAbsoluteCommandIsIdempotent(t *testing.T) {
	home := t.TempDir()

	absPath := "/usr/local/bin/engram"
	mcpPath := filepath.Join(home, ".claude", "mcp", "engram.json")
	if err := os.MkdirAll(filepath.Dir(mcpPath), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	setupContent := []byte(`{
  "command": "/usr/local/bin/engram",
  "args": ["mcp", "--tools=agent"]
}
`)
	if err := os.WriteFile(mcpPath, setupContent, 0o644); err != nil {
		t.Fatalf("WriteFile(engram.json) error = %v", err)
	}

	first, err := Inject(home, claudeAdapter())
	if err != nil {
		t.Fatalf("Inject() first error = %v", err)
	}

	second, err := Inject(home, claudeAdapter())
	if err != nil {
		t.Fatalf("Inject() second error = %v", err)
	}
	if second.Changed {
		t.Fatalf("Inject() second changed = true after absolute-path setup; want idempotent (no change)")
	}

	// Absolute path must still be present in the supported registry.
	content, err := os.ReadFile(claude.UserConfigPath(home))
	if err != nil {
		t.Fatalf("ReadFile(user registry) error = %v", err)
	}
	if !strings.Contains(string(content), absPath) {
		t.Fatalf("absolute command path %q was lost after second Inject(); got:\n%s", absPath, string(content))
	}
	_ = first // first result not the focus of this test
}

func TestInjectClaudePreservesUnmanagedLegacyShapes(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
	}{
		{"command array", []byte(`{"command":["/usr/local/bin/engram"],"args":["mcp","--tools=agent"]}`)},
		{"bare args", []byte(`{"command":"/usr/local/bin/engram","args":["mcp"]}`)},
		{"extra semantics", []byte(`{"command":"/usr/local/bin/engram","args":["mcp","--tools=agent"],"env":{"CUSTOM":"1"}}`)},
		{"custom command", []byte(`{"command":"custom-engram","args":["mcp","--tools=agent"]}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			legacyPath := filepath.Join(home, ".claude", "mcp", "engram.json")
			if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(legacyPath, tt.content, 0o640); err != nil {
				t.Fatal(err)
			}
			if _, err := Inject(home, claudeAdapter()); err != nil {
				t.Fatalf("Inject() error = %v", err)
			}
			got, err := os.ReadFile(legacyPath)
			if err != nil || !bytes.Equal(got, tt.content) {
				t.Fatalf("unmanaged legacy config changed: got=%q error=%v", got, err)
			}
		})
	}
}

func TestInjectClaudeMigratesCellarCommandToStablePath(t *testing.T) {
	home := t.TempDir()

	mockEngramLookPath(t, "/usr/local/bin/engram", "")

	mcpPath := filepath.Join(home, ".claude", "mcp", "engram.json")
	if err := os.MkdirAll(filepath.Dir(mcpPath), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	setupContent := []byte(`{
  "command": "/usr/local/Cellar/engram/1.14.1/bin/engram",
  "args": ["mcp", "--tools=agent"]
}
`)
	if err := os.WriteFile(mcpPath, setupContent, 0o644); err != nil {
		t.Fatalf("WriteFile(engram.json) error = %v", err)
	}

	result, err := Inject(home, claudeAdapter())
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("Inject() changed = false; expected Cellar command migration")
	}

	registryPath := claude.UserConfigPath(home)
	content, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("ReadFile(user registry) error = %v", err)
	}
	text := string(content)
	if strings.Contains(text, "/Cellar/") {
		t.Fatalf("engram.json still contains versioned Homebrew Cellar path; got:\n%s", text)
	}
	if !strings.Contains(text, "/usr/local/bin/engram") {
		t.Fatalf("engram.json did not migrate to stable Homebrew symlink; got:\n%s", text)
	}
	assertArgsHaveToolsAgent(t, registryPath)
}

func TestInjectCodexIsIdempotent(t *testing.T) {
	validCodexRuntime(t)
	home := t.TempDir()

	first, err := Inject(home, codexAdapter())
	if err != nil {
		t.Fatalf("Inject(codex) first error = %v", err)
	}
	if !first.Changed {
		t.Fatalf("Inject(codex) first changed = false")
	}

	second, err := Inject(home, codexAdapter())
	if err != nil {
		t.Fatalf("Inject(codex) second error = %v", err)
	}
	if second.Changed {
		t.Fatalf("Inject(codex) second changed = true (should be idempotent)")
	}

	// Verify only one [mcp_servers.engram] block.
	configPath := filepath.Join(home, ".codex", "config.toml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(config.toml) error = %v", err)
	}
	count := strings.Count(string(content), "[mcp_servers.engram]")
	if count != 1 {
		t.Fatalf("config.toml has %d [mcp_servers.engram] blocks, want exactly 1; got:\n%s", count, string(content))
	}
}

// ─── Codex profile injection tests ───────────────────────────────────────────

// TestInjectCodexWritesProfiles asserts that Inject for the Codex adapter
// writes the three gentle-ai SDD profile files into ~/.codex/.
func TestInjectCodexWritesProfiles(t *testing.T) {
	validCodexRuntime(t)
	home := t.TempDir()

	_, err := Inject(home, codexAdapter())
	if err != nil {
		t.Fatalf("Inject(codex) error = %v", err)
	}

	profiles := []struct {
		name            string
		reasoningEffort string
	}{
		{"sdd-strong.config.toml", "medium"},
		{"sdd-mid.config.toml", "high"},
		{"sdd-cheap.config.toml", "high"},
	}

	for _, p := range profiles {
		path := filepath.Join(home, ".codex", p.name)
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("profile %q not written by Inject: %v", p.name, readErr)
		}
		want := `"` + p.reasoningEffort + `"`
		if !strings.Contains(string(content), want) {
			t.Fatalf("profile %q: want model_reasoning_effort = %s; got:\n%s", p.name, want, string(content))
		}
	}
}

// TestInjectCodexProfilesIdempotent asserts that running Inject twice leaves
// the profile files unchanged on the second run and does not duplicate keys.
func TestInjectCodexProfilesIdempotent(t *testing.T) {
	validCodexRuntime(t)
	home := t.TempDir()

	if _, err := Inject(home, codexAdapter()); err != nil {
		t.Fatalf("first Inject(codex) error = %v", err)
	}
	second, err := Inject(home, codexAdapter())
	if err != nil {
		t.Fatalf("second Inject(codex) error = %v", err)
	}
	if second.Changed {
		t.Fatal("second Inject(codex) changed = true, want false (profiles are idempotent)")
	}

	for _, name := range []string{"sdd-strong.config.toml", "sdd-mid.config.toml", "sdd-cheap.config.toml"} {
		content, readErr := os.ReadFile(filepath.Join(home, ".codex", name))
		if readErr != nil {
			t.Fatalf("profile %q missing after second Inject: %v", name, readErr)
		}
		count := strings.Count(string(content), "model_reasoning_effort")
		if count != 1 {
			t.Fatalf("profile %q: expected 1 model_reasoning_effort key after second Inject, got %d; content:\n%s", name, count, string(content))
		}
	}
}

// TestProfileFallbackAgreesWithRenderFallback asserts that resolveProfileAssignments
// with nil inputs produces the same per-carril effort as RenderCodexPhaseEfforts with
// nil inputs (both must use the Recommended preset as the canonical nil fallback).
func TestProfileFallbackAgreesWithRenderFallback(t *testing.T) {
	// Profile fallback: nil carrilModels + nil phaseEfforts
	assignments := resolveProfileAssignments(nil, nil)

	// Build a quick carril→effort map from the profile assignments.
	profileEffort := make(map[string]string, len(assignments))
	for _, a := range assignments {
		profileEffort[a.Profile] = a.ReasoningEffort
	}

	// Render fallback: nil inputs → CodexModelPresetRecommended
	renderOut := model.RenderCodexPhaseEfforts(nil, nil)

	// For each carril, the render table and the profile files must agree.
	// The render check is per-row: we find the carril's row and assert the effort
	// cell appears within that specific row (not just anywhere in the table).
	cases := []struct {
		carril     string
		wantEffort string
	}{
		{"sdd-strong", "medium"},
		{"sdd-mid", "high"},
		{"sdd-cheap", "high"},
	}
	for _, tc := range cases {
		got := profileEffort[tc.carril]
		if got != tc.wantEffort {
			t.Errorf("profile fallback for %q = %q, want %q", tc.carril, got, tc.wantEffort)
		}
		// Render-side: find the carril's row and check the effort cell is in THAT row.
		needle := "| `" + tc.carril + "`"
		rowStart := strings.Index(renderOut, needle)
		if rowStart == -1 {
			t.Errorf("render fallback table missing row for carril %q; table:\n%s", tc.carril, renderOut)
			continue
		}
		rowEnd := len(renderOut)
		for i := rowStart + 1; i < len(renderOut); i++ {
			if renderOut[i] == '\n' {
				rowEnd = i
				break
			}
		}
		row := renderOut[rowStart:rowEnd]
		effortCell := "| `" + tc.wantEffort + "` |"
		if !strings.Contains(row, effortCell) {
			t.Errorf("render fallback carril %q row = %q: want effort cell %q", tc.carril, row, effortCell)
		}
	}
}

// ─── Codex multi-agent config injection tests ────────────────────────────────

// TestInjectCodexMultiAgentDefaultOn asserts that after a plain Inject call,
// config.toml contains [features] with multi_agent = true. Codex SDD enables
// multi-agent delegation by default so the per-phase reasoning_effort table applies.
func TestInjectCodexMultiAgentDefaultOn(t *testing.T) {
	validCodexRuntime(t)
	home := t.TempDir()

	if _, err := Inject(home, codexAdapter()); err != nil {
		t.Fatalf("Inject(codex) error = %v", err)
	}

	configPath := filepath.Join(home, ".codex", "config.toml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(config.toml) error = %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "[features]") {
		t.Fatalf("config.toml missing [features] section; got:\n%s", text)
	}
	if !strings.Contains(text, "multi_agent = true") {
		t.Fatalf("config.toml missing multi_agent = true (enabled by default); got:\n%s", text)
	}
	if strings.Contains(text, "multi_agent = false") {
		t.Fatalf("config.toml must NOT have multi_agent = false by default; got:\n%s", text)
	}
}

// TestInjectCodexMultiAgentOptIn asserts that InjectWithOptions with
// CodexMultiAgent=true writes multi_agent = true in [features].
func TestInjectCodexMultiAgentOptIn(t *testing.T) {
	validCodexRuntime(t)
	home := t.TempDir()

	opts := InjectOptions{CodexMultiAgent: true}
	if _, err := InjectWithOptions(home, codexAdapter(), opts); err != nil {
		t.Fatalf("InjectWithOptions(codex, multiAgent=true) error = %v", err)
	}

	configPath := filepath.Join(home, ".codex", "config.toml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(config.toml) error = %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "multi_agent = true") {
		t.Fatalf("config.toml missing multi_agent = true after opt-in; got:\n%s", text)
	}
}

// TestInjectCodexMultiAgentDefaults asserts that the [agents] section is always
// written with max_threads = 4 and max_depth = 2 regardless of the opt-in flag.
func TestInjectCodexMultiAgentDefaults(t *testing.T) {
	validCodexRuntime(t)
	home := t.TempDir()

	if _, err := Inject(home, codexAdapter()); err != nil {
		t.Fatalf("Inject(codex) error = %v", err)
	}

	configPath := filepath.Join(home, ".codex", "config.toml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(config.toml) error = %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "[agents]") {
		t.Fatalf("config.toml missing [agents] section; got:\n%s", text)
	}
	if !strings.Contains(text, "max_threads = 4") {
		t.Fatalf("config.toml missing max_threads = 4; got:\n%s", text)
	}
	if !strings.Contains(text, "max_depth = 2") {
		t.Fatalf("config.toml missing max_depth = 2; got:\n%s", text)
	}
}

// TestInjectCodexMultiAgentIdempotent asserts that running Inject twice
// produces exactly one [features] section and one [agents] section with no
// duplicate keys, and that the engram and context7 blocks are not disturbed.
func TestInjectCodexMultiAgentIdempotent(t *testing.T) {
	validCodexRuntime(t)
	home := t.TempDir()

	if _, err := Inject(home, codexAdapter()); err != nil {
		t.Fatalf("first Inject(codex) error = %v", err)
	}
	second, err := Inject(home, codexAdapter())
	if err != nil {
		t.Fatalf("second Inject(codex) error = %v", err)
	}
	if second.Changed {
		// Read content for diagnostics.
		content, _ := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
		t.Fatalf("second Inject(codex) changed = true, want false (multi-agent keys are idempotent); config.toml:\n%s", string(content))
	}

	configPath := filepath.Join(home, ".codex", "config.toml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(config.toml) error = %v", err)
	}
	text := string(content)

	if count := strings.Count(text, "[features]"); count != 1 {
		t.Fatalf("expected 1 [features] section, got %d; config.toml:\n%s", count, text)
	}
	if count := strings.Count(text, "[agents]"); count != 1 {
		t.Fatalf("expected 1 [agents] section, got %d; config.toml:\n%s", count, text)
	}
	if count := strings.Count(text, "multi_agent"); count != 1 {
		t.Fatalf("expected 1 multi_agent key, got %d; config.toml:\n%s", count, text)
	}
	if count := strings.Count(text, "max_threads"); count != 1 {
		t.Fatalf("expected 1 max_threads key, got %d; config.toml:\n%s", count, text)
	}
	if count := strings.Count(text, "max_depth"); count != 1 {
		t.Fatalf("expected 1 max_depth key, got %d; config.toml:\n%s", count, text)
	}
	// Engram MCP block must still be present.
	if !strings.Contains(text, "[mcp_servers.engram]") {
		t.Fatalf("config.toml missing [mcp_servers.engram] after idempotency run; got:\n%s", text)
	}
}

// ─── Absolute path resolution tests ──────────────────────────────────────────

// mockEngramLookPath sets EngramLookPath to a mock and restores it after the test.
func mockEngramLookPath(t *testing.T, result string, errMsg string) {
	t.Helper()
	orig := EngramLookPath
	EngramLookPath = func(string) (string, error) {
		if errMsg != "" {
			return "", fmt.Errorf("%s", errMsg)
		}
		return result, nil
	}
	t.Cleanup(func() { EngramLookPath = orig })
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v; content:\n%s", path, err, raw)
	}
	return parsed
}

func nestedValue(t *testing.T, root map[string]any, path ...string) (any, bool) {
	t.Helper()
	var current any = root
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func assertNestedString(t *testing.T, root map[string]any, want string, path ...string) {
	t.Helper()
	got, ok := nestedValue(t, root, path...)
	if !ok {
		t.Fatalf("missing JSON path %v in %#v", path, root)
	}
	if got != want {
		t.Fatalf("JSON path %v = %#v, want %q", path, got, want)
	}
}

func assertNestedStrings(t *testing.T, root map[string]any, want []string, path ...string) {
	t.Helper()
	got, ok := nestedValue(t, root, path...)
	if !ok {
		t.Fatalf("missing JSON path %v in %#v", path, root)
	}
	items, ok := got.([]any)
	if !ok {
		t.Fatalf("JSON path %v = %#v, want string array", path, got)
	}
	if len(items) != len(want) {
		t.Fatalf("JSON path %v length = %d, want %d (%#v)", path, len(items), len(want), got)
	}
	for i, wantItem := range want {
		if items[i] != wantItem {
			t.Fatalf("JSON path %v[%d] = %#v, want %q", path, i, items[i], wantItem)
		}
	}
}

func assertNestedStringsUnordered(t *testing.T, root map[string]any, want []string, path ...string) {
	t.Helper()
	got, ok := nestedValue(t, root, path...)
	if !ok {
		t.Fatalf("missing JSON path %v in %#v", path, root)
	}
	items, ok := got.([]any)
	if !ok {
		t.Fatalf("JSON path %v = %#v, want string array", path, got)
	}
	if len(items) != len(want) {
		t.Fatalf("JSON path %v length = %d, want %d (%#v)", path, len(items), len(want), got)
	}
	remaining := make(map[string]int, len(want))
	for _, item := range want {
		remaining[item]++
	}
	for _, item := range items {
		itemString, ok := item.(string)
		if !ok {
			t.Fatalf("JSON path %v contains non-string item %#v", path, item)
		}
		remaining[itemString]--
	}
	for item, count := range remaining {
		if count != 0 {
			t.Fatalf("JSON path %v missing/extra %q count delta %d; got %#v", path, item, count, got)
		}
	}
}

func assertNestedBool(t *testing.T, root map[string]any, want bool, path ...string) {
	t.Helper()
	got, ok := nestedValue(t, root, path...)
	if !ok {
		t.Fatalf("missing JSON path %v in %#v", path, root)
	}
	if got != want {
		t.Fatalf("JSON path %v = %#v, want %v", path, got, want)
	}
}

func assertNestedMissing(t *testing.T, root map[string]any, path ...string) {
	t.Helper()
	if got, ok := nestedValue(t, root, path...); ok {
		t.Fatalf("JSON path %v present = %#v, want missing", path, got)
	}
}

// TestEngramInjectUsesAbsolutePathWhenAvailable verifies that when engram is
// resolvable on PATH, its absolute path is written into the MCP config file
// for agents that use StrategyMCPConfigFile (e.g. Windsurf).

// TestEngramInjectFallsBackToRelativeWhenNotFound verifies that when engram
// cannot be resolved on PATH, the config falls back to the relative "engram"
// command string.

// TestEngramInjectAbsolutePathForOpenCodeMergeStrategy verifies that the
// absolute path is used when the StrategyMergeIntoSettings strategy is
// applied for OpenCode.

// TestEngramInjectAbsolutePathForGeminiMergeStrategy verifies that the
// absolute path is also used when the StrategyMergeIntoSettings strategy is
// applied (e.g. Gemini CLI).

func unmarshalObjectForTest(t *testing.T, content []byte) map[string]any {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal(content, &root); err != nil {
		t.Fatalf("Unmarshal JSON error = %v; content:\n%s", err, content)
	}
	return root
}

func objectAtForTest(t *testing.T, root map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := root[key]
	if !ok {
		t.Fatalf("missing object key %q in %#v", key, root)
	}
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("key %q has type %T, want object", key, value)
	}
	return object
}

// TestInjectEngramHermesYAMLOverlay verifies that Inject writes the engram MCP
// server block under mcp_servers: in ~/.hermes/config.yaml (StrategyMergeIntoYAML),
// and that a second call is idempotent (Changed=false).

// TestEngramYAMLCommandRecoveryCustomPath verifies that a custom absolute
// engram command already in config.yaml is preserved (not clobbered) on re-run.

// TestEngramYAMLCommandRecoveryVersionedCellar verifies that a versioned Homebrew
// cellar path is stabilized to the bare "engram" command on re-run.

// TestEngramYAMLCommandRecoveryAbsent verifies that when no prior engram entry
// exists in config.yaml, the stable "engram" fallback is written.

// TestEngramYAMLCommandRecoveryListShape verifies that a YAML list-shaped command
// (command: - /path/engram) has its first element recovered correctly.

// ---------------------------------------------------------------------------
// Decision 1 per-adapter slim/full selection matrix (16 adapters).
// ---------------------------------------------------------------------------

// aboveFloorVersion is a version comfortably above the v1.4.0 gate (matches
// the live evidence cited in design.md Decision 1: engram 1.18.0).
const aboveFloorVersion = "1.18.0"

// TestProtocolForSelectsSlimOrFullPerDecision1Matrix is the 16-row table test
// required by task 1.2: Claude Code -> slim (gated on engram >= v1.4.0), all
// other adapters with a setup slug or system-prompt surface -> full. Pi is
// covered separately below since it never renders protocol text at all
// (existing MCP-only precedent, unchanged by this change).

// TestProtocolForPiRendersNoProtocolText asserts the existing MCP-only
// precedent: Pi never writes protocol text via Inject(), regardless of
// version, because piEngramProvisioner short-circuits before protocolFor is
// ever consulted.
func TestProtocolForPiRendersNoProtocolText(t *testing.T) {
	home := t.TempDir()

	result, err := InjectWithOptions(home, piAdapter(), InjectOptions{Version: aboveFloorVersion})
	if err != nil {
		t.Fatalf("InjectWithOptions(pi) error = %v", err)
	}

	for _, path := range result.Files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		if strings.Contains(string(content), "mem_save") || strings.Contains(string(content), "Engram Persistent Memory") {
			t.Fatalf("Pi file %q unexpectedly contains protocol text; got:\n%s", path, content)
		}
	}
}

// ---------------------------------------------------------------------------
// Decision 1 version-gate boundary (task 1.3).
// ---------------------------------------------------------------------------

func TestProtocolForVersionGateBoundary(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		wantSlim bool
	}{
		{"below floor", "1.3.9", false},
		{"unknown/unparseable version", "not-a-version", false},
		{"empty version (VerifyVersion failed)", "", false},
		{"exact floor v1.4.0 (inclusive boundary)", "1.4.0", true},
		{"above floor", aboveFloorVersion, true},
	}

	slim := protocolSlim()
	full := protocolFull()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := protocolFor(model.AgentClaudeCode, InjectOptions{Version: tt.version})
			want := full
			if tt.wantSlim {
				want = slim
			}
			if got != want {
				t.Fatalf("protocolFor(claude-code, version=%q) did not match expected section (wantSlim=%v)", tt.version, tt.wantSlim)
			}
		})
	}
}

// TestInjectWithOptionsThreadsVersionIntoClaudeSlimSelection is the
// integration-level counterpart of the boundary test above: it exercises the
// full InjectWithOptions -> CLAUDE.md write path and asserts the rendered
// section flips from full to slim once InjectOptions.Version crosses the
// v1.4.0 floor.
func TestInjectWithOptionsThreadsVersionIntoClaudeSlimSelection(t *testing.T) {
	belowFloorHome := t.TempDir()
	if _, err := InjectWithOptions(belowFloorHome, claudeAdapter(), InjectOptions{Version: "1.3.9"}); err != nil {
		t.Fatalf("InjectWithOptions(claude, below floor) error = %v", err)
	}
	belowContent, err := os.ReadFile(filepath.Join(belowFloorHome, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatalf("ReadFile(CLAUDE.md) error = %v", err)
	}
	if !strings.Contains(string(belowContent), "needs_review") {
		t.Fatal("below-floor version must render the FULL section (expected 'needs_review' from full text)")
	}

	aboveFloorHome := t.TempDir()
	if _, err := InjectWithOptions(aboveFloorHome, claudeAdapter(), InjectOptions{Version: aboveFloorVersion}); err != nil {
		t.Fatalf("InjectWithOptions(claude, above floor) error = %v", err)
	}
	aboveContent, err := os.ReadFile(filepath.Join(aboveFloorHome, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatalf("ReadFile(CLAUDE.md) error = %v", err)
	}
	if strings.Contains(string(aboveContent), "needs_review") {
		t.Fatal("above-floor version must render the SLIM section (must not contain full-only 'needs_review' text)")
	}
	if !strings.Contains(string(aboveContent), "SessionStart hook") {
		t.Fatal("above-floor version must render the SLIM section with its pointer to the full protocol location")
	}
}

// TestInjectWithOptionsReInjectConvergesFullToSlimAndBack is the spec
// scenario "Re-inject converges to target state" (Idempotent injection and
// clean uninstall across upgrades): a previously-injected full section MUST
// converge to slim (and vice versa) via the existing marker-based mechanism
// when the target verdict changes across two Inject calls, without
// duplicating or corrupting markers.
func TestInjectWithOptionsReInjectConvergesFullToSlimAndBack(t *testing.T) {
	home := t.TempDir()
	claudeMDPath := filepath.Join(home, ".claude", "CLAUDE.md")

	// 1. Full (below-floor / unknown version — safe default).
	if _, err := InjectWithOptions(home, claudeAdapter(), InjectOptions{}); err != nil {
		t.Fatalf("InjectWithOptions(full) error = %v", err)
	}
	afterFull, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("ReadFile(CLAUDE.md) error = %v", err)
	}
	if !strings.Contains(string(afterFull), "needs_review") {
		t.Fatalf("expected FULL section after first inject; got:\n%s", afterFull)
	}
	if n := strings.Count(string(afterFull), "<!-- gentle-ai:engram-protocol -->"); n != 1 {
		t.Fatalf("expected exactly 1 open marker after first inject, got %d", n)
	}

	// 2. Re-inject with an above-floor version — MUST converge to slim, in
	// place, no duplicate markers.
	if _, err := InjectWithOptions(home, claudeAdapter(), InjectOptions{Version: aboveFloorVersion}); err != nil {
		t.Fatalf("InjectWithOptions(slim) error = %v", err)
	}
	afterSlim, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("ReadFile(CLAUDE.md) error = %v", err)
	}
	if strings.Contains(string(afterSlim), "needs_review") {
		t.Fatalf("expected SLIM section after re-inject with above-floor version; got:\n%s", afterSlim)
	}
	if !strings.Contains(string(afterSlim), "SessionStart hook") {
		t.Fatalf("expected SLIM pointer content after re-inject; got:\n%s", afterSlim)
	}
	if n := strings.Count(string(afterSlim), "<!-- gentle-ai:engram-protocol -->"); n != 1 {
		t.Fatalf("expected exactly 1 open marker after re-inject to slim (no duplication), got %d", n)
	}
	if n := strings.Count(string(afterSlim), "<!-- /gentle-ai:engram-protocol -->"); n != 1 {
		t.Fatalf("expected exactly 1 close marker after re-inject to slim (no duplication), got %d", n)
	}

	// 3. Re-inject back to full — MUST converge back, still no duplication.
	if _, err := InjectWithOptions(home, claudeAdapter(), InjectOptions{}); err != nil {
		t.Fatalf("InjectWithOptions(back to full) error = %v", err)
	}
	afterBackToFull, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("ReadFile(CLAUDE.md) error = %v", err)
	}
	if !strings.Contains(string(afterBackToFull), "needs_review") {
		t.Fatalf("expected FULL section after re-inject back to full; got:\n%s", afterBackToFull)
	}
	if n := strings.Count(string(afterBackToFull), "<!-- gentle-ai:engram-protocol -->"); n != 1 {
		t.Fatalf("expected exactly 1 open marker after re-inject back to full (no duplication), got %d", n)
	}
}

func TestInjectCodexOrchestratorAssignmentWritesTopLevelModel(t *testing.T) {
	validCodexRuntime(t)
	home := t.TempDir()
	opts := InjectOptions{CodexOrchestratorAssignment: model.CodexPresetOrchestratorAssignment(string(model.CodexPresetRecommended))}
	if _, err := InjectWithOptions(home, codexAdapter(), opts); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, `model = "gpt-5.6-sol"`) || !strings.Contains(text, `model_reasoning_effort = "medium"`) {
		t.Fatalf("top-level orchestrator assignment missing:\n%s", text)
	}
}

func TestInjectCodexNilOrchestratorAssignmentPreservesTopLevelModel(t *testing.T) {
	validCodexRuntime(t)
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("model = \"user-model\"\nmodel_reasoning_effort = \"high\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InjectWithOptions(home, codexAdapter(), InjectOptions{}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `model = "user-model"`) || !strings.Contains(string(content), `model_reasoning_effort = "high"`) {
		t.Fatalf("nil assignment clobbered top-level model:\n%s", content)
	}
}
