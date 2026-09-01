package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const issue3065SourceLineage = "issue-3065-escalated-predecessor"
const issue3065SuccessorLineage = "issue-3065-staged-escalated-successor"

func issue3065Journeys() []Journey {
	return []Journey{{
		ID:     "j115-recovery-selector-is-collected-before-authorization",
		Review: reviewOptedIn,
		Title:  "Default workspace-overlay recovery collects an unrepresentable selector before authorization",
		Source: "https://github.com/Gentleman-Programming/gentle-ai/issues/3065",
		Steps: []Step{
			{Name: "fixture: linked worktree and remote", Fixture: linkedWorktreeWithRemote},
			{Name: "fixture: commit base-diff predecessor candidate", Fixture: prepareIssue3065CurrentChangeCandidate},
			{Name: "start escalated predecessor review", Requires: statusCapability, Composite: startIssue3065Predecessor},
			{Name: "capture predecessor finding", Requires: captureResultCapability, Composite: func(r *journeyRun) error {
				return captureCorrectableFindingFor(r, "--lineage", issue3065SourceLineage, "--base-ref", r.sandbox.Scratch["staged-recovery-base"], "--committed-only")
			}},
			{Name: "capture predecessor correction plan", Requires: captureCorrectionPlanCapability, Composite: func(r *journeyRun) error {
				return captureCorrectionPlanFor(r, issue3065SourceLineage, 3, "--base-ref", r.sandbox.Scratch["staged-recovery-base"], "--committed-only")
			}},
			{Name: "fixture: correct predecessor candidate", Fixture: func(sandbox *Sandbox) error {
				if err := writeIssue3065CorrectionCandidate(sandbox); err != nil {
					return err
				}
				return sandbox.git(sandbox.Repo, "commit", "-qm", "issue #3065 corrected candidate")
			}},
			{Name: "failed predecessor validation escalates authority", Requires: capturedProviderValidatorStatusCapability, Composite: func(r *journeyRun) error {
				return captureIssue3065FailedValidationFor(r, issue3065SourceLineage, "--base-ref", r.sandbox.Scratch["staged-recovery-base"], "--committed-only")
			}},
			{Name: "fixture: stage legal staged-overlay recovery target", Fixture: stageIssue3065RecoveryTarget},
			{Name: "execute staged-overlay recovery successor", Requires: statusCapability, Composite: recoverIssue3065StagedCorrection},
			{Name: "capture successor finding", Requires: captureResultCapability, Composite: captureIssue3065SuccessorFinding},
			{Name: "capture successor correction plan", Requires: captureCorrectionPlanCapability, Composite: func(r *journeyRun) error {
				return captureCorrectionPlanFor(r, issue3065SuccessorLineage, 3, "--base-ref", r.sandbox.Scratch["staged-recovery-base"], "--projection", "staged", "--workspace-overlay")
			}},
			{Name: "fixture: correct staged successor candidate", Fixture: writeIssue3065CorrectionCandidate},
			{Name: "failed successor validation escalates authority", Requires: capturedProviderValidatorStatusCapability, Composite: func(r *journeyRun) error {
				return captureIssue3065FailedValidationFor(r, issue3065SuccessorLineage, "--base-ref", r.sandbox.Scratch["staged-recovery-base"], "--projection", "staged", "--workspace-overlay")
			}},
			{Name: "fixture: change final workspace-overlay target", Fixture: stageIssue3065RecoveryTarget},
			{Name: "default workspace-overlay STATUS collects target before authorization", Requires: statusCapability, Composite: proveIssue3065RecoveryCollection},
		},
	}}
}

func startIssue3065Predecessor(r *journeyRun) error {
	envelope, err := readStatusFor(r, "--lineage", issue3065SourceLineage, "--base-ref", r.sandbox.Scratch["staged-recovery-base"], "--committed-only")
	if err != nil || envelope.NextTransition.Kind != "execute" || envelope.NextTransition.Execute.Operation != "review.start" {
		return fmt.Errorf("predecessor start transition = %+v, %v", envelope.NextTransition, err)
	}
	started, err := runPrintedTransition(r, envelope)
	if err != nil {
		return err
	}
	if started.ExitCode != 0 {
		return fmt.Errorf("predecessor start failed: %s", firstLine(started.Stderr))
	}
	return nil
}

func recoverIssue3065StagedCorrection(r *journeyRun) error {
	selectors := []string{"--lineage", issue3065SourceLineage, "--base-ref", r.sandbox.Scratch["staged-recovery-base"], "--projection", "staged", "--workspace-overlay"}
	probeObservation := r.run(productArgsFor(r, append([]string{"review", "status", "--contract", reviewContract, "--next-transition", "--action-eligibility"}, selectors...)...), false)
	var probe waveCorrectionStatus
	if err := decodeWaveObservation(probeObservation, &probe, "issue #3065 staged recovery probe"); err != nil {
		return err
	}
	if probe.Authority == nil || probe.Authority.LineageID != issue3065SourceLineage || probe.Action != "recover" || probe.ActionDisposition != "escalated" || probe.NextTransition == nil || probe.NextTransition.Kind != "collect" {
		reasonCode := ""
		if probe.NextTransition != nil {
			reasonCode = probe.NextTransition.ReasonCode
		}
		return fmt.Errorf("issue #3065 staged recovery was not negotiated: action=%q disposition=%q authority=%v reason=%q", probe.Action, probe.ActionDisposition, probe.Authority, reasonCode)
	}
	const actor, reason = "bench-maintainer", "authorize staged correction scope expansion"
	authorization := "gentle-ai.review-recovery-authorization/v1\npredecessor_lineage=" + issue3065SourceLineage +
		"\npredecessor_revision=" + probe.Authority.Revision + "\ntarget_identity=" + probe.TargetIdentity +
		"\nsuccessor_lineage=" + issue3065SuccessorLineage + "\nactor=" + actor + "\nreason=" + reason
	authorized := append(selectors, "--recovery-successor-lineage", issue3065SuccessorLineage, "--recovery-reason", reason,
		"--recovery-actor", actor, "--recovery-authorization", authorization)
	envelope, err := readStatusFor(r, authorized...)
	if err != nil || envelope.NextTransition.Kind != "execute" || envelope.NextTransition.Execute.Operation != "review.recover" {
		return fmt.Errorf("authorized issue #3065 staged recovery = %+v, %v", envelope.NextTransition, err)
	}
	recovered, err := runPrintedTransition(r, envelope)
	if err != nil || recovered.ExitCode != 0 {
		return fmt.Errorf("issue #3065 staged recovery execution = exit %d err=%v", recovered.ExitCode, err)
	}
	result, err := decodeWaveOperation(recovered, "issue #3065 staged recovery")
	if err != nil || result.LineageID != issue3065SuccessorLineage || result.State != "reviewing" {
		return fmt.Errorf("issue #3065 staged successor = %+v, %v", result, err)
	}
	fresh, err := readStatusFor(r, "--lineage", issue3065SuccessorLineage, "--base-ref", r.sandbox.Scratch["staged-recovery-base"], "--projection", "staged", "--workspace-overlay")
	if err != nil || fresh.Authority.LineageID != issue3065SuccessorLineage || fresh.Authority.State != "reviewing" || fresh.NextTransition.Kind != "collect" {
		return fmt.Errorf("issue #3065 staged successor did not start a fresh review: %+v, %v", fresh, err)
	}
	return nil
}

func writeIssue3065CorrectionCandidate(sandbox *Sandbox) error {
	if err := sandbox.write(filepath.Join(sandbox.Repo, "candidate.go"), "package candidate\n\nfunc value() int {\n\treturn 3\n}\n"); err != nil {
		return err
	}
	return sandbox.git(sandbox.Repo, "add", "candidate.go")
}

func prepareIssue3065CurrentChangeCandidate(sandbox *Sandbox) error {
	base, err := gitOut(sandbox, sandbox.Repo, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	sandbox.Scratch["staged-recovery-base"] = base
	if err := sandbox.write(filepath.Join(sandbox.Repo, "candidate.go"), "package candidate\n\nfunc value() int {\n\treturn 1\n}\n"); err != nil {
		return err
	}
	if err := sandbox.git(sandbox.Repo, "add", "candidate.go"); err != nil {
		return err
	}
	return sandbox.git(sandbox.Repo, "commit", "-qm", "issue #3065 source candidate")
}

func stageIssue3065RecoveryTarget(sandbox *Sandbox) error {
	if err := sandbox.write(filepath.Join(sandbox.Repo, "candidate.go"), "package candidate\n\nfunc value() int {\n\treturn 2\n}\n"); err != nil {
		return err
	}
	if err := sandbox.write(filepath.Join(sandbox.Repo, "migration.sql"), "CREATE TABLE recovered (id INTEGER);\n"); err != nil {
		return err
	}
	if err := sandbox.git(sandbox.Repo, "add", "candidate.go", "migration.sql"); err != nil {
		return err
	}
	return nil
}

func captureIssue3065SuccessorFinding(r *journeyRun) error {
	selectors := []string{"--lineage", issue3065SuccessorLineage, "--base-ref", r.sandbox.Scratch["staged-recovery-base"], "--projection", "staged", "--workspace-overlay"}
	envelope, err := readStatusFor(r, selectors...)
	if err != nil || envelope.NextTransition.Kind != "collect" || len(envelope.NextTransition.Collect.Inputs) != 1 || envelope.NextTransition.Collect.Inputs[0].Name != "reviewer_result" {
		return fmt.Errorf("issue #3065 successor reviewer transition = %+v, %v", envelope.NextTransition, err)
	}
	input := envelope.NextTransition.Collect.Inputs[0]
	payload, err := json.Marshal(map[string]any{
		"subject_hash": input.ArtifactSubject.SubjectHash,
		"inspection":   map[string]any{"status": "completed", "paths": envelope.paths()},
		"findings": []any{map[string]any{
			"location": "candidate.go:3", "severity": "CRITICAL", "claim": "candidate returns the wrong value",
			"proof_refs": []string{"candidate.go:3 changed hunk fails the focused check"}, "evidence_class": "deterministic", "causal_disposition": "introduced",
		}},
		"evidence": []string{"focused differential check failed on the frozen staged successor"},
	})
	if err != nil {
		return err
	}
	path, err := writeScratch(r.sandbox, "issue3065-successor-reviewer.json", payload)
	if err != nil {
		return err
	}
	observation := r.run([]string{"review", "capture-result", "--cwd", r.sandbox.Repo, "--lineage", envelope.argument("lineage"), "--target", envelope.argument("target"), "--expected-revision", envelope.argument("expected-revision"), "--lens", envelope.argument("lens"), "--order", envelope.argument("order"), "--input", path}, true)
	if observation.ExitCode != 0 {
		return fmt.Errorf("capture issue #3065 successor finding: %s", firstLine(observation.Stderr))
	}
	var captured struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal([]byte(observation.Stdout), &captured); err != nil || captured.State != "correction_required" {
		return fmt.Errorf("issue #3065 successor capture = %s", observation.Stdout)
	}
	status, terminal, err := correctionStatusFromLastEventCapture(r, observation)
	if err != nil {
		return err
	}
	if !terminal {
		return fmt.Errorf("issue #3065 successor capture omitted correction status continuation")
	}
	return rememberCorrectionStatusContinuation(r, issue3065SuccessorLineage, status)
}

func captureIssue3065FailedValidationFor(r *journeyRun, lineage string, selectors ...string) error {
	return captureProviderValidatorSlotWithResultFor(r, lineage, false, selectors...)
}

func proveIssue3065RecoveryCollection(r *journeyRun) error {
	statePath, err := storeStatePath(r.sandbox, issue3065SuccessorLineage)
	if err != nil {
		return err
	}
	beforeState, err := os.ReadFile(statePath)
	if err != nil {
		return err
	}
	beforeTree, err := gitOut(r.sandbox, r.sandbox.Repo, "write-tree")
	if err != nil {
		return err
	}
	observation := r.run(productArgsFor(r, "review", "status", "--contract", reviewContract, "--next-transition", "--lineage", issue3065SuccessorLineage, "--base-ref", r.sandbox.Scratch["staged-recovery-base"], "--workspace-overlay"), false)
	if observation.ExitCode != 0 {
		return fmt.Errorf("issue #3065 STATUS failed: %s", firstLine(observation.Stderr))
	}
	var status struct {
		Authority struct {
			State string `json:"state"`
		} `json:"authority"`
		Projection struct {
			Kind       string `json:"kind"`
			Projection string `json:"projection"`
		} `json:"projection"`
		NextTransition struct {
			Kind       string `json:"kind"`
			ReasonCode string `json:"reason_code"`
			Collect    struct {
				Inputs []struct {
					Name             string `json:"name"`
					CaptureOperation string `json:"capture_operation"`
				} `json:"inputs"`
			} `json:"collect"`
		} `json:"next_transition"`
	}
	if err := json.Unmarshal([]byte(observation.Stdout), &status); err != nil {
		return err
	}
	if status.Authority.State != "escalated" || status.Projection.Kind != "base-workspace-overlay" || status.Projection.Projection != "workspace" ||
		status.NextTransition.Kind != "collect" || status.NextTransition.ReasonCode != "recovery_target_unrepresentable" || len(status.NextTransition.Collect.Inputs) != 1 ||
		status.NextTransition.Collect.Inputs[0].Name != "recovery_target_selector" || status.NextTransition.Collect.Inputs[0].CaptureOperation != "external.select_recovery_target" {
		return fmt.Errorf("issue #3065 default workspace-overlay status = %+v", status)
	}
	afterState, err := os.ReadFile(statePath)
	if err != nil {
		return err
	}
	afterTree, err := gitOut(r.sandbox, r.sandbox.Repo, "write-tree")
	if err != nil {
		return err
	}
	if !bytes.Equal(beforeState, afterState) || beforeTree != afterTree {
		return fmt.Errorf("issue #3065 STATUS mutated predecessor state or index: state_equal=%t tree_before=%q tree_after=%q", bytes.Equal(beforeState, afterState), beforeTree, afterTree)
	}
	return nil
}
