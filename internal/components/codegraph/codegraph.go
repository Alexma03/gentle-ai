// Package codegraph owns the CodeGraph integration boundary.
//
// CodeGraph used to be represented as a generic community tool.  The
// implementation is still shared with that compatibility surface during this
// migration, but callers that manage the stack use this package so CodeGraph
// has an explicit component boundary.  Keeping the bridge here makes the
// migration reversible without duplicating path, rollback, or Pi semantics.
package codegraph

import (
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/communitytool"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

type Availability = communitytool.Availability
type AgentStatusKind = communitytool.AgentStatusKind
type Definition = communitytool.Definition
type Result = communitytool.Result
type Status = communitytool.Status
type AgentStatus = communitytool.AgentStatus
type Detector = communitytool.Detector
type DetectorFunc = communitytool.DetectorFunc
type Runner = communitytool.Runner
type RunnerFunc = communitytool.RunnerFunc
type GuidanceInjectionResult = communitytool.GuidanceInjectionResult

type PiChildClassification = communitytool.PiChildClassification
type PiCodeGraphChild = communitytool.PiCodeGraphChild
type PiCodeGraphOptions = communitytool.PiCodeGraphOptions
type PiCodeGraphResult = communitytool.PiCodeGraphResult
type PiCodeGraphMCPVerification = communitytool.PiCodeGraphMCPVerification
type PiCodeGraphMCPTool = communitytool.PiCodeGraphMCPTool
type PiCodeGraphMCPProbeResult = communitytool.PiCodeGraphMCPProbeResult

const (
	AvailabilityAvailable = communitytool.AvailabilityAvailable
	AvailabilityMissing   = communitytool.AvailabilityMissing

	AgentStatusUnavailable = communitytool.AgentStatusUnavailable
	AgentStatusConfigured  = communitytool.AgentStatusConfigured
	AgentStatusMissing     = communitytool.AgentStatusMissing
)

var ErrPiCodeGraphAdapterHealthUnavailable = communitytool.ErrPiCodeGraphAdapterHealthUnavailable

// Definitions returns the explicit CodeGraph component definition. The
// returned definition retains the legacy CommunityToolCodeGraph ID so older
// state and command surfaces can be read during migration.
func Definitions() []Definition {
	return communitytool.Definitions()
}

func DefinitionFor(id model.CommunityToolID) (Definition, bool) {
	return communitytool.DefinitionFor(id)
}

// Install configures CodeGraph for the selected home and workspace. It is the
// canonical component entry point; the generic community-tool ID is fixed to
// CodeGraph so callers cannot accidentally install another tool here.
func Install(homeDir, workspaceDir string, runner Runner, detector Detector) (Result, error) {
	return communitytool.InstallWithHome(model.CommunityToolCodeGraph, workspaceDir, homeDir, runner, detector)
}

// InstallWithHome is an explicit-name alias for Install, useful to callers
// that already carry a runtime home override.
func InstallWithHome(homeDir, workspaceDir string, runner Runner, detector Detector) (Result, error) {
	return Install(homeDir, workspaceDir, runner, detector)
}

func DetectStatus(homeDir string, detector Detector) Status {
	return communitytool.DetectStatus(model.CommunityToolCodeGraph, homeDir, detector)
}

func Verify(homeDir string, detector Detector) Status {
	return DetectStatus(homeDir, detector)
}

func CodeGraphCommands() [][]string {
	return communitytool.CodeGraphCommands()
}

func CodeGraphCommandsForDetector(detector Detector) ([][]string, error) {
	return communitytool.CodeGraphCommandsForDetector(detector)
}

func CodeGraphCommandsForDetectorAndTargets(detector Detector, targets []string) ([][]string, error) {
	return communitytool.CodeGraphCommandsForDetectorAndTargets(detector, targets)
}

func CodeGraphGuidanceMarkdown() string {
	return communitytool.CodeGraphGuidanceMarkdown()
}

func InjectGuidance(homeDir string) (GuidanceInjectionResult, error) {
	return communitytool.InjectCodeGraphGuidance(homeDir)
}

func RefreshGuidanceIfConfigured(homeDir string, detector Detector) (GuidanceInjectionResult, bool, error) {
	return communitytool.RefreshCodeGraphGuidanceIfConfigured(homeDir, detector)
}

func HasConfigured(homeDir string, detector Detector) bool {
	return communitytool.HasConfiguredCodeGraph(homeDir, detector)
}

func HasManagedGuidance(homeDir string) bool {
	return communitytool.HasManagedCodeGraphGuidance(homeDir)
}

func HasAnyGuidance(homeDir string) bool {
	return communitytool.HasAnyCodeGraphGuidance(homeDir)
}

func CleanLegacyGuidance(homeDir string) (GuidanceInjectionResult, error) {
	return communitytool.CleanLegacyCodeGraphGuidance(homeDir)
}

func GuidancePaths(homeDir string) []string {
	return communitytool.CodeGraphGuidancePaths(homeDir)
}

// ManagedPaths is the complete install/sync/rollback path declaration for the
// CodeGraph component. It includes detected-agent wiring and managed guidance.
func ManagedPaths(homeDir string) []string {
	return communitytool.CodeGraphManagedPaths(homeDir)
}

// BackupPaths is intentionally an alias of ManagedPaths: CodeGraph's setup and
// guidance writers are transactional, so the exact same paths must be backed
// up before install and sync.
func BackupPaths(homeDir string) []string {
	return ManagedPaths(homeDir)
}

func NeedsOpenCodeReconcile(homeDir string) bool {
	return communitytool.NeedsOpenCodeCodeGraphReconcile(homeDir)
}

func ReconcileOpenCode(homeDir string, runner Runner) (GuidanceInjectionResult, error) {
	return communitytool.ReconcileOpenCodeCodeGraph(homeDir, runner)
}

func ReconcilePi(options PiCodeGraphOptions) (PiCodeGraphResult, error) {
	return communitytool.ReconcilePiCodeGraph(options)
}

func PreservePiPending(result PiCodeGraphResult, err error) (PiCodeGraphResult, error) {
	return communitytool.PreservePiCodeGraphPending(result, err)
}

func PiConfigured(homeDir, workspaceDir string) (bool, string) {
	return communitytool.PiCodeGraphConfigured(homeDir, workspaceDir)
}

func PiPaths(homeDir, workspaceDir string) []string {
	return communitytool.PiCodeGraphPaths(homeDir, workspaceDir)
}

func ValidatePiRoot(root, homeDir string) error {
	return communitytool.ValidatePiCodeGraphRoot(root, homeDir)
}

func RefreshPiIfConfigured(homeDir, workspaceDir string) (PiCodeGraphResult, bool, error) {
	return communitytool.RefreshPiCodeGraphIfConfigured(homeDir, workspaceDir)
}

func UninstallPi(homeDir string) (PiCodeGraphResult, error) {
	return communitytool.UninstallPiCodeGraph(homeDir)
}
