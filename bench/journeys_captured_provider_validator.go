package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const capturedProviderValidatorLineage = "captured-provider-validator"
const capturedProviderValidatorRejectionLineage = "captured-provider-validator-rejection"

var capturedProviderValidatorStatusCapability = &Capability{Verb: []string{"review", "status"}, Flags: []string{
	"--cwd", "--contract", "--agent", "--lineage", "--next-transition",
}}

// capturedProviderValidatorJourneys proves the STATUS-to-terminal-validator
// continuation through the runtime-neutral native submission descriptor.
func capturedProviderValidatorJourneys() []Journey {
	return []Journey{
		{
			ID:     "j106-captured-provider-validator-terminal-capture",
			Review: reviewOptedIn,
			Title:  "#3587: captured provider validator closes only through its exact active lineage",
			Source: "#3587 provider-slot continuation: an occupied Go-admitted validator slot is an exact active-lineage provider fact, and its capture is the terminal event",
			Steps: []Step{
				{Name: "fixture: repo", Fixture: baseRepo},
				{Name: "fixture: stage correction candidate", Fixture: stageCaptureEvidenceDescriptorCorrection},
				{Name: "start correction review with an exact active lineage", Requires: startNamedCapability, Args: productArgs("review", "start", "--lineage", capturedProviderValidatorLineage)},
				{Name: "capture correction finding and the full selected lens set for the exact active lineage", Requires: captureResultCapability, Composite: func(r *journeyRun) error {
					return captureExactSelectedReviewerSlots(r, capturedProviderValidatorLineage, true)
				}},
				{Name: "capture the Go-issued bounded correction plan", Requires: captureCorrectionPlanCapability, Composite: func(r *journeyRun) error {
					return captureCorrectionPlanFor(r, capturedProviderValidatorLineage, 2)
				}},
				{Name: "fixture: correct the reviewed candidate", Fixture: writeCorrectedCandidate},
				{Name: "capture the Go-issued validator Task through the native relay protocol", Requires: capturedProviderValidatorStatusCapability, Composite: captureProviderValidatorSlot},
				{Name: "the terminal validator capture exposes acknowledgement before the exact lineage burns", Requires: statusCapability, Composite: func(r *journeyRun) error {
					return requireAtomicLineageAcknowledged(r, capturedProviderValidatorLineage)
				}},
			},
		},
		{
			ID:     "j123-rejected-provider-validator-starts-fresh-high-risk-review",
			Review: reviewOptedIn,
			Title:  "#3799: a rejected validator leaves a changed normal candidate for a fresh high-risk review",
			Source: "#3799 Boundary B: inline actionable rejection evidence is terminal for its exact authority; a normal candidate edit receives only selectorless STATUS/START and all four new lenses",
			Steps: []Step{
				{Name: "fixture: repo", Fixture: baseRepo},
				{Name: "fixture: stage high-risk correction candidate", Fixture: stageAtomicHighRiskCorrectionCandidate},
				{Name: "start correction review with an exact active lineage", Requires: startNamedCapability, Args: productArgs("review", "start", "--lineage", capturedProviderValidatorRejectionLineage)},
				{Name: "capture correction finding and all four selected lens slots", Requires: captureResultCapability, Composite: func(r *journeyRun) error {
					return captureAtomicReviewerSlots(r, capturedProviderValidatorRejectionLineage, true)
				}},
				{Name: "capture the Go-issued bounded correction plan", Requires: captureCorrectionPlanCapability, Composite: func(r *journeyRun) error {
					return captureCorrectionPlanFor(r, capturedProviderValidatorRejectionLineage, 2)
				}},
				{Name: "fixture: correct the reviewed candidate", Fixture: writeCorrectedCandidate},
				{Name: "capture inline actionable rejection evidence from Boundary A", Requires: capturedProviderValidatorStatusCapability, Composite: func(r *journeyRun) error {
					return captureRejectedProviderValidatorSlotFor(r, capturedProviderValidatorRejectionLineage)
				}},
				{Name: "fixture: make a normal candidate edit", Fixture: writeNormalCandidateAfterRejectedValidator},
				{Name: "selectorless STATUS/START creates a different fresh lineage with all four required lenses", Requires: atomicReviewStatusCapability, Composite: startFreshRejectedValidatorReview},
			},
		},
	}
}

func captureProviderValidatorSlot(r *journeyRun) error {
	return captureProviderValidatorSlotFor(r, capturedProviderValidatorLineage)
}

// captureProviderValidatorSlotFor relays the provider-owned validator request
// that STATUS binds to one correction. The relay's successful completion is
// the final event: it captures validation, approves, and exposes acknowledgement before the lineage burns.
func captureProviderValidatorSlotFor(r *journeyRun, lineage string) error {
	return captureProviderValidatorSlotWithResult(r, lineage, true)
}

func captureRejectedProviderValidatorSlotFor(r *journeyRun, lineage string) error {
	return captureProviderValidatorSlotWithResult(r, lineage, false)
}

func captureProviderValidatorSlotWithResult(r *journeyRun, lineage string, passed bool) error {
	return captureProviderValidatorSlotWithResultFor(r, lineage, passed)
}

func captureProviderValidatorSlotWithResultFor(r *journeyRun, lineage string, passed bool, selectors ...string) error {
	status, err := readProviderValidatorStatus(r, lineage, true, selectors...)
	if err != nil {
		return err
	}
	if status.ValidationRequest == nil || status.NextTransition == nil || status.NextTransition.Kind != "collect" ||
		status.NextTransition.ReasonCode != "targeted_validation_required" || status.NextTransition.Collect == nil ||
		len(status.NextTransition.Collect.Inputs) != 1 {
		return fmt.Errorf("validator capture status = %+v", status)
	}
	input := status.NextTransition.Collect.Inputs[0]
	if input.Name != "provider_targeted_validator" || input.CaptureOperation != "review.capture-validation" {
		return fmt.Errorf("validator capture input = %+v", input)
	}
	originalEvidence := "original acceptance check passed"
	if !passed {
		originalEvidence = "the corrected candidate still fails the original criterion"
	}
	payload, err := json.Marshal(map[string]any{
		"targeted_validation_request_hash": status.ValidationRequest.RequestHash,
		"correction_target_identity":       status.ValidationRequest.CorrectionTargetIdentity,
		"original_criteria":                map[string]any{"passed": passed, "evidence": []string{originalEvidence}},
		"correction_regression":            map[string]any{"passed": true, "evidence": []string{"the correction introduced no unrelated regression"}},
		"follow_ups":                       []any{},
		"passed_note_unused":               "provider diagnostic ignored during raw targeted-validator admission",
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(r.sandbox.Home, "validator-result.json"), append(payload, '\n'), 0o600); err != nil {
		return err
	}
	binDir := filepath.Join(r.sandbox.Root, "provider-bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	stub := `#!/bin/sh
set -eu
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output-last-message" ]; then out="$2"; shift 2; continue; fi
  shift
done
test -n "$out"
cp "$HOME/validator-result.json" "$out"
`
	if err := os.WriteFile(filepath.Join(binDir, "codex"), []byte(stub), 0o755); err != nil {
		return err
	}
	r.sandbox.PathOverride = binDir
	arguments := []string{"review", "capture-validation"}
	for _, argument := range input.Arguments {
		arguments = append(arguments, "--"+argument.Name+"="+argument.Value)
	}
	result, err := decodeWaveOperation(r.runAt(r.sandbox.Repo, arguments, true), "targeted validation")
	if err != nil {
		return err
	}
	wantState := "approved"
	if !passed {
		wantState = "escalated"
	}
	if result.State != wantState || result.LineageID != lineage {
		return fmt.Errorf("targeted validation result = %+v, want state %q", result, wantState)
	}
	return nil
}

func finalizeCapturedProviderValidatorSlot(r *journeyRun) error {
	status, err := readCapturedProviderValidatorStatus(r, false)
	if err != nil {
		return err
	}
	if status.Authority == nil || status.ValidationRequest == nil || status.NextTransition == nil || status.NextTransition.Kind != "execute" ||
		status.NextTransition.ReasonCode != "captured_provider_targeted_validation_ready" || status.NextTransition.Execute == nil ||
		status.NextTransition.Execute.Operation != "review.finalize" {
		return fmt.Errorf("generic captured-provider transition = %+v", status.NextTransition)
	}
	context := executeArgument(status.NextTransition.Execute.Arguments, "repository-context")
	want := []string{
		"--contract=" + reviewContractV2,
		"--lineage=" + capturedProviderValidatorLineage,
		"--expected-revision=" + status.Authority.Revision,
		"--target=" + status.ValidationRequest.CorrectionTargetIdentity,
		"--request-hash=" + status.ValidationRequest.RequestHash,
		"--repository-context=" + context,
		"--captured-evidence=true",
	}
	got := make([]string, len(status.NextTransition.Execute.Arguments))
	for index, argument := range status.NextTransition.Execute.Arguments {
		got[index] = argument.Token
	}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") || strings.Contains(strings.Join(got, "\n"), "--agent=") ||
		strings.Contains(strings.Join(got, "\n"), "--validation=") || context == "" {
		return fmt.Errorf("generic captured-provider finalize argv = %v, want %v", got, want)
	}
	result, err := decodeWaveOperation(r.runAt(r.sandbox.Root, append([]string{"review", "finalize"}, got...), false), "generic captured-provider finalize")
	if err != nil || result.State != "approved" || result.LineageID != capturedProviderValidatorLineage {
		return fmt.Errorf("generic captured-provider finalize result = %+v, %v", result, err)
	}
	return requireAtomicLineageAcknowledged(r, capturedProviderValidatorLineage)
}

func readCapturedProviderValidatorStatus(r *journeyRun, withProvider bool) (waveCorrectionStatus, error) {
	return readProviderValidatorStatus(r, capturedProviderValidatorLineage, withProvider)
}

func readProviderValidatorStatus(r *journeyRun, lineage string, withProvider bool, selectors ...string) (waveCorrectionStatus, error) {
	arguments := []string{"review", "status", "--contract", reviewContractV2, "--next-transition", "--lineage", lineage}
	arguments = append(arguments, selectors...)
	if withProvider {
		arguments = append(arguments, "--agent", "codex")
	}
	observation := r.run(productArgsFor(r, arguments...), false)
	var status waveCorrectionStatus
	return status, decodeWaveObservation(observation, &status, "provider validator status")
}

func executeArgument(arguments []waveTransitionArgument, name string) string {
	for _, argument := range arguments {
		if argument.Name == name {
			return argument.Value
		}
	}
	return ""
}

func writeNormalCandidateAfterRejectedValidator(sandbox *Sandbox) error {
	return sandbox.write(filepath.Join(sandbox.Repo, "candidate.go"), "package candidate\n\nfunc value() int { return 4 }\n")
}

func startFreshRejectedValidatorReview(r *journeyRun) error {
	lineage, err := startAtomicTransactionFromSelectorlessStatus(r, capturedProviderValidatorRejectionLineage)
	if err != nil {
		return fmt.Errorf("selectorless START after rejected validator: %w", err)
	}
	r.sandbox.Lineage = lineage
	return requireExplicitAtomicFourLensStatusFor(r, lineage)
}
