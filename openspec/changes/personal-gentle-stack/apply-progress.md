# Apply Progress: Personal Gentle Stack — PR1 Foundation

## Work Unit Evidence

| Evidence | Required value | Result |
| --- | --- | --- |
| Work unit | U1 / PR1 Foundation | Complete for Phase 1 tasks 1.1–1.7 only. Retained-client thinning, the Nicobailon Pi package, retirement cohorts, release work, tracker updates, and PR delivery remain out of scope. |
| Focused test command and exact result | Smallest commands proving this unit | See the exact verification table below: all listed focused commands exited 0. The primary foundation command passed 3 packages; migration correction tests passed 2 packages; the CLI correction selector passed 2 tests; the full supporting focused package commands passed. |
| Runtime harness command/scenario and exact result | Real integration/runtime path, or N/A with reason | **N/A — no external runtime harness is applicable to this foundation slice.** Install/sync production entry points are exercised by in-process `RunInstall`/`RunSync` tests with temporary homes; client binaries, Pi package installation, and cross-process runtime parity belong to later slices. |
| Rollback boundary | Exact files/behavior independently reversible | Revert this PR1 commit set in reverse order, or use `state.RestoreMigration` / `state.RestoreMigrationFromDisk` to restore the raw state bytes, managed files/symlinks, modes, and migration manifest. No later-slice files or delivery metadata are required for rollback. |

### Scope and constraints

- Confirmed client boundary: Claude Code, Codex, Cursor, Google Antigravity, and Pi on all supported OSes.
- Engram and Context7 remain retained. CodeGraph is a first-class lifecycle component with a compatibility bridge to the legacy community-tool state and Pi behavior.
- State migration now runs from real install/sync entry points under the existing install-state lock before planning and before `RequireMigrationResolved`; unresolved agent, component, and community-tool values remain an explicit selection gate.
- Native SDD attempt acquisition/settlement is parent-owned. This worker did not acquire, replace, or settle an attempt and did not persist Engram memories.

## Completed Tasks

- [x] 1.1 RED selector characterization: canonical equivalent roots (absolute, symlink alias, lexical `..`), non-project roots, restricted home/temp roots, and relative git-root escape rejection.
- [x] 1.2 CodeGraph component facade and lifecycle wiring: install, detect, managed/backup paths, guidance, OpenCode compatibility reconciliation, and Pi reconciliation continue through an explicit `ComponentCodeGraph` path.
- [x] 1.3 RED review candidate characterization: staged projection, `commit -a` committed-only base-diff projection, and an empty-index staged projection.
- [x] 1.4 Existing immutable snapshot/candidate selection semantics are proven to preserve staged, workspace, committed-only, and empty-index bytes without mutating the caller index.
- [x] 1.5 Versioned, reversible state migration: schema versioning, raw-state digest/snapshot, authorized managed-path rollback manifest, CodeGraph mapping, retired/unresolved reporting, locked install/sync integration, explicit selection gate, explicit restore, future-schema refusal, tamper rejection, atomic preflight, and interrupted-migration preservation.
- [x] 1.6 Canonical exact-five client definitions feed the default registry, catalog, config scan, CLI validation/default detection, capability claims, and doctor binary metadata. Legacy adapter/capability constructors remain isolated for migration/rollback inspection only.
- [x] 1.7 Read-only CodeGraph doctor evidence reports unavailable CLI and parity degradation, includes an explicit retained-client-unchanged statement, and covers canonical and legacy enabled-state detection.

## Exact Verification Evidence

All focused commands below were run from `/home/alex/Projects/forks/gentle-ai-worktrees/pr1-foundation` after the correction commits and exited 0:

```text
go test ./internal/agents ./internal/state ./internal/components/codegraph -count=1
PASS — 3 packages: internal/agents, internal/state, internal/components/codegraph.

go test ./internal/cli -run 'TestCanonicalCodeGraphProjectRoot|TestRunCodeGraphInit|TestReviewCandidatePreserves|TestReviewCommittedOnlySelector|TestReviewStagedProjectionPreservesEmptyIndex|TestMigrateInstallState|TestInstallPlanningBlocksUnresolvedStateMigration|TestRunInstallMigratesLegacyStateBeforePlanning|TestRunSyncMigratesLegacyStateBeforePlanning|TestCheckCodeGraph|TestCodeGraphEnabled|TestRunDoctorIncludesCodeGraphParityCheckWhenEnabled' -count=1
PASS — 1 package; 16 top-level tests and 26 tests including subtests.

go test ./internal/state -run 'TestMigrate|TestRequireMigration|TestRestoreMigration|TestResolveSelection' -count=1
PASS — 1 package; 11 passing tests.

go test ./internal/catalog ./internal/system -count=1
PASS — 2 packages: internal/catalog, internal/system.

go test ./internal/agents/capabilitymanifest -count=1
PASS — 1 package.

go test ./internal/components/communitytool -run 'TestCodeGraphCompatibility|TestExcludedAgentsNever|TestCodeGraphNativeOwnedPaths' -count=1
PASS — 1 package; selected CodeGraph compatibility tests passed.

git diff --check codex/personal-gentle-stack-tracker..HEAD
PASS — no whitespace errors.
```

The baseline focused run also passed except the pre-existing environment-dependent `TestEngramPathGuidanceDefault` because `go/bin` was absent. A full CLI package run remains unsuitable as PR1 evidence because it includes legacy identity tests and long external-runtime cases; it was not used to claim a terminal full-suite result.

### Correction RED evidence

Before the production corrections, the newly added regression commands failed as required: the two CLI integration cases returned nil instead of migrating and gating; the state rollback cases accepted an outside manifest path, removed a managed path before rejecting a traversed snapshot, and partially restored an earlier entry; and component/community-tool resolution left the report pending. These failures were observed before `2e535426` and `1fb2ad7b`; the same commands are green in the table above after correction.

## Known Cross-Slice Failures / Blockers

The full `go test ./internal/components/communitytool -count=1` package is **not green** after the exact-five registry and CodeGraph compatibility contract. It still contains stale tests for OpenCode, Gemini CLI, Kiro, Hermes, and other retired/default-registry identities, including `TestCodeGraphGuidanceInjectsForRepresentativeAgents`, three OpenCode `DetectStatus` tests, three OpenCode reconciliation tests, and targeted OpenCode/Gemini install assertions. These failures are deferred to PR2 task 2.5 and related retirement updates; this PR does not reintroduce legacy runtime behavior or expand into retirement cleanup.

The full root suite is therefore a known cross-slice blocker, not evidence that PR1's focused foundation behavior is complete at repository scope. Parent verification must preserve this distinction.

## Commits

This ledger records every implementation and evidence commit through `567569b1`. A later commit that only corrects this ledger's snapshot wording is intentionally outside the recorded implementation boundary.

- `3a14948e` — `test(codegraph): characterize canonical root equivalence`
- `2eb8763e` — `feat(codegraph): promote lifecycle to a first-class component`
- `023a70ac` — `test(review): characterize commit-state candidates`
- `81a0e080` — `feat(state): add reversible locked migration`
- `88699227` — `fix(state): harden migration schema and rollback`
- `1273e423` — `test(state): cover migration interruption rollback`
- `5f2f48a3` — `feat(registry): centralize retained client definitions`
- `84035d85` — `test(registry): align consumers with retained set`
- `a32abcd2` — `refactor(manifest): isolate legacy capability claims`
- `c524aeef` — `refactor(cli): validate agents through canonical registry`
- `c6564b5d` — `feat(doctor): report CodeGraph parity degradation`
- `347edd78` — `test(doctor): cover enabled CodeGraph parity`
- `967633f0` — `docs(sdd): record PR1 foundation progress`
- `1757e905` — `test(state): characterize migration integration and rollback safety`
- `2e535426` — `fix(state): run migration before install and sync planning`
- `1fb2ad7b` — `fix(state): secure migration restore and selection resolution`
- `8b76266e` — `fix(state): anchor raw migration snapshots`
- `0ff03225` — `docs(sdd): document migration correction evidence`
- `c0805976` — `docs(sdd): complete PR1 correction commit ledger`
- `441509ed` — `docs(sdd): record correction red evidence`
- `567569b1` — `docs(sdd): include all correction commits`

## Rollback Boundary

The PR1 foundation is reversible as a contiguous commit set in reverse order. The migration runtime boundary is independently reversible through `state.RestoreMigration` / `state.RestoreMigrationFromDisk`; validation rejects untrusted backup IDs, authorization roots, manifest targets, symlink-parent escapes, duplicate targets, and snapshot traversal before mutation. Restore stages and validates all snapshots first, then applies atomic file replacements with best-effort transaction rollback on an apply failure.

## Next Steps

Parent should independently run native SDD verification and settle the existing attempt. PR2 should address the recorded legacy communitytool test/cleanup boundary in task 2.5 and continue with retained-client work. Phase 2 and later task checkboxes remain unchecked.

## PR2 Phase 2: Retained Clients

The PR1 snapshot above is preserved verbatim. The cumulative state below records the completed PR2 Phase 2 slice and supersedes only the historical next-step status in that snapshot.

### Work Unit Evidence

| Evidence | Required value | Result |
| --- | --- | --- |
| Work unit | U2 / PR2 Clients/Pi | Complete for Phase 2 tasks 2.1–2.5 only. Retirement cohorts and contracts/release work remain out of scope. |
| Focused test command and exact result | Smallest commands proving this unit | `go test ./internal/agents/pi ./internal/cli ./internal/components/codegraph ./internal/system -count=1` — PASS; all four packages exited 0. `go run ./internal/gofmtcheck` — PASS. `bash scripts/deadcode-ratchet.sh` — PASS (`no new unreachable functions`). `go test ./... -count=1` — PASS; root suite exited 0. |
| Runtime harness command/scenario and exact result | Real integration/runtime path | The production `agentInstallStep.Run` path now probes a real Pi `--mode rpc` process through Pi's documented extension API and passes the captured Nicobailon response through the canonical acceptance boundary. Focused production-caller tests cover absent, malformed, wrong-version, incomplete, and retired responses with no fallback; canonical v1 is accepted. Manual real-Pi probe exited 0. |
| Rollback boundary | Exact files/behavior independently reversible | Revert the PR2 commit set in reverse order to remove retained-client corrections without reverting PR1. The Pi runtime gate is independently reversible by reverting `c8c9cf56`; the dead-code cleanup/baseline update is independently reversible by reverting `21d81ac1`; test-only evidence commits can be reverted independently. State/migration and rollback vocabulary remain unchanged. |

### Completed Tasks

- [x] 2.1 Preserved Antigravity `.gemini/GEMINI.md` behavior and removed Gemini CLI selection from the retained-client surface.
- [x] 2.2 Moved shared policy and CodeGraph ownership into core/component code while keeping adapters as client-specific projections; boundary tests pass.
- [x] 2.3 Added RED evidence and canonical Pi package/RPC acceptance coverage, including retired identity rejection.
- [x] 2.4 Implemented canonical `npm:pi-subagents` installation, Nicobailon v1 readiness validation, and no-fallback behavior.
- [x] 2.5 Removed the residual generic one-item CodeGraph/communitytool wrappers after parity and made the deadcode ratchet pass.

### Commit and RED Evidence

The Phase 2 implementation and evidence commits, in order, are:

- `376d4476` — initial Phase 2 retained-client/Pi work.
- `2db09342` — promoted CodeGraph lifecycle ownership into the component.
- `e435d296` — aligned retained-client lifecycle coverage.
- `805a5017` — `test(pi): pin runtime RPC acceptance red cases`.
- `b31725ff` — `fix(pi): enforce canonical subagents RPC readiness`.
- `5ddc341d` — `refactor(codegraph): own Pi policy and drop generic wrappers`.
- `4e336b82` — `test(clients): replace retired compatibility skips`.
- `eef3733a` — `test(cli): cover Pi RPC install acceptance boundary`.
- `c8c9cf56` — `feat(pi): gate install on canonical subagents RPC`.
- `21d81ac1` — `refactor(deadcode): remove obsolete compatibility shims`.
- `6ac21b4c` — `test(cli): assert Pi install has no RPC fallback`.

RED evidence is visible in the test-only history before each corresponding implementation:

```text
go test ./internal/agents/pi -run 'TestAdapter(AcceptsCanonicalPiSubagentsProvider|RejectsAbsentMalformedWrongVersionIncompleteAndRetiredPiSubagents)$' -count=1
internal/agents/pi/rpc_runtime_test.go:28:36: NewAdapter().AcceptSubagentsRPCResponse undefined
FAIL github.com/gentleman-programming/gentle-ai/v2/internal/agents/pi [build failed]
```

The production-boundary RED test was then added in `eef3733a` before its seam existed:

```text
go test ./internal/cli -run 'TestPiAgentInstall(AcceptsCanonicalSubagentsRPCAtProductionBoundary|RejectsUnvalidatedSubagentsRPCAtProductionBoundary)$' -count=1
internal/cli/run_integration_test.go:102:18: undefined: probePiSubagentsRPC
FAIL github.com/gentleman-programming/gentle-ai/v2/internal/cli [build failed]
```

`c8c9cf56` supplied the production probe/caller; the same production-boundary command is now green. `6ac21b4c` adds assertions that exactly one canonical install command runs and no retired/Tintinweb fallback command appears.

### Independent Audit and Scope Boundary

The terminal independent audit passed at `6ac21b4c`: the actual production caller invokes `AcceptSubagentsRPCResponse`, rejects absent/malformed/wrong-version/incomplete/retired replies, accepts only canonical Nicobailon v1, and has no fallback. The deadcode baseline removes only stale entries for deleted or newly reachable shims; explicit migration/rollback compatibility symbols remain documented and tested. No tasks or apply-progress changes were made during implementation, no attempt was acquired or settled, and no push, PR, or merge was performed.

Phase 3 retirement cohorts and Phase 4 contracts/release tasks remain out of scope and remain unchecked in `tasks.md`.

### Current Next Steps

Parent may proceed to independent SDD verification. Do not infer Phase 3/4 completion from this Phase 2 record.
