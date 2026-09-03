package engram

import (
	"reflect"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/system"
)

func TestInstallCommandUsesV2MainOnEveryPlatform(t *testing.T) {
	want := [][]string{{"go", "install", "github.com/Gentleman-Programming/engram/v2/cmd/engram@main"}}
	for _, profile := range []system.PlatformProfile{{OS: "linux"}, {OS: "darwin", PackageManager: "brew"}, {OS: "windows"}} {
		got, err := InstallCommand(profile)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s command=%v err=%v want=%v", profile.OS, got, err, want)
		}
	}
}
