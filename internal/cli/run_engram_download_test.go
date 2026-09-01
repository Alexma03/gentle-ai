package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/components/engram"
	"github.com/gentleman-programming/gentle-ai/v2/internal/system"
)

// TestRunInstallLinuxEngramUsesDownloadNotGoInstall verifies that after the fix,
// Linux engram installation does NOT use "go install" but instead calls
// DownloadLatestBinary (i.e. no "go install" in recorder.get()).
func TestRunInstallLinuxEngramUsesDownloadNotGoInstall(t *testing.T) {
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
	cmdLookPath = missingBinaryLookPath
	recorder := &commandRecorder{}
	runCommand = recorder.record

	// Override the engram download function to succeed without hitting GitHub.
	origDownloadFn := engramDownloadFn
	engramDownloadFn = func(profile system.PlatformProfile) (string, error) {
		// Simulate a successful binary download to a temp path.
		return "/tmp/fake-engram", nil
	}
	t.Cleanup(func() { engramDownloadFn = origDownloadFn })

	detection := linuxDetectionResult(system.LinuxDistroUbuntu, "apt")
	result, err := RunInstall(
		[]string{"--agent", "claude-code", "--component", "engram"},
		detection,
	)
	if err != nil {
		t.Fatalf("RunInstall() error = %v", err)
	}

	if !result.Verify.Ready {
		t.Fatalf("verification ready = false, report = %#v", result.Verify)
	}

	// Must NOT have called "go install" for engram.
	for _, cmd := range recorder.get() {
		if strings.Contains(cmd, "go install") && strings.Contains(cmd, "engram") {
			t.Fatalf("Linux engram install should NOT use go install, got command: %s", cmd)
		}
	}
}

// TestRunInstallEngramDownloadAddsBinDirToPath verifies that after downloading
// the engram binary, its directory is prepended to PATH so that subsequent
// commands (engram setup, resolveEngramCommand) can find it.
func TestRunInstallEngramDownloadAddsBinDirToPath(t *testing.T) {
	home := t.TempDir()
	restoreHome := osUserHomeDir
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	restorePath := os.Getenv("PATH")
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
		os.Setenv("PATH", restorePath)
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	cmdLookPath = missingBinaryLookPath
	recorder := &commandRecorder{}
	runCommand = recorder.record

	fakeBinDir := filepath.Join(home, "engram-bin")
	os.MkdirAll(fakeBinDir, 0o755)
	fakeBinaryPath := filepath.Join(fakeBinDir, "engram")

	origDownloadFn := engramDownloadFn
	engramDownloadFn = func(profile system.PlatformProfile) (string, error) {
		return fakeBinaryPath, nil
	}
	t.Cleanup(func() { engramDownloadFn = origDownloadFn })

	detection := linuxDetectionResult(system.LinuxDistroUbuntu, "apt")
	_, err := RunInstall(
		[]string{"--agent", "claude-code", "--component", "engram"},
		detection,
	)
	if err != nil {
		t.Fatalf("RunInstall() error = %v", err)
	}

	currentPath := os.Getenv("PATH")
	if !strings.Contains(currentPath, fakeBinDir) {
		t.Fatalf("PATH should contain engram bin dir %q after download, got PATH=%q", fakeBinDir, currentPath)
	}
}

func TestRunInstallFreshEngramDownloadUsesDownloadedBinaryForVersionProbeAndSetup(t *testing.T) {
	home := t.TempDir()
	fakeBinDir := filepath.Join(home, "engram-bin")
	fakeBinaryPath := filepath.Join(fakeBinDir, "engram")

	restoreHome := osUserHomeDir
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	restoreAddUserPath := addUserPath
	restoreVerifyVersionCommand := verifyEngramVersionCommand
	restoreProbeCommand := probeEngramProtocolFlagCommand
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
		addUserPath = restoreAddUserPath
		verifyEngramVersionCommand = restoreVerifyVersionCommand
		probeEngramProtocolFlagCommand = restoreProbeCommand
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	cmdLookPath = missingBinaryLookPath
	recorder := &commandRecorder{}
	runCommand = recorder.record
	addUserPath = func(dir string) error {
		if dir != fakeBinDir {
			t.Fatalf("addUserPath() dir = %q, want %q", dir, fakeBinDir)
		}
		return os.ErrPermission
	}
	var versionCommand string
	verifyEngramVersionCommand = func(command string) (string, error) {
		versionCommand = command
		return "engram 1.18.0", nil
	}
	var probeCommand string
	probeEngramProtocolFlagCommand = func(_ context.Context, command string) (string, error) {
		probeCommand = command
		return "Usage: engram setup <slug> [--protocol=slim|full]\n", nil
	}

	origDownloadFn := engramDownloadFn
	engramDownloadFn = func(profile system.PlatformProfile) (string, error) {
		return fakeBinaryPath, nil
	}
	t.Cleanup(func() { engramDownloadFn = origDownloadFn })

	detection := linuxDetectionResult(system.LinuxDistroUbuntu, "apt")
	_, err := RunInstall(
		[]string{"--agent", "claude-code", "--component", "engram"},
		detection,
	)
	if err != nil {
		t.Fatalf("RunInstall() error = %v", err)
	}

	commands := recorder.get()
	foundSetupWithDownloadedBinary := false
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, fakeBinaryPath+" setup ") {
			foundSetupWithDownloadedBinary = true
		}
		if strings.HasPrefix(cmd, "engram setup ") {
			t.Fatalf("engram setup should use downloaded binary %q, got commands: %v", fakeBinaryPath, commands)
		}
	}
	if !foundSetupWithDownloadedBinary {
		t.Fatalf("expected setup to use downloaded binary %q, got commands: %v", fakeBinaryPath, commands)
	}
	if versionCommand != fakeBinaryPath {
		t.Fatalf("expected version gate to use downloaded binary %q, got %q", fakeBinaryPath, versionCommand)
	}
	if probeCommand != fakeBinaryPath {
		t.Fatalf("expected protocol probe to use downloaded binary %q, got %q", fakeBinaryPath, probeCommand)
	}
}

// TestRunInstallWindowsEngramUsesDownloadNotGoInstall verifies Windows path.
func TestRunInstallWindowsEngramUsesDownloadNotGoInstall(t *testing.T) {
	home := t.TempDir()
	restoreHome := osUserHomeDir
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	restoreAddUserPath := addUserPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
		addUserPath = restoreAddUserPath
	})

	osUserHomeDir = func() (string, error) { return home, nil }
	cmdLookPath = missingBinaryLookPath
	recorder := &commandRecorder{}
	runCommand = recorder.record
	fakeBinDir := filepath.Join(home, "engram-bin")
	fakeBinaryPath := filepath.Join(fakeBinDir, "engram.exe")
	var addedPath string
	addUserPath = func(dir string) error {
		addedPath = dir
		return nil
	}

	origDownloadFn := engramDownloadFn
	engramDownloadFn = func(profile system.PlatformProfile) (string, error) {
		return fakeBinaryPath, nil
	}
	t.Cleanup(func() { engramDownloadFn = origDownloadFn })

	detection := system.DetectionResult{
		System: system.SystemInfo{
			OS:        "windows",
			Arch:      "amd64",
			Supported: true,
			Profile: system.PlatformProfile{
				OS:             "windows",
				PackageManager: "winget",
				Supported:      true,
			},
		},
	}

	result, err := RunInstall(
		[]string{"--agent", "claude-code", "--component", "engram"},
		detection,
	)
	if err != nil {
		t.Fatalf("RunInstall() error = %v", err)
	}

	if !result.Verify.Ready {
		t.Fatalf("verification ready = false, report = %#v", result.Verify)
	}
	if addedPath != fakeBinDir {
		t.Fatalf("Windows engram install should request adding downloaded binary dir to PATH, got %q", addedPath)
	}

	// Must NOT have called "go install" for engram.
	for _, cmd := range recorder.get() {
		if strings.Contains(cmd, "go install") && strings.Contains(cmd, "engram") {
			t.Fatalf("Windows engram install should NOT use go install, got command: %s", cmd)
		}
	}
}

// TestRunInstallMacOSEngramStillUsesBrew verifies macOS unchanged.
func TestRunInstallMacOSEngramStillUsesBrew(t *testing.T) {
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
	cmdLookPath = missingBinaryLookPath
	recorder := &commandRecorder{}
	runCommand = recorder.record

	// DownloadFn should NOT be called for macOS (brew handles it).
	origDownloadFn := engramDownloadFn
	engramDownloadFn = func(profile system.PlatformProfile) (string, error) {
		t.Error("DownloadLatestBinary should NOT be called on macOS (brew handles it)")
		return "", nil
	}
	t.Cleanup(func() { engramDownloadFn = origDownloadFn })

	detection := macOSDetectionResult()
	result, err := RunInstall(
		[]string{"--agent", "claude-code", "--component", "engram"},
		detection,
	)
	if err != nil {
		t.Fatalf("RunInstall() error = %v", err)
	}
	if !result.Verify.Ready {
		t.Fatalf("verification ready = false")
	}

	// Must use brew install engram.
	commands := recorder.get()
	foundBrew := false
	for _, cmd := range commands {
		if strings.Contains(cmd, "brew install engram") {
			foundBrew = true
		}
	}
	if !foundBrew {
		t.Fatalf("expected brew install engram on macOS, got commands: %v", commands)
	}
}

func TestRunInstallBetaEngramUsesMainGoInstallAndInstalledBinary(t *testing.T) {
	home := t.TempDir()
	gobin := filepath.Join(home, "go-bin")
	binaryName := "engram"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	betaEngram := filepath.Join(gobin, binaryName)

	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	restoreGoEnv := goEnv
	restorePath := os.Getenv("PATH")
	restoreVerifyVersionCommand := verifyEngramVersionCommand
	restoreProbeCommand := probeEngramProtocolFlagCommand
	t.Cleanup(func() {
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
		goEnv = restoreGoEnv
		os.Setenv("PATH", restorePath)
		verifyEngramVersionCommand = restoreVerifyVersionCommand
		probeEngramProtocolFlagCommand = restoreProbeCommand
	})

	cmdLookPath = func(name string) (string, error) {
		if name == "engram" {
			return "/usr/local/bin/engram", nil
		}
		return missingBinaryLookPath(name)
	}
	goEnv = func(keys ...string) (map[string]string, error) {
		return map[string]string{"GOBIN": gobin, "GOPATH": filepath.Join(home, "go")}, nil
	}
	recorder := &commandRecorder{}
	runCommand = recorder.record
	var versionCommand string
	verifyEngramVersionCommand = func(command string) (string, error) {
		versionCommand = command
		return "engram 1.18.0", nil
	}
	var probeCommand string
	probeEngramProtocolFlagCommand = func(_ context.Context, command string) (string, error) {
		probeCommand = command
		return "Usage: engram setup <slug> [--protocol=slim|full]\n", nil
	}

	detection := linuxDetectionResult(system.LinuxDistroUbuntu, "apt")
	_, err := RunInstall(
		[]string{"--agent", "claude-code", "--component", "engram", "--channel", "beta"},
		detection,
	)
	if err != nil {
		t.Fatalf("RunInstall() error = %v", err)
	}

	commands := recorder.get()
	foundGoInstall := false
	foundSetupWithBetaBinary := false
	for _, cmd := range commands {
		if cmd == "go install github.com/Gentleman-Programming/engram/cmd/engram@main" {
			foundGoInstall = true
		}
		if strings.HasPrefix(cmd, betaEngram+" setup ") {
			foundSetupWithBetaBinary = true
		}
	}
	if !foundGoInstall {
		t.Fatalf("expected beta engram go install from main, got commands: %v", commands)
	}
	if !foundSetupWithBetaBinary {
		t.Fatalf("expected setup to use beta engram binary %q, got commands: %v", betaEngram, commands)
	}
	if versionCommand != betaEngram {
		t.Fatalf("expected version gate to use beta engram binary %q, got %q", betaEngram, versionCommand)
	}
	if probeCommand != betaEngram {
		t.Fatalf("expected protocol probe to use beta engram binary %q, got %q", betaEngram, probeCommand)
	}
}

// Make sure the engram package's DownloadLatestBinary is accessible.
var _ = engram.DownloadLatestBinary
