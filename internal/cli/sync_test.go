package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/agents"
	"github.com/gentleman-programming/gentle-ai/v2/internal/agents/claude"
	"github.com/gentleman-programming/gentle-ai/v2/internal/agents/codex"
	"github.com/gentleman-programming/gentle-ai/v2/internal/backup"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/agentguidance"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/codegraph"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/engram"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/filemerge"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/persona"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/pipeline"
	"github.com/gentleman-programming/gentle-ai/v2/internal/planner"
	"github.com/gentleman-programming/gentle-ai/v2/internal/state"
)

// ─── Phase 1: ParseSyncFlags ───────────────────────────────────────────────

func TestParseSyncFlagsDefaults(t *testing.T) {
	flags, err := ParseSyncFlags([]string{})
	if err != nil {
		t.Fatalf("ParseSyncFlags() error = %v", err)
	}

	if len(flags.Agents) != 0 {
		t.Errorf("Agents = %v, want empty", flags.Agents)
	}
	if flags.DryRun {
		t.Errorf("DryRun = true, want false")
	}
	if flags.IncludePermissions {
		t.Errorf("IncludePermissions = true, want false")
	}
	if flags.SDDMode != "" {
		t.Errorf("SDDMode = %q, want empty", flags.SDDMode)
	}
}

// TestRunSyncRejectsUnsupportedAgent closes install/sync surface audit
// finding 3: `gentle-ai sync --agent cluade` (a typo) previously printed
// "All managed assets are already up to date. No files changed." — the user
// believed they synced, but asAgentIDs silently converted the typo into an
// AgentID nothing ever matches, so DiscoverAgents-equivalent resolution
// produced a no-op instead of an error.

// assertRunSyncRejectsRetiredSelector verifies sync applies the canonical
// client allowlist before discovering or mutating any legacy client assets.
func assertRunSyncRejectsRetiredSelector(t *testing.T, raw string) {
	t.Helper()
	home := t.TempDir()
	previousHome := osUserHomeDir
	previousBackupHome := backup.UserHomeDirFn
	osUserHomeDir = func() (string, error) { return home, nil }
	backup.UserHomeDirFn = func() (string, error) { return home, nil }
	t.Cleanup(func() {
		osUserHomeDir = previousHome
		backup.UserHomeDirFn = previousBackupHome
	})

	result, err := RunSync([]string{"--agents", raw})
	if err == nil {
		t.Fatalf("RunSync(%q) error = nil, want retired selector rejected", raw)
	}
	if !strings.Contains(err.Error(), "unsupported agent") || !strings.Contains(err.Error(), raw) {
		t.Fatalf("RunSync(%q) error = %q, want unsupported agent %q", raw, err.Error(), raw)
	}
	if result.Execution.Apply.Steps != nil || result.Execution.Prepare.Steps != nil || result.ChangedFiles != nil {
		t.Fatalf("rejected selector produced sync execution or changed files: %#v", result)
	}
}

func TestRunSyncRejectsUnsupportedAgent(t *testing.T) {
	home := t.TempDir()
	original := osUserHomeDir
	osUserHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { osUserHomeDir = original })

	_, err := RunSync([]string{"--agent", "cluade"})
	if err == nil {
		t.Fatal("RunSync() error = nil, want an error rejecting the unsupported agent")
	}
	if !strings.Contains(err.Error(), "cluade") {
		t.Fatalf("error = %q, want it to name the offending value %q", err.Error(), "cluade")
	}
}

func TestParseSyncFlagsSDDMode(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{
			name: "absent defaults to empty",
			args: []string{},
			want: "",
		},
		{
			name: "single",
			args: []string{"--sdd-mode", "single"},
			want: "single",
		},
		{
			name: "multi",
			args: []string{"--sdd-mode", "multi"},
			want: "multi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := ParseSyncFlags(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSyncFlags() error = %v, wantErr %v", err, tt.wantErr)
			}
			if flags.SDDMode != tt.want {
				t.Errorf("SDDMode = %q, want %q", flags.SDDMode, tt.want)
			}
		})
	}
}

func TestParseSyncFlagsIncludePermissionsAndRejectsRetiredThemeFlag(t *testing.T) {
	flags, err := ParseSyncFlags([]string{"--include-permissions"})
	if err != nil {
		t.Fatalf("ParseSyncFlags() error = %v", err)
	}
	if !flags.IncludePermissions {
		t.Errorf("IncludePermissions = false, want true")
	}
	if _, err := ParseSyncFlags([]string{"--include-theme"}); err == nil {
		t.Fatal("ParseSyncFlags() accepted retired --include-theme flag")
	}
}

func TestParseSyncFlagsDryRun(t *testing.T) {
	flags, err := ParseSyncFlags([]string{"--dry-run"})
	if err != nil {
		t.Fatalf("ParseSyncFlags() error = %v", err)
	}
	if !flags.DryRun {
		t.Errorf("DryRun = false, want true")
	}
}

func TestParseSyncFlagsSkillsCSV(t *testing.T) {
	flags, err := ParseSyncFlags([]string{"--skills", "sdd-apply,go-testing"})
	if err != nil {
		t.Fatalf("ParseSyncFlags() error = %v", err)
	}

	want := []string{"sdd-apply", "go-testing"}
	if !reflect.DeepEqual(flags.Skills, want) {
		t.Errorf("Skills = %v, want %v", flags.Skills, want)
	}
}

func TestParseSyncFlagsUnknownFlagReturnsError(t *testing.T) {
	_, err := ParseSyncFlags([]string{"--unknown-flag"})
	if err == nil {
		t.Fatalf("ParseSyncFlags() expected error for unknown flag")
	}
}

// TestParseSyncFlagsMistypedFlagNamesTheSupportedFlags closes install/sync
// surface audit finding 4: a mistyped flag like `-sdd` (meant to be
// `-sdd-mode`) produced only "flag provided but not defined: -sdd" with the
// FlagSet's own usage output suppressed (fs.SetOutput(ioDiscard{})) and no
// pointer to how to discover the real flags. The fix captures the FlagSet's
// own canonical usage text (derived from the registered flags themselves,
// not a hand-written list) instead of discarding it, and names
// `gentle-ai sync --help`.
func TestParseSyncFlagsMistypedFlagNamesTheSupportedFlags(t *testing.T) {
	_, err := ParseSyncFlags([]string{"-sdd", "single"})
	if err == nil {
		t.Fatal("ParseSyncFlags() error = nil, want an error for the undefined -sdd flag")
	}
	msg := err.Error()
	if !strings.Contains(msg, "flag provided but not defined: -sdd") {
		t.Fatalf("error = %q, want it to preserve the original flag package error", msg)
	}
	if !strings.Contains(msg, "gentle-ai sync --help") {
		t.Fatalf("error = %q, want it to point at `gentle-ai sync --help`", msg)
	}
	// The usage block is generated by the FlagSet itself from its registered
	// flags (canonical source), so a real flag like --sdd-mode must appear.
	if !strings.Contains(msg, "-sdd-mode") {
		t.Fatalf("error = %q, want it to include the FlagSet's own usage text naming -sdd-mode", msg)
	}
}

// TestParseSyncFlagsHelpFlagRendersUsage proves `gentle-ai sync --help` now
// actually surfaces the supported flags instead of the bare
// "flag: help requested" text it produced before (the FlagSet's usage output
// was being discarded via ioDiscard{}).
func TestParseSyncFlagsHelpFlagRendersUsage(t *testing.T) {
	_, err := ParseSyncFlags([]string{"--help"})
	if err == nil {
		t.Fatal("ParseSyncFlags() error = nil, want flag.ErrHelp wrapped with usage text")
	}
	if !strings.Contains(err.Error(), "-sdd-mode") {
		t.Fatalf("error = %q, want it to include the FlagSet's own usage text naming -sdd-mode", err.Error())
	}
}

// TestParseSyncFlagsPositionalArgumentNamesTheAgentFlag closes install/sync
// surface audit finding 5: `gentle-ai sync claude` (a positional agent name
// instead of a flag) produced only `unexpected sync argument "claude"` with
// no pointer to the correct --agent form.
func TestParseSyncFlagsPositionalArgumentNamesTheAgentFlag(t *testing.T) {
	_, err := ParseSyncFlags([]string{"claude"})
	if err == nil {
		t.Fatal("ParseSyncFlags() error = nil, want an error for the positional argument")
	}
	msg := err.Error()
	if !strings.Contains(msg, `unexpected sync argument "claude"`) {
		t.Fatalf("error = %q, want it to preserve the original message", msg)
	}
	if !strings.Contains(msg, "--agent claude") {
		t.Fatalf("error = %q, want it to name the --agent claude form", msg)
	}
}

// ─── Phase 1: BuildSyncSelection ──────────────────────────────────────────

func TestBuildSyncSelectionIncludePermissionsWhenFlagSet(t *testing.T) {
	agents := []model.AgentID{model.AgentClaudeCode}
	flags := SyncFlags{IncludePermissions: true}

	sel := BuildSyncSelection(flags, agents)

	found := false
	for _, comp := range sel.Components {
		if comp == model.ComponentPermission {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("BuildSyncSelection() expected ComponentPermission when --include-permissions is set")
	}
}

// ─── Phase 2: DiscoverAgents ───────────────────────────────────────────────

func TestDiscoverAgentsReturnsAgentsWithConfigDirPresent(t *testing.T) {
	home := t.TempDir()

	// Create the GlobalConfigDir for claude-code: ~/.claude/
	claudeConfigDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeConfigDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	discovered := DiscoverAgents(home)

	found := false
	for _, id := range discovered {
		if id == model.AgentClaudeCode {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DiscoverAgents() expected claude-code when ~/.claude/ exists, got %v", discovered)
	}
}

func TestDiscoverAgentsReturnsEmptyWhenNoConfigDirsPresent(t *testing.T) {
	home := t.TempDir()
	// Empty home dir — no agent config dirs exist.

	discovered := DiscoverAgents(home)

	if len(discovered) != 0 {
		t.Errorf("DiscoverAgents() expected empty, got %v", discovered)
	}
}

func TestDiscoverAgentsMultiplePresent(t *testing.T) {
	home := t.TempDir()

	// Create both retained Claude and Codex config dirs
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	discovered := DiscoverAgents(home)

	if len(discovered) < 2 {
		t.Errorf("DiscoverAgents() expected at least 2 agents when both config dirs exist, got %v", discovered)
	}
}

// TestDiscoverAgentsDelegatesCanonicalDiscovery proves that DiscoverAgents
// derives results from canonical adapter-driven discovery rather than a stale
// hardcoded list. After the Phase 2 rewire, the wrapper must:
//   - return agents whose adapter.GlobalConfigDir exists on disk
//   - not return agents whose config dir is absent
//   - produce the same set as agents.DiscoverInstalled for the same homeDir
func TestDiscoverAgentsDelegatesCanonicalDiscovery(t *testing.T) {
	home := t.TempDir()

	// Create only the codex config dir — a less-common agent that would be
	// absent from a minimal stale hardcoded list if someone forgot to update it.
	// This verifies the wrapper consults the registry, not a frozen snapshot.
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	discovered := DiscoverAgents(home)

	// codex MUST be discovered because its config dir exists.
	found := false
	for _, id := range discovered {
		if id == model.AgentCodex {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DiscoverAgents() did not return codex even though ~/.codex/ exists; got %v — wrapper must delegate to canonical registry discovery", discovered)
	}

	// No other agents should appear — their dirs don't exist.
	for _, id := range discovered {
		if id != model.AgentCodex {
			t.Errorf("DiscoverAgents() returned unexpected agent %q — no other config dirs were created", id)
		}
	}
}

// ─── Phase 3: componentSyncStep ───────────────────────────────────────────

// TestComponentSyncStepPreservesSlimEngramProtocol pins bug #1824: install
// threads the detected engram binary version into engram.InjectOptions.Version
// (internal/cli/run.go), so a verified Claude Code install renders the SLIM
// engram-protocol CLAUDE.md section. The sync path built InjectOptions WITHOUT
// Version, so every `gentle-ai sync` silently re-inflated the slim section
// back to the full (~6.7 KB) one. Sync must detect the version identically
// (resolveEngramVersion) and keep the installed slim section byte-identical.
func TestComponentSyncStepPreservesSlimEngramProtocol(t *testing.T) {
	home := t.TempDir()

	const installedEngramVersion = "engram 1.18.0" // above the v1.4.0 slim floor
	restoreVerify := verifyEngramVersion
	t.Cleanup(func() { verifyEngramVersion = restoreVerify })
	verifyEngramVersion = func() (string, error) { return installedEngramVersion, nil }

	adapter, err := agents.NewAdapter(model.AgentClaudeCode)
	if err != nil {
		t.Fatalf("NewAdapter(claude-code) error = %v", err)
	}

	// Install path: Version is threaded, so the SLIM section is written.
	if _, err := engram.InjectWithOptions(home, adapter, engram.InjectOptions{Version: installedEngramVersion}); err != nil {
		t.Fatalf("install-path InjectWithOptions error = %v", err)
	}
	claudeMD := filepath.Join(home, ".claude", "CLAUDE.md")
	registryPath := filepath.Join(home, ".claude.json")
	installed, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("ReadFile(CLAUDE.md) after install error = %v", err)
	}
	if strings.Contains(string(installed), "needs_review") || !strings.Contains(string(installed), "SessionStart hook") {
		t.Fatalf("precondition failed: install did not write the SLIM section:\n%s", installed)
	}
	installedRegistry, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("ReadFile(user registry) after install error = %v", err)
	}

	// Sync over the installed state must preserve the slim section.
	var changed []string
	step := componentSyncStep{
		id:           "sync:engram",
		component:    model.ComponentEngram,
		homeDir:      home,
		agents:       []model.AgentID{model.AgentClaudeCode},
		changedFiles: &changed,
	}
	if err := step.Run(); err != nil {
		t.Fatalf("componentSyncStep.Run() error = %v", err)
	}
	afterSync, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("ReadFile(CLAUDE.md) after sync error = %v", err)
	}
	if strings.Contains(string(afterSync), "needs_review") {
		t.Fatalf("sync re-inflated the SLIM engram-protocol section back to FULL (bug #1824):\n%s", afterSync)
	}
	if !bytes.Equal(installed, afterSync) {
		t.Fatalf("sync must leave the installed CLAUDE.md byte-identical\ninstalled:\n%s\nafter sync:\n%s", installed, afterSync)
	}
	afterSyncRegistry, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("ReadFile(user registry) after sync error = %v", err)
	}
	if !bytes.Equal(installedRegistry, afterSyncRegistry) {
		t.Fatal("sync must leave Claude's user registry byte-identical")
	}

	// Idempotency guard: a second sync must not report the section changed.
	changed = nil
	if err := step.Run(); err != nil {
		t.Fatalf("second componentSyncStep.Run() error = %v", err)
	}
	afterSecondSync, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("ReadFile(user registry) after second sync error = %v", err)
	}
	if !bytes.Equal(installedRegistry, afterSecondSync) {
		t.Fatal("repeated sync must leave Claude's user registry byte-identical")
	}
	for _, f := range changed {
		if f == claudeMD {
			t.Errorf("second sync reported CLAUDE.md as changed; want no-op on the engram-protocol section")
		}
	}
	afterSecond, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("ReadFile(CLAUDE.md) after second sync error = %v", err)
	}
	if !bytes.Equal(installed, afterSecond) {
		t.Fatalf("second sync must remain byte-identical to the installed CLAUDE.md")
	}
}

func TestComponentSyncStepCodexRuntimeGate(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{name: "old runtime leaves profiles untouched", version: "codex-cli 0.143.9", wantErr: true},
		{name: "exact runtime writes profiles", version: "codex-cli 0.144.0"},
		{name: "new runtime writes profiles", version: "codex-cli 0.145.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := codex.SetRuntimeVersionCommandForTest(tt.version, nil)
			t.Cleanup(restore)
			home := t.TempDir()
			codexDir := filepath.Join(home, ".codex")
			if err := os.MkdirAll(codexDir, 0o755); err != nil {
				t.Fatal(err)
			}
			profiles := []string{"sdd-strong.config.toml", "sdd-mid.config.toml", "sdd-cheap.config.toml"}
			for _, name := range profiles {
				if err := os.WriteFile(filepath.Join(codexDir, name), []byte("user-content\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			step := componentSyncStep{component: model.ComponentEngram, homeDir: home, agents: []model.AgentID{model.AgentCodex}}
			err := step.Run()
			if (err != nil) != tt.wantErr {
				t.Fatalf("componentSyncStep.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			for _, name := range profiles {
				content, readErr := os.ReadFile(filepath.Join(codexDir, name))
				if readErr != nil {
					t.Fatal(readErr)
				}
				if tt.wantErr && string(content) != "user-content\n" {
					t.Errorf("old runtime modified %s: %q", name, content)
				}
				if !tt.wantErr && !strings.Contains(string(content), "gpt-5.6-") {
					t.Errorf("valid runtime did not write GPT-5.6 profile %s: %q", name, content)
				}
			}
		})
	}
}

func TestComponentSyncStepWritesPiPersonaToWorkspaceAndReportsChangedFile(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	path := filepath.Join(workspace, ".pi", "gentle-ai", "persona.json")
	var changed []string
	step := componentSyncStep{
		id:           "sync:persona",
		component:    model.ComponentPersona,
		homeDir:      home,
		workspaceDir: workspace,
		agents:       []model.AgentID{model.AgentPi},
		selection:    model.Selection{Persona: model.PersonaNeutral},
		changedFiles: &changed,
	}

	if err := step.Run(); err != nil {
		t.Fatalf("first Pi persona sync error = %v", err)
	}
	if !containsPath(changed, path) {
		t.Fatalf("first Pi persona sync changed files = %v, missing %q", changed, path)
	}
	if _, err := os.Stat(filepath.Join(home, ".pi", "gentle-ai", "persona.json")); !os.IsNotExist(err) {
		t.Fatalf("Pi sync wrote a home-scoped persona config; stat err = %v", err)
	}
	if got := readTextFile(t, path); got != "{\n  \"mode\": \"neutral\"\n}\n" {
		t.Fatalf("Pi persona config = %q, want neutral mode", got)
	}

	changed = nil
	if err := step.Run(); err != nil {
		t.Fatalf("second Pi persona sync error = %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("second Pi persona sync changed files = %v, want none", changed)
	}
}

func TestSyncPersonaPathsAndBackupTargetsTrackOnlyPiWorkspaceConfig(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentPi},
		Components: []model.ComponentID{model.ComponentPersona},
		Persona:    model.PersonaNeutral,
	}
	adapters := resolveAdapters(selection.Agents)
	want := filepath.Join(workspace, ".pi", "gentle-ai", "persona.json")
	unwanted := filepath.Join(home, ".pi", "gentle-ai", "persona.json")

	paths := syncPersonaPathsWithWorkspace(home, workspace, selection, adapters)
	if !containsPath(paths, want) || containsPath(paths, unwanted) {
		t.Fatalf("sync persona paths = %v, want only workspace Pi config %q", paths, want)
	}
	targets, err := syncBackupTargets(home, workspace, selection, adapters)
	if err != nil {
		t.Fatalf("syncBackupTargets() error = %v", err)
	}
	if !containsPath(targets, want) || containsPath(targets, unwanted) {
		t.Fatalf("sync backup targets = %v, want only workspace Pi config %q", targets, want)
	}

	selection.Persona = model.PersonaCustom
	if paths := syncPersonaPathsWithWorkspace(home, workspace, selection, adapters); len(paths) != 0 {
		t.Fatalf("custom sync persona paths = %v, want none", paths)
	}
}

func TestPiPersonaSyncSnapshotRestoresWorkspaceConfig(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	path := filepath.Join(workspace, ".pi", "gentle-ai", "persona.json")
	mustWriteFile(t, path, []byte("{\n  \"mode\": \"gentleman\"\n}\n"))

	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentPi},
		Components: []model.ComponentID{model.ComponentPersona},
		Persona:    model.PersonaNeutral,
	}
	targets, err := syncBackupTargets(home, workspace, selection, resolveAdapters(selection.Agents))
	if err != nil {
		t.Fatalf("syncBackupTargets() error = %v", err)
	}
	before, err := snapshotSyncFiles(targets)
	if err != nil {
		t.Fatalf("snapshotSyncFiles() error = %v", err)
	}
	step := componentSyncStep{
		component:    model.ComponentPersona,
		homeDir:      home,
		workspaceDir: workspace,
		agents:       selection.Agents,
		selection:    selection,
	}
	if err := step.Run(); err != nil {
		t.Fatalf("Pi persona sync error = %v", err)
	}
	if got := readTextFile(t, path); got == "{\n  \"mode\": \"gentleman\"\n}\n" {
		t.Fatal("Pi persona sync did not change the workspace config before rollback")
	}
	if err := restoreSyncFiles(before); err != nil {
		t.Fatalf("restoreSyncFiles() error = %v", err)
	}
	if got, want := readTextFile(t, path), "{\n  \"mode\": \"gentleman\"\n}\n"; got != want {
		t.Fatalf("restored Pi persona config = %q, want %q", got, want)
	}
}

// Retired OpenCode selections are rejected before compatibility-only plugin
// refresh code can be reached. The old plugin scenario remains represented by
// an explicit refusal test rather than a misleading skipped integration test.

func TestSyncBackupTargetsIncludeClaudeEngramLegacyMigrationSource(t *testing.T) {
	home := t.TempDir()
	selection := model.Selection{Agents: []model.AgentID{model.AgentClaudeCode}, Components: []model.ComponentID{model.ComponentEngram}}
	targets, err := syncBackupTargets(home, "", selection, resolveAdapters(selection.Agents))
	if err != nil {
		t.Fatalf("syncBackupTargets() error = %v", err)
	}
	want := filepath.Join(home, ".claude", "mcp", "engram.json")
	if !containsPath(targets, want) {
		t.Fatalf("sync backup targets missing legacy migration source %q: %v", want, targets)
	}
}

func TestSyncBackupTargetsIncludeClaudeContext7CleanupPath(t *testing.T) {
	home := t.TempDir()
	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentClaudeCode},
		Components: []model.ComponentID{model.ComponentContext7},
	}

	targets, err := syncBackupTargets(home, "", selection, resolveAdapters(selection.Agents))
	if err != nil {
		t.Fatalf("syncBackupTargets() error = %v", err)
	}
	want := filepath.Join(home, ".claude", "settings.json")
	if !containsPath(targets, want) {
		t.Fatalf("sync backup targets missing Claude Context7 cleanup path %q: %v", want, targets)
	}
}

type failingSyncStep struct{}

func (failingSyncStep) ID() string { return "sync:test:fail-after-engram" }
func (failingSyncStep) Run() error { return errors.New("forced failure after Engram migration") }

func TestRunSyncRollbackRestoresClaudeEngramMigrationSource(t *testing.T) {
	home := t.TempDir()
	registryPath := filepath.Join(home, ".claude.json")
	legacyPath := filepath.Join(home, ".claude", "mcp", "engram.json")
	registryBefore := []byte(`{"oauthAccount":{"emailAddress":"user@example.com"},"mcpServers":{"codegraph":{"command":"codegraph"}}}`)
	legacyBefore := []byte("{\n  \"command\": \"/usr/local/bin/engram\",\n  \"args\": [\"mcp\", \"--tools=agent\"]\n}\n")
	if err := os.WriteFile(registryPath, registryBefore, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, legacyBefore, 0o604); err != nil {
		t.Fatal(err)
	}
	registryInfo, _ := os.Stat(registryPath)
	legacyInfo, _ := os.Stat(legacyPath)
	promptPath := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := os.WriteFile(promptPath, []byte("original prompt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	restoreBackupHome := backup.UserHomeDirFn
	backup.UserHomeDirFn = func() (string, error) { return home, nil }
	t.Cleanup(func() { backup.UserHomeDirFn = restoreBackupHome })

	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentClaudeCode},
		Components: []model.ComponentID{model.ComponentEngram},
	}
	for attempt := 1; attempt <= 2; attempt++ {
		rt, err := newSyncRuntime(home, selection)
		if err != nil {
			t.Fatal(err)
		}
		plan := rt.stagePlan()
		plan.Apply = append(plan.Apply, failingSyncStep{})
		result := pipeline.NewOrchestrator(pipeline.DefaultRollbackPolicy()).Execute(plan)
		if result.Err == nil {
			t.Fatalf("sync transaction attempt %d error = nil; want forced post-migration failure", attempt)
		}
		if len(result.Apply.Steps) < 3 || result.Apply.Steps[1].StepID != "sync:component:engram" || result.Apply.Steps[1].Status != pipeline.StepStatusSucceeded {
			t.Fatalf("attempt %d Engram migration did not complete before failure: error=%v steps=%#v", attempt, result.Err, result.Apply.Steps)
		}
		if !result.Rollback.Success {
			t.Fatalf("sync rollback attempt %d failed: error=%v steps=%#v", attempt, result.Rollback.Err, result.Rollback.Steps)
		}
		if rt.state.rollbackSnapshotDir != "" {
			t.Fatalf("sync rollback attempt %d left transaction snapshot %q", attempt, rt.state.rollbackSnapshotDir)
		}
		for _, file := range []struct {
			path string
			data []byte
			mode os.FileMode
		}{{registryPath, registryBefore, registryInfo.Mode().Perm()}, {legacyPath, legacyBefore, legacyInfo.Mode().Perm()}} {
			got, err := os.ReadFile(file.path)
			if err != nil || !bytes.Equal(got, file.data) {
				t.Fatalf("rollback attempt %d did not restore %q bytes: got=%q error=%v", attempt, file.path, got, err)
			}
			if runtime.GOOS != "windows" {
				info, err := os.Stat(file.path)
				if err != nil || info.Mode().Perm() != file.mode {
					t.Fatalf("rollback attempt %d did not restore %q mode: got=%v error=%v want=%v", attempt, file.path, info, err, file.mode)
				}
			}
		}
	}
	backups, err := os.ReadDir(filepath.Join(home, ".gentle-ai", "backups"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("persistent backup count = %d, want 1 after duplicate transaction", len(backups))
	}
}

func TestRestoreSyncFilesNeverWidensZeroMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exact zero-mode restoration is a POSIX permission contract")
	}
	tests := []struct {
		name  string
		setup func(t *testing.T) (string, syncFileSnapshot)
	}{
		{
			name: "regular file",
			setup: func(t *testing.T) (string, syncFileSnapshot) {
				path := filepath.Join(t.TempDir(), "managed.json")
				mustWriteFile(t, path, []byte("changed"))
				return path, syncFileSnapshot{exists: true, data: []byte("original"), mode: 0}
			},
		},
		{
			name: "symlink chain target",
			setup: func(t *testing.T) (string, syncFileSnapshot) {
				root := t.TempDir()
				targetPath := filepath.Join(root, "versions", "managed.json")
				innerPath := filepath.Join(root, "current.json")
				path := filepath.Join(root, "config", "managed.json")
				mustWriteFile(t, targetPath, []byte("changed"))
				if err := os.Symlink(targetPath, innerPath); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(innerPath, path); err != nil {
					t.Fatal(err)
				}
				return path, syncFileSnapshot{exists: true, data: []byte("original"), mode: 0, symlink: true, linkTarget: innerPath, targetPath: targetPath, targetExists: true}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, snapshot := tt.setup(t)
			originalWriter := writeSyncFileAtomic
			t.Cleanup(func() { writeSyncFileAtomic = originalWriter })
			writeSyncFileAtomic = func(path string, content []byte, mode os.FileMode) (filemerge.WriteResult, error) {
				if mode != 0o600 {
					t.Fatalf("atomic restore mode = %o, want restrictive 0600", mode)
				}
				result, err := originalWriter(path, content, mode)
				if err == nil {
					info, statErr := os.Stat(path)
					if statErr != nil {
						t.Fatal(statErr)
					}
					if got := info.Mode().Perm(); got != 0o600 {
						t.Fatalf("mode immediately after atomic replace = %o, want 0600", got)
					}
				}
				return result, err
			}

			if err := restoreSyncFiles(map[string]syncFileSnapshot{path: snapshot}); err != nil {
				t.Fatalf("restoreSyncFiles() error = %v", err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != 0 {
				t.Fatalf("restored mode = %o, want exact 000", got)
			}
		})
	}
}

func TestCodeGraphGuidanceSyncStepRepairsCodexConfigOnlyGuidance(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".codex", "config.toml")
	agentsPath := filepath.Join(home, ".codex", "AGENTS.md")
	mustWriteFile(t, configPath, []byte(strings.Join([]string{
		`[mcp_servers.codegraph]`,
		`command = "codegraph"`,
	}, "\n")))

	restoreLookPath := cmdLookPath
	t.Cleanup(func() { cmdLookPath = restoreLookPath })
	cmdLookPath = func(name string) (string, error) {
		if name != "codegraph" {
			return "", os.ErrNotExist
		}
		return "/bin/codegraph", nil
	}

	var changed []string
	step := codeGraphGuidanceSyncStep{
		id:           "sync:community-tool:codegraph-guidance",
		homeDir:      home,
		changedFiles: &changed,
	}
	if err := step.Run(); err != nil {
		t.Fatalf("codeGraphGuidanceSyncStep.Run() error = %v", err)
	}

	body, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", agentsPath, err)
	}
	text := string(body)
	for _, want := range []string{"<!-- gentle-ai:codegraph-guidance -->", "immediately run `gentle-ai codegraph init --cwd <project-root>`"} {
		if !strings.Contains(text, want) {
			t.Fatalf("Codex AGENTS.md missing managed CodeGraph guidance %q:\n%s", want, text)
		}
	}
	if !reflect.DeepEqual(changed, []string{agentsPath}) {
		t.Fatalf("changed files = %#v, want %#v", changed, []string{agentsPath})
	}
}

func TestRunSyncMigratesLegacyManagedPiCodeGraphSelection(t *testing.T) {
	home := t.TempDir()
	if err := state.Write(home, state.InstallState{InstalledAgents: []string{"pi"}, Persona: "neutral"}); err != nil {
		t.Fatal(err)
	}
	writeManagedPiCodeGraphManifest(t, home)

	previousRefresh := refreshPiCodeGraphIfConfigured
	refreshed := false
	refreshPiCodeGraphIfConfigured = func(string, string) (codegraph.PiCodeGraphResult, bool, error) {
		refreshed = true
		return codegraph.PiCodeGraphResult{}, true, nil
	}
	t.Cleanup(func() { refreshPiCodeGraphIfConfigured = previousRefresh })

	result, err := RunSyncWithSelection(home, model.Selection{Agents: []model.AgentID{model.AgentPi}, Persona: model.PersonaNeutral})
	if err != nil {
		t.Fatalf("RunSyncWithSelection() error = %v", err)
	}
	if !result.Selection.HasCommunityTool(model.CommunityToolCodeGraph) || !refreshed {
		t.Fatalf("legacy Pi selection = %v, refreshed = %t; want selected and reconciled", result.Selection.CommunityTools, refreshed)
	}
	persisted, err := state.Read(home)
	if err != nil {
		t.Fatal(err)
	}
	if !persisted.CommunityToolsConfigured || !reflect.DeepEqual(persisted.CommunityTools, []string{"codegraph"}) {
		t.Fatalf("persisted community tools = (%v, %t), want migrated CodeGraph selection", persisted.CommunityTools, persisted.CommunityToolsConfigured)
	}
}

// TestRunSyncReportsLegacySelectionMigrationPersistenceFailure verifies that
// failed legacy migration persistence restores both managed assets and state.

func writeManagedPiCodeGraphManifest(t *testing.T, home string) {
	t.Helper()
	manifestPath := filepath.Join(home, ".gentle-ai", "pi-codegraph.json")
	mcpPath := filepath.Join(home, ".pi", "agent", "mcp.json")
	mustWriteFile(t, manifestPath, []byte(`{"mcpPath":`+strconv.Quote(mcpPath)+`,"mcp":{"afterHash":"managed"},"children":{}}`))
	if err := os.Chmod(manifestPath, 0o600); err != nil {
		t.Fatal(err)
	}
}

func hasStepID(steps []pipeline.Step, id string) bool {
	for _, step := range steps {
		if step.ID() == id {
			return true
		}
	}
	return false
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

// ─── Phase 4: RunSync integration tests ───────────────────────────────────

func TestRunSyncDoesNotInvokeEngramSetup(t *testing.T) {
	home := t.TempDir()
	restoreHome := osUserHomeDir
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }

	var commandsCalled []string
	runCommand = func(name string, args ...string) error {
		commandsCalled = append(commandsCalled, name+" "+strings.Join(args, " "))
		return nil
	}

	_, err := RunSync([]string{"--agents", "claude-code"})
	if err != nil {
		t.Fatalf("RunSync() error = %v", err)
	}

	for _, cmd := range commandsCalled {
		if strings.Contains(cmd, "engram setup") {
			t.Errorf("RunSync must NOT invoke engram setup, got command: %s", cmd)
		}
	}
}

func TestRunSyncDoesNotInstallBinaries(t *testing.T) {
	home := t.TempDir()
	restoreHome := osUserHomeDir
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	// Simulate all binaries as missing.
	cmdLookPath = func(name string) (string, error) {
		return "", os.ErrNotExist
	}

	var commandsCalled []string
	runCommand = func(name string, args ...string) error {
		commandsCalled = append(commandsCalled, name+" "+strings.Join(args, " "))
		return nil
	}

	_, err := RunSync([]string{"--agents", "claude-code"})
	if err != nil {
		t.Fatalf("RunSync() error = %v", err)
	}

	// No binary installation commands.
	for _, cmd := range commandsCalled {
		if strings.Contains(cmd, "brew install") || strings.Contains(cmd, "go install") ||
			strings.Contains(cmd, "git clone") || strings.Contains(cmd, "npm install") {
			t.Errorf("RunSync must NOT install binaries, got command: %s", cmd)
		}
	}
}

func TestRunSyncPreservesUnmanagedAdjacentFiles(t *testing.T) {
	home := t.TempDir()

	// Create a user-owned config file adjacent to the managed Claude files.
	userConfigDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(userConfigDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	userConfigPath := filepath.Join(userConfigDir, "my-custom-config.json")
	const userContent = `{"my": "custom"}`
	if err := os.WriteFile(userConfigPath, []byte(userContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	restoreHome := osUserHomeDir
	restoreBackupHome := backup.UserHomeDirFn
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		backup.UserHomeDirFn = restoreBackupHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	backup.UserHomeDirFn = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }

	_, err := RunSync([]string{"--agents", "claude-code"})
	if err != nil {
		t.Fatalf("RunSync() error = %v", err)
	}

	// User's custom file must be byte-for-byte unchanged.
	after, err := os.ReadFile(userConfigPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(after) != userContent {
		t.Errorf("user config modified by sync: got %q, want %q", string(after), userContent)
	}
}

func TestRunSyncDryRunDoesNotWriteFiles(t *testing.T) {
	home := t.TempDir()
	restoreHome := osUserHomeDir
	restoreBackupHome := backup.UserHomeDirFn
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		backup.UserHomeDirFn = restoreBackupHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	backup.UserHomeDirFn = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }

	result, err := RunSync([]string{"--agents", "claude-code", "--dry-run"})
	if err != nil {
		t.Fatalf("RunSync() error = %v", err)
	}

	if !result.DryRun {
		t.Fatalf("DryRun = false, want true")
	}

	if len(result.Execution.Apply.Steps) != 0 || len(result.Execution.Prepare.Steps) != 0 {
		t.Fatalf("execution should be empty in dry-run")
	}

	// No CLAUDE.md should have been created.
	claudeMDPath := filepath.Join(home, ".claude", "CLAUDE.md")
	if _, err := os.Stat(claudeMDPath); err == nil {
		t.Errorf("dry-run should NOT create files, but %q was created", claudeMDPath)
	}
}

func TestRunSyncIsIdempotent(t *testing.T) {
	home := t.TempDir()
	restoreHome := osUserHomeDir
	restoreBackupHome := backup.UserHomeDirFn
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		backup.UserHomeDirFn = restoreBackupHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	backup.UserHomeDirFn = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }

	args := []string{"--agents", "claude-code", "--sdd-mode", "single"}

	// Run 1
	result1, err := RunSync(args)
	if err != nil {
		t.Fatalf("RunSync() run 1 error = %v", err)
	}
	if !result1.Verify.Ready {
		t.Fatalf("run 1: Verify.Ready = false")
	}

	claudeMDPath := filepath.Join(home, ".claude", "CLAUDE.md")
	contentAfterRun1, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("ReadFile() run 1 error = %v", err)
	}

	// Run 2
	result2, err := RunSync(args)
	if err != nil {
		t.Fatalf("RunSync() run 2 error = %v", err)
	}
	if !result2.Verify.Ready {
		t.Fatalf("run 2: Verify.Ready = false")
	}

	contentAfterRun2, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("ReadFile() run 2 error = %v", err)
	}

	if string(contentAfterRun1) != string(contentAfterRun2) {
		t.Errorf("CLAUDE.md changed between sync run 1 and run 2 (idempotency violation):\n--- run1 ---\n%s\n--- run2 ---\n%s",
			contentAfterRun1, contentAfterRun2)
	}
}

// ─── Gap 1: No-op / No managed assets ─────────────────────────────────────

// TestRunSyncNoOpWhenNoAgentsDiscovered verifies the spec scenario:
// "No managed assets to sync — system completes without modifying unrelated
// files and reports that no managed sync actions were needed."
func TestRunSyncNoOpWhenNoAgentsDiscovered(t *testing.T) {
	home := t.TempDir()
	restoreHome := osUserHomeDir
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	// Empty home — no agent config dirs exist, so DiscoverAgents returns nil.
	osUserHomeDir = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }

	// No --agents flag and no config dirs — auto-discovery yields nothing.
	result, err := RunSync([]string{})
	if err != nil {
		t.Fatalf("RunSync() no-op error = %v", err)
	}

	// No agents discovered.
	if len(result.Agents) != 0 {
		t.Errorf("expected no agents discovered, got %v", result.Agents)
	}

	// Must be marked as no-op.
	if !result.NoOp {
		t.Errorf("SyncResult.NoOp = false, want true when no agents are discovered")
	}

	// Must produce a human-readable message saying no managed sync actions were needed.
	report := RenderSyncReport(result)
	if !containsAny(report, "no managed", "no sync", "nothing to sync", "0 actions") {
		t.Errorf("RenderSyncReport() should indicate no managed actions; got:\n%s", report)
	}
}

// ─── Gap 2: Report managed actions executed ────────────────────────────────

// TestRenderSyncReportIncludesManagedActions verifies that the sync output
// reports the managed actions that were executed, not just verification results.
func TestRenderSyncReportIncludesManagedActions(t *testing.T) {
	home := t.TempDir()
	restoreHome := osUserHomeDir
	restoreBackupHome := backup.UserHomeDirFn
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		backup.UserHomeDirFn = restoreBackupHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	backup.UserHomeDirFn = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }

	result, err := RunSync([]string{"--agents", "claude-code", "--sdd-mode", "single"})
	if err != nil {
		t.Fatalf("RunSync() error = %v", err)
	}

	report := RenderSyncReport(result)

	// Must mention the sync was executed (not just verification).
	if !containsAny(report, "synced", "sync", "managed", "component", "agent") {
		t.Errorf("RenderSyncReport() should mention managed actions; got:\n%s", report)
	}

	// Must list the agents involved.
	if !containsAny(report, "claude-code") {
		t.Errorf("RenderSyncReport() should list agents; got:\n%s", report)
	}
}

// ─── Gap 3: Unmanaged-lookalike-file exclusion ─────────────────────────────

// TestRunSyncExcludesUnmanagedLookalikeFile verifies the spec scenario:
// "User modified an unmanaged file that resembles a managed target —
// gentle-ai sync excludes it from the plan and does not adopt it."
//
// We create a file with the same NAME as a managed target but in a directory
// that is NOT part of the managed inventory (simulating an unmanaged lookalike).
// After sync, the lookalike must remain byte-for-byte unchanged.
func TestRunSyncExcludesUnmanagedLookalikeFile(t *testing.T) {
	home := t.TempDir()

	// Create a directory structure that is NOT the agent config dir.
	// "CLAUDE.md" is a known managed file for Claude (under ~/.claude/).
	// We place a lookalike at a path the sync runtime does NOT own.
	lookalikeDir := filepath.Join(home, "projects", "myapp")
	if err := os.MkdirAll(lookalikeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	lookalikePath := filepath.Join(lookalikeDir, "AGENTS.md")
	const lookalikeContent = "# My project AGENTS.md — NOT managed by gentle-ai"
	if err := os.WriteFile(lookalikePath, []byte(lookalikeContent), 0o644); err != nil {
		t.Fatalf("WriteFile() lookalike error = %v", err)
	}

	restoreHome := osUserHomeDir
	restoreBackupHome := backup.UserHomeDirFn
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		backup.UserHomeDirFn = restoreBackupHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	backup.UserHomeDirFn = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }

	_, err := RunSync([]string{"--agents", "claude-code"})
	if err != nil {
		t.Fatalf("RunSync() error = %v", err)
	}

	// The lookalike file must be byte-for-byte unchanged.
	after, err := os.ReadFile(lookalikePath)
	if err != nil {
		t.Fatalf("ReadFile() lookalike error = %v", err)
	}
	if string(after) != lookalikeContent {
		t.Errorf("sync modified unmanaged lookalike file: got %q, want %q", string(after), lookalikeContent)
	}

	// The managed Claude prompt path should have been written.
	managedPath := filepath.Join(home, ".claude", "CLAUDE.md")
	if _, err := os.Stat(managedPath); err != nil {
		t.Errorf("expected managed Claude prompt at %q to be created by sync: %v", managedPath, err)
	}
}

// ─── Verify Gaps ──────────────────────────────────────────────────────────

// TestRunSyncNoOpWhenAssetsAlreadyCurrent verifies the spec scenario:
// "No managed assets to sync — when all managed assets are already current
// (second sync on an already-synced home), the command reports no-op."
//
// This is distinct from TestRunSyncNoOpWhenNoAgentsDiscovered: agents ARE
// present, but all inject calls write nothing new (WriteFileAtomic is no-op).
func TestRunSyncNoOpWhenAssetsAlreadyCurrent(t *testing.T) {
	home := t.TempDir()
	restoreHome := osUserHomeDir
	restoreBackupHome := backup.UserHomeDirFn
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		backup.UserHomeDirFn = restoreBackupHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	backup.UserHomeDirFn = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }

	args := []string{"--agents", "claude-code", "--sdd-mode", "single"}

	// First sync — writes files, changes > 0.
	result1, err := RunSync(args)
	if err != nil {
		t.Fatalf("RunSync() first run error = %v", err)
	}
	if result1.NoOp {
		t.Fatalf("first sync should NOT be no-op; files were written for the first time")
	}
	if result1.FilesChanged == 0 {
		t.Fatalf("first sync: FilesChanged = 0, expected > 0 (files were written)")
	}

	// Second sync — all assets already current, WriteFileAtomic is a no-op.
	result2, err := RunSync(args)
	if err != nil {
		t.Fatalf("RunSync() second run error = %v", err)
	}

	// Must detect true no-op: agents are present but nothing changed.
	if !result2.NoOp {
		t.Errorf("second sync: SyncResult.NoOp = false, want true (all assets already current)")
	}
	if result2.FilesChanged != 0 {
		t.Errorf("second sync: FilesChanged = %d, want 0 (no files changed)", result2.FilesChanged)
	}

	report := RenderSyncReport(result2)
	if !containsAny(report, "no managed", "no sync", "nothing to sync", "0 actions", "already current", "up to date") {
		t.Errorf("RenderSyncReport() should indicate no changes on second run; got:\n%s", report)
	}
}

// TestSyncActionsExecutedReflectsChangedFiles verifies that "Sync actions
// executed" in the report reflects actual file changes, not step count.
//
// On a fresh home, files are written so the count must be > 0.
// On a second sync, nothing changes so the count must be 0.
func TestSyncActionsExecutedReflectsChangedFiles(t *testing.T) {
	home := t.TempDir()
	restoreHome := osUserHomeDir
	restoreBackupHome := backup.UserHomeDirFn
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		backup.UserHomeDirFn = restoreBackupHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	backup.UserHomeDirFn = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }

	args := []string{"--agents", "claude-code", "--sdd-mode", "single"}

	// First sync: files are new, so FilesChanged > 0.
	result1, err := RunSync(args)
	if err != nil {
		t.Fatalf("RunSync() first run error = %v", err)
	}
	if result1.FilesChanged == 0 {
		t.Errorf("first sync: FilesChanged = 0, want > 0")
	}
	report1 := RenderSyncReport(result1)
	// The report must state how many files were actually changed.
	if !containsAny(report1, "files changed", "file changed", "sync actions executed") {
		t.Errorf("first sync report should state changed-file count; got:\n%s", report1)
	}

	// Second sync: nothing new — FilesChanged must be 0.
	result2, err := RunSync(args)
	if err != nil {
		t.Fatalf("RunSync() second run error = %v", err)
	}
	if result2.FilesChanged != 0 {
		t.Errorf("second sync: FilesChanged = %d, want 0 (idempotent)", result2.FilesChanged)
	}
}

// ─── Task 5.5: Profile sync integration ───────────────────────────────────────

// TestRunSyncWithProfilesIntegration is the Task 5.5 integration test.
// It verifies the full profile sync flow:
// 1. Creates a temp home directory with a minimal opencode.json
// 2. Runs sync with 3 named profiles (cheap, premium, balanced)
// 3. Asserts all 33 profile agent keys are in the resulting opencode.json (11 × 3)
// 4. Asserts model assignments are set correctly on the orchestrators
// 5. Asserts prompt files exist in ~/.config/opencode/prompts/sdd/
// 6. Runs sync AGAIN with no changes → asserts filesChanged=0 (idempotent)

// TestRunSyncDetectsExistingProfilesOnRegularSync verifies Task 5.3 behavior:
// when no explicit profiles are provided (normal sync), DetectProfiles is called
// to find existing profiles and their prompts are regenerated.

// containsAny returns true if s contains any of the given substrings (case-insensitive).
func containsAny(s string, subs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// ─── T21: RunSyncWithSelection ─────────────────────────────────────────────

// TestRunSyncWithSelection_NoAgentsIsNoOp verifies that providing an empty
// agent list returns a no-op result without error.
func TestRunSyncWithSelection_NoAgentsIsNoOp(t *testing.T) {
	home := t.TempDir()

	sel := model.Selection{
		Agents:     nil,
		Components: []model.ComponentID{model.ComponentSDD, model.ComponentEngram},
	}

	result, err := RunSyncWithSelection(home, sel)
	if err != nil {
		t.Fatalf("RunSyncWithSelection() with no agents: error = %v", err)
	}
	if !result.NoOp {
		t.Errorf("RunSyncWithSelection() with no agents: NoOp = false, want true")
	}
}

// TestRunSyncWithSelection_WritesExpectedFiles verifies that the function
// creates managed asset files for the provided agents and components.

// TestRunSyncWithSelection_FilesChangedOnFreshHome verifies that syncing a
// fresh home dir results in FilesChanged > 0.

// TestRunSyncWithSelection_IsIdempotent verifies that running twice produces
// FilesChanged=0 on the second run (all assets already current).

// TestRunSyncWithSelection_SelectionAgentsForwarded verifies that the agents in
// the selection are reflected in the result.

// ─── State-aware DiscoverAgents ────────────────────────────────────────────

// TestDiscoverAgentsUsesStateFileWhenPresent verifies that DiscoverAgents
// returns only the agents recorded in state.json when the file exists and is
// non-empty, ignoring any agent config dirs that happen to be on disk.
//
// This covers issue #107: a user who installed only OpenCode should not have
// VS Code injected just because ~/.config/Code/ exists.

// TestDiscoverAgentsFallsBackToFSDiscoveryWhenStateMissing verifies that
// DiscoverAgents falls back to filesystem discovery when state.json is absent.
// This is the backward-compat path for users who installed before state
// persistence was added.
func TestDiscoverAgentsFallsBackToFSDiscoveryWhenStateMissing(t *testing.T) {
	home := t.TempDir()
	// No state.Write — state.json does not exist.

	// Create the claude-code config dir.
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	discovered := DiscoverAgents(home)

	// FS discovery must return claude-code since ~/.claude/ exists.
	found := false
	for _, id := range discovered {
		if id == model.AgentClaudeCode {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DiscoverAgents() fallback did not return claude-code; got %v", discovered)
	}
}

// TestDiscoverAgentsFallsBackToFSDiscoveryWhenStateEmpty verifies that
// DiscoverAgents falls back to filesystem discovery when state.json exists but
// contains an empty agent list — treating it the same as absent.
func TestDiscoverAgentsFallsBackToFSDiscoveryWhenStateEmpty(t *testing.T) {
	home := t.TempDir()

	// Write state with zero agents.
	if err := state.Write(home, state.InstallState{InstalledAgents: []string{}}); err != nil {
		t.Fatalf("state.Write() error = %v", err)
	}

	// Create the claude-code config dir.
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	discovered := DiscoverAgents(home)

	// FS discovery must pick up claude-code from disk.
	found := false
	for _, id := range discovered {
		if id == model.AgentClaudeCode {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DiscoverAgents() empty-state fallback did not return claude-code; got %v", discovered)
	}
}

// TestDiscoverAgentsStateMultipleAgents verifies that multiple agents persisted
// in state.json are all returned, in order.

// ─── Task 5: --strict-tdd flag ───────────────────────────────────────────────

// TestParseSyncFlagsStrictTDD verifies that --strict-tdd flag is parsed correctly.
func TestParseSyncFlagsStrictTDD(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{
			name: "absent defaults to false",
			args: []string{},
			want: false,
		},
		{
			name: "explicit true",
			args: []string{"--strict-tdd"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := ParseSyncFlags(tt.args)
			if err != nil {
				t.Fatalf("ParseSyncFlags() error = %v", err)
			}
			if flags.StrictTDD != tt.want {
				t.Errorf("StrictTDD = %v, want %v", flags.StrictTDD, tt.want)
			}
		})
	}
}

// TestBuildSyncSelectionStrictTDD verifies that StrictTDD flag is passed
// through to the Selection when building sync selection.
func TestBuildSyncSelectionStrictTDD(t *testing.T) {
	flags := SyncFlags{StrictTDD: true}
	sel := BuildSyncSelection(flags, nil)
	if !sel.StrictTDD {
		t.Errorf("Selection.StrictTDD = false, want true (should be propagated from flags)")
	}

	flagsDisabled := SyncFlags{StrictTDD: false}
	selDisabled := BuildSyncSelection(flagsDisabled, nil)
	if selDisabled.StrictTDD {
		t.Errorf("Selection.StrictTDD = true, want false")
	}
}

func TestRunSyncRestoresConfiguredSelectionAndExplicitOverrides(t *testing.T) {
	home := t.TempDir()
	if err := state.Write(home, state.InstallState{InstalledAgents: []string{"cursor"}, SelectionConfigured: true, Components: []model.ComponentID{model.ComponentEngram}, Skills: []model.SkillID{model.SkillCommentWriter}, Preset: model.PresetCustom}); err != nil {
		t.Fatal(err)
	}
	original := osUserHomeDir
	osUserHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { osUserHomeDir = original })
	plain, err := RunSync([]string{"--dry-run"})
	if err != nil || !reflect.DeepEqual(plain.Selection.Components, []model.ComponentID{model.ComponentEngram}) || !reflect.DeepEqual(plain.Selection.Skills, []model.SkillID{model.SkillCommentWriter}) || plain.Selection.Preset != model.PresetCustom {
		t.Fatalf("plain sync selection = %#v, err = %v", plain.Selection, err)
	}
	overridden, err := RunSync([]string{"--dry-run", "--skill", "sdd-init", "--sdd-mode", "multi", "--strict-tdd"})
	if err != nil || !reflect.DeepEqual(overridden.Selection.Skills, []model.SkillID{model.SkillSDDInit}) || !overridden.Selection.HasComponent(model.ComponentSkills) || overridden.Selection.SDDMode != model.SDDModeMulti || !overridden.Selection.StrictTDD {
		t.Fatalf("overridden sync selection = %#v, err = %v", overridden.Selection, err)
	}
	if err := state.Write(home, state.InstallState{InstalledAgents: []string{"cursor"}}); err != nil {
		t.Fatal(err)
	}
	result, err := RunSync([]string{"--dry-run"})
	if err != nil || result.Selection.Preset != model.PresetFullGentleman || !result.Selection.HasComponent(model.ComponentSkills) {
		t.Fatalf("legacy sync selection = %#v, err = %v", result.Selection, err)
	}
}

// ─── Phase 5: Profile CLI flags ───────────────────────────────────────────────

// TestParseSyncFlagsProfileSingleModel verifies that --profile name:provider/model
// produces a Profile with Name set and OrchestratorModel populated.

// TestParseSyncFlagsProfileMultiple verifies that multiple --profile flags
// produce multiple profiles.

// TestParseSyncFlagsProfilePhaseAssignment verifies that --profile-phase
// name:phase:provider/model sets PhaseAssignments["phase"] on the named profile.

// TestParseSyncFlagsProfileInvalidFormatReturnsError verifies that --profile
// with a missing colon separator returns an error.
func TestParseSyncFlagsProfileInvalidFormatReturnsError(t *testing.T) {
	_, err := ParseSyncFlags([]string{"--profile", "invalid"})
	if err == nil {
		t.Fatalf("expected error for --profile 'invalid' (missing colon), got nil")
	}
}

// TestParseSyncFlagsProfileEmptyNameReturnsError verifies that --profile with
// an empty name (:model) returns an error.
func TestParseSyncFlagsProfileEmptyNameReturnsError(t *testing.T) {
	_, err := ParseSyncFlags([]string{"--profile", ":anthropic/claude-haiku-3-5-20241022"})
	if err == nil {
		t.Fatalf("expected error for --profile ':model' (empty name), got nil")
	}
}

// TestParseSyncFlagsProfileReservedNameReturnsError verifies that --profile
// with the reserved name "default" returns an error.
func TestParseSyncFlagsProfileReservedNameReturnsError(t *testing.T) {
	_, err := ParseSyncFlags([]string{"--profile", "default:anthropic/claude-haiku-3-5-20241022"})
	if err == nil {
		t.Fatalf("expected error for --profile 'default:model' (reserved name), got nil")
	}
}

// TestParseSyncFlagsProfilePhaseUnknownPhaseReturnsError verifies that
// --profile-phase with an unknown phase name returns an error.
func TestParseSyncFlagsProfilePhaseUnknownPhaseReturnsError(t *testing.T) {
	_, err := ParseSyncFlags([]string{
		"--profile", "cheap:anthropic/claude-haiku-3-5-20241022",
		"--profile-phase", "cheap:sdd-bogus:anthropic/claude-haiku-3-5-20241022",
	})
	if err == nil {
		t.Fatalf("expected error for --profile-phase with unknown phase 'sdd-bogus', got nil")
	}
}

// TestBuildSyncSelectionProfilesForwarded verifies that Profiles from SyncFlags
// are forwarded to the model.Selection's overrides for use in the sync pipeline.

// ─── Persist model assignments across sync runs ─────────────────────────────

// TestRunSyncLoadsPersistedModelAssignments verifies that when state.json
// contains model assignments and no CLI flags override them, RunSync populates
// the selection with the persisted assignments rather than falling back to the
// "balanced" preset defaults.
func TestRunSyncLoadsPersistedModelAssignments(t *testing.T) {
	home := t.TempDir()
	restoreHome := osUserHomeDir
	restoreBackupHome := backup.UserHomeDirFn
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		backup.UserHomeDirFn = restoreBackupHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	backup.UserHomeDirFn = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }

	// Pre-seed state.json with model assignments from a previous install.
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	err := state.Write(home, state.InstallState{
		InstalledAgents: []string{"claude-code"},
		ClaudeModelAssignments: map[string]string{
			"orchestrator": "opus",
			"sdd-apply":    "sonnet",
		},
		ModelAssignments: map[string]state.ModelAssignmentState{
			"sdd-init": {ProviderID: "anthropic", ModelID: "claude-sonnet-4"},
		},
	})
	if err != nil {
		t.Fatalf("state.Write: %v", err)
	}

	// Run sync WITHOUT --claude-model or --model flags — assignments should
	// come from persisted state.
	result, err := RunSync([]string{"--agents", "claude-code", "--sdd-mode", "single"})
	if err != nil {
		t.Fatalf("RunSync() error = %v", err)
	}

	// Claude assignments must be loaded, excluding the main orchestrator model
	// because Claude Code controls the session model itself.
	if _, exists := result.Selection.ClaudeModelAssignments["orchestrator"]; exists {
		t.Errorf("ClaudeModelAssignments should not load persisted orchestrator model: %v", result.Selection.ClaudeModelAssignments)
	}
	if got := result.Selection.ClaudeModelAssignments["sdd-apply"]; got != "sonnet" {
		t.Errorf("ClaudeModelAssignments[sdd-apply] = %q, want %q", got, "sonnet")
	}
	// Persisted assignments must be loaded.
	ma := result.Selection.ModelAssignments["sdd-init"]
	if ma.ProviderID != "anthropic" || ma.ModelID != "claude-sonnet-4" {
		t.Errorf("ModelAssignments[sdd-init] = %+v, want anthropic/claude-sonnet-4", ma)
	}
}

func TestRunSyncLoadsPersistedModelAssignmentsPreservesEffort(t *testing.T) {
	home := t.TempDir()
	restoreHome := osUserHomeDir
	restoreBackupHome := backup.UserHomeDirFn
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		backup.UserHomeDirFn = restoreBackupHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	backup.UserHomeDirFn = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }

	if err := state.Write(home, state.InstallState{
		InstalledAgents: []string{"claude-code"},
		ModelAssignments: map[string]state.ModelAssignmentState{
			"sdd-apply": {ProviderID: "anthropic", ModelID: "claude-opus-4", Effort: "high"},
		},
	}); err != nil {
		t.Fatalf("state.Write: %v", err)
	}

	result, err := RunSync([]string{"--agents", "claude-code", "--sdd-mode", "single", "--dry-run"})
	if err != nil {
		t.Fatalf("RunSync() error = %v", err)
	}

	assignment := result.Selection.ModelAssignments["sdd-apply"]
	if assignment.Effort != "high" {
		t.Fatalf("Effort = %q, want high", assignment.Effort)
	}
}

// TestRunSyncDoesNotOverridePersistedAssignmentsOnSecondSync verifies the
// full cycle: sync1 loads persisted assignments → sync2 still has them.
// This is the core promise of the fix.
func TestRunSyncDoesNotOverridePersistedAssignmentsOnSecondSync(t *testing.T) {
	home := t.TempDir()
	restoreHome := osUserHomeDir
	restoreBackupHome := backup.UserHomeDirFn
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		backup.UserHomeDirFn = restoreBackupHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	backup.UserHomeDirFn = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }

	// Seed state with assignments.
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	err := state.Write(home, state.InstallState{
		InstalledAgents: []string{"claude-code"},
		ClaudeModelAssignments: map[string]string{
			"sdd-apply": "sonnet",
		},
		ModelAssignments: map[string]state.ModelAssignmentState{
			"sdd-init": {ProviderID: "anthropic", ModelID: "claude-sonnet-4"},
		},
	})
	if err != nil {
		t.Fatalf("state.Write: %v", err)
	}

	// First sync — loads from state.
	_, err = RunSync([]string{"--agents", "claude-code", "--sdd-mode", "single"})
	if err != nil {
		t.Fatalf("RunSync(1) error = %v", err)
	}
	firstPrompt, readErr := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md"))
	if readErr != nil {
		t.Fatalf("CLI sync did not write CLAUDE.md: %v", readErr)
	}

	// Second sync — should still have the assignments.
	result, err := RunSync([]string{"--agents", "claude-code", "--sdd-mode", "single"})
	if err != nil {
		t.Fatalf("RunSync(2) error = %v", err)
	}

	if got := result.Selection.ClaudeModelAssignments["sdd-apply"]; got != "sonnet" {
		t.Errorf("After second sync: ClaudeModelAssignments[sdd-apply] = %q, want %q", got, "sonnet")
	}
	ma := result.Selection.ModelAssignments["sdd-init"]
	if ma.ProviderID != "anthropic" || ma.ModelID != "claude-sonnet-4" {
		t.Errorf("After second sync: ModelAssignments[sdd-init] = %+v, want anthropic/claude-sonnet-4", ma)
	}
	secondPrompt, readErr := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md"))
	if readErr != nil {
		t.Fatalf("ReadFile(CLAUDE.md) after second sync: %v", readErr)
	}
	if !bytes.Equal(firstPrompt, secondPrompt) {
		t.Fatal("CLI sync was not byte-idempotent")
	}
}

// TestRunSyncWithNoPersistedAssignmentsDoesNotPanic verifies graceful behavior
// when state.json has no model assignments (backward compat with old state).
func TestRunSyncWithNoPersistedAssignmentsDoesNotPanic(t *testing.T) {
	home := t.TempDir()
	restoreHome := osUserHomeDir
	restoreBackupHome := backup.UserHomeDirFn
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		backup.UserHomeDirFn = restoreBackupHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	backup.UserHomeDirFn = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }

	// State with agents but NO model assignments (pre-feature state files).
	err := state.Write(home, state.InstallState{
		InstalledAgents: []string{"claude-code"},
	})
	if err != nil {
		t.Fatalf("state.Write: %v", err)
	}

	result, err := RunSync([]string{"--agents", "claude-code", "--sdd-mode", "single"})
	if err != nil {
		t.Fatalf("RunSync() error = %v", err)
	}

	// Should work fine — empty maps, no panic.
	if len(result.Selection.ClaudeModelAssignments) != 0 {
		t.Errorf("expected empty ClaudeModelAssignments, got %v", result.Selection.ClaudeModelAssignments)
	}
}

// ─── Phase 2: Persona-in-sync regression tests ─────────────────────────────

func setSyncTestHome(t *testing.T, home string) {
	t.Helper()
	rOSHome := osUserHomeDir
	rBackup := backup.UserHomeDirFn
	rRun := runCommand
	rLook := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = rOSHome
		backup.UserHomeDirFn = rBackup
		runCommand = rRun
		cmdLookPath = rLook
	})
	osUserHomeDir = func() (string, error) { return home, nil }
	backup.UserHomeDirFn = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }
}

func TestSyncBackupManifestIncludesCodexHooksJSON(t *testing.T) {
	home := t.TempDir()
	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	mustWriteFile(t, hooksPath, []byte("{\"hooks\":{\"SessionStart\":[]}}\n"))
	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentCodex},
		Components: []model.ComponentID{model.ComponentSDD},
	}
	targets, err := syncBackupTargets(home, "", selection, resolveAdapters(selection.Agents))
	if err != nil {
		t.Fatalf("syncBackupTargets() error = %v", err)
	}
	if !containsPath(targets, hooksPath) {
		t.Fatalf("sync backup targets missing %q\ntargets=%v", hooksPath, targets)
	}

	manifest, err := backup.NewSnapshotter().Create(filepath.Join(home, "snapshot"), targets)
	if err != nil {
		t.Fatalf("Snapshotter.Create() error = %v", err)
	}
	for _, entry := range manifest.Entries {
		if entry.OriginalPath == hooksPath {
			if !entry.Existed {
				t.Fatalf("hooks.json manifest entry marked absent: %#v", entry)
			}
			return
		}
	}
	t.Fatalf("snapshot manifest omitted %q: %#v", hooksPath, manifest.Entries)
}

func TestSyncCodexGentlemanConvergesWithHooksJSON(t *testing.T) {
	home := t.TempDir()
	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentCodex},
		Components: []model.ComponentID{model.ComponentPersona, model.ComponentSDD},
		Persona:    model.PersonaGentleman,
		SDDMode:    model.SDDModeSingle,
		CodexCarrilModelAssignments: map[string]string{
			"sdd-strong": "gpt-5.5",
			"sdd-mid":    "gpt-5.5",
			"sdd-cheap":  "gpt-5.4-mini",
		},
	}
	run := func() (int, []string) {
		rt, err := newSyncRuntime(home, selection)
		if err != nil {
			t.Fatalf("newSyncRuntime() error = %v", err)
		}
		plan := rt.stagePlan()
		before, err := snapshotSyncFiles(rt.managedPaths)
		if err != nil {
			t.Fatalf("snapshotSyncFiles() error = %v", err)
		}
		execution := pipeline.NewOrchestrator(pipeline.DefaultRollbackPolicy()).Execute(plan)
		if execution.Err != nil {
			t.Fatalf("sync stage plan error = %v", execution.Err)
		}
		changed, err := changedSyncFiles(rt.changedFiles, before)
		if err != nil {
			t.Fatalf("changedSyncFiles() error = %v", err)
		}
		return len(changed), changed
	}
	firstFiles, _ := run()
	if firstFiles == 0 {
		t.Fatal("first sync reported no managed changes")
	}
	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	firstHooks := readTextFile(t, hooksPath)

	secondFiles, changed := run()
	if secondFiles != 0 || len(changed) != 0 {
		t.Fatalf("second sync changed %d files: %v; want no changes", secondFiles, changed)
	}
	if got := readTextFile(t, hooksPath); got != firstHooks {
		t.Fatalf("hooks.json changed between converged syncs")
	}
}

// TestBuildSyncSelectionDoesNotHardcodePersona verifies that BuildSyncSelection
// leaves Persona empty so RunSync can resolve it from state.

// TestSyncPersonaPathsExcludeOpenCodeAgentJson verifies the install/sync
// contract split: syncPersonaPaths must NOT declare opencode.json (that JSON
// merge is install-only because it conflicts with SDD), even when the
// canonical Claude adapter is used.

func TestSyncPersonaPathsDeclareManagedClaudeOutputStyle(t *testing.T) {
	home := t.TempDir()
	reg, _ := agents.NewDefaultRegistry()
	a, _ := reg.Get(model.AgentClaudeCode)

	tests := []struct {
		name       string
		persona    model.PersonaID
		wantStyle  string
		unwanted   string
		wantConfig string
	}{
		{
			name:       "gentleman",
			persona:    model.PersonaGentleman,
			wantStyle:  filepath.Join(home, ".claude", "output-styles", "gentleman.md"),
			unwanted:   filepath.Join(home, ".claude", "output-styles", "neutral.md"),
			wantConfig: filepath.Join(home, ".claude", "settings.json"),
		},
		{
			name:       "neutral",
			persona:    model.PersonaNeutral,
			wantStyle:  filepath.Join(home, ".claude", "output-styles", "neutral.md"),
			unwanted:   filepath.Join(home, ".claude", "output-styles", "gentleman.md"),
			wantConfig: filepath.Join(home, ".claude", "settings.json"),
		},
		{
			name:       "legacy neutral alias",
			persona:    model.PersonaGentlemanNeutralArtifacts,
			wantStyle:  filepath.Join(home, ".claude", "output-styles", "neutral.md"),
			unwanted:   filepath.Join(home, ".claude", "output-styles", "gentleman.md"),
			wantConfig: filepath.Join(home, ".claude", "settings.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := syncPersonaPaths(home, model.Selection{Persona: tt.persona}, []agents.Adapter{a})

			if !containsPath(paths, tt.wantStyle) {
				t.Fatalf("syncPersonaPaths(%q) missing managed style %q; got %v", tt.persona, tt.wantStyle, paths)
			}
			if !containsPath(paths, tt.wantConfig) {
				t.Fatalf("syncPersonaPaths(%q) missing settings path %q; got %v", tt.persona, tt.wantConfig, paths)
			}
			if containsPath(paths, tt.unwanted) {
				t.Fatalf("syncPersonaPaths(%q) included wrong managed style %q; got %v", tt.persona, tt.unwanted, paths)
			}
		})
	}
}

// TestSyncBackupTargetsCaptureBothManagedOutputStyles pins the persona-switch
// backup fix: the pre-sync snapshot must capture BOTH managed output-style
// files so switching personas (which removes the previously selected file) can
// be rolled back. Verification stays on the selected file (asserted by
// TestSyncPersonaPathsDeclareManagedClaudeOutputStyle).
func TestSyncBackupTargetsCaptureBothManagedOutputStyles(t *testing.T) {
	home := t.TempDir()
	reg, _ := agents.NewDefaultRegistry()
	a, _ := reg.Get(model.AgentClaudeCode)

	gentleman := filepath.Join(home, ".claude", "output-styles", "gentleman.md")
	neutral := filepath.Join(home, ".claude", "output-styles", "neutral.md")

	for _, persona := range []model.PersonaID{model.PersonaGentleman, model.PersonaNeutral} {
		selection := model.Selection{Persona: persona, Components: []model.ComponentID{model.ComponentPersona}}
		targets, err := syncBackupTargets(home, "", selection, []agents.Adapter{a})
		if err != nil {
			t.Fatalf("syncBackupTargets(%q) error = %v", persona, err)
		}

		if !containsPath(targets, gentleman) {
			t.Errorf("syncBackupTargets(%q) missing gentleman.md; got %v", persona, targets)
		}
		if !containsPath(targets, neutral) {
			t.Errorf("syncBackupTargets(%q) missing neutral.md; got %v", persona, targets)
		}
	}
}

func TestPersonaSyncOutputStyleSwitchIsIdempotent(t *testing.T) {
	home := t.TempDir()
	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentClaudeCode},
		Components: []model.ComponentID{model.ComponentPersona},
		Persona:    model.PersonaGentlemanNeutralArtifacts,
	}
	gentleman := filepath.Join(home, ".claude", "output-styles", "gentleman.md")

	if _, err := persona.Inject(home, claude.NewAdapter(), model.PersonaGentleman); err != nil {
		t.Fatalf("Inject(gentleman) error = %v", err)
	}
	if _, err := os.Stat(gentleman); err != nil {
		t.Fatalf("precondition: gentleman output style missing: %v", err)
	}

	first, err := RunSyncWithSelection(home, selection)
	if err != nil {
		t.Fatalf("first RunSyncWithSelection() error = %v", err)
	}
	if first.FilesChanged == 0 || first.NoOp {
		t.Fatalf("first sync files changed = %d, no-op = %t; want output-style switch", first.FilesChanged, first.NoOp)
	}
	if _, err := os.Stat(gentleman); !os.IsNotExist(err) {
		t.Fatalf("first sync left retired gentleman output style: %v", err)
	}

	second, err := RunSyncWithSelection(home, selection)
	if err != nil {
		t.Fatalf("second RunSyncWithSelection() error = %v", err)
	}
	if second.FilesChanged != 0 || !second.NoOp {
		t.Fatalf("second sync files changed = %d, no-op = %t; want idempotent no-op", second.FilesChanged, second.NoOp)
	}
}

// TestBackupTargetsCaptureBothManagedOutputStyles is the install-side twin.
func TestBackupTargetsCaptureBothManagedOutputStyles(t *testing.T) {
	home := t.TempDir()

	gentleman := filepath.Join(home, ".claude", "output-styles", "gentleman.md")
	neutral := filepath.Join(home, ".claude", "output-styles", "neutral.md")

	for _, persona := range []model.PersonaID{model.PersonaGentleman, model.PersonaNeutral} {
		selection := model.Selection{Persona: persona, Components: []model.ComponentID{model.ComponentPersona}}
		resolved := planner.ResolvedPlan{
			Agents:            []model.AgentID{model.AgentClaudeCode},
			OrderedComponents: []model.ComponentID{model.ComponentPersona},
		}
		targets, err := backupTargets(home, "", ScopeGlobal, selection, resolved)
		if err != nil {
			t.Fatalf("backupTargets(%q) error = %v", persona, err)
		}

		if !containsPath(targets, gentleman) {
			t.Errorf("backupTargets(%q) missing gentleman.md; got %v", persona, targets)
		}
		if !containsPath(targets, neutral) {
			t.Errorf("backupTargets(%q) missing neutral.md; got %v", persona, targets)
		}
	}
}

// TestRunSyncRegeneratesPersonaBlockBetweenMarkers verifies the core fix:
// when an old persona block lives between markers, sync replaces it with the
// embedded asset for the current version.
func TestRunSyncRegeneratesPersonaBlockBetweenMarkers(t *testing.T) {
	home := t.TempDir()
	setSyncTestHome(t, home)

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Write a stale managed persona block — what an older version of gentle-ai
	// would have emitted. The sync must replace this with the v1.26 directive.
	stalePersona := "# pre-existing notes by user\n\n" +
		"<!-- gentle-ai:persona -->\n" +
		"## Skills (Auto-load based on context)\n\nstale 2-row table here.\n" +
		"<!-- /gentle-ai:persona -->\n"
	claudeMD := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte(stalePersona), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := state.Write(home, state.InstallState{
		InstalledAgents: []string{"claude-code"},
		Persona:         "gentleman",
	}); err != nil {
		t.Fatalf("state.Write: %v", err)
	}

	if _, err := RunSync([]string{"--agents", "claude-code", "--sdd-mode", "single"}); err != nil {
		t.Fatalf("RunSync() error = %v", err)
	}

	body, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(body)
	if !strings.Contains(got, "# pre-existing notes by user") {
		t.Errorf("CLAUDE.md content outside markers was not preserved; got:\n%s", got)
	}
	if strings.Contains(got, "Skills (Auto-load based on context)") {
		t.Errorf("CLAUDE.md still contains the stale Auto-load table; got:\n%s", got)
	}
	if !strings.Contains(got, "Contextual Skill Loading (MANDATORY)") {
		t.Errorf("CLAUDE.md missing the new Contextual Skill Loading directive; got:\n%s", got)
	}
}

// TestRunSyncReadsPersonaFromState verifies that sync uses the persona the
// user installed (from state.json) rather than always defaulting to Gentleman.
func TestRunSyncReadsPersonaFromState(t *testing.T) {
	home := t.TempDir()
	setSyncTestHome(t, home)

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := state.Write(home, state.InstallState{
		InstalledAgents: []string{"claude-code"},
		Persona:         "neutral",
	}); err != nil {
		t.Fatalf("state.Write: %v", err)
	}

	res, err := RunSync([]string{"--agents", "claude-code", "--sdd-mode", "single"})
	if err != nil {
		t.Fatalf("RunSync() error = %v", err)
	}
	if got, want := res.Selection.Persona, model.PersonaNeutral; got != want {
		t.Errorf("Selection.Persona = %q, want %q (read from state.json)", got, want)
	}
}

// TestRunSyncFallsBackToNeutralWhenStateLacksPersona verifies missing persona
// state resolves to neutral/default-safe behavior instead of reactivating
// Gentleman regional voice.
func TestRunSyncFallsBackToNeutralWhenStateLacksPersona(t *testing.T) {
	home := t.TempDir()
	setSyncTestHome(t, home)

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := state.Write(home, state.InstallState{
		InstalledAgents: []string{"claude-code"},
		// No Persona field — pre-feature state.
	}); err != nil {
		t.Fatalf("state.Write: %v", err)
	}

	res, err := RunSync([]string{"--agents", "claude-code", "--sdd-mode", "single"})
	if err != nil {
		t.Fatalf("RunSync() error = %v", err)
	}
	if got, want := res.Selection.Persona, model.PersonaNeutral; got != want {
		t.Errorf("Selection.Persona = %q, want %q (safe fallback for missing state persona)", got, want)
	}
}

func TestRunSyncWithSelectionPiUsesNeutralForMissingPersonaField(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Chdir(workspace)
	piPath := filepath.Join(workspace, ".pi", "gentle-ai", "persona.json")
	mustWriteFile(t, piPath, []byte("{\n  \"mode\": \"gentleman\"\n}\n"))
	mustWriteFile(t, state.Path(home), []byte(`{"installed_agents":["pi"]}`))

	result, err := RunSyncWithSelection(home, model.Selection{
		Agents:     []model.AgentID{model.AgentPi},
		Components: []model.ComponentID{model.ComponentPersona},
	})
	if err != nil {
		t.Fatalf("RunSyncWithSelection() error = %v", err)
	}
	if got, want := result.Selection.Persona, model.PersonaNeutral; got != want {
		t.Fatalf("Selection.Persona = %q, want %q", got, want)
	}
	if got, want := readTextFile(t, piPath), "{\n  \"mode\": \"neutral\"\n}\n"; got != want {
		t.Fatalf("Pi persona config = %q, want %q", got, want)
	}
}

func TestRunSyncWithSelectionPiRejectsInvalidPersistedPersonaWithoutMutation(t *testing.T) {
	tests := []struct {
		name       string
		stateJSON  string
		stateAsDir bool
		wantErr    string
	}{
		{name: "malformed JSON", stateJSON: `{"installed_agents":["pi"],"persona":`, wantErr: "state migration"},
		{name: "unreadable state", stateAsDir: true, wantErr: "state migration"},
		{name: "unsupported persona", stateJSON: `{"installed_agents":["pi"],"persona":"unknown"}`, wantErr: `unsupported persona "unknown"`},
		{name: "explicit empty persona", stateJSON: `{"schema_version":1,"installed_agents":["pi"],"persona":""}`, wantErr: "explicitly empty persona"},
		{name: "whitespace-only persona", stateJSON: `{"installed_agents":["pi"],"persona":" \t "}`, wantErr: "whitespace-only persona"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			workspace := t.TempDir()
			t.Chdir(workspace)
			piPath := filepath.Join(workspace, ".pi", "gentle-ai", "persona.json")
			originalPi := []byte("{\n  \"mode\": \"gentleman\"\n}\n")
			mustWriteFile(t, piPath, originalPi)

			if tt.stateAsDir {
				if err := os.MkdirAll(state.Path(home), 0o755); err != nil {
					t.Fatalf("MkdirAll(state path) error = %v", err)
				}
			} else {
				mustWriteFile(t, state.Path(home), []byte(tt.stateJSON))
			}

			result, err := RunSyncWithSelection(home, model.Selection{
				Agents:     []model.AgentID{model.AgentPi},
				Components: []model.ComponentID{model.ComponentPersona},
			})
			if err == nil {
				t.Fatal("RunSyncWithSelection() error = nil, want persisted persona validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("RunSyncWithSelection() error = %q, want %q", err, tt.wantErr)
			}
			if result.Selection.Persona != "" {
				t.Fatalf("Selection.Persona = %q, want unchanged empty persona", result.Selection.Persona)
			}
			if got := readTextFile(t, piPath); got != string(originalPi) {
				t.Fatalf("Pi persona config mutated after rejected sync: got %q, want %q", got, originalPi)
			}
		})
	}
}

func TestRunSyncPiRejectsUnsupportedPersistedPersonaBeforeMutation(t *testing.T) {
	home := t.TempDir()
	setSyncTestHome(t, home)
	workspace := t.TempDir()
	t.Chdir(workspace)
	piPath := filepath.Join(workspace, ".pi", "gentle-ai", "persona.json")
	originalPi := []byte("{\n  \"mode\": \"gentleman\"\n}\n")
	mustWriteFile(t, piPath, originalPi)
	mustWriteFile(t, state.Path(home), []byte(`{"installed_agents":["pi"],"persona":"unknown"}`))

	result, err := RunSync([]string{"--agents", "pi"})
	if err == nil {
		t.Fatal("RunSync() error = nil, want unsupported persisted persona error")
	}
	if !strings.Contains(err.Error(), `unsupported persona "unknown"`) {
		t.Fatalf("RunSync() error = %q, want unsupported persona", err)
	}
	if result.Selection.Persona != "" {
		t.Fatalf("Selection.Persona = %q, want unchanged empty persona", result.Selection.Persona)
	}
	if got := readTextFile(t, piPath); got != string(originalPi) {
		t.Fatalf("Pi persona config mutated after rejected sync: got %q, want %q", got, originalPi)
	}
}

func TestRunSyncWithSelectionPiCustomPersistedPersonaIsByteStable(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Chdir(workspace)
	piPath := filepath.Join(workspace, ".pi", "gentle-ai", "persona.json")
	originalPi := []byte("user-owned Pi persona bytes\n")
	mustWriteFile(t, piPath, originalPi)
	mustWriteFile(t, state.Path(home), []byte(`{"installed_agents":["pi"],"persona":"custom"}`))

	result, err := RunSyncWithSelection(home, model.Selection{
		Agents:     []model.AgentID{model.AgentPi},
		Components: []model.ComponentID{model.ComponentPersona},
	})
	if err != nil {
		t.Fatalf("RunSyncWithSelection() error = %v", err)
	}
	if got, want := result.Selection.Persona, model.PersonaCustom; got != want {
		t.Fatalf("Selection.Persona = %q, want %q", got, want)
	}
	if got := readTextFile(t, piPath); got != string(originalPi) {
		t.Fatalf("custom Pi persona config changed: got %q, want %q", got, originalPi)
	}
}

// ─── TUI path: RunSyncWithSelection persona resolution from state ───────────

// TestRunSyncWithSelection_PersonaResolvesFromStateNeutral verifies that when
// the TUI calls RunSyncWithSelection with an empty persona, the persisted
// persona from state.json is used — not the Gentleman default.
func TestRunSyncWithSelection_PersonaResolvesFromStateNeutral(t *testing.T) {
	home := t.TempDir()
	setSyncTestHome(t, home)

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := state.Write(home, state.InstallState{
		InstalledAgents: []string{"claude-code"},
		Persona:         "neutral",
	}); err != nil {
		t.Fatalf("state.Write: %v", err)
	}

	// TUI path: empty persona — must be resolved from state.
	sel := model.Selection{
		Agents:     []model.AgentID{model.AgentClaudeCode},
		Components: []model.ComponentID{model.ComponentPersona},
		Persona:    "", // empty — the bug scenario
	}

	result, err := RunSyncWithSelection(home, sel)
	if err != nil {
		t.Fatalf("RunSyncWithSelection() error = %v", err)
	}

	if got, want := result.Selection.Persona, model.PersonaNeutral; got != want {
		t.Errorf("result.Selection.Persona = %q, want %q (should be resolved from state.json)", got, want)
	}
}

// TestRunSyncWithSelection_PersonaResolvesFromStateCustom verifies that a
// "custom" persona persisted in state is restored on the TUI sync path.
func TestRunSyncWithSelection_PersonaResolvesFromStateCustom(t *testing.T) {
	home := t.TempDir()
	setSyncTestHome(t, home)

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := state.Write(home, state.InstallState{
		InstalledAgents: []string{"claude-code"},
		Persona:         "custom",
	}); err != nil {
		t.Fatalf("state.Write: %v", err)
	}

	sel := model.Selection{
		Agents:     []model.AgentID{model.AgentClaudeCode},
		Components: []model.ComponentID{model.ComponentPersona},
		Persona:    "",
	}

	result, err := RunSyncWithSelection(home, sel)
	if err != nil {
		t.Fatalf("RunSyncWithSelection() error = %v", err)
	}

	if got, want := result.Selection.Persona, model.PersonaCustom; got != want {
		t.Errorf("result.Selection.Persona = %q, want %q (should be resolved from state.json)", got, want)
	}
}

// TestRunSyncWithSelection_PersonaFallsBackToNeutralWhenStateHasNone verifies
// missing state persona resolves to neutral/default-safe behavior.
func TestRunSyncWithSelection_PersonaFallsBackToNeutralWhenStateHasNone(t *testing.T) {
	home := t.TempDir()
	setSyncTestHome(t, home)

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Raw legacy state with no Persona field — old install before persona
	// persistence. The omitted field is intentionally not an invalid value.
	if err := os.MkdirAll(filepath.Dir(state.Path(home)), 0o755); err != nil {
		t.Fatalf("MkdirAll state: %v", err)
	}
	if err := os.WriteFile(state.Path(home), []byte(`{"installed_agents":["claude-code"]}`), 0o644); err != nil {
		t.Fatalf("WriteFile state: %v", err)
	}

	sel := model.Selection{
		Agents:     []model.AgentID{model.AgentClaudeCode},
		Components: []model.ComponentID{model.ComponentPersona},
		Persona:    "",
	}

	result, err := RunSyncWithSelection(home, sel)
	if err != nil {
		t.Fatalf("RunSyncWithSelection() error = %v", err)
	}

	if got, want := result.Selection.Persona, model.PersonaNeutral; got != want {
		t.Errorf("result.Selection.Persona = %q, want %q (safe fallback for missing state persona)", got, want)
	}
}

// TestRunSyncWithSelection_ExplicitEmptyPersistedPersonaFailsClosed verifies
// that an explicit empty persona is not treated as the omitted legacy field.
func TestRunSyncWithSelection_ExplicitEmptyPersistedPersonaFailsClosed(t *testing.T) {
	home := t.TempDir()
	setSyncTestHome(t, home)

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	originalPersona := "<!-- gentle-ai:persona -->\nexisting valid persona\n<!-- /gentle-ai:persona -->\n"
	personaPath := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := os.WriteFile(personaPath, []byte(originalPersona), 0o644); err != nil {
		t.Fatalf("WriteFile persona: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(state.Path(home)), 0o755); err != nil {
		t.Fatalf("MkdirAll state: %v", err)
	}
	originalState := []byte(`{"schema_version":1,"installed_agents":["claude-code"],"persona":""}`)
	if err := os.WriteFile(state.Path(home), originalState, 0o644); err != nil {
		t.Fatalf("WriteFile state: %v", err)
	}

	result, err := RunSyncWithSelection(home, model.Selection{
		Agents:     []model.AgentID{model.AgentClaudeCode},
		Components: []model.ComponentID{model.ComponentPersona},
	})
	if err == nil {
		t.Fatal("RunSyncWithSelection() error = nil, want explicit empty persisted persona error")
	}
	if !strings.Contains(err.Error(), "explicitly empty persona") {
		t.Fatalf("RunSyncWithSelection() error = %q, want explicit empty persona context", err)
	}
	if result.Selection.Persona != "" {
		t.Fatalf("result.Selection.Persona = %q, want unchanged empty persona", result.Selection.Persona)
	}

	gotPersona, readErr := os.ReadFile(personaPath)
	if readErr != nil {
		t.Fatalf("ReadFile persona: %v", readErr)
	}
	if string(gotPersona) != originalPersona {
		t.Fatalf("persona config changed after rejected sync: got %q, want %q", gotPersona, originalPersona)
	}
	gotState, readErr := os.ReadFile(state.Path(home))
	if readErr != nil {
		t.Fatalf("ReadFile state: %v", readErr)
	}
	if !bytes.Equal(gotState, originalState) {
		t.Fatalf("state changed after rejected sync: got %q, want %q", gotState, originalState)
	}
}

// TestRunSyncWithSelection_ExplicitPersonaWinsOverState verifies that when the
// caller provides a non-empty persona (e.g. the user just picked one in the
// ModelConfig TUI step), that explicit choice is preserved even if state says
// something different.
func TestRunSyncWithSelection_ExplicitPersonaWinsOverState(t *testing.T) {
	home := t.TempDir()
	setSyncTestHome(t, home)

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// State says "gentleman" but the caller explicitly chose "neutral".
	if err := state.Write(home, state.InstallState{
		InstalledAgents: []string{"claude-code"},
		Persona:         "gentleman",
	}); err != nil {
		t.Fatalf("state.Write: %v", err)
	}

	sel := model.Selection{
		Agents:     []model.AgentID{model.AgentClaudeCode},
		Components: []model.ComponentID{model.ComponentPersona},
		Persona:    model.PersonaNeutral, // explicit — must not be overridden by state
	}

	result, err := RunSyncWithSelection(home, sel)
	if err != nil {
		t.Fatalf("RunSyncWithSelection() error = %v", err)
	}

	if got, want := result.Selection.Persona, model.PersonaNeutral; got != want {
		t.Errorf("result.Selection.Persona = %q, want %q (explicit selection must win over state)", got, want)
	}
}

// TestRunSyncWithSelection_UnknownPersistedPersonaFailsClosed verifies that an
// unsupported persisted persona is rejected before sync can rewrite persona
// assets.
func TestRunSyncWithSelection_UnknownPersistedPersonaFailsClosed(t *testing.T) {
	home := t.TempDir()
	setSyncTestHome(t, home)

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	originalPersona := "<!-- gentle-ai:persona -->\nexisting valid persona\n<!-- /gentle-ai:persona -->\n"
	personaPath := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := os.WriteFile(personaPath, []byte(originalPersona), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(state.Path(home)), 0o755); err != nil {
		t.Fatalf("MkdirAll state: %v", err)
	}
	if err := os.WriteFile(state.Path(home), []byte(`{"installed_agents":["claude-code"],"persona":"Gentleman"}`), 0o644); err != nil {
		t.Fatalf("WriteFile state: %v", err)
	}

	sel := model.Selection{
		Agents:     []model.AgentID{model.AgentClaudeCode},
		Components: []model.ComponentID{model.ComponentPersona},
		Persona:    "", // empty — resolution from state must happen
	}

	result, err := RunSyncWithSelection(home, sel)
	if err == nil {
		t.Fatal("RunSyncWithSelection() error = nil, want unsupported persisted persona error")
	}
	if result.Selection.Persona != "" {
		t.Fatalf("result.Selection.Persona = %q, want unchanged empty persona", result.Selection.Persona)
	}
	gotPersona, readErr := os.ReadFile(personaPath)
	if readErr != nil {
		t.Fatalf("ReadFile persona: %v", readErr)
	}
	if string(gotPersona) != originalPersona {
		t.Fatalf("persona config changed after rejected sync: got %q, want %q", gotPersona, originalPersona)
	}
}

func TestRunSyncWithSelection_WhitespaceOnlyPersistedPersonaFailsClosed(t *testing.T) {
	home := t.TempDir()
	setSyncTestHome(t, home)

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	originalPersona := "<!-- gentle-ai:persona -->\nexisting valid persona\n<!-- /gentle-ai:persona -->\n"
	personaPath := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := os.WriteFile(personaPath, []byte(originalPersona), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(state.Path(home)), 0o755); err != nil {
		t.Fatalf("MkdirAll state: %v", err)
	}
	if err := os.WriteFile(state.Path(home), []byte(`{"installed_agents":["claude-code"],"persona":" \t "}`), 0o644); err != nil {
		t.Fatalf("WriteFile state: %v", err)
	}

	result, err := RunSyncWithSelection(home, model.Selection{
		Agents:     []model.AgentID{model.AgentClaudeCode},
		Components: []model.ComponentID{model.ComponentPersona},
	})
	if err == nil {
		t.Fatal("RunSyncWithSelection() error = nil, want whitespace-only persisted persona error")
	}
	if !strings.Contains(err.Error(), "whitespace-only persona") {
		t.Fatalf("RunSyncWithSelection() error = %q, want whitespace-only persona context", err)
	}
	if result.Selection.Persona != "" {
		t.Fatalf("result.Selection.Persona = %q, want unchanged empty persona", result.Selection.Persona)
	}
	gotPersona, readErr := os.ReadFile(personaPath)
	if readErr != nil {
		t.Fatalf("ReadFile persona: %v", readErr)
	}
	if string(gotPersona) != originalPersona {
		t.Fatalf("persona config changed after rejected sync: got %q, want %q", gotPersona, originalPersona)
	}
}

func TestRunSyncWithSelection_UnreadablePersistedStateFailsClosed(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, home string)
	}{
		{
			name: "malformed JSON",
			setup: func(t *testing.T, home string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(state.Path(home)), 0o755); err != nil {
					t.Fatalf("MkdirAll state: %v", err)
				}
				if err := os.WriteFile(state.Path(home), []byte("{malformed"), 0o644); err != nil {
					t.Fatalf("WriteFile state: %v", err)
				}
			},
		},
		{
			name: "state path is unreadable",
			setup: func(t *testing.T, home string) {
				t.Helper()
				if err := os.MkdirAll(state.Path(home), 0o755); err != nil {
					t.Fatalf("Mkdir state path: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			setSyncTestHome(t, home)

			if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			originalPersona := "<!-- gentle-ai:persona -->\nexisting valid persona\n<!-- /gentle-ai:persona -->\n"
			personaPath := filepath.Join(home, ".claude", "CLAUDE.md")
			if err := os.WriteFile(personaPath, []byte(originalPersona), 0o644); err != nil {
				t.Fatalf("WriteFile persona: %v", err)
			}
			tt.setup(t, home)

			result, err := RunSyncWithSelection(home, model.Selection{
				Agents:     []model.AgentID{model.AgentClaudeCode},
				Components: []model.ComponentID{model.ComponentPersona},
			})
			if err == nil {
				t.Fatal("RunSyncWithSelection() error = nil, want persisted state read error")
			}
			if !strings.Contains(err.Error(), "state migration") {
				t.Fatalf("RunSyncWithSelection() error = %q, want state read context", err)
			}
			if result.Selection.Persona != "" {
				t.Fatalf("result.Selection.Persona = %q, want unchanged empty persona", result.Selection.Persona)
			}
			gotPersona, readErr := os.ReadFile(personaPath)
			if readErr != nil {
				t.Fatalf("ReadFile persona: %v", readErr)
			}
			if string(gotPersona) != originalPersona {
				t.Fatalf("persona config changed after rejected sync: got %q, want %q", gotPersona, originalPersona)
			}
		})
	}
}

func TestRunSync_UnsupportedPersistedPersonaFailsClosed(t *testing.T) {
	home := t.TempDir()
	setSyncTestHome(t, home)

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	originalPersona := "<!-- gentle-ai:persona -->\nexisting valid persona\n<!-- /gentle-ai:persona -->\n"
	personaPath := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := os.WriteFile(personaPath, []byte(originalPersona), 0o644); err != nil {
		t.Fatalf("WriteFile persona: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(state.Path(home)), 0o755); err != nil {
		t.Fatalf("MkdirAll state: %v", err)
	}
	if err := os.WriteFile(state.Path(home), []byte(`{"installed_agents":["claude-code"],"persona":"unknown"}`), 0o644); err != nil {
		t.Fatalf("WriteFile state: %v", err)
	}

	result, err := RunSync([]string{"--agents", "claude-code"})
	if err == nil {
		t.Fatal("RunSync() error = nil, want unsupported persisted persona error")
	}
	if !strings.Contains(err.Error(), `unsupported persona "unknown"`) {
		t.Fatalf("RunSync() error = %q, want unsupported persona", err)
	}
	if result.Selection.Persona != "" {
		t.Fatalf("result.Selection.Persona = %q, want unchanged empty persona", result.Selection.Persona)
	}
	gotPersona, readErr := os.ReadFile(personaPath)
	if readErr != nil {
		t.Fatalf("ReadFile persona: %v", readErr)
	}
	if string(gotPersona) != originalPersona {
		t.Fatalf("persona config changed after rejected sync: got %q, want %q", gotPersona, originalPersona)
	}
	if result.Selection.Persona != "" {
		t.Fatalf("Selection.Persona = %q, want unchanged empty persona", result.Selection.Persona)
	}
}

// ─── Changed file path reporting ────────────────────────────────────────────

// TestRenderSyncReportIncludesChangedFilePaths verifies that RenderSyncReport
// lists individual file paths when ChangedFiles is populated.

// TestRenderSyncReportNoOpOmitsChangedFilePaths verifies that RenderSyncReport
// does not list individual file path bullets in the no-op case.

// ─── Deduplication ──────────────────────────────────────────────────────────

func TestDedupPathsNilOnEmpty(t *testing.T) {
	got := dedupPaths(nil)
	if got != nil {
		t.Errorf("dedupPaths(nil) = %v, want nil", got)
	}
	got = dedupPaths([]string{})
	if got != nil {
		t.Errorf("dedupPaths([]) = %v, want nil", got)
	}
}

// ─── Dry-run persona resolution ───────────────────────────────────────────────

// TestRunSyncDryRunResolvesPersonaFromState verifies that --dry-run mode
// resolves the persona from state.json instead of leaving it empty.
// This is a regression test: the dry-run branch returns early and never calls
// RunSyncWithSelection, so without an explicit resolvePersonaFromState call the
// persona is never populated.

// TestRunSyncDryRunFallsBackToNeutralWhenStateLacksPersona verifies that
// --dry-run mode falls back to neutral/default-safe behavior when state has no
// recorded persona.

// ─── WU-3 RED: RunSync restores CodexCarrilModelAssignments ──────────────────

// setupCodexSyncHome creates a temp home with a state.json containing the codex
// agent and the provided carril model map, returning the home directory.
func setupCodexSyncHome(t *testing.T, carrilModels map[string]string, effortAssignments map[string]string) string {
	return setupCodexSyncHomeWithPhaseModels(t, carrilModels, effortAssignments, nil)
}

func setupCodexSyncHomeWithPhaseModels(t *testing.T, carrilModels map[string]string, effortAssignments map[string]string, phaseModels map[string]string) string {
	t.Helper()
	t.Cleanup(codex.SetRuntimeVersionCommandForTest("codex-cli 0.144.0", nil))
	home := t.TempDir()
	s := state.InstallState{
		InstalledAgents:             []string{"codex"},
		CodexModelAssignments:       effortAssignments,
		CodexCarrilModelAssignments: carrilModels,
		CodexPhaseModelAssignments:  phaseModels,
	}
	if err := state.Write(home, s); err != nil {
		t.Fatalf("state.Write() error = %v", err)
	}
	return home
}

func TestRunSync_RestoresCodexCarrilAssignments(t *testing.T) {
	restoreHome := osUserHomeDir
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }

	tests := []struct {
		name      string
		persisted map[string]string
		want      map[string]string
		runs      int
	}{
		{name: "migrates exact legacy defaults repeatedly", persisted: map[string]string{"sdd-strong": "gpt-5.5", "sdd-mid": "gpt-5.5", "sdd-cheap": "gpt-5.4-mini"}, want: model.DefaultCarrilModels(), runs: 2},
		{name: "preserves custom tuple", persisted: map[string]string{"sdd-strong": "gpt-5.5", "sdd-mid": "gpt-5.4", "sdd-cheap": "gpt-5.4-mini"}, want: map[string]string{"sdd-strong": "gpt-5.5", "sdd-mid": "gpt-5.4", "sdd-cheap": "gpt-5.4-mini"}, runs: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(codex.SetRuntimeVersionCommandForTest("codex-cli 0.144.0", nil))
			home := t.TempDir()
			if err := state.Write(home, state.InstallState{InstalledAgents: []string{"codex"}, CodexCarrilModelAssignments: tt.persisted}); err != nil {
				t.Fatalf("state.Write: %v", err)
			}
			osUserHomeDir = func() (string, error) { return home, nil }

			var firstProfiles map[string][]byte
			for run := 0; run < tt.runs; run++ {
				result, err := RunSync([]string{"--agents", "codex"})
				if err != nil {
					t.Fatalf("RunSync() run %d error = %v", run+1, err)
				}
				if !reflect.DeepEqual(result.Selection.CodexCarrilModelAssignments, tt.want) {
					t.Fatalf("run %d carril assignments = %#v, want %#v", run+1, result.Selection.CodexCarrilModelAssignments, tt.want)
				}
				profiles := make(map[string][]byte, 3)
				for _, name := range []string{"sdd-strong.config.toml", "sdd-mid.config.toml", "sdd-cheap.config.toml"} {
					body, err := os.ReadFile(filepath.Join(home, ".codex", name))
					if err != nil {
						t.Fatalf("run %d ReadFile(%s): %v", run+1, name, err)
					}
					profiles[name] = body
				}
				if run == 0 {
					if result.NoOp || result.FilesChanged == 0 {
						t.Fatalf("first sync metadata = NoOp %v, FilesChanged %d; want managed changes", result.NoOp, result.FilesChanged)
					}
					firstProfiles = profiles
				} else {
					for name, first := range firstProfiles {
						if !bytes.Equal(profiles[name], first) {
							t.Fatalf("%s changed between repeated syncs", name)
						}
					}
					if !result.NoOp || result.FilesChanged != 0 || len(result.ChangedFiles) != 0 {
						t.Fatalf("second sync metadata = NoOp %v, FilesChanged %d, ChangedFiles %#v; want no changes", result.NoOp, result.FilesChanged, result.ChangedFiles)
					}
				}
			}

			content, err := os.ReadFile(filepath.Join(home, ".codex", "sdd-strong.config.toml"))
			if err != nil {
				t.Fatalf("ReadFile(sdd-strong.config.toml): %v", err)
			}
			if !strings.Contains(string(content), tt.want["sdd-strong"]) {
				t.Fatalf("sdd-strong.config.toml missing %q; got:\n%s", tt.want["sdd-strong"], content)
			}
		})
	}
}

// TestRunSync_RestoresCodexEffortAssignments verifies that RunSync reads
// CodexModelAssignments (phase→effort) from state.json and writes them to
// profile files.
func TestRunSync_RestoresCodexEffortAssignments(t *testing.T) {
	efforts := map[string]string{
		"sdd-propose": "xhigh", "sdd-design": "xhigh", "sdd-verify": "xhigh",
		"jd-judge-a": "xhigh", "jd-judge-b": "xhigh", "default": "xhigh",
		"sdd-apply": "high", "jd-fix-agent": "high",
		"sdd-explore": "low", "sdd-spec": "low", "sdd-tasks": "low",
		"sdd-archive": "low", "sdd-onboard": "low",
	}
	home := setupCodexSyncHome(t, nil, efforts)

	restoreHome := osUserHomeDir
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }

	_, err := RunSync([]string{"--agents", "codex"})
	if err != nil {
		t.Fatalf("RunSync() error = %v", err)
	}

	// sdd-strong profile should have xhigh.
	strongProfile := filepath.Join(home, ".codex", "sdd-strong.config.toml")
	content, readErr := os.ReadFile(strongProfile)
	if readErr != nil {
		t.Fatalf("ReadFile(%q) error = %v", strongProfile, readErr)
	}
	if !strings.Contains(string(content), "xhigh") {
		t.Errorf("sdd-strong.config.toml: expected xhigh effort; got:\n%s", content)
	}
}

// TestRunSync_RestoresCodexPhaseModelAssignments verifies that plain
// `gentle-ai sync` preserves Custom per-phase Codex model assignments from
// state.json and renders the per-phase model table into AGENTS.md.
func TestRunSync_RestoresCodexPhaseModelAssignments(t *testing.T) {
	efforts := map[string]string{
		"sdd-propose": "xhigh", "sdd-design": "xhigh", "sdd-verify": "xhigh",
		"jd-judge-a": "xhigh", "jd-judge-b": "xhigh", "default": "xhigh",
		"sdd-apply": "high", "jd-fix-agent": "high",
		"sdd-explore": "low", "sdd-spec": "low", "sdd-tasks": "low",
		"sdd-archive": "low", "sdd-onboard": "low",
	}
	phaseModels := map[string]string{
		"default":     "gpt-5.4-mini",
		"sdd-propose": "gpt-5.5",
		"sdd-apply":   "o3",
	}
	home := setupCodexSyncHomeWithPhaseModels(t, nil, efforts, phaseModels)

	restoreHome := osUserHomeDir
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }

	_, err := RunSync([]string{"--agents", "codex"})
	if err != nil {
		t.Fatalf("RunSync() error = %v", err)
	}

	agentsMD := filepath.Join(home, ".codex", "AGENTS.md")
	content, readErr := os.ReadFile(agentsMD)
	if readErr != nil {
		t.Fatalf("ReadFile(%q) error = %v", agentsMD, readErr)
	}
	text := string(content)
	if !strings.Contains(text, "| Phase | Model |") {
		t.Fatalf("AGENTS.md missing per-phase Model table header; got:\n%s", text)
	}
	if !strings.Contains(text, "| `sdd-propose` | `gpt-5.5` | `xhigh` |") {
		t.Fatalf("AGENTS.md missing custom sdd-propose model row; got:\n%s", text)
	}
	if !strings.Contains(text, "| `sdd-apply` | `o3` | `high` |") {
		t.Fatalf("AGENTS.md missing custom sdd-apply model row; got:\n%s", text)
	}
	if strings.Contains(text, "| `sdd-strong` |") {
		t.Fatalf("AGENTS.md rendered carril table instead of Custom per-phase table; got:\n%s", text)
	}
}

// ─── Organic routing guidance is refreshed for every configured agent ──────
//
// Sync must reach the same unconditional guarantee install does: a persisted
// selection without the optional SDD component still routes work (issue #1794).

// runSyncInjectionSteps executes every staged sync apply step and returns the
// paths the runtime reported as actually changed.
func runSyncInjectionSteps(t *testing.T, home string, selection model.Selection) []string {
	t.Helper()

	rt, err := newSyncRuntime(home, selection)
	if err != nil {
		t.Fatalf("newSyncRuntime() error = %v", err)
	}
	for _, step := range rt.stagePlan().Apply {
		if err := step.Run(); err != nil {
			t.Fatalf("Run(%s) error = %v", step.ID(), err)
		}
	}
	return rt.changedFiles
}

// runSyncComponentSteps executes only the component steps of a sync plan.
func runSyncComponentSteps(t *testing.T, home string, selection model.Selection) {
	t.Helper()

	rt, err := newSyncRuntime(home, selection)
	if err != nil {
		t.Fatalf("newSyncRuntime() error = %v", err)
	}
	for _, step := range rt.stagePlan().Apply {
		if _, isComponent := step.(componentSyncStep); !isComponent {
			continue
		}
		if err := step.Run(); err != nil {
			t.Fatalf("Run(%s) error = %v", step.ID(), err)
		}
	}
}

func TestSyncDeliversRoutingGuidanceWithoutSDDComponent(t *testing.T) {
	home := t.TempDir()

	runSyncInjectionSteps(t, home, model.Selection{
		Agents:     []model.AgentID{model.AgentClaudeCode},
		Components: []model.ComponentID{model.ComponentPersona},
		Persona:    model.PersonaGentleman,
	})

	prompt := readTextFile(t, systemPromptFileFor(t, home, model.AgentClaudeCode))
	if !strings.Contains(prompt, routingOpenMarker) || !strings.Contains(prompt, routingCloseMarker) {
		t.Fatalf("sync without the SDD component left the agent unrouted:\n%s", prompt)
	}
}

func TestSyncRoutingGuidanceIsIndependentOfSDDSelection(t *testing.T) {
	const sddMarker = "<!-- gentle-ai:sdd-orchestrator -->"

	withoutSDD := t.TempDir()
	runSyncInjectionSteps(t, withoutSDD, model.Selection{
		Agents: []model.AgentID{model.AgentClaudeCode},
	})

	withSDD := t.TempDir()
	runSyncInjectionSteps(t, withSDD, model.Selection{
		Agents:     []model.AgentID{model.AgentClaudeCode},
		Components: []model.ComponentID{model.ComponentSDD},
		SDDMode:    model.SDDModeSingle,
	})

	plain := readTextFile(t, systemPromptFileFor(t, withoutSDD, model.AgentClaudeCode))
	sdd := readTextFile(t, systemPromptFileFor(t, withSDD, model.AgentClaudeCode))

	if !strings.Contains(plain, routingOpenMarker) || !strings.Contains(sdd, routingOpenMarker) {
		t.Fatalf("routing guidance is not independent of the SDD selection\nwithout sdd:\n%s\nwith sdd:\n%s", plain, sdd)
	}
	if strings.Contains(plain, sddMarker) {
		t.Fatalf("sync without the SDD component gained SDD orchestration assets:\n%s", plain)
	}
	if !strings.Contains(sdd, sddMarker) {
		t.Fatalf("sync with the SDD component lost SDD orchestration assets:\n%s", sdd)
	}
}

// TestSyncRoutingGuidanceSurvivesOpenCodeSDDInjection replays only the SDD
// component step on an already-guided home. Scheduling guidance last would hide
// the hazard on a full plan while still destroying guidance in this sequence.

func TestSyncStripsLegacyTriggerRulesSection(t *testing.T) {
	home := t.TempDir()

	promptPath := systemPromptFileFor(t, home, model.AgentClaudeCode)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(promptPath), err)
	}
	seeded := filemerge.InjectMarkdownSection("# My own notes\n", "trigger-rules", "Retired WorkRun ceremony\n")
	if err := os.WriteFile(promptPath, []byte(seeded), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", promptPath, err)
	}

	runSyncInjectionSteps(t, home, model.Selection{Agents: []model.AgentID{model.AgentClaudeCode}})

	prompt := readTextFile(t, promptPath)
	if strings.Contains(prompt, legacyTriggerRulesOpenMarker) {
		t.Fatalf("legacy trigger-rules section survived the sync:\n%s", prompt)
	}
	if !strings.Contains(prompt, "# My own notes") {
		t.Fatalf("stripping the legacy section destroyed unmanaged user content:\n%s", prompt)
	}
}

// ─── Routing guidance is part of sync's rollback contract ──────────────────
//
// The routing guidance step runs for every synced agent regardless of the
// persisted components, so its target has to be snapshotted even when no
// component contributes the same file.

func TestSyncBackupTargetsIncludeRoutingGuidancePathsWithoutAnyComponent(t *testing.T) {
	home := t.TempDir()
	agent := model.AgentClaudeCode
	selection := model.Selection{Agents: []model.AgentID{agent}}

	targets, err := syncBackupTargets(home, "", selection, resolveAdapters(selection.Agents))
	if err != nil {
		t.Fatalf("syncBackupTargets() error = %v", err)
	}

	routing, err := agentguidance.RoutingPaths(home, agent)
	if err != nil {
		t.Fatalf("RoutingPaths(%q) error = %v", agent, err)
	}
	if len(routing) == 0 {
		t.Fatalf("RoutingPaths(%q) returned no path; the test proves nothing", agent)
	}
	for _, path := range routing {
		if !containsPath(targets, path) {
			t.Fatalf("syncBackupTargets missing routing guidance path %q\ntargets = %v", path, targets)
		}
	}
}
