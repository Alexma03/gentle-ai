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
PASS — 1 package; 16 passing test cases (including subtests).

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

## Known Cross-Slice Failures / Blockers

The full `go test ./internal/components/communitytool -count=1` package is **not green** after the exact-five registry and CodeGraph compatibility contract. It still contains stale tests for OpenCode, Gemini CLI, Kiro, Hermes, and other retired/default-registry identities, including `TestCodeGraphGuidanceInjectsForRepresentativeAgents`, three OpenCode `DetectStatus` tests, three OpenCode reconciliation tests, and targeted OpenCode/Gemini install assertions. These failures are deferred to PR2 task 2.5 and related retirement updates; this PR does not reintroduce legacy runtime behavior or expand into retirement cleanup.

The full root suite is therefore a known cross-slice blocker, not evidence that PR1's focused foundation behavior is complete at repository scope. Parent verification must preserve this distinction.

## Commits

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

## Rollback Boundary

The PR1 foundation is reversible as a contiguous commit set in reverse order. The migration runtime boundary is independently reversible through `state.RestoreMigration` / `state.RestoreMigrationFromDisk`; validation rejects untrusted backup IDs, authorization roots, manifest targets, symlink-parent escapes, duplicate targets, and snapshot traversal before mutation. Restore stages and validates all snapshots first, then applies atomic file replacements with best-effort transaction rollback on an apply failure.

## Next Steps

Parent should independently run native SDD verification and settle the existing attempt. PR2 should address the recorded legacy communitytool test/cleanup boundary in task 2.5 and continue with retained-client work. Phase 2 and later task checkboxes remain unchecked.
