# Design: Personal Gentle Stack

## Technical Approach

Contract the runtime around one five-client registry before deleting code. Migrate legacy state and promote CodeGraph while the old readers still exist; then prune unreachable clients/components in reversible slices. The core keeps policy, planning, Engram, Context7, transactions, and review/SDD contracts. Claude Code, Codex, Cursor, Antigravity, and Pi adapters expose only detection, paths, capabilities, and projection.

## Architecture Decisions

| Decision | Choice and rationale | Rejected alternative |
|---|---|---|
| Registry | Expand `internal/agents/registry.go` with immutable client descriptors and constructors; all catalog, factory, config-scan, validation, capability, TUI, and lifecycle consumers query it. One deep boundary prevents allowlist drift. | Parallel catalogs/switches; compile-time duplication caused the current 16-client spread. |
| State migration | Add a versioned `internal/state/migrate.go` transaction. Under the existing state lock, snapshot raw `state.json` plus managed paths, write an adjacent migration report containing retired values and backup identity, map `community_tools:["codegraph"]` to `ComponentCodeGraph`, and require user selection for unresolved entries before planning. Compatibility readers remain only until rollback is proven. | Silently dropping unknown IDs or deleting legacy fields first. |
| Integrations | Move `internal/components/communitytool/` behavior to explicit `internal/components/codegraph/`; retain `internal/components/engram/` and Context7 routing. Avoid another one-item plugin abstraction. | Keeping generic community-tool/plugin machinery. |
| Client ownership | Keep five adapters thin. Antigravity alone preserves `~/.gemini/GEMINI.md`; Gemini CLI is absent from selection. Pi installs only Nicobailon's `npm:pi-subagents`, validates versioned RPC capabilities, and rejects j0k3r/Tintinweb identities without fallback or migration installation. | Client-specific policy and compatibility package chains. |

## Data Flow

```text
CLI/TUI -> Registry -> selection validator -> state migration -> planner
                                                     |
                         backup manifest <- transaction -> core integrations
                                                     |
                                      five projection adapters -> verify
```

Failures restore managed paths, raw state, and manifest through existing `pipeline.Orchestrator`, `backup`, atomic writers, and OS-specific path-containment code.

## File Changes

| Path | Action | Purpose |
|---|---|---|
| `internal/agents/{registry.go,factory.go,interface.go}` | Modify | Authoritative descriptors and thin adapter contract. |
| `internal/catalog/`, `internal/system/config_scan.go`, `internal/model/` | Modify/delete | Consume registry; add CodeGraph/state schema; remove retired IDs/components. |
| `internal/state/migrate.go` | Create | Reported reversible migration. |
| `internal/components/codegraph/` | Create/move | First-class lifecycle and Pi reconciliation. |
| `internal/agents/{claude,codex,cursor,antigravity,pi}/` | Modify | Projection only; shared Gemini and canonical Pi package rules. |
| `internal/agents/<retired>/`, `internal/components/{communitytool,gga,theme,opencodeplugin}/` | Delete | Remove retired runtime surfaces after replacement gates pass. |
| `internal/{cli,tui,assets,review,sdd}/`, `docs/`, `README.md` | Modify | Reduced inventory, embeds, guidance, and review contract. |

## Interfaces / Contracts

`Registry` exposes `Definitions()`, `Adapter(id)`, `Validate(ids)`, and capability lookup; mutation stays private. `MigrationReport` records schema versions, raw-state digest/snapshot reference, mapped, retired, unresolved, and rollback manifest. CodeGraph exposes install/detect/managed-paths/backup/verify/sync/doctor/uninstall, including Pi divergence. Doctor reports unavailable/degraded CodeGraph without planning or mutating retained-client state.

Final review maps frozen IDs to paths, records evidence or N/A and rollback, permits at most one applicable refuter, and scopes targeted validation to corroborated findings. Runtime and review selection remain user-owned; tasks create no authority or budgets. RDD stays informational and never governs ordinary delivery.

## Testing Strategy

Root RED tests cover the exact five-client set, migration/restore and interruption, CodeGraph parity, Antigravity shared paths, canonical Pi package/capability rejection, and Linux/macOS/Windows containment. Ratchets scan catalogs, factories, manifests, embeds, help, updater registries, assets, and current docs for retired IDs, GGA/theme/logo/plugin, j0k3r, and Tintinweb. Run `go run ./internal/gofmtcheck`, `go vet ./...`, and `go test ./...`. From `bench/`, run `go build ./...`, `go vet ./...`, and `go test ./...`; journeys assert retired commands and clients cannot reappear. Gentle Pi must pass contract generation/parity, packed-package, SDD/review, and CodeGraph reconciliation tests.

## Threat Matrix

| Boundary | Applicability | Safe/failure behavior and planned RED tests |
|---|---|---|
| Documentation-like paths | N/A | Executable-file classification is unchanged. |
| Git repository selection | Applicable | Canonical CodeGraph/review roots accept equivalent relative/absolute roots and reject non-repositories, escapes, and symlink ambiguity; test each selector. |
| Commit state | Applicable | Immutable review preserves staged, `commit -a`, and empty-index semantics; test each state before pruning templates. |
| Push state | N/A | Delivery automation is unchanged. |
| PR commands | N/A | PR composition is out of scope. |

## Migration / Rollout

1. Land registry, migration, and CodeGraph; prove rollback on all OSes.
2. Remove GGA/themes/logo/plugins, then retired clients in slices; remove OpenCode last. Update `go:embed`, fixtures, goldens, updater, docs, and ratchets atomically per slice.
3. Thin retained adapters and remove redundant task-level authority while preserving one immutable final-candidate correction transaction.
4. Release Gentle AI with schema/capability version; then update Gentle Pi's exact binary pin, checksums/SumDB, mirrored contracts, generated runtime, assets, and package metadata. Publish Gentle Pi only after parity/pack tests. Roll back as the prior binary-plus-Pi release pair.

## Open Questions

None. Permissions, Agent Builder, TUI depth, install scopes, and release breadth remain outside this change.
