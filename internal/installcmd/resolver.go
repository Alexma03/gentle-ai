package installcmd

import (
	"fmt"
	goversion "go/version"
	"os"
	"os/exec"
	"strings"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
	"github.com/gentleman-programming/gentle-ai/v2/internal/system"
)

// cmdLookPath, osStat, osGetenv, and cmdGoVersion are package-level vars for testability.
var cmdLookPath = exec.LookPath
var osStat = os.Stat
var osGetenv = os.Getenv
var cmdGoVersion = func() ([]byte, error) {
	return exec.Command("go", "version").Output()
}

// CommandSequence represents an ordered list of commands to run in sequence.
// Each inner slice is a single command with its arguments (e.g., ["brew", "install", "engram"]).
// Multi-step installs (e.g., tap + install) are expressed as multiple entries.
type CommandSequence = [][]string

type Resolver interface {
	ResolveAgentInstall(profile system.PlatformProfile, agent model.AgentID) (CommandSequence, error)
	ResolveComponentInstall(profile system.PlatformProfile, component model.ComponentID) (CommandSequence, error)
	ResolveDependencyInstall(profile system.PlatformProfile, dependency string) (CommandSequence, error)
}

type profileResolver struct{}

func NewResolver() Resolver {
	return profileResolver{}
}

func (profileResolver) ResolveAgentInstall(profile system.PlatformProfile, agent model.AgentID) (CommandSequence, error) {
	switch agent {
	case model.AgentClaudeCode:
		return resolveClaudeCodeInstall(profile), nil
	default:
		return nil, fmt.Errorf("install command is not supported for agent %q", agent)
	}
}

// resolveClaudeCodeInstall returns the npm install command sequence gentle-ai
// shows for Claude Code — display text only, never executed by gentle-ai
// (see agentInstallStep in internal/cli/run.go). On Linux with system npm,
// sudo is required. With nvm/fnm/volta, it is not. On Windows and macOS,
// sudo is never needed.
//
// --ignore-scripts blocks postinstall hooks, the primary supply-chain attack
// vector for npm packages. The version advises "latest" rather than a pin:
// a pin only guarded against a tampered "latest" tag when gentle-ai itself
// ran the command unattended. Now a human reads and runs it, and a stale
// hardcoded version goes wrong the moment a newer release ships (the same
// drift this shape fixed for Codex's GPT-5.6 update advice).
func resolveClaudeCodeInstall(profile system.PlatformProfile) CommandSequence {
	const pkg = "@anthropic-ai/claude-code@latest"
	if profile.OS == "linux" && !profile.NpmWritable {
		return CommandSequence{{"sudo", "npm", "install", "-g", "--ignore-scripts", pkg}}
	}
	return CommandSequence{{"npm", "install", "-g", "--ignore-scripts", pkg}}
}

// npmBasedAgents is the set of agents whose auto-install runs npm commands.
// When any of these agents is selected, npm (and therefore Node.js) must be
// present before the pipeline reaches the agent install step.
//
// AgentPi is included because InstallCommand always runs engramInitCommand(),
// which executes either `pnpm dlx` or `npm exec` (both require Node.js). The
// npm-presence check is a sound proxy for Node.js availability.
var npmBasedAgents = map[model.AgentID]struct{}{
	model.AgentClaudeCode: {},
	model.AgentCodex:      {},
	model.AgentPi:         {},
}

// ValidateAgentInstallPreflight validates agent-specific prerequisites that must
// exist before running installation commands.
func ValidateAgentInstallPreflight(profile system.PlatformProfile, agent model.AgentID) error {
	if _, ok := npmBasedAgents[agent]; ok {
		if err := validateNpmInstallPreflight(profile); err != nil {
			return err
		}
	}
	switch agent {
	case model.AgentPi:
		return validatePiInstallPreflight()
	default:
		return nil
	}
}

func validatePiInstallPreflight() error {
	if _, err := cmdLookPath("pi"); err != nil {
		return fmt.Errorf("Pi requires the `pi` executable in PATH before installing Gentle AI Pi packages")
	}

	return nil
}

// validateNpmInstallPreflight ensures npm (and therefore Node.js) is available
// before attempting any npm-based agent install. Called for all agents in
// npmBasedAgents so the user gets a clear, actionable error instead of a
// cryptic "exec: npm: executable file not found in PATH" mid-pipeline.
func validateNpmInstallPreflight(profile system.PlatformProfile) error {
	if _, err := cmdLookPath("npm"); err != nil {
		hint := system.InstallHintForDep("node", profile)
		return fmt.Errorf(
			"Node.js / npm is required but `npm` was not found in PATH.\n"+
				"Install Node.js (npm is included) and retry:\n"+
				"  %s",
			hint,
		)
	}
	return nil
}

func (profileResolver) ResolveComponentInstall(profile system.PlatformProfile, component model.ComponentID) (CommandSequence, error) {
	switch component {
	case model.ComponentEngram:
		return resolveEngramInstall(profile)
	default:
		return nil, fmt.Errorf("install command is not supported for component %q", component)
	}
}

func (profileResolver) ResolveDependencyInstall(profile system.PlatformProfile, dependency string) (CommandSequence, error) {
	if dependency == "" {
		return nil, fmt.Errorf("dependency name is required")
	}

	switch profile.PackageManager {
	case "brew":
		return CommandSequence{{"brew", "install", dependency}}, nil
	case "apt":
		return CommandSequence{{"sudo", "apt-get", "install", "-y", dependency}}, nil
	case "pacman":
		return CommandSequence{{"sudo", "pacman", "-S", "--noconfirm", dependency}}, nil
	case "dnf":
		return CommandSequence{{"sudo", "dnf", "install", "-y", dependency}}, nil
	case "winget":
		return CommandSequence{{"winget", "install", "--id", dependency, "-e", "--accept-source-agreements", "--accept-package-agreements"}}, nil
	default:
		return nil, fmt.Errorf(
			"unsupported package manager %q for os=%q distro=%q",
			profile.PackageManager,
			profile.OS,
			profile.LinuxDistro,
		)
	}
}

// validateGoForModuleInstall checks that Go >=1.25.10 is installed and GO111MODULE is not
// disabled before attempting `go install`. Returns an actionable error if any check fails.
// MUST NOT be called for brew-based installs (brew manages Go transitively).
func validateGoForModuleInstall(profile system.PlatformProfile) error {
	if _, err := cmdLookPath("go"); err != nil {
		return fmt.Errorf(
			"Go 1.25.10+ is required to install Engram but was not found in PATH.\n" +
				"Please install Go from https://go.dev/dl/ and restart your terminal.")
	}

	out, err := cmdGoVersion()
	if err != nil {
		return fmt.Errorf(
			"Go 1.25.10+ is required but could not verify the installed version.\n" +
				"Please ensure Go is properly installed: https://go.dev/dl/")
	}

	// Parse "go version go1.XX.Y platform/arch" and enforce the patch floor.
	parts := strings.Fields(string(out))
	if len(parts) < 3 || !goversion.IsValid(parts[2]) {
		return fmt.Errorf("Go 1.25.10+ is required but the installed version could not be parsed.\nPlease update Go: https://go.dev/dl/")
	}
	if goversion.Compare(parts[2], "go1.25.10") < 0 {
		return fmt.Errorf(
			"Go 1.25.10+ is required to install Engram, but found %s.\n"+
				"Please update Go: https://go.dev/dl/", parts[2])
	}

	if osGetenv("GO111MODULE") == "off" {
		fix := "export GO111MODULE=on  # then retry"
		if profile.OS == "windows" {
			fix = `$env:GO111MODULE = "on"  # PowerShell, then retry`
		}
		return fmt.Errorf("Go modules are disabled (GO111MODULE=off).\nRun: %s", fix)
	}

	return nil
}

// ValidateComponentInstallPreflight validates prerequisites for components
// that execute installation commands during the apply stage.
func ValidateComponentInstallPreflight(profile system.PlatformProfile, component model.ComponentID) error {
	if component == model.ComponentEngram {
		return validateGoForModuleInstall(profile)
	}
	return nil
}

// resolveEngramInstall retains the legacy stable command sequence for API
// compatibility. The active Engram component path installs v2 @main directly.
func resolveEngramInstall(profile system.PlatformProfile) (CommandSequence, error) {
	switch profile.PackageManager {
	case "brew":
		// macOS (or Linux with Homebrew): brew manages Go transitively — no preflight needed.
		return CommandSequence{
			{"brew", "tap", "Gentleman-Programming/homebrew-tap"},
			{"brew", "install", "engram"},
		}, nil
	default:
		return nil, fmt.Errorf(
			"engram on %q/%q uses direct binary download — use engram.DownloadLatestBinary() instead of CommandSequence",
			profile.OS, profile.PackageManager,
		)
	}
}
