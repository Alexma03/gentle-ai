package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCodeGraphInitValidatesCanonicalProjectAndPropagatesInitFailure(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "project")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}

	originalRoot := codeGraphGitTopLevel
	originalInit := codeGraphInit
	originalHome := codeGraphUserHomeDir
	originalTemp := codeGraphTempDir
	t.Cleanup(func() {
		codeGraphGitTopLevel = originalRoot
		codeGraphInit = originalInit
		codeGraphUserHomeDir = originalHome
		codeGraphTempDir = originalTemp
	})
	codeGraphGitTopLevel = func(path string) (string, error) {
		assertSameFile(t, path, root)
		return root, nil
	}
	codeGraphUserHomeDir = func() (string, error) { return filepath.Join(workspace, "home"), nil }
	codeGraphTempDir = func() string { return filepath.Join(workspace, "temporary") }

	var output bytes.Buffer
	var called []string
	codeGraphInit = func(name string, args ...string) error {
		called = append([]string{name}, args...)
		return nil
	}
	if err := RunCodeGraph([]string{"init", "--cwd", root}, &output); err != nil {
		t.Fatalf("RunCodeGraph() error = %v", err)
	}
	if len(called) != 3 || called[0] != "codegraph" || called[1] != "init" {
		t.Fatalf("command = %v, want codegraph init <root>", called)
	}
	assertSameFile(t, called[2], root)
	if !strings.Contains(output.String(), called[2]) {
		t.Fatalf("output = %q, want canonical root", output.String())
	}

	codeGraphInit = func(string, ...string) error { return errors.New("init failed") }
	if err := RunCodeGraph([]string{"init", "--cwd", root}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "init failed") {
		t.Fatalf("subprocess error = %v, want propagated init failure", err)
	}
}

func TestRunCodeGraphInitRejectsUnsafeOrUnrecognizedRoots(t *testing.T) {
	workspace := t.TempDir()
	home := filepath.Join(workspace, "home")
	temp := filepath.Join(workspace, "temporary")
	for _, path := range []string{home, temp} {
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(temp, "outside")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(workspace, "escape")
	if err := os.Symlink(outside, symlink); err != nil {
		t.Fatal(err)
	}

	originalRoot := codeGraphGitTopLevel
	originalInit := codeGraphInit
	originalHome := codeGraphUserHomeDir
	originalTemp := codeGraphTempDir
	t.Cleanup(func() {
		codeGraphGitTopLevel = originalRoot
		codeGraphInit = originalInit
		codeGraphUserHomeDir = originalHome
		codeGraphTempDir = originalTemp
	})
	codeGraphGitTopLevel = func(path string) (string, error) {
		if path == filepath.Join(workspace, "not-a-project") {
			return "", errors.New("not a git repository")
		}
		return path, nil
	}
	codeGraphUserHomeDir = func() (string, error) { return home, nil }
	codeGraphTempDir = func() string { return temp }
	codeGraphInit = func(string, ...string) error { t.Fatal("codegraph init must not run for rejected roots"); return nil }

	volumeRoot := filepath.VolumeName(workspace) + string(filepath.Separator)
	for _, path := range []string{"", volumeRoot, home, temp, outside, symlink, filepath.Join(workspace, "not-a-project")} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			if err := RunCodeGraph([]string{"init", "--cwd", path}, &bytes.Buffer{}); err == nil {
				t.Fatalf("RunCodeGraph(%q) error = nil, want rejection", path)
			}
		})
	}
}

func TestRunCodeGraphInitAcceptsProjectBelowHome(t *testing.T) {
	workspace := t.TempDir()
	home := filepath.Join(workspace, "home")
	root := filepath.Join(home, "work", "project-feature")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	originalRoot := codeGraphGitTopLevel
	originalInit := codeGraphInit
	originalHome := codeGraphUserHomeDir
	originalTemp := codeGraphTempDir
	t.Cleanup(func() {
		codeGraphGitTopLevel = originalRoot
		codeGraphInit = originalInit
		codeGraphUserHomeDir = originalHome
		codeGraphTempDir = originalTemp
	})
	codeGraphGitTopLevel = func(path string) (string, error) { return path, nil }
	codeGraphUserHomeDir = func() (string, error) { return home, nil }
	codeGraphTempDir = func() string { return filepath.Join(workspace, "temporary") }

	var calledRoot string
	codeGraphInit = func(name string, args ...string) error {
		if name == "codegraph" && len(args) == 2 && args[0] == "init" {
			calledRoot = args[1]
		}
		return nil
	}
	if err := RunCodeGraph([]string{"init", "--cwd", root}, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunCodeGraph() error = %v", err)
	}
	if calledRoot == "" {
		t.Fatal("codegraph init was not called for a project below HOME")
	}
	assertSameFile(t, calledRoot, root)
}

// TestCanonicalCodeGraphProjectRootAcceptsEquivalentGitRootForms exercises
// the selector boundary with the path forms users can legitimately provide:
// a relative candidate, a symlink alias, and a git root with lexical `..`
// segments. All forms must bind to the same canonical project root before the
// upstream CodeGraph command is invoked.
func TestCanonicalCodeGraphProjectRootAcceptsEquivalentGitRootForms(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(workspace, "repo-alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	originalRoot := codeGraphGitTopLevel
	originalHome := codeGraphUserHomeDir
	originalTemp := codeGraphTempDir
	originalInit := codeGraphInit
	t.Cleanup(func() {
		codeGraphGitTopLevel = originalRoot
		codeGraphUserHomeDir = originalHome
		codeGraphTempDir = originalTemp
		codeGraphInit = originalInit
	})
	codeGraphUserHomeDir = func() (string, error) { return filepath.Join(workspace, "home"), nil }
	codeGraphTempDir = func() string { return filepath.Join(workspace, "temporary") }
	codeGraphGitTopLevel = func(path string) (string, error) {
		assertSameFile(t, path, root)
		return filepath.Join(root, "..", filepath.Base(root)), nil
	}
	codeGraphInit = func(string, ...string) error { return nil }

	for _, candidate := range []string{root, alias, filepath.Join(root, ".")} {
		t.Run(candidate, func(t *testing.T) {
			got, err := canonicalCodeGraphProjectRoot(candidate)
			if err != nil {
				t.Fatalf("canonicalCodeGraphProjectRoot(%q) error = %v", candidate, err)
			}
			assertSameFile(t, got, root)
		})
	}
}

// TestCanonicalCodeGraphProjectRootRejectsRelativeGitEscape ensures a
// provider cannot return a relative root that resolves outside the selected
// candidate. This closes the selector's symlink/relative-path ambiguity before
// any CodeGraph process is started.
func TestCanonicalCodeGraphProjectRootRejectsRelativeGitEscape(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	originalRoot := codeGraphGitTopLevel
	originalHome := codeGraphUserHomeDir
	originalTemp := codeGraphTempDir
	t.Cleanup(func() {
		codeGraphGitTopLevel = originalRoot
		codeGraphUserHomeDir = originalHome
		codeGraphTempDir = originalTemp
	})
	codeGraphUserHomeDir = func() (string, error) { return filepath.Join(workspace, "home"), nil }
	codeGraphTempDir = func() string { return filepath.Join(workspace, "temporary") }
	codeGraphGitTopLevel = func(string) (string, error) { return filepath.Join(workspace, ".."), nil }

	if _, err := canonicalCodeGraphProjectRoot(root); err == nil {
		t.Fatal("canonicalCodeGraphProjectRoot accepted a git root outside the selected project")
	}
}

func assertSameFile(t *testing.T, got, want string) {
	t.Helper()
	gotInfo, gotErr := os.Stat(got)
	wantInfo, wantErr := os.Stat(want)
	if gotErr != nil || wantErr != nil || !os.SameFile(gotInfo, wantInfo) {
		t.Fatalf("paths %q and %q do not identify the same file: %v, %v", got, want, gotErr, wantErr)
	}
}
