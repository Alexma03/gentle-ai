## Exploration: Personal Gentle Stack — Gentle AI lane

### Current State

Gentle AI is currently a product-wide installer and lifecycle manager rather than a small shared core. Its supported-agent catalog, component set, persistence schema, install/sync paths, TUI, assets, documentation, and tests all encode a broad public distribution. The requested personal stack narrows that distribution to five clients—Claude Code, Codex, Cursor, Google Antigravity, and Pi—and retains Engram while promoting CodeGraph to a first-class integration.

The current agent set is duplicated rather than governed by one authoritative registry:

| Surface | Current coupling |
| --- | --- |
| `internal/model/types.go` | Declares 16 `AgentID` constants. |
| `internal/catalog/agents.go` | Declares the same 16 agents for selection, labels, and validation; also retains a two-agent MVP registry for compatibility. |
| `internal/agents/factory.go` | Repeats the 16-agent default list and an exhaustive adapter-construction switch. |
| `internal/agents/capabilitymanifest/manifest.go` and `internal/agents/researchcapability/` | Maintain separate closed per-agent capability maps. |
| `internal/system/config_scan.go` | Repeats all 16 configuration paths to avoid an import cycle. |
| `internal/tui/model.go` | Repeats detection and client-specific flows in a 5,000+ line orchestration surface. |

The confirmed retained clients are `claude`, `codex`, `cursor`, `antigravity`, and `pi`. The confirmed retired clients are `opencode`, `kilocode`, `gemini-cli`, `vscode-copilot`, `windsurf`, `kimi`, `qwen-code`, `kiro-ide`, `openclaw`, `trae`, and `hermes`. Direct retired-agent constants occur in 147 tracked files, so the change is not an adapter-directory deletion alone. OpenCode in particular is intertwined with review transport, model profiles, background policy, themes, plugins, branding, update metadata, and Pi compatibility.

The component model is similarly product-wide. `internal/model/types.go`, `internal/catalog/components.go`, and `internal/model/presets.go` expose Engram, SDD, Skills, Context7, Persona, Permissions, GGA, multiple theme variants, and an OpenCode logo. Presets combine these into Minimal, Ecosystem, Full, and Custom matrices. `internal/planner/graph.go` makes SDD depend on Engram and Skills depend on SDD, while Persona receives ordering rules because several adapters replace whole files. The requested removals—GGA, themes, logos, branding, and community marketplace/plugins—therefore affect planning, component application, sync, uninstall, update, backup manifests, TUI selection, docs, and golden tests.

CodeGraph is currently hidden behind a generic abstraction that has only one implementation:

- `model.CommunityToolID` has one value: CodeGraph.
- `internal/components/communitytool.Definitions()` returns one definition.
- `internal/components/communitytool/` owns CodeGraph installation, version detection, managed paths, guidance, backup/restore integration, and Pi reconciliation.
- Install, sync, uninstall, state, and TUI all refer to a generic `CommunityTools` selection even though no second community tool exists.
- For the retained clients, CodeGraph is native for Claude Code, Cursor, Codex, and Antigravity, and reconciled for Pi. This is already a coherent first-class compatibility boundary.

The operational lifecycle is valuable but carries the broad catalog through every stage. Install and sync normalize a selection, build the planner graph, snapshot managed paths, apply per-agent and per-component steps, verify results, roll back on failure, and persist state. Doctor checks installed-agent prerequisites, state, asset versions, Engram reachability, and disk. Uninstall is adapter- and component-driven and creates backups. Update tracks Gentle AI, Engram, GGA, and OpenCode community plugins. The backup subsystem is root-constrained and backward-compatible. These transactional properties are core safety mechanisms and should be retained while their inventories are reduced.

The persisted state is a migration boundary, not disposable cache. It records installed agents, selected components and skills, preset, community tools, persona state, SDD and strict-TDD settings, per-client model routing, OpenCode profiles/plugins, Pi background intent, update state, and RDD mode. Current adapter resolution can silently skip unsupported persisted agent IDs. If the catalog is reduced without an explicit state migration, retired selections can disappear without a user-visible explanation and rollback metadata can become incomplete.

Assets amplify the physical-deletion risk. `internal/assets/assets.go` names every embedded directory in one `//go:embed` directive, so deleting a directory before changing the directive breaks compilation. Retired clients own adapter-specific SDD, persona, skill, subagent, and configuration assets; tests and goldens mirror those paths. Antigravity is a special case: it uses the shared `.gemini` configuration root and `GEMINI.md` conventions while retaining distinct Antigravity assets and adapter behavior. Removing Gemini CLI must not remove shared filesystem behavior that Antigravity still requires.

The repository also contains 687 root-module test files and a separate `bench/` Go module with 19 test files. Client and component matrices are asserted through unit tests, integration tests, capability tests, asset tests, and goldens. Historical OpenSpec archives document earlier public behavior and should remain immutable; current docs and generated/reference maps should describe the personal fork instead.

#### Gentle Pi coupling

Gentle Pi is not merely a consumer of stable APIs. It is tightly versioned to and partially mirrors Gentle AI:

| Coupling lane | Evidence and consequence |
| --- | --- |
| Binary distribution | `scripts/gentle-ai-installer.mjs` pins an exact Gentle AI version and checksums; `lib/gentle-ai-binary.ts` enforces binary resolution and integrity. Any CLI change requires a coordinated release and pin update. |
| Review transport | `lib/native-review-cli.ts`, `extensions/gentle-ai.ts`, and generated runtime modules invoke the native review lifecycle and decode exact schemas. The environment handshake and ordered command tokens are contract-critical. |
| Mirrored contracts | `contracts/review-integration/v1/`, `contracts/review-integration/v2/`, provider-contract mirrors, generator scripts, and parity tests duplicate published Gentle AI schemas. |
| SDD status and assets | `lib/sdd-status.ts` implements a local `gentle-pi.sdd-status/v1` resolver instead of delegating to Gentle AI's native dispatcher. `lib/sdd-preflight.ts`, `assets/agents/`, `assets/chains/`, `assets/support/`, and `assets/migrations/managed-assets-v0.13.json` install another managed SDD surface. |
| Model/persona integration | The Pi extension owns UI, model selection, persona projection, startup guardrails, and SDD hooks, while Gentle AI also persists Pi model/background/persona state. |
| Packaging and presentation | Pi publishes themes, prompts, skills, and logo metadata, so the branding/theme removal crosses the repository boundary. |

The desired boundary is therefore narrower than the current implementation: Gentle AI should own canonical state, dependency rules, schemas, Engram/CodeGraph integration, safe lifecycle operations, and provider-neutral review/SDD behavior. Gentle Pi should own Pi tool registration, Pi UI, host transport, and projection of core outputs into Pi. Pi should not independently decide the SDD dependency graph or duplicate review state machines. Orca should remain the owner of Linear, worktree creation/management, and external isolation; Gentle AI may still canonicalize and validate Git worktree identity where that is required for review safety, but it should not become a worktree orchestrator. No first-class Linear integration was found in the product path; the only Linear MCP fixture verifies that Engram injection preserves unrelated user configuration.

The inspected snapshots were Gentle AI `b8d15e8f` on `custom/main` and Gentle Pi `abb2c929` on `custom/main`. Gentle AI is 9 commits ahead and 24 commits behind its local `upstream/main`; Gentle Pi is 10 commits ahead and not behind its local upstream. A large destructive edit now would intersect an already-moving upstream surface.

### Affected Areas

- **Canonical catalog and capability boundary:** `internal/model/types.go`, `internal/catalog/agents.go`, `internal/agents/factory.go`, `internal/agents/registry.go`, `internal/agents/capabilitymanifest/`, `internal/agents/researchcapability/`, and `internal/system/config_scan.go`.
- **Retained thin adapters:** `internal/agents/claude/`, `internal/agents/codex/`, `internal/agents/cursor/`, `internal/agents/antigravity/`, and `internal/agents/pi/`. Each should expose client paths and projection mechanics, not own product policy.
- **Retired adapters:** `internal/agents/opencode/`, `kilocode/`, `gemini/`, `vscode/`, `windsurf/`, `kimi/`, `qwen/`, `kiro/`, `openclaw/`, `trae/`, and `hermes/`, plus their manifests, capability claims, tests, and fixtures.
- **Component and preset model:** `internal/catalog/components.go`, `internal/model/presets.go`, `internal/model/selection.go`, and `internal/planner/`. GGA, themes, logo, and OpenCode plugin components are confirmed removals; the preset matrix must be simplified only after the desired personal component profile is explicit.
- **First-class CodeGraph integration:** promote the behavior now in `internal/components/communitytool/` into an explicit CodeGraph component/service, then replace `CommunityToolID`, generic definitions, TUI picker, state field, and generic CLI routing. Preserve native support for four retained clients and Pi reconciliation.
- **Core retained components:** `internal/components/engram/`, with SDD, Skills, and Persona retained only to the extent that their assets and routing serve the five-client personal flow.
- **Confirmed component removals:** `internal/components/gga/`, `internal/components/theme/`, `internal/components/opencodeplugin/`, their release/update descriptors, state, backup paths, TUI flows, assets, and tests.
- **Undecided generic components:** Context7 and Permissions are not covered by a confirmed decision. Keep them outside the destructive scope until their value is assessed against the personal workflow.
- **Lifecycle commands:** `internal/cli/run.go`, `install.go`, `sync.go`, `doctor.go`, `uninstall.go`, `update.go`, `backup.go`, validation, detection, and managed-path calculation. Preserve transaction, verification, rollback, and path-containment semantics while reducing dispatch tables.
- **State and migrations:** `internal/state/`, legacy persona aliases, OpenCode model/profile fields, community-tool selection, compatibility-skill refresh, and legacy managed-asset cleanup. Introduce explicit retirement and CodeGraph migration before deleting old readers.
- **Assets:** `internal/assets/assets.go` and client/component asset directories. Update the embed declaration atomically with physical deletion; protect Antigravity's shared `.gemini` semantics.
- **TUI:** `internal/tui/`, especially agent/component/community-tool/plugin/model/background/update/uninstall flows. Its future scope is a separate product decision: simplify to a fixed personal dashboard or remove in favor of declarative CLI use.
- **Documentation and presentation:** `README.md`, `docs/architecture.md`, `docs/components.md`, `docs/usage.md`, `docs/codebase/`, current release/installation guidance, `TRADEMARKS.md`, brand images, TUI logo, and community-oriented roadmap/PRD material. Do not rewrite archived OpenSpec history.
- **Verification:** root tests and goldens, the separate `bench/` module, installer/sync/uninstall integration tests, capability exhaustiveness, asset embedding, state migration, and cross-repository contract/parity tests.
- **Gentle Pi companion:** `scripts/gentle-ai-installer.mjs`, `lib/gentle-ai-binary.ts`, `lib/native-review-cli.ts`, `lib/sdd-status.ts`, `lib/sdd-preflight.ts`, `extensions/gentle-ai.ts`, `contracts/`, generated `runtime/` modules, managed assets/migrations, package metadata, and their tests.

Other generic surfaces are plausible removal candidates but lack enough product evidence for an automatic decision:

1. Context7 and the Permissions overlay.
2. The multi-preset/custom-selection matrix versus one declarative personal profile.
3. `internal/agentbuilder/`, whose generic UI currently recognizes a shrinking set of clients and may overlap the retained Skills workflow.
4. The full Bubble Tea installer/TUI versus a smaller status/doctor surface.
5. Workspace/global installation modes. Orca owns worktree orchestration, but workspace-scoped client configuration may still be useful.
6. Cross-platform packaging and release channels beyond the owner's actual machines.
7. The separate bench, RDD, and native review lifecycle. They are large, but current evidence shows they provide integrity and immutable-candidate safety rather than generic marketplace breadth.
8. Root JavaScript package metadata if no release or CI consumer remains after the fork is narrowed.

### Approaches

1. **Physical prune first** — Delete retired adapters, components, assets, tests, and docs immediately, then repair compilation and runtime paths.
   - **Pros:** Fastest visible line-count reduction; compiler failures expose some direct dependencies; little temporary compatibility code.
   - **Cons:** Mixes policy, persistence migration, CodeGraph promotion, asset embedding, and Pi contract changes into one failure surface. It can silently discard persisted selections, break `go:embed`, remove Antigravity-shared behavior, invalidate Pi's pinned binary/contracts, weaken rollback manifests, and create a large upstream merge-conflict set.
   - **Effort:** High, with high recovery and integration risk.

2. **Contract the canonical registry, migrate state, then delete unreachable code** — Establish the five-client and retained-component boundary first; add explicit migrations and diagnostics; promote CodeGraph; prove lifecycle behavior; then remove code and assets in bounded slices.
   - **Pros:** Makes the allowlist measurable, preserves rollback, gives old installations an upgrade path, separates product decisions from mechanical deletion, and permits each retired surface to be removed with its tests. It also creates a stable boundary for selective upstream integration.
   - **Cons:** Temporarily carries compatibility readers and deprecated fields; requires deliberate negative tests proving retired clients/components cannot re-enter; total work is split across more reviewable changes.
   - **Effort:** High, but with substantially lower recovery and cross-repository risk.

### Recommendation

Use **Approach 2: registry-first contraction followed by unreachable-code deletion**. The canonical registry should become the single source for client identity, detection metadata, adapter construction, and capability declaration. Compatibility readers may understand retired IDs, but runtime planning must emit an explicit migration/retirement report rather than silently skipping them.

Define the target architecture as four deep boundaries:

1. **Small core:** canonical five-client profile, persisted desired state, dependency planning, safe backup/restore, doctor, update coordination, SDD/review contracts, and lifecycle transactions.
2. **First-class integrations:** Engram and CodeGraph, each behind a focused service that owns installation, detection, projection, managed paths, verification, sync, and uninstall. Do not replace a one-item generic community-tool framework with another generic plugin registry.
3. **Thin client adapters:** path discovery, format translation, and host-specific injection for Claude Code, Codex, Cursor, Antigravity, and Pi. Policy and dependency decisions remain in the core.
4. **External ownership boundary:** Orca owns Linear, worktree lifecycle, and external isolation. Gentle AI retains only the Git/worktree identity checks necessary to bind safe review and SDD state.

Apply the change in reversible sequence:

1. **Freeze and characterize:** record the five-client allowlist, retained operational invariants, current state fixtures, managed paths, and Gentle Pi contract/version baseline. Reconcile the 24 upstream Gentle AI commits first or explicitly freeze a fork base before large deletions.
2. **Introduce the canonical profile and migration:** drive catalog, factory, validation, discovery, and capability maps from one authoritative definition. Add a state migration that reports retired agents/components, preserves rollback metadata, and rejects unsupported new selections explicitly.
3. **Promote CodeGraph before removing `communitytool`:** introduce `ComponentCodeGraph` or an equivalent explicit core integration; migrate legacy `community_tools:["codegraph"]`; preserve version/detection, safe init, guidance, managed paths, backup/restore, doctor visibility, four native retained clients, and Pi reconciliation. Only then delete `CommunityToolID`, generic definitions, picker, and runner plumbing.
4. **Remove confirmed product surfaces:** remove GGA, themes, logos, branding, marketplace/community plugins, and their update/state/TUI/docs/test paths. Keep current product docs accurate while leaving archived OpenSpec changes immutable.
5. **Retire clients in coherent slices:** update registry/capabilities, adapter, config scan, lifecycle dispatch, assets/embed declaration, fixtures/goldens, docs, and migrations together for each slice. Treat Gemini CLI removal and Antigravity preservation as an explicit shared-path test case. Remove OpenCode last among retired clients because it has the widest core and Pi coupling.
6. **Thin the retained adapters:** move SDD/model/persona decisions into core services; keep only host projection in each adapter. For Pi, make native Gentle AI status/contracts authoritative and retire duplicated Pi resolution logic only after parity and migration tests pass.
7. **Coordinate the Gentle Pi release:** publish a Gentle AI binary/schema version, update the exact Pi pin/checksums/SumDB source, regenerate runtime modules, refresh mirrored contracts, update managed-asset migrations, and verify packed-package, review, SDD, model/persona, and CodeGraph reconciliation paths.
8. **Decide optional generic surfaces separately:** collect evidence for Context7, Permissions, presets, Agent Builder, TUI, install scopes, platform/release support, and bench/RDD. Their removal must not be smuggled into the confirmed pruning scope.
9. **Add dead-surface ratchets:** tests should fail if a retired agent/component returns to catalogs, assets, embeds, help text, updater registries, persisted new state, or Pi package metadata. Selectively port upstream fixes through the new core/adapter boundary instead of repeatedly merging product-breadth changes.

This sequence preserves a checkpoint after every semantic boundary. Destructive deletion begins only after the replacement registry, state migration, and CodeGraph integration are working, so a rollback can restore both files and interpretable state.

### Risks

- **Silent state loss:** current adapter resolution skips unknown IDs. A catalog-only contraction can appear successful while dropping installed-client intent and managed paths.
- **CodeGraph regression:** deleting `communitytool` before first-class migration removes installation, version checks, guidance, backup, uninstall, and Pi reconciliation together.
- **Antigravity collateral damage:** Gemini CLI and Antigravity share `.gemini` conventions; directory-name deletion is not a safe ownership rule.
- **Gentle Pi protocol breakage:** Pi pins the binary and mirrors strict review/SDD schemas. CLI, contract, or asset changes require an atomic cross-repository release plan.
- **False thinness:** merely moving duplicated logic into another adapter package leaves core policy replicated. Thinness must be enforced by ownership tests and one canonical status/dependency implementation.
- **Embed and golden failures:** physical asset deletion must be atomic with `go:embed`, managed-path, fixture, and golden updates.
- **Rollback incompleteness:** removing legacy readers or paths too early can make pre-prune backups impossible to interpret or restore safely.
- **Upstream integration cost:** Gentle AI is already behind upstream. A broad deletion branch will turn routine upstream merges into repeated delete/modify conflicts; the narrowed core boundary should become the explicit cherry-pick filter.
- **Over-pruning safety infrastructure:** backup, doctor, RDD/review binding, worktree identity checks, and the bench may look generic by size but currently enforce safety invariants. Remove them only with separate evidence and an accepted replacement or risk decision.
- **Historical rewriting:** deleting archived OpenSpec evidence would erase rationale without reducing runtime complexity. Limit documentation cleanup to active/current surfaces.
- **Unconfirmed personal constraints:** platform, TUI, install scope, Context7, Permissions, Agent Builder, and release-channel choices remain undecided and must not be inferred.

### Ready for Proposal

Yes for the confirmed boundary. A proposal can now specify the five-client allowlist, confirmed component removals, first-class CodeGraph migration, explicit state upgrade behavior, retained lifecycle safety, thin-adapter ownership, Orca boundary, and coordinated Gentle Pi release.

The proposal should treat Context7, Permissions, the preset/custom-selection matrix, Agent Builder, TUI depth, install scopes, platform/release breadth, and bench/RDD lifecycle as explicit decision gates or out of scope. It should not make those choices implicitly. The first implementation milestone should end after canonical registry/state migration and CodeGraph promotion; physical pruning should be a later milestone with independent rollback and verification evidence.
