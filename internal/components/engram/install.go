package engram

import "github.com/gentleman-programming/gentle-ai/v2/internal/system"

// InstallCommand retains the component resolver API while selecting the same
// Engram v2 main source on every supported platform.
func InstallCommand(system.PlatformProfile) ([][]string, error) {
	return [][]string{{"go", "install", engramCanonicalPackage + "@main"}}, nil
}
