package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPersonalInstallerDestinationAndActivationContract(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the personal Bash installer is exercised on Unix runners")
	}

	for _, tc := range []struct {
		name       string
		args       bool
		envGOBIN   bool
		goGOBIN    bool
		shadowPath bool
		wantOK     bool
	}{
		{name: "explicit install directory", args: true, wantOK: true},
		{name: "GOBIN environment", envGOBIN: true, wantOK: true},
		{name: "go env GOBIN", goGOBIN: true, wantOK: true},
		{name: "shadowed command fails closed", args: true, shadowPath: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			installDir := filepath.Join(root, "install dir")
			fakeBin := filepath.Join(root, "fake-bin")
			if err := os.MkdirAll(fakeBin, 0o755); err != nil {
				t.Fatal(err)
			}
			writeFakeGoForPersonalInstaller(t, fakeBin)

			pathEntries := []string{installDir, fakeBin, os.Getenv("PATH")}
			if tc.shadowPath {
				shadow := filepath.Join(root, "shadow")
				if err := os.MkdirAll(shadow, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(shadow, "gentle-ai"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
					t.Fatal(err)
				}
				pathEntries = []string{shadow, fakeBin, installDir, os.Getenv("PATH")}
			}

			args := []string{filepath.Join("..", "..", "scripts", "install-personal.sh")}
			if tc.args {
				args = append(args, "--install-dir", installDir)
			}
			cmd := exec.Command("bash", args...)
			cmd.Env = append(environmentWithout("GOBIN", "GENTLE_AI_INSTALL_DIR"),
				"HOME="+root,
				"PATH="+strings.Join(pathEntries, string(os.PathListSeparator)),
				"FAKE_GO_GOBIN=",
			)
			if tc.envGOBIN {
				cmd.Env = append(cmd.Env, "GOBIN="+installDir)
			}
			if tc.goGOBIN {
				cmd.Env = append(cmd.Env, "FAKE_GO_GOBIN="+installDir)
			}
			output, err := cmd.CombinedOutput()
			installed := filepath.Join(installDir, "gentle-ai")
			if _, statErr := os.Stat(installed); statErr != nil {
				t.Fatalf("installed binary missing: %v\n%s", statErr, output)
			}
			if tc.wantOK && err != nil {
				t.Fatalf("installer failed: %v\n%s", err, output)
			}
			if !tc.wantOK && err == nil {
				t.Fatalf("installer accepted a shadowed command:\n%s", output)
			}
			wantInvocation := "Run exactly: " + strings.ReplaceAll(installed, " ", "\\ ") + " sync"
			if !strings.Contains(string(output), wantInvocation) {
				t.Fatalf("installer output missing %q:\n%s", wantInvocation, output)
			}
		})
	}
}

func writeFakeGoForPersonalInstaller(t *testing.T, dir string) {
	t.Helper()
	script := "#!/bin/sh\nset -eu\n" +
		"if [ \"$1\" = \"env\" ]; then\n" +
		"  case \"$2\" in\n" +
		"    GOBIN) printf '%s\\n' \"$FAKE_GO_GOBIN\" ;;\n" +
		"    GOPATH) printf '%s\\n' \"$HOME/go\" ;;\n" +
		"  esac\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"build\" ]; then\n" +
		"  while [ \"$#\" -gt 0 ]; do\n" +
		"    if [ \"$1\" = \"-o\" ]; then\n" +
		"      shift\n" +
		"      printf '#!/bin/sh\\nprintf \"gentle-ai test\\\\n\"\\n' > \"$1\"\n" +
		"      exit 0\n" +
		"    fi\n" +
		"    shift\n" +
		"  done\n" +
		"fi\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func environmentWithout(names ...string) []string {
	blocked := make(map[string]bool, len(names))
	for _, name := range names {
		blocked[name] = true
	}
	var result []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !blocked[name] {
			result = append(result, entry)
		}
	}
	return result
}
