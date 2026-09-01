# Proposal: Personal Gentle Stack

## Intent

Narrow Gentle AI to Claude Code, Codex, Cursor, Google Antigravity, and Pi while preserving cross-platform safety.

## Scope

### In Scope
- Establish one five-client registry with thin adapters.
- Retain Engram and Context7; promote CodeGraph before removing generic community-tool/plugin plumbing.
- Remove retired clients, GGA, themes, logos, branding, marketplaces/plugins, and unreachable UI/docs/tests.
- Explicitly migrate retired persisted selections.
- Install only `npm:pi-subagents` for Pi; remove `pi-subagents-j0k3r` and all Tintinweb dependencies, adapters, fallbacks, and docs.
- Preserve transactional install/update/doctor/uninstall/backup and complex-final-candidate RDD; reduce redundant task-level authority contracts.

### Out of Scope
- Dropping Linux, macOS, or Windows.
- Rewriting archived OpenSpec history.
- Absorbing Orca-owned Linear, task-worktree, or repository-isolation behavior.

## Capabilities

### New Capabilities
- `personal-client-profile`: Authoritative client registry and adapter boundary.
- `persisted-selection-retirement`: Reported, reversible retirement migration.
- `codegraph-integration`: First-class lifecycle, managed paths, backup, and Pi reconciliation.
- `personal-lifecycle-safety`: Transactional reduced-inventory operations.
- `pi-subagents-package`: Canonical package installation and advertised RPC capabilities.

### Modified Capabilities
- `antigravity-support`: Preserve shared `.gemini` conventions without Gemini CLI.
- `gga`: Remove all requirements and lifecycle surfaces.
- `sdd-orchestrator-assets`: Project retained clients and reduced task-level contracts.
- `review-findings-ledger`: Preserve immutable final-candidate review without excessive per-task ceremony.

## Approach

Contract the registry, add migrations, and promote CodeGraph first. Prove lifecycle parity, then prune unreachable code/assets in reversible slices. Core owns policy, state, dependencies, and safety; adapters own client projection. Coordinate schema, binary, package, and generated-contract releases with Gentle Pi.

## Affected Areas

| Area | Impact |
|---|---|
| `internal/{model,catalog,agents,system}` | Registry, capabilities, adapters |
| `internal/{components,planner,state,cli}` | Integrations, migration, lifecycle |
| `internal/{tui,assets,review,sdd}`, `bench/` | UX, contracts, fixtures |
| `docs/`, `README.md`, release metadata | Documentation, packaging |
| Gentle Pi companion | Pins, contracts, runtime, package |

## Risks

| Risk | Mitigation |
|---|---|
| State loss | Migrate before deletion |
| Antigravity/CodeGraph regression | Shared-path parity tests |
| Gentle Pi drift | Coordinated release |

## Rollback Plan

Keep compatibility readers, raw retired selections, and backup manifests through migration. Land registry/migration/CodeGraph before deletion; revert each prune slice independently. Restore Gentle Pi's prior binary pin and package manifest if parity fails.

## Dependencies

- Gentle Pi companion change for pins, contracts, generated runtime, and packaging.
- Versioned [`pi-subagents` RPC](https://github.com/nicobailon/pi-subagents/blob/main/docs/extension-api.md) and [Pi npm package/scoping](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md) contracts.

## Success Criteria

- [ ] Only five retained clients are selectable on all three OSes.
- [ ] Legacy migration is explicit and reversible.
- [ ] Engram, Context7, CodeGraph, and transactional lifecycle checks pass.
- [ ] Pi installs only `npm:pi-subagents` and verifies required RPC capabilities.
- [ ] Retired/Tintinweb surfaces are absent; Gentle Pi parity and final-candidate RDD pass.
