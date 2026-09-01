# Tasks: Personal Gentle Stack

## Review Workload Forecast
Complexity high: registry, state, lifecycle, assets. Boundaries: foundation, clients, cohorts. Risk high: rollback, OS safety, assets, modules. Delivery: ask-on-risk. Chain strategy: feature-branch-chain.

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain

### Suggested Work Units
- U1/PR1 Foundation; proof `go test ./internal/agents ./internal/state ./internal/components/codegraph`; N/A fakes; rollback foundation.
Bases: PR1 base=feature/tracker branch; PR2 base=PR1 branch; PR3 base=PR2; PR4 base=PR3.
- U2/PR2 Clients/Pi; proof `go test ./internal/agents ./internal/catalog ./internal/components/...`; N/A package fakes; rollback adapters.
- U3/PR3 Retirement cohorts; proof root `go test ./...`; bench `go test ./...`; journeys; rollback cohort.
- U4/PR4 Contracts/release; proof root format/vet/test plus bench build/vet/test; N/A parity; rollback generated contracts/pin pair.

## Phase 1: Foundation
- [x] 1.1 RED repo-selector cases: equivalent roots, non-repo, escape, symlink ambiguity (`internal/cli`, CodeGraph; D:none; P:failures).
- [x] 1.2 Implement canonical roots, managed paths, backup/Pi sync (`internal/components/codegraph/`, `internal/cli/codegraph.go`; D:1.1; P:tests).
- [x] 1.3 RED commit cases: staged, `commit -a`, empty-index review (`internal/cli/review_*_test.go`; D:none; P:failures).
- [x] 1.4 Preserve commit semantics in candidate capture/selection (review CLI; D:1.3; P:tests).
- [x] 1.5 Add locked `MigrationReport`, mapping, unresolved gate, restore (`internal/state/migrate.go`, `state.go`, `internal/model/`; D:1.2; P:migration/restore).
- [x] 1.6 Expand five-client registry/consumers (`internal/agents/{registry,factory,interface}.go`, catalog/system; D:1.5; P:catalog tests).
- [x] 1.7 Verify CodeGraph enabled/unavailable doctor and parity (CodeGraph/Pi tests; D:1.2; P:focused tests).

## Phase 2: Retained Clients
- [x] 2.1 Preserve Antigravity `.gemini/GEMINI.md`; remove Gemini CLI selection (adapter/model/catalog tests; D:1.6; P:tests).
- [x] 2.2 Thin Claude/Codex/Cursor/Antigravity/Pi projections (adapter files; D:1.6; P:adapter tests).
- [x] 2.3 RED Pi package/RPC acceptance and retired identity rejection (`internal/agents/pi/`; D:2.2; P:failures).
- [x] 2.4 Implement only `npm:pi-subagents`, versioned RPC checks, no fallback (Pi/manifests; D:2.3; P:package tests).
- [x] 2.5 Remove old generic CodeGraph framework after parity (`internal/components/communitytool/`; D:1.7; P:ratchets).

## Phase 3: Retirement Cohorts
- [ ] 3.1 Remove cohort A adapters/tests: Hermes, KiloCode, Kimi (`internal/agents/`; D:2.5; P:IDs rejected).
- [ ] 3.2 Remove cohort B adapters/tests: Kiro, OpenClaw, Qwen, Trae, VS Code, Windsurf (`internal/agents/`; D:3.1; P:rejected).
- [ ] 3.3 Remove OpenCode last (`internal/agents/opencode`, `internal/opencode`; D:3.2; P:no refs).
- [ ] 3.4 Remove GGA, Windows shim, install/update paths (`internal/components/gga`, `internal/assets/gga`; D:3.1; P:ratchets).
- [ ] 3.5 Remove themes, logos, branding (`internal/components/theme`, `internal/assets/`; D:3.4; P:asset scan).
- [ ] 3.6 Remove marketplace/plugins and Tintinweb references (catalog/update/docs; D:3.5; P:scan).
- [ ] 3.7 Remove stale embed manifests/indexes (`internal/assets/`; D:3.6; P:embed tests).
- [ ] 3.8 Remove retired updater/release metadata (`internal/update/upgrade/`, release files; D:3.7; P:updater tests).
- [ ] 3.9 Update docs, fixtures, goldens, bench journeys (`README.md`, `docs/`, `bench/`; D:3.8; P:root/bench tests).

## Phase 4: Contracts and Release
- [ ] 4.1 Update five retained SDD templates/assets; remove task authority/budget (`internal/assets/`, SDD tests; D:2.2; P:parity).
- [ ] 4.2 Preserve one immutable final-candidate correction transaction (`internal/cli/review*`, SDD tests; D:1.4; P:review tests).
- [ ] 4.3 Pin Gentle Pi binary and checksums/SumDB (companion manifests; D:2.4; P:pin verification).
- [ ] 4.4 Update generated Pi runtime/contracts/assets/package metadata and CodeGraph reconciliation (companion; D:4.3; P:parity/packed).
- [ ] 4.5 Cross-platform containment; root `go run ./internal/gofmtcheck`, `go vet ./...`, `go test ./...`; bench `go build ./...`, `go vet ./...`, `go test ./...` [D:all; P:results].
