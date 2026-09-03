package engram

import (
	"reflect"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/system"
)

func TestInstallCommandNeverSelectsStableHomebrew(t *testing.T) {
	got, err := InstallCommand(system.PlatformProfile{OS: "darwin", PackageManager: "brew"})
	want := [][]string{{"go", "install", "github.com/Gentleman-Programming/engram/v2/cmd/engram@main"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("command=%v err=%v want=%v", got, err, want)
	}
}
