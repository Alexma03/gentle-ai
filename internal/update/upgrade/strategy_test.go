package upgrade

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/system"
	"github.com/gentleman-programming/gentle-ai/v2/internal/update"
)

func TestMain(m *testing.M) {
	if err := os.Unsetenv("GENTLE_AI_CHANNEL"); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

// --- TestRunStrategy_BrewUpgrade ---

func TestRunStrategy_BrewUpgrade(t *testing.T) {
	mockHomebrewOwnership(t, update.HomebrewFormula)
	origExecCommand := execCommand
	t.Cleanup(func() { execCommand = origExecCommand })

	var gotName string
	var gotArgs []string
	execCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = args
		return mockCmd("echo", "Upgraded engram")
	}

	r := update.UpdateResult{
		Tool: update.ToolInfo{
			Name:          "engram",
			InstallMethod: update.InstallBrew,
		},
		LatestVersion: "0.4.0",
	}
	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew"}

	_, err := runStrategy(context.Background(), r, profile)
	if err != nil {
		t.Fatalf("runStrategy brew: unexpected error: %v", err)
	}

	if gotName != "brew" {
		t.Errorf("exec name = %q, want %q", gotName, "brew")
	}
	if len(gotArgs) < 3 || gotArgs[0] != "upgrade" || gotArgs[1] != "--formula" || gotArgs[2] != "engram" {
		t.Errorf("exec args = %v, want [upgrade --formula engram]", gotArgs)
	}
}

// --- TestRunStrategy_GoInstallUpgrade ---

func TestRunStrategy_GoInstallUpgrade(t *testing.T) {
	origExecCommand := execCommand
	t.Cleanup(func() { execCommand = origExecCommand })

	var gotName string
	var gotArgs []string
	execCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = args
		return mockCmd("echo", "go install ok")
	}

	r := update.UpdateResult{
		Tool: update.ToolInfo{
			Name:          "engram",
			InstallMethod: update.InstallGoInstall,
			GoImportPath:  "github.com/Gentleman-Programming/engram/cmd/engram",
		},
		LatestVersion: "0.4.0",
	}
	profile := system.PlatformProfile{OS: "linux", PackageManager: "apt"}

	_, err := runStrategy(context.Background(), r, profile)
	if err != nil {
		t.Fatalf("runStrategy go-install: unexpected error: %v", err)
	}

	if gotName != "go" {
		t.Errorf("exec name = %q, want %q", gotName, "go")
	}
	// Expected: go install github.com/Gentleman-Programming/engram/cmd/engram@v0.4.0
	wantArg0, wantArg1 := "install", "github.com/Gentleman-Programming/engram/cmd/engram@v0.4.0"
	if len(gotArgs) < 2 || gotArgs[0] != wantArg0 || gotArgs[1] != wantArg1 {
		t.Errorf("exec args = %v, want [%s %s]", gotArgs, wantArg0, wantArg1)
	}
}

func TestRunStrategy_BetaGentleAISelfUpgradeUsesGoInstallMain(t *testing.T) {
	origExecCommand := execCommand
	t.Cleanup(func() { execCommand = origExecCommand })

	var gotName string
	var gotArgs []string
	var gotCmd *exec.Cmd
	execCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = args
		gotCmd = mockCmd("true")
		return gotCmd
	}

	r := update.UpdateResult{
		Tool: update.ToolInfo{
			Name:          "gentle-ai",
			Owner:         "Gentleman-Programming",
			Repo:          "gentle-ai",
			InstallMethod: update.InstallBinary,
		},
		LatestVersion: "main@972997650b51",
		Status:        update.UpdateAvailable,
	}
	profile := system.PlatformProfile{OS: "linux", PackageManager: "apt", Supported: true}

	_, err := runStrategy(context.Background(), r, profile)
	if err != nil {
		t.Fatalf("runStrategy beta gentle-ai: unexpected error: %v", err)
	}

	if gotName != "go" {
		t.Fatalf("exec name = %q, want %q", gotName, "go")
	}
	wantArgs := []string{"install", "github.com/gentleman-programming/gentle-ai/v2/cmd/gentle-ai@main"}
	if len(gotArgs) != len(wantArgs) || gotArgs[0] != wantArgs[0] || gotArgs[1] != wantArgs[1] {
		t.Fatalf("exec args = %v, want %v", gotArgs, wantArgs)
	}
	for _, want := range []string{
		"GONOSUMDB=github.com/gentleman-programming/gentle-ai/v2",
		"GOPRIVATE=github.com/gentleman-programming/gentle-ai/v2",
		"GONOPROXY=github.com/gentleman-programming/gentle-ai/v2",
	} {
		if !envContains(gotCmd.Env, want) {
			t.Fatalf("go install env missing %q in %v", want, gotCmd.Env)
		}
	}
}

func envContains(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}

func TestGoProxyBypassEnvPreservesExistingPatterns(t *testing.T) {
	module := "github.com/gentleman-programming/gentle-ai/v2"
	env := goProxyBypassEnv([]string{
		"PATH=/usr/bin",
		"GONOSUMDB=example.com/private",
		"GOPRIVATE=github.com/acme/*",
		"GONOPROXY=github.com/gentleman-programming/gentle-ai/v2",
	}, module)

	for _, want := range []string{
		"PATH=/usr/bin",
		"GONOSUMDB=github.com/gentleman-programming/gentle-ai/v2,example.com/private",
		"GOPRIVATE=github.com/gentleman-programming/gentle-ai/v2,github.com/acme/*",
		"GONOPROXY=github.com/gentleman-programming/gentle-ai/v2",
	} {
		if !envContains(env, want) {
			t.Fatalf("env missing %q in %v", want, env)
		}
	}
}

// --- TestRunStrategy_GoInstallMissingImportPath ---

func TestRunStrategy_GoInstallMissingImportPath(t *testing.T) {
	r := update.UpdateResult{
		Tool: update.ToolInfo{
			Name:          "engram",
			InstallMethod: update.InstallGoInstall,
			GoImportPath:  "", // missing
		},
		LatestVersion: "0.4.0",
	}
	profile := system.PlatformProfile{OS: "linux", PackageManager: "apt"}

	_, err := runStrategy(context.Background(), r, profile)
	if err == nil {
		t.Errorf("expected error when GoImportPath is empty, got nil")
	}
}

// --- TestRunStrategy_UnsupportedMethodManualFallback ---

func TestRunStrategy_UnsupportedMethodManualFallback(t *testing.T) {
	r := update.UpdateResult{
		Tool: update.ToolInfo{
			Name:          "some-tool",
			InstallMethod: update.InstallMethod("unsupported-method"),
		},
		LatestVersion: "1.0.0",
	}
	profile := system.PlatformProfile{OS: "linux", PackageManager: "apt"}

	_, err := runStrategy(context.Background(), r, profile)
	// Unsupported method → manual fallback error.
	if err == nil {
		t.Errorf("expected error for unsupported install method, got nil")
	}
}

// --- TestRunStrategy_BrewUpgradeFailure ---

func TestRunStrategy_BrewUpgradeFailure(t *testing.T) {
	origExecCommand := execCommand
	t.Cleanup(func() { execCommand = origExecCommand })

	execCommand = func(name string, args ...string) *exec.Cmd {
		return mockCmd("false") // always fails
	}

	r := update.UpdateResult{
		Tool: update.ToolInfo{
			Name:          "engram",
			InstallMethod: update.InstallBrew,
		},
		LatestVersion: "0.4.0",
	}
	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew"}

	_, err := runStrategy(context.Background(), r, profile)
	if err == nil {
		t.Errorf("expected error when brew upgrade fails, got nil")
	}
}

// --- TestRunStrategy_GoInstallFailure ---

func TestRunStrategy_GoInstallFailure(t *testing.T) {
	origExecCommand := execCommand
	t.Cleanup(func() { execCommand = origExecCommand })

	execCommand = func(name string, args ...string) *exec.Cmd {
		return mockCmd("false")
	}

	r := update.UpdateResult{
		Tool: update.ToolInfo{
			Name:          "engram",
			InstallMethod: update.InstallGoInstall,
			GoImportPath:  "github.com/Gentleman-Programming/engram/cmd/engram",
		},
		LatestVersion: "0.4.0",
	}
	profile := system.PlatformProfile{OS: "linux", PackageManager: "apt"}

	_, err := runStrategy(context.Background(), r, profile)
	if err == nil {
		t.Errorf("expected error when go install fails, got nil")
	}
}

// TestEffectiveMethodGentleAIOnWindowsUsesFailClosedBinaryPolicy verifies that
// Windows never routes gentle-ai through a remote installer, and that when no
// usable `go install` target is declared it falls back to the binary strategy —
// which on Windows is an explicit refusal naming a runnable source-install
// command, not a download.
func TestEffectiveMethodGentleAIOnWindowsUsesFailClosedBinaryPolicy(t *testing.T) {
	tests := []struct {
		name string
		tool update.ToolInfo
	}{
		{
			name: "binary remains policy boundary",
			tool: update.ToolInfo{Name: "gentle-ai", InstallMethod: update.InstallBinary},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			profile := system.PlatformProfile{OS: "windows", PackageManager: "winget", GoAvailable: true}
			method := effectiveMethod(tc.tool, profile)
			if method != update.InstallBinary {
				t.Errorf("effectiveMethod(%q) = %q, want %q", tc.tool.Name, method, update.InstallBinary)
			}
		})
	}

	// Renamed from "Go availability still requires an explicit source install",
	// which encoded the previous policy: Windows refused to self-upgrade even
	// with Go on PATH. That policy has been changed deliberately. No signed
	// Windows binary is published, so there is no asset to download and verify
	// with minisign; a pinned `go install <importPath>@vX.Y.Z` — still checked
	// against the Go checksum database, since goInstallUpgrade does not touch
	// cmd.Env — is the only automatic upgrade path Windows has.
	t.Run("Go availability upgrades through a pinned go install", func(t *testing.T) {
		tool := update.ToolInfo{Name: "gentle-ai", InstallMethod: update.InstallBinary, GoImportPath: "github.com/Gentleman-Programming/gentle-ai/v2/cmd/gentle-ai"}
		profile := system.PlatformProfile{OS: "windows", PackageManager: "winget", GoAvailable: true}
		method := effectiveMethod(tool, profile)
		if method != update.InstallGoInstall {
			t.Errorf("effectiveMethod(%q) = %q, want %q", tool.Name, method, update.InstallGoInstall)
		}
	})
}

// --- TestEffectiveMethod_NonGentleAIToolsOnWindowsUseBinary ---

// TestEffectiveMethod_NonGentleAIToolsOnWindowsUseBinary verifies that tools
// OTHER than gentle-ai on Windows still use their declared install method
// (binary, script, etc.).
func TestEffectiveMethod_NonGentleAIToolsOnWindowsUseBinary(t *testing.T) {
	tests := []struct {
		name string
		tool update.ToolInfo
		want update.InstallMethod
	}{
		{
			name: "engram uses binary",
			tool: update.ToolInfo{Name: "engram", InstallMethod: update.InstallBinary},
			want: update.InstallBinary,
		},
		{
			name: "unknown tool uses binary",
			tool: update.ToolInfo{Name: "other", InstallMethod: update.InstallBinary},
			want: update.InstallBinary,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			profile := system.PlatformProfile{OS: "windows", PackageManager: "winget", GoAvailable: true}
			method := effectiveMethod(tc.tool, profile)
			if method != tc.want {
				t.Errorf("effectiveMethod(%q) = %q, want %q", tc.tool.Name, method, tc.want)
			}
		})
	}
}

// --- TestEffectiveMethod ---

func TestEffectiveMethod(t *testing.T) {
	origHomebrewPackageInstalled := homebrewPackageInstalled
	t.Cleanup(func() { homebrewPackageInstalled = origHomebrewPackageInstalled })

	tests := []struct {
		name          string
		tool          update.ToolInfo
		profile       system.PlatformProfile
		brewInstalled bool
		want          update.InstallMethod
	}{
		{
			name:          "brew-owned package overrides go-install",
			tool:          update.ToolInfo{Name: "engram", InstallMethod: update.InstallGoInstall},
			profile:       system.PlatformProfile{PackageManager: "brew"},
			brewInstalled: true,
			want:          update.InstallBrew,
		},
		{
			name:    "brew profile without package ownership respects declared method",
			tool:    update.ToolInfo{Name: "gentle-ai", InstallMethod: update.InstallBinary},
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			want:    update.InstallBinary,
		},
		{
			name:    "brew profile without package ownership can use go-install fallback",
			tool:    update.ToolInfo{Name: "mytool", InstallMethod: update.InstallBinary, GoImportPath: "github.com/example/mytool/cmd/mytool"},
			profile: system.PlatformProfile{PackageManager: "brew", GoAvailable: true},
			want:    update.InstallGoInstall,
		},
		{
			name:    "apt profile respects declared method (go-install)",
			tool:    update.ToolInfo{Name: "engram", InstallMethod: update.InstallGoInstall},
			profile: system.PlatformProfile{PackageManager: "apt"},
			want:    update.InstallGoInstall,
		},
		// Auto-detect order: brew-owned package → go-install → binary (issue #246).
		{
			name:          "auto-detect: brew-owned package wins regardless of GoImportPath",
			tool:          update.ToolInfo{Name: "mytool", InstallMethod: update.InstallBinary, GoImportPath: "github.com/example/mytool/cmd/mytool"},
			profile:       system.PlatformProfile{PackageManager: "brew", GoAvailable: true},
			brewInstalled: true,
			want:          update.InstallBrew,
		},
		{
			name:    "auto-detect: brew missing + go available + GoImportPath set → go-install",
			tool:    update.ToolInfo{Name: "mytool", InstallMethod: update.InstallBinary, GoImportPath: "github.com/example/mytool/cmd/mytool"},
			profile: system.PlatformProfile{PackageManager: "apt", GoAvailable: true},
			want:    update.InstallGoInstall,
		},
		{
			name:    "auto-detect: brew missing + go missing + GoImportPath set → binary fallback",
			tool:    update.ToolInfo{Name: "mytool", InstallMethod: update.InstallBinary, GoImportPath: "github.com/example/mytool/cmd/mytool"},
			profile: system.PlatformProfile{PackageManager: "apt", GoAvailable: false},
			want:    update.InstallBinary,
		},
		{
			name:    "auto-detect: go available but GoImportPath empty → binary (no upgrade)",
			tool:    update.ToolInfo{Name: "mytool", InstallMethod: update.InstallBinary, GoImportPath: ""},
			profile: system.PlatformProfile{PackageManager: "apt", GoAvailable: true},
			want:    update.InstallBinary,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			homebrewPackageInstalled = func(toolName string) bool {
				return toolName == tc.tool.Name && tc.brewInstalled
			}

			got := effectiveMethod(tc.tool, tc.profile)
			if got != tc.want {
				t.Errorf("effectiveMethod = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHomebrewPackageInstalledWithRequiresActiveBrewPath(t *testing.T) {
	brewPrefix := filepath.Join(t.TempDir(), "opt", "gentle-ai")
	brewBin := filepath.Join(brewPrefix, "bin", "gentle-ai")
	nonBrewBin := filepath.Join(t.TempDir(), "gentle-ai")

	run := func(name string, args ...string) *exec.Cmd {
		if name != "brew" {
			return mockCmd("false")
		}
		if len(args) >= 3 && args[0] == "list" && args[1] == "--formula" && args[2] == "gentle-ai" {
			return mockCmd("true")
		}
		if len(args) == 2 && args[0] == "--prefix" && args[1] == "gentle-ai" {
			return mockCmd("echo", brewPrefix)
		}
		return mockCmd("false")
	}

	if !homebrewPackageInstalledWith(run, func(string) (string, error) { return brewBin, nil }, "gentle-ai") {
		t.Fatal("expected brew-owned active path to be treated as Homebrew installed")
	}
	if homebrewPackageInstalledWith(run, func(string) (string, error) { return nonBrewBin, nil }, "gentle-ai") {
		t.Fatal("expected shadowing non-brew active path to avoid Homebrew")
	}
	if homebrewPackageInstalledWith(func(string, ...string) *exec.Cmd { return mockCmd("false") }, func(string) (string, error) { return brewBin, nil }, "gentle-ai") {
		t.Fatal("expected brew list failure to avoid Homebrew")
	}
	if homebrewPackageInstalledWith(func(string, ...string) *exec.Cmd { return mockCmd("true") }, func(string) (string, error) { return "", errors.New("not found") }, "gentle-ai") {
		t.Fatal("expected active path lookup failure to avoid Homebrew")
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// --- TestBrewUpgrade_RunsUpdateBeforeUpgrade ---

// TestBrewUpgrade_RunsUpdateBeforeUpgrade verifies that brewUpgrade calls
// `brew update` BEFORE `brew upgrade <toolName>`, and that the order is correct.
func TestBrewUpgrade_RunsUpdateBeforeUpgrade(t *testing.T) {
	origExecCommand := execCommand
	t.Cleanup(func() { execCommand = origExecCommand })

	var callOrder []string
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "brew" && len(args) > 0 {
			callOrder = append(callOrder, args[0]) // "update" or "upgrade"
		}
		return mockCmd("echo", "ok")
	}

	err := brewUpgrade(context.Background(), update.UpdateResult{Tool: update.ToolInfo{Name: "gentle-ai"}}, update.HomebrewFormula)
	if err != nil {
		t.Fatalf("brewUpgrade: unexpected error: %v", err)
	}

	// Must have called brew tap, scoped trust, brew update AND brew upgrade — in that order.
	if len(callOrder) < 4 {
		t.Fatalf("expected 4 brew calls (tap, trust, update, upgrade), got %d: %v", len(callOrder), callOrder)
	}
	if callOrder[1] != "trust" {
		t.Errorf("second brew call = %q, want %q", callOrder[1], "trust")
	}
	if callOrder[2] != "update" {
		t.Errorf("third brew call = %q, want %q", callOrder[2], "update")
	}
	if callOrder[3] != "upgrade" {
		t.Errorf("fourth brew call = %q, want %q", callOrder[3], "upgrade")
	}
}

// --- TestBrewUpgrade_UpdateFailureIsNonFatal ---

// TestBrewUpgrade_UpdateFailureIsNonFatal verifies that when `brew update` fails
// but `brew upgrade` succeeds, the overall result is success (non-fatal update failure).
func TestBrewUpgrade_UpdateFailureIsNonFatal(t *testing.T) {
	origExecCommand := execCommand
	t.Cleanup(func() { execCommand = origExecCommand })

	var callArgs []string
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "brew" && len(args) > 0 {
			callArgs = append(callArgs, args[0])
			if args[0] == "update" {
				// brew update fails (e.g. no network).
				return mockCmd("false")
			}
		}
		// brew upgrade succeeds.
		return mockCmd("echo", "Upgraded gentle-ai")
	}

	err := brewUpgrade(context.Background(), update.UpdateResult{Tool: update.ToolInfo{Name: "gentle-ai"}}, update.HomebrewFormula)
	// brew update failed but brew upgrade succeeded → overall success.
	if err != nil {
		t.Errorf("expected success when brew update fails but brew upgrade succeeds, got: %v", err)
	}

	// Brew trust, update, and upgrade must have been called (after the tap).
	if len(callArgs) < 4 {
		t.Fatalf("expected 4 brew calls, got %d: %v", len(callArgs), callArgs)
	}
	if callArgs[1] != "trust" {
		t.Errorf("second brew call = %q, want %q", callArgs[1], "trust")
	}
	if callArgs[2] != "update" {
		t.Errorf("third brew call = %q, want %q", callArgs[2], "update")
	}
	if callArgs[3] != "upgrade" {
		t.Errorf("fourth brew call = %q, want %q", callArgs[3], "upgrade")
	}
}

// --- TestBrewUpgrade_TapsBeforeUpdateAndUpgrade ---

// TestBrewUpgrade_TapsAndTrustsBeforeUpdateAndUpgrade verifies that brewUpgrade calls
// `brew tap Gentleman-Programming/homebrew-tap` and scoped artifact trust BEFORE
// `brew update` and `brew upgrade <toolName>`. This makes the upgrade idempotent
// when a user has lost the tap and works with Homebrew tap trust enforcement.
func TestBrewUpgrade_TapsAndTrustsBeforeUpdateAndUpgrade(t *testing.T) {
	origExecCommand := execCommand
	t.Cleanup(func() { execCommand = origExecCommand })

	type call struct {
		subcommand string
		args       []string
	}
	var calls []call
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "brew" && len(args) > 0 {
			c := call{subcommand: args[0], args: append([]string(nil), args[1:]...)}
			calls = append(calls, c)
		}
		if name == "engram" {
			return mockCmd("echo", "engram 1.2.3")
		}
		return mockCmd("echo", "ok")
	}

	if err := brewUpgrade(context.Background(), update.UpdateResult{Tool: update.ToolInfo{Name: "engram", DetectCmd: []string{"engram", "version"}}, LatestVersion: "1.2.3"}, update.HomebrewCask); err != nil {
		t.Fatalf("brewUpgrade: unexpected error: %v", err)
	}

	if len(calls) < 4 {
		t.Fatalf("expected 4 brew calls (tap, trust, update, upgrade), got %d: %+v", len(calls), calls)
	}
	if calls[0].subcommand != "tap" {
		t.Errorf("first brew call subcommand = %q, want %q", calls[0].subcommand, "tap")
	}
	if len(calls[0].args) != 1 || calls[0].args[0] != "Gentleman-Programming/homebrew-tap" {
		t.Errorf("first brew call args = %v, want [Gentleman-Programming/homebrew-tap]", calls[0].args)
	}
	if calls[1].subcommand != "trust" {
		t.Errorf("second brew call = %q, want %q", calls[1].subcommand, "trust")
	}
	if len(calls[1].args) != 2 || calls[1].args[0] != "--cask" || calls[1].args[1] != "gentleman-programming/tap/engram" {
		t.Errorf("second brew call args = %v, want [--cask gentleman-programming/tap/engram]", calls[1].args)
	}
	if calls[2].subcommand != "update" {
		t.Errorf("third brew call = %q, want %q", calls[2].subcommand, "update")
	}
	if calls[3].subcommand != "upgrade" {
		t.Errorf("fourth brew call = %q, want %q", calls[3].subcommand, "upgrade")
	}
}

func TestBrewUpgrade_FormulaToolUsesFormulaTrust(t *testing.T) {
	origExecCommand := execCommand
	t.Cleanup(func() { execCommand = origExecCommand })

	var trustArgs []string
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "brew" && len(args) > 0 && args[0] == "trust" {
			trustArgs = append([]string(nil), args[1:]...)
		}
		return mockCmd("echo", "ok")
	}

	if err := brewUpgrade(context.Background(), update.UpdateResult{Tool: update.ToolInfo{Name: "gentle-ai"}}, update.HomebrewFormula); err != nil {
		t.Fatalf("brewUpgrade: unexpected error: %v", err)
	}

	if len(trustArgs) != 2 || trustArgs[0] != "--formula" || trustArgs[1] != "gentleman-programming/tap/gentle-ai" {
		t.Fatalf("brew trust args = %v, want [--formula gentleman-programming/tap/gentle-ai]", trustArgs)
	}
}

func TestHomebrewFailureAdviceTapTrust(t *testing.T) {
	output := `Error: Refusing to load formula gentleman-programming/tap/gentle-ai from untrusted tap.
Run brew trust --formula gentleman-programming/tap/gentle-ai to trust it.`
	advice := homebrewFailureAdvice("gentle-ai", output)
	for _, want := range []string{
		"brew trust --formula gentleman-programming/tap/gentle-ai",
		"brew upgrade --formula gentle-ai",
	} {
		if !strings.Contains(advice, want) {
			t.Fatalf("tap trust advice missing %q:\n%s", want, advice)
		}
	}
}

func TestHomebrewFailureAdviceCaskTapTrust(t *testing.T) {
	output := `Error: Refusing to load cask gentleman-programming/tap/engram from untrusted tap.
Run brew trust --cask gentleman-programming/tap/engram to trust it.`
	advice := homebrewFailureAdvice("engram", output)
	for _, want := range []string{
		"brew trust --cask gentleman-programming/tap/engram",
		"brew upgrade --cask engram",
	} {
		if !strings.Contains(advice, want) {
			t.Fatalf("cask tap trust advice missing %q:\n%s", want, advice)
		}
	}
	if strings.Contains(advice, "--formula") {
		t.Fatalf("cask tap trust advice must not suggest --formula:\n%s", advice)
	}
}

func TestHomebrewFailureAdviceBubblewrap(t *testing.T) {
	output := `Error: Bubblewrap is installed but cannot create a rootless sandbox.
Homebrew's Linux sandbox requires rootless Bubblewrap and unprivileged user namespaces.`
	advice := homebrewFailureAdvice("gentle-ai", output)
	if strings.Contains(strings.ToLower(advice), "preferred fix") {
		t.Fatalf("bubblewrap advice must not frame host policy changes as preferred defaults:\n%s", advice)
	}
	for _, want := range []string{
		"explicit admin/security decision",
		"sudo sysctl -w kernel.unprivileged_userns_clone=1",
		"sudo sysctl -w user.max_user_namespaces=28633",
		"sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0 || true",
		"HOMEBREW_NO_SANDBOX_LINUX=1 brew upgrade --formula gentle-ai",
	} {
		if !strings.Contains(advice, want) {
			t.Fatalf("bubblewrap advice missing %q:\n%s", want, advice)
		}
	}
}

// --- verify exec.Cmd.Run() failure is correctly wrapped ---
func TestRunStrategy_ExecErrorWrapped(t *testing.T) {
	origExecCommand := execCommand
	t.Cleanup(func() { execCommand = origExecCommand })

	execCommand = func(name string, args ...string) *exec.Cmd {
		return mockCmd("false")
	}

	r := update.UpdateResult{
		Tool: update.ToolInfo{
			Name:          "engram",
			InstallMethod: update.InstallBrew,
		},
		LatestVersion: "0.4.0",
	}
	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew"}

	_, err := runStrategy(context.Background(), r, profile)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Error should have a non-empty message.
	if err.Error() == "" {
		t.Errorf("error should have a message")
	}

	// Error should wrap an *exec.ExitError (from running "false").
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Logf("note: error is not directly an ExitError (may be wrapped): %v", err)
	}
}

func TestEngramUpgradeUsesDownloadNotGoInstall(t *testing.T) {
	origExecCommand := execCommand
	origEngramDownloadFn := engramDownloadFn
	t.Cleanup(func() {
		execCommand = origExecCommand
		engramDownloadFn = origEngramDownloadFn
	})

	execCalled := false
	execCommand = func(name string, args ...string) *exec.Cmd {
		execCalled = true
		return mockCmd("echo", "should not be called")
	}

	downloadCalled := false
	engramDownloadFn = func(profile system.PlatformProfile) (string, error) {
		downloadCalled = true
		return "/fake/path/engram.exe", nil
	}

	r := update.UpdateResult{
		Tool: update.ToolInfo{
			Name:          "engram",
			Owner:         "Gentleman-Programming",
			Repo:          "engram",
			InstallMethod: update.InstallBinary, // should be InstallBinary after fix
		},
		LatestVersion: "0.5.0",
	}
	profile := system.PlatformProfile{OS: "windows", PackageManager: "winget"}

	_, err := runStrategy(context.Background(), r, profile)
	if err != nil {
		t.Fatalf("runStrategy engram windows: unexpected error: %v", err)
	}

	// Must call binary download, NOT go install.
	if !downloadCalled {
		t.Errorf("expected engramDownloadFn to be called, but it was not")
	}
	if execCalled {
		t.Errorf("exec (go install) should NOT be called for engram on Windows — use binary download")
	}
}

// --- TestEngramUpgradeLinuxUsesDownload ---

// TestEngramUpgradeLinuxUsesDownload verifies that on Linux (non-brew),
// engram upgrade uses the binary download function, not go install.
func TestEngramUpgradeLinuxUsesDownload(t *testing.T) {
	origExecCommand := execCommand
	origEngramDownloadFn := engramDownloadFn
	t.Cleanup(func() {
		execCommand = origExecCommand
		engramDownloadFn = origEngramDownloadFn
	})

	execCalled := false
	execCommand = func(name string, args ...string) *exec.Cmd {
		execCalled = true
		return mockCmd("echo", "should not be called")
	}

	downloadCalled := false
	engramDownloadFn = func(profile system.PlatformProfile) (string, error) {
		downloadCalled = true
		return "/home/user/.local/bin/engram", nil
	}

	r := update.UpdateResult{
		Tool: update.ToolInfo{
			Name:          "engram",
			Owner:         "Gentleman-Programming",
			Repo:          "engram",
			InstallMethod: update.InstallBinary, // should be InstallBinary after fix
		},
		LatestVersion: "0.5.0",
	}
	profile := system.PlatformProfile{OS: "linux", PackageManager: "apt"}

	_, err := runStrategy(context.Background(), r, profile)
	if err != nil {
		t.Fatalf("runStrategy engram linux: unexpected error: %v", err)
	}

	if !downloadCalled {
		t.Errorf("expected engramDownloadFn to be called for engram on Linux, but it was not")
	}
	if execCalled {
		t.Errorf("exec (go install) should NOT be called for engram on Linux — use binary download")
	}
}

func TestEngramBinaryUpgrade_StableChannelCallsDownloadFn(t *testing.T) {
	tests := []struct {
		name   string
		envVal string
	}{
		{name: "channel unset", envVal: ""},
		{name: "channel explicit stable", envVal: "stable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GENTLE_AI_CHANNEL", tt.envVal)

			origDownloadFn := engramDownloadFn
			origExecCommand := execCommand
			t.Cleanup(func() {
				engramDownloadFn = origDownloadFn
				execCommand = origExecCommand
			})

			downloadCalled := false
			engramDownloadFn = func(profile system.PlatformProfile) (string, error) {
				downloadCalled = true
				return "/tmp/engram", nil
			}

			// go install must NOT be called for stable channel.
			execCommand = func(name string, args ...string) *exec.Cmd {
				t.Errorf("execCommand called unexpectedly for stable channel: %s %v", name, args)
				return mockCmd("echo", "unexpected")
			}

			profile := system.PlatformProfile{OS: "linux", PackageManager: "apt"}
			err := engramBinaryUpgrade(profile)
			if err != nil {
				t.Fatalf("engramBinaryUpgrade stable: unexpected error: %v", err)
			}
			if !downloadCalled {
				t.Error("expected engramDownloadFn to be called for stable channel, but it was not")
			}
		})
	}
}

// TestEngramBinaryUpgrade_BetaChannelUsesGoInstallMain verifies that when
// GENTLE_AI_CHANNEL=beta, engramBinaryUpgrade delegates to
// engramBetaInstallFn (the consolidated beta path, backed by
// engram.DownloadLatestBinary(profile, true) in production). The stable
// engramDownloadFn must NOT be called.
func TestEngramBinaryUpgrade_BetaChannelUsesGoInstallMain(t *testing.T) {
	t.Setenv("GENTLE_AI_CHANNEL", "beta")

	origDownloadFn := engramDownloadFn
	origBetaFn := engramBetaInstallFn
	t.Cleanup(func() {
		engramDownloadFn = origDownloadFn
		engramBetaInstallFn = origBetaFn
	})

	engramDownloadFn = func(profile system.PlatformProfile) (string, error) {
		t.Error("engramDownloadFn (stable path) must NOT be called for beta channel")
		return "", nil
	}

	var betaCalled bool
	engramBetaInstallFn = func(profile system.PlatformProfile) (string, error) {
		betaCalled = true
		return "/tmp/engram-beta", nil
	}

	profile := system.PlatformProfile{OS: "linux", PackageManager: "apt"}
	err := engramBinaryUpgrade(profile)
	if err != nil {
		t.Fatalf("engramBinaryUpgrade beta: unexpected error: %v", err)
	}
	if !betaCalled {
		t.Fatal("expected engramBetaInstallFn (beta path) to be called, but it was not")
	}
}
