# Apply Progress: Personal Gentle Stack — PR1 Foundation

## Work Unit Evidence

- Work unit: U1 / PR1 Foundation (`codex/personal-gentle-stack-pr1-foundation`).
- Scope completed: Phase 1 tasks 1.1–1.7 only. Retained-client thinning, Pi package changes, retirement cohorts, release work, tracker updates, and PR delivery remain out of scope.
- Confirmed runtime boundary: Claude Code, Codex, Cursor, Google Antigravity, and Pi. Engram and Context7 remain retained. CodeGraph is a first-class lifecycle component with a compatibility bridge to the old community-tool implementation.
- Native SDD attempt and settlement are parent-owned; this worker did not acquire or settle an attempt and did not persist Engram memories.

## Completed Tasks

- [x] 1.1 RED selector characterization: canonical equivalent roots (absolute, symlink alias, lexical `..`), non-project roots, restricted home/temp roots, and relative git-root escape rejection.
- [x] 1.2 CodeGraph component facade and lifecycle wiring: install, detect, managed/backup paths, guidance, OpenCode compatibility reconciliation, and Pi reconciliation continue through an explicit `ComponentCodeGraph` path.
- [x] 1.3 RED review candidate characterization: staged projection, `commit -a` committed-only base-diff projection, and an empty-index staged projection.
- [x] 1.4 Existing immutable snapshot/candidate selection semantics are proven to preserve staged, workspace, committed-only, and empty-index bytes without mutating the caller index.
- [x] 1.5 Versioned, reversible state migration: schema versioning, raw-state digest/snapshot, managed-path rollback manifest, CodeGraph mapping, retired/unresolved reporting, selection gate, lock adapter, explicit restore, future-schema refusal, and interrupted-migration preservation.
- [x] 1.6 Canonical exact-five client definitions feed the default registry, catalog, config scan, CLI validation/default detection, capability claims, and doctor binary metadata. Legacy adapter/capability constructors remain isolated for migration/rollback inspection only.
- [x] 1.7 Read-only CodeGraph doctor evidence reports unavailable CLI and parity degradation, includes an explicit retained-client-unchanged statement, and covers canonical and legacy enabled-state detection.

## Exact Verification Evidence

Passing focused commands from this worktree:

```text
go test ./internal/agents ./internal/state ./internal/components/codegraph -count=1
go test ./internal/cli -run 'TestCanonicalCodeGraphProjectRoot|TestRunCodeGraphInit|TestReviewCandidatePreserves|TestReviewCommittedOnlySelector|TestReviewStagedProjectionPreservesEmptyIndex|TestMigrateInstallState|TestInstallPlanningBlocksUnresolvedStateMigration|TestCheckCodeGraph|TestCodeGraphEnabled|TestRunDoctorIncludesCodeGraphParityCheckWhenEnabled' -count=1
go test ./internal/catalog ./internal/system -count=1
go test ./internal/agents/capabilitymanifest -count=1
go test ./internal/components/communitytool -run 'TestCodeGraphCompatibility|TestExcludedAgentsNever|TestCodeGraphNativeOwnedPaths' -count=1
```

All commands above passed. The baseline focused command before implementation also passed except the pre-existing environment-dependent `TestEngramPathGuidanceDefault` (`go/bin` was absent).

## Known Cross-Slice Failures / Blockers

No PR1 implementation blocker remains. The full `internal/components/communitytool` package still contains legacy tests that intentionally exercise OpenCode, Gemini CLI, Kiro, Hermes, and other retired/default-registry identities. They fail after the canonical registry and CodeGraph compatibility table contract to five clients; representative failures include `TestCodeGraphGuidanceInjectsForRepresentativeAgents`, the three OpenCode `DetectStatus` tests, the three OpenCode reconciliation tests, and targeted OpenCode/Gemini install-command assertions. Those tests belong to the later retained-client/CodeGraph cleanup boundary (PR2 task 2.5 and related retirement updates), not this foundation slice. No legacy runtime behavior was reintroduced merely to satisfy them.

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

## Rollback Boundary

Revert the commits above as a contiguous PR1 foundation slice, in reverse order if selectively reverting. Migration rollback is explicit and user-owned through `state.RestoreMigration` / `state.RestoreMigrationFromDisk`; it restores the raw state bytes, managed files/symlinks, modes, and rollback manifest snapshot. No commit was pushed, merged, or used to alter a tracker.

## Next Steps

Parent should independently run the native verification/settlement and decide whether to carry the known legacy communitytool test updates into PR2 task 2.5. Phase 2 and later task checkboxes remain unchecked.
