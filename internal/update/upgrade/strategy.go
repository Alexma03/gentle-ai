package upgrade

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/gentleman-programming/gentle-ai/v2/internal/cli"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/engram"
	"github.com/gentleman-programming/gentle-ai/v2/internal/system"
	"github.com/gentleman-programming/gentle-ai/v2/internal/update"
)

// engramDownloadFn installs Engram v2 main for the normal channel.
// Package-level var for testability — swapped in tests to avoid real network calls.
var engramDownloadFn = func(profile system.PlatformProfile) (string, error) {
	return engram.DownloadLatestBinary(profile, false)
}

// engramBetaInstallFn installs engram from HEAD via `go install @main` (beta channel).
// It delegates to engram.DownloadLatestBinary(profile, true), which is the single
// canonical beta path shared with the install-time flow. Package-level var for
// testability — swapped in tests to avoid real go install/network calls.
var engramBetaInstallFn = func(profile system.PlatformProfile) (string, error) {
	return engram.DownloadLatestBinary(profile, true)
}

// execCommand is a package-level var declared in executor.go (same package).

var detectOS = func() string { return runtime.GOOS }

type strategyOutcome struct {
	exitRequested   bool
	observedVersion string
}

// runStrategy executes the upgrade for a single tool using the appropriate strategy
// for the given platform profile.
//
// Strategy routing:
//   - brew profile → brewUpgrade (regardless of tool's declared method)
//   - go-install method + apt/pacman/other → goInstallUpgrade
//   - binary method + linux/darwin → binaryUpgrade
//   - binary method + windows → manualFallback (gentle-ai explains the signed-distribution hold)
//   - unknown method → manualFallback with explicit message
func runStrategy(ctx context.Context, r update.UpdateResult, profile system.PlatformProfile, preflightDestination ...string) (bool, error) {
	if r.Tool.Name == "engram" && strings.EqualFold(r.Tool.Owner, "Gentleman-Programming") && r.Tool.Repo == "engram" && r.Tool.InstallMethod == update.InstallBinary {
		return false, engramBinaryUpgrade(profile)
	}
	ownership := update.HomebrewNone
	if profile.PackageManager == "brew" {
		var err error
		ownership, err = homebrewOwnershipDetector(r.Tool.Name)
		if err != nil {
			return false, fmt.Errorf("detect Homebrew ownership for %s: %w", r.Tool.Name, err)
		}
	}
	if isBetaGentleAIUpgrade(r) && profile.OS != "windows" && ownership == update.HomebrewNone {
		return false, goInstallMainUpgrade(r.Tool)
	}

	method := effectiveMethod(r.Tool, profile)
	if ownership != update.HomebrewNone {
		method = update.InstallBrew
	}

	switch method {
	case update.InstallBrew:
		return false, brewUpgrade(ctx, r, ownership)
	case update.InstallGoInstall:
		return false, goInstallUpgrade(ctx, r, profile, firstString(preflightDestination))
	case update.InstallBinary:
		return false, binaryUpgrade(ctx, r, profile)
	default:
		return false, &ManualFallbackError{
			Hint: fmt.Sprintf("upgrade %q: unsupported install method %q — please update manually. See: https://github.com/Gentleman-Programming/%s",
				r.Tool.Name, method, r.Tool.Repo),
		}
	}
}

func runStrategyWithOutcome(ctx context.Context, r update.UpdateResult, profile system.PlatformProfile, preflightDestination ...string) (strategyOutcome, error) {
	exitRequested, err := runStrategy(ctx, r, profile, preflightDestination...)
	return strategyOutcome{exitRequested: exitRequested}, err
}

// brewUpgrade runs `brew update` (non-fatal) then `brew upgrade <toolName>`.
//
// brew update refreshes the local formula cache so that Homebrew is aware of
// new versions published since the user last ran it. If update fails (e.g. no
// network), the upgrade is still attempted using the existing cache — a stale
// cache is better than no upgrade at all.
func brewUpgrade(ctx context.Context, r update.UpdateResult, ownership update.HomebrewOwnership) error {
	toolName := r.Tool.Name
	flag := "--" + string(ownership)
	// Ensure the Gentleman-Programming homebrew tap is present before upgrading.
	// Non-fatal: brew tap is a no-op when already present; if it fails for any other
	// reason, the subsequent brew upgrade will surface the real error. See issue #455:
	// without this, a lost tap (untap, machine swap, brew cleanup) makes upgrades fail
	// with "No available formula" for engram/gentle-ai.
	tapCmd := execCommand("brew", "tap", "Gentleman-Programming/homebrew-tap")
	tapCmd.Stdin = nil
	_ = tapCmd.Run()

	// Trust only the Gentleman Programming artifact being upgraded. Homebrew 6 can
	// require explicit trust for non-official taps; this is intentionally scoped to
	// our formula/cask, not the whole tap or third-party taps. Older Homebrew versions
	// may not support `brew trust`, so this is non-fatal and the upgrade output
	// below remains the source of truth.
	trustCmd := execCommand("brew", "trust", flag, gentlemanProgrammingTapRef(toolName))
	trustCmd.Stdin = nil
	_ = trustCmd.Run()

	// Update Homebrew formula cache before upgrading.
	// Non-fatal: if update fails (e.g. no network), attempt upgrade with existing cache.
	updateCmd := execCommand("brew", "update")
	updateCmd.Stdin = nil
	_ = updateCmd.Run() // ignore error intentionally

	upgradeCmd := execCommand("brew", "upgrade", flag, toolName)
	upgradeCmd.Stdin = nil
	if out, err := upgradeCmd.CombinedOutput(); err != nil {
		return formatBrewUpgradeError(toolName, ownership, err, string(out))
	}
	if ownership == update.HomebrewCask {
		return verifyLegacyCaskTarget(r)
	}
	return nil
}

var brewVersionRegexp = regexp.MustCompile(`\d+\.\d+(?:\.\d+)?(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?(?:\+[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?`)

func verifyLegacyCaskTarget(r update.UpdateResult) error {
	migration := legacyCaskMigration(r.Tool.Name)
	if len(r.Tool.DetectCmd) == 0 {
		return fmt.Errorf("cannot verify Homebrew cask %s after upgrade; %s", r.Tool.Name, migration)
	}
	cmd := execCommand(r.Tool.DetectCmd[0], r.Tool.DetectCmd[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("verify Homebrew cask %s after upgrade: %w; %s", r.Tool.Name, err, migration)
	}
	installed := brewVersionRegexp.FindString(string(out))
	target := brewVersionRegexp.FindString(r.LatestVersion)
	if installed == "" || target == "" || installed != target {
		return fmt.Errorf("Homebrew cask %s remains at %q after targeting %q; %s", r.Tool.Name, installed, target, migration)
	}
	return nil
}

func gentlemanProgrammingTapRef(toolName string) string {
	return "gentleman-programming/tap/" + strings.TrimSpace(toolName)
}

func homebrewTrustFlag(toolName string) string {
	if strings.TrimSpace(toolName) == "engram" {
		return "--cask"
	}
	return "--formula"
}

func legacyCaskMigration(toolName string) string {
	return fmt.Sprintf("migrate the legacy cask to the current formula:\n  brew uninstall --cask %s\n  brew install --formula %s", strings.TrimSpace(toolName), gentlemanProgrammingTapRef(toolName))
}

func formatBrewUpgradeError(toolName string, ownership update.HomebrewOwnership, err error, output string) error {
	message := fmt.Sprintf("brew upgrade --%s %s: %v (output: %s)", ownership, toolName, err, output)
	if advice := homebrewFailureAdvice(toolName, output, ownership); advice != "" {
		message += "\n\n" + advice
	}
	if ownership == update.HomebrewCask {
		message += "\n\n" + legacyCaskMigration(toolName)
	}
	return errors.New(message)
}

func homebrewFailureAdvice(toolName string, output string, detected ...update.HomebrewOwnership) string {
	lower := strings.ToLower(output)
	ref := gentlemanProgrammingTapRef(toolName)
	flag := homebrewTrustFlag(toolName)
	if len(detected) > 0 {
		flag = "--" + string(detected[0])
	}

	if strings.Contains(lower, "untrusted tap") || strings.Contains(lower, "tap trust is required") || strings.Contains(lower, "homebrew_require_tap_trust") {
		artifact := strings.TrimPrefix(flag, "--")
		if strings.Contains(lower, "--cask") || strings.Contains(lower, "load cask") {
			flag = "--cask"
			artifact = "cask"
		} else if strings.Contains(lower, "--formula") || strings.Contains(lower, "load formula") {
			flag = "--formula"
			artifact = "formula"
		}
		return fmt.Sprintf("Homebrew requires explicit trust for external taps. Trust only this Gentle AI %s, then retry:\n  brew trust %s %s\n  brew upgrade %s %s", artifact, flag, ref, flag, toolName)
	}

	if strings.Contains(lower, "bubblewrap is installed but cannot create a rootless sandbox") ||
		strings.Contains(lower, "rootless sandbox") ||
		strings.Contains(lower, "homebrew_no_sandbox_linux") {
		return "Homebrew on Linux could not create its Bubblewrap rootless sandbox. This requires an explicit admin/security decision: enabling unprivileged user namespaces lets Homebrew use its sandbox but changes host kernel/AppArmor policy. If acceptable, run:\n  sudo sysctl -w kernel.unprivileged_userns_clone=1\n  sudo sysctl -w user.max_user_namespaces=28633\n  sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0 || true\n\nFinal workaround if your distro policy forbids this sandbox:\n  HOMEBREW_NO_SANDBOX_LINUX=1 brew upgrade " + flag + " " + toolName
	}

	return ""
}

// goInstallUpgrade runs `go install <importPath>@v<version>`.
//
// Generic Go-managed tools retain the post-install warning because the new
// binary was genuinely written. Windows gentle-ai self-upgrades are different:
// they must prove that Go owns the active executable before writing, or skip to
// a manual recovery instead of creating a second PATH-visible binary.
func goInstallUpgrade(ctx context.Context, r update.UpdateResult, profile system.PlatformProfile, preflightDestination string) error {
	tool := r.Tool
	latestVersion := r.LatestVersion
	if tool.GoImportPath == "" {
		return fmt.Errorf("upgrade %q: GoImportPath is empty — cannot run go install", tool.Name)
	}

	// GOBIN/GOPATH are static Go configuration that `go install` does not
	// change, so they are read up front; the PATH lookup happens afterwards so
	// a first-time install resolves correctly.
	destDir := preflightDestination
	var destErr error
	if destDir == "" {
		destDir, destErr = goInstallDestinationDir()
		if err := preflightWindowsGentleAIGoInstallWithDestination(r, profile, destDir, destErr); err != nil {
			return err
		}
	}

	// Pin release installs to their exact version. Beta checks advertise
	// main@<sha>, which Go installs by resolving the main branch, not by
	// prepending a v to that display value.
	target := fmt.Sprintf("%s@v%s", tool.GoImportPath, latestVersion)
	betaGentleAI := isBetaGentleAIUpgrade(r)
	if betaGentleAI {
		target = tool.GoImportPath + "@main"
	}
	cmd := execCommand("go", "install", target)
	cmd.Stdin = nil
	if betaGentleAI {
		cmd.Env = goProxyBypassEnv(cmd.Env, gentleAIModulePath(tool))
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go install %s: %w (output: %s)", target, err, string(out))
	}

	warnGoInstallDestination(tool.Name, detectOS(), destDir, destErr)
	return nil
}

func preflightWindowsGentleAIGoInstall(r update.UpdateResult, profile system.PlatformProfile) (string, error) {
	if profile.OS != "windows" || r.Tool.Name != "gentle-ai" {
		return "", nil
	}
	destDir, destErr := goInstallDestinationDir()
	if err := preflightWindowsGentleAIGoInstallWithDestination(r, profile, destDir, destErr); err != nil {
		return "", err
	}
	return destDir, nil
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func preflightWindowsGentleAIGoInstallWithDestination(r update.UpdateResult, profile system.PlatformProfile, destDir string, destErr error) error {
	if profile.OS != "windows" || r.Tool.Name != "gentle-ai" {
		return nil
	}
	if destErr != nil {
		return &ManualFallbackError{Hint: gentleAIWindowsGoInstallProvenanceHint(r, "", "")}
	}

	destination := absoluteBinaryPath(filepath.Join(destDir, goInstallBinaryName(r.Tool.Name, profile.OS)))
	active, err := lookPathFn(r.Tool.Name)
	if err != nil {
		return &ManualFallbackError{Hint: gentleAIWindowsGoInstallProvenanceHint(r, destination, "")}
	}
	active = absoluteBinaryPath(active)
	if !sameBinaryPathForOS(destination, active, profile.OS) {
		return &ManualFallbackError{Hint: gentleAIWindowsGoInstallProvenanceHint(r, destination, active)}
	}
	return nil
}

func gentleAIWindowsGoInstallProvenanceHint(r update.UpdateResult, destination, active string) string {
	details := "could not determine the Go installation destination"
	switch {
	case destination != "" && active == "":
		details = fmt.Sprintf("could not resolve the active gentle-ai executable before Go would write to %s", destination)
	case destination != "" && active != "":
		details = fmt.Sprintf("resolves gentle-ai to %s, but Go would write to %s", active, destination)
	}

	hint := fmt.Sprintf("Windows self-upgrade %s. No files were changed. ", details)
	if active != "" {
		hint += fmt.Sprintf("Keep %s as the active installation, or intentionally migrate to %s with:\n  ", active, destination)
	} else {
		hint += "Confirm the active installation, then intentionally migrate with:\n  "
	}
	hint += update.GentleAISourceInstallCommand(r.LatestVersion)
	if destination != "" {
		hint += fmt.Sprintf("\nAfter a successful migration, ensure only %s resolves for gentle-ai on PATH.", destination)
	}
	return hint
}

func isBetaGentleAIUpgrade(r update.UpdateResult) bool {
	return r.Tool.Name == "gentle-ai" &&
		strings.EqualFold(r.Tool.Owner, "Gentleman-Programming") &&
		r.Tool.Repo == "gentle-ai" &&
		strings.HasPrefix(strings.TrimSpace(r.LatestVersion), "main@")
}

// goInstallMainUpgrade installs gentle-ai from HEAD on the beta channel. It runs
// the same `go install` mechanism as goInstallUpgrade and therefore carries the
// same risk of writing somewhere the shell does not resolve, so it performs the
// same non-fatal destination verification.
func goInstallMainUpgrade(tool update.ToolInfo) error {
	module := gentleAIModulePath(tool)

	destDir, destErr := goInstallDestinationDir()

	target := module + "/cmd/gentle-ai@main"
	cmd := execCommand("go", "install", target)
	cmd.Stdin = nil
	cmd.Env = goProxyBypassEnv(cmd.Env, module)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go install %s: %w (output: %s)", target, err, strings.TrimSpace(string(out)))
	}

	warnGoInstallDestination(tool.Name, detectOS(), destDir, destErr)
	return nil
}

func gentleAIModulePath(tool update.ToolInfo) string {
	repository := strings.ToLower(fmt.Sprintf("github.com/%s/%s", strings.TrimSpace(tool.Owner), strings.TrimSpace(tool.Repo)))
	if repository == "github.com//" {
		repository = "github.com/gentleman-programming/gentle-ai"
	}
	// Go derives the module path from the repository plus the major-version
	// suffix: for major 2 and above the module path must end in /vN or the
	// toolchain refuses every resolution of that repository, including the
	// branch pseudo-versions this beta path installs.
	return repository + "/v2"
}

func goProxyBypassEnv(base []string, module string) []string {
	if base == nil {
		base = os.Environ()
	}
	env := append([]string{}, base...)
	for _, key := range []string{"GONOSUMDB", "GOPRIVATE", "GONOPROXY"} {
		env = setEnvValue(env, key, prependGoPattern(getEnvValue(env, key), module))
	}
	return env
}

func getEnvValue(env []string, key string) string {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			return strings.TrimPrefix(env[i], prefix)
		}
	}
	return ""
}

func setEnvValue(env []string, key, value string) []string {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func prependGoPattern(existing, pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return existing
	}
	parts := strings.Split(existing, ",")
	for _, part := range parts {
		if strings.TrimSpace(part) == pattern {
			return existing
		}
	}
	if strings.TrimSpace(existing) == "" {
		return pattern
	}
	return pattern + "," + existing
}

// binaryUpgrade handles binary-release upgrades via GitHub Releases asset download.
//
// engram has its own cross-platform binary downloader (DownloadLatestBinary) that
// works on all platforms including Windows. Other Windows binary upgrades return
// ManualFallbackError so the executor surfaces them as UpgradeSkipped.
func binaryUpgrade(ctx context.Context, r update.UpdateResult, profile system.PlatformProfile) error {
	if profile.OS == "windows" && r.Tool.Name == "gentle-ai" {
		return &ManualFallbackError{Hint: gentleAIWindowsSourceInstallHint(r)}
	}

	// engram: always use its dedicated binary downloader regardless of platform
	// (except brew, which is handled by effectiveMethod before we get here).
	if r.Tool.Name == "engram" {
		return engramBinaryUpgrade(profile)
	}

	if profile.OS == "windows" {
		// Windows binary auto-upgrade is not supported for generic tools yet.
		// Return a ManualFallbackError so the executor surfaces this as UpgradeSkipped
		// with an actionable hint — NOT as UpgradeFailed.
		hint := r.UpdateHint
		if hint == "" {
			hint = fmt.Sprintf("Download manually from https://github.com/Gentleman-Programming/%s/releases", r.Tool.Repo)
		}
		return &ManualFallbackError{
			Hint: fmt.Sprintf("upgrade %q on Windows requires manual update: %s", r.Tool.Name, hint),
		}
	}

	// For Linux/macOS binary installs: delegate to the download package.
	return downloadAndReplace(ctx, r, profile)
}

func gentleAIWindowsSourceInstallHint(r update.UpdateResult) string {
	return update.WindowsDistributionHoldMessage + " " +
		"No binary or remote script was downloaded or executed. Install/update from source with Go 1.25.10+:\n  " +
		update.GentleAISourceInstallCommand(r.LatestVersion)
}

// engramBinaryUpgrade installs Engram v2 main through the existing channel
// seams. Both normal and beta paths now select the same upstream source.
func engramBinaryUpgrade(profile system.PlatformProfile) error {
	// Resolve the install channel from the environment. Unknown values fall back
	// to stable (ResolveInstallChannel returns an error for truly unrecognized
	// values; we treat those as stable and emit a warning so users are not silently
	// misrouted).
	channel, err := cli.ResolveInstallChannel("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: unrecognized GENTLE_AI_CHANNEL value (%v); defaulting to stable\n", err)
		channel = cli.ChannelStable
	}

	var binaryPath string
	if channel.IsBeta() {
		// Beta channel: install engram from HEAD via engramBetaInstallFn, which
		// delegates to engram.DownloadLatestBinary(profile, true). This is the
		// single canonical beta path shared with the install-time flow in
		// internal/cli/run.go (installBetaEngramFromMain). The previous inline
		// `go install` block is removed — all beta logic lives in download.go.
		binaryPath, err = engramBetaInstallFn(profile)
		if err != nil {
			return fmt.Errorf("install engram from main (beta): %w", err)
		}
	} else {
		// Normal channel: install the same v2 main source as beta.
		binaryPath, err = engramDownloadFn(profile)
		if err != nil {
			return fmt.Errorf("install Engram v2 from main: %w", err)
		}
	}

	// Add install dir to PATH. On Windows this also persists via PowerShell (user registry).
	binDir := filepath.Dir(binaryPath)
	if err := system.AddToUserPath(binDir); err != nil {
		// Non-fatal: the binary was downloaded or installed successfully. Warn and continue.
		fmt.Fprintf(os.Stderr, "WARNING: could not add %s to PATH: %v\n", binDir, err)
	}
	return nil
}

// downloadAndReplace downloads the release asset and atomically replaces the binary.
// Implemented in download.go.
func downloadAndReplace(ctx context.Context, r update.UpdateResult, profile system.PlatformProfile) error {
	return Download(ctx, r, profile)
}
