package update

import (
	"fmt"
	"strings"

	"github.com/gentleman-programming/gentle-ai/v2/internal/system"
)

const WindowsDistributionHoldMessage = "Windows binary distribution and Scoop are temporarily unavailable until publicly trusted Authenticode signing is enforced."

// GentleAISourceInstallCommand returns the safe source-install fallback for an
// exact release, beta main build, or the latest release when version is empty.
func GentleAISourceInstallCommand(version string) string {
	target := "latest"
	version = strings.TrimSpace(version)
	if strings.HasPrefix(version, "main@") {
		target = "main"
	} else if version != "" {
		target = "v" + strings.TrimPrefix(version, "v")
	}
	return "go install github.com/gentleman-programming/gentle-ai/v2/cmd/gentle-ai@" + target
}

// updateHint returns a platform-specific instruction string for updating the given tool.
func updateHint(tool ToolInfo, profile system.PlatformProfile) string {
	switch tool.Name {
	case "gentle-ai":
		return gentleAIHint(profile)
	case "engram":
		return engramHint(profile)
	default:
		return ""
	}
}

func updateHintForOwnership(tool ToolInfo, profile system.PlatformProfile, ownership HomebrewOwnership) string {
	if profile.PackageManager == "brew" && ownership != HomebrewNone {
		return fmt.Sprintf("brew upgrade --%s %s", ownership, tool.Name)
	}
	return updateHint(tool, profile)
}

// gentleAIHint is the stable-channel instruction only. When the checker
// resolves a main-head beta target, applyBetaMainHeadStatus overrides this
// hint with GentleAISourceInstallCommand so the printed instruction installs
// the advertised target instead of the latest stable release.
func gentleAIHint(profile system.PlatformProfile) string {
	if profile.PackageManager == "brew" && homebrewPackageInstalled("gentle-ai") {
		return "brew upgrade gentle-ai"
	}

	switch profile.OS {
	case "linux":
		return "curl -fsSL https://raw.githubusercontent.com/Gentleman-Programming/gentle-ai/main/scripts/install.sh | bash"
	case "darwin":
		return "gentle-ai upgrade (downloads pre-built binary)"
	case "windows":
		return WindowsDistributionHoldMessage + " Install/update from source with Go 1.25.10+: " + GentleAISourceInstallCommand("")
	default:
		return ""
	}
}

func engramHint(profile system.PlatformProfile) string {
	if profile.PackageManager == "brew" && homebrewPackageInstalled("engram") {
		return "brew upgrade engram"
	}
	return "gentle-ai upgrade (downloads pre-built binary)"
}
