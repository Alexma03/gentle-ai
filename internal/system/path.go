package system

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type userPathPowerShellRunner interface {
	Run(context.Context, ...string) ([]byte, error)
}

// UserPathAddition records the PATH entries this invocation created.
type UserPathAddition struct {
	ProcessAdded    bool
	PersistentAdded bool
}

var (
	userPathGOOS                = runtime.GOOS
	userPathRunningInGoTest     = runningInGoTest
	newUserPathPowerShellRunner = func() userPathPowerShellRunner { return NewPowerShellRunner() }
)

// AddToUserPath adds a directory to the Windows user PATH persistently.
// Uses PowerShell to modify the user-scoped environment variable in the registry,
// which survives terminal restarts without requiring admin privileges.
//
// On non-Windows platforms this is a no-op (returns nil immediately after adding
// to the current process PATH). This is safe to call on all platforms since the
// binary is cross-compiled — build tags are NOT used.
func AddToUserPath(dir string) error {
	_, err := AddToUserPathWithResult(dir)
	return err
}

// AddToUserPathWithResult adds dir and reports exactly which PATH scopes changed.
// Callers that must compensate a failed larger transaction can safely restore only
// entries created by this invocation.
func AddToUserPathWithResult(dir string) (UserPathAddition, error) {
	addition := UserPathAddition{ProcessAdded: !processPathContains(dir)}
	if userPathGOOS != "windows" {
		// Still add to the current process PATH on non-Windows (harmless for callers).
		return addition, addToProcessPath(dir)
	}
	if userPathRunningInGoTest() {
		// Go tests must not mutate the real Windows user PATH registry. Keep the
		// test process behavior identical for callers that need the new directory
		// available later in the same run.
		return addition, addToProcessPath(dir)
	}

	// 1. Update the current process PATH so subsequent commands in this run can
	//    find the newly installed binary immediately.
	if err := addToProcessPath(dir); err != nil {
		return UserPathAddition{}, err
	}

	// 2. Persist via PowerShell: modifies the user-scoped PATH in the registry.
	//    This change survives terminal restarts and applies to all future processes
	//    for this user without requiring admin privileges.
	//
	//    escapePowerShellString replaces ' with '' (PowerShell's escape for single quotes
	//    within single-quoted strings) to prevent injection via path names like C:\O'Brien.
	safeDir := escapePowerShellString(dir)
	script := fmt.Sprintf(
		`$ErrorActionPreference = 'Stop'; `+
			`$current = [Environment]::GetEnvironmentVariable('PATH', 'User'); `+
			`$exists = $false; if ($current) { $exists = $current.Split(';') | Where-Object { [string]::Compare($_.Trim().Trim('"'), '%s', $true) -eq 0 } | Select-Object -First 1 }; `+
			`if (-not $exists) { `+
			`$updated = if ($current) { '%s;' + $current } else { '%s' }; `+
			`[Environment]::SetEnvironmentVariable('PATH', $updated, 'User'); 'changed' `+
			`} else { 'unchanged' }`,
		safeDir, safeDir, safeDir,
	)
	output, err := newUserPathPowerShellRunner().Run(context.Background(), "-NoProfile", "-NonInteractive", "-Command", script)
	if err != nil {
		return addition, err
	}
	switch strings.TrimSpace(string(output)) {
	case "changed":
		addition.PersistentAdded = true
	case "unchanged":
	default:
		return addition, fmt.Errorf("add persistent user PATH: unexpected PowerShell result %q", strings.TrimSpace(string(output)))
	}
	return addition, nil
}

// RemoveFromUserPath removes one matching directory from the Windows user PATH
// and the current process PATH. Matching trims whitespace and quotes and is
// case-insensitive, while all unrelated entries retain their original order.

// RollbackUserPathAddition restores only PATH entries created by
// AddToUserPathWithResult.

func runningInGoTest() bool {
	return flag.Lookup("test.v") != nil
}

// escapePowerShellString escapes a string for safe use inside a PowerShell
// single-quoted string literal by replacing each ' with ” (PowerShell's escape
// sequence for a literal single quote within single-quoted strings).
func escapePowerShellString(s string) string {
	return PowerShellSingleQuoted(s)
}

// addToProcessPath prepends dir to the current process PATH if it is not already
// present. This is a low-level helper called by AddToUserPath.
func addToProcessPath(dir string) error {
	currentPath := os.Getenv("PATH")

	// Already present in process PATH? Skip.
	for _, p := range filepath.SplitList(currentPath) {
		if strings.EqualFold(filepath.Clean(p), filepath.Clean(dir)) {
			return nil
		}
	}

	if currentPath == "" {
		return os.Setenv("PATH", dir)
	}
	return os.Setenv("PATH", dir+string(os.PathListSeparator)+currentPath)
}

func processPathContains(dir string) bool {
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.EqualFold(filepath.Clean(entry), filepath.Clean(dir)) {
			return true
		}
	}
	return false
}
