package codegraph

import "github.com/gentleman-programming/gentle-ai/v2/internal/model"

// Install configures the first-class CodeGraph component. The legacy
// CommunityTool ID is fixed here so callers cannot accidentally install a
// different generic tool through the CodeGraph boundary.
func Install(homeDir, workspaceDir string, runner Runner, detector Detector) (Result, error) {
	return InstallWithHome(model.CommunityToolCodeGraph, workspaceDir, homeDir, runner, detector)
}

// DetectStatus reports CodeGraph status for the selected home directory.
func DetectStatus(homeDir string, detector Detector) Status {
	return DetectStatusByID(model.CommunityToolCodeGraph, homeDir, detector)
}

// Verify is an explicit alias for the read-only status projection.
func Verify(homeDir string, detector Detector) Status {
	return DetectStatus(homeDir, detector)
}

// InjectGuidance configures CodeGraph guidance for supported retained clients.
func InjectGuidance(homeDir string) (GuidanceInjectionResult, error) {
	return InjectCodeGraphGuidance(homeDir)
}

func RefreshGuidanceIfConfigured(homeDir string, detector Detector) (GuidanceInjectionResult, bool, error) {
	return RefreshCodeGraphGuidanceIfConfigured(homeDir, detector)
}

func HasConfigured(homeDir string, detector Detector) bool {
	return HasConfiguredCodeGraph(homeDir, detector)
}

func HasManagedGuidance(homeDir string) bool {
	return HasManagedCodeGraphGuidance(homeDir)
}

func HasAnyGuidance(homeDir string) bool {
	return HasAnyCodeGraphGuidance(homeDir)
}

func CleanLegacyGuidance(homeDir string) (GuidanceInjectionResult, error) {
	return CleanLegacyCodeGraphGuidance(homeDir)
}

func GuidancePaths(homeDir string) []string {
	return CodeGraphGuidancePaths(homeDir)
}

// ManagedPaths is the complete install/sync/rollback path declaration for the
// CodeGraph component.
func ManagedPaths(homeDir string) []string {
	return CodeGraphManagedPaths(homeDir)
}

// BackupPaths intentionally aliases ManagedPaths: setup and guidance writers
// are transactional, so the same paths must be captured before mutation.
func BackupPaths(homeDir string) []string {
	return ManagedPaths(homeDir)
}

func NeedsOpenCodeReconcile(homeDir string) bool {
	return NeedsOpenCodeCodeGraphReconcile(homeDir)
}

func ReconcileOpenCode(homeDir string, runner Runner) (GuidanceInjectionResult, error) {
	return ReconcileOpenCodeCodeGraph(homeDir, runner)
}

func ReconcilePi(options PiCodeGraphOptions) (PiCodeGraphResult, error) {
	return ReconcilePiCodeGraph(options)
}

func PreservePiPending(result PiCodeGraphResult, err error) (PiCodeGraphResult, error) {
	return PreservePiCodeGraphPending(result, err)
}

func PiConfigured(homeDir, workspaceDir string) (bool, string) {
	return PiCodeGraphConfigured(homeDir, workspaceDir)
}

func PiPaths(homeDir, workspaceDir string) []string {
	return PiCodeGraphPaths(homeDir, workspaceDir)
}

func ValidatePiRoot(root, homeDir string) error {
	return ValidatePiCodeGraphRoot(root, homeDir)
}

func RefreshPiIfConfigured(homeDir, workspaceDir string) (PiCodeGraphResult, bool, error) {
	return RefreshPiCodeGraphIfConfigured(homeDir, workspaceDir)
}

func UninstallPi(homeDir string) (PiCodeGraphResult, error) {
	return UninstallPiCodeGraph(homeDir)
}
