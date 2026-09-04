# Supported Agents

← [Back to README](../README.md)

---

## Agent Matrix

| Agent           | ID               | Skills       | MCP | Delegation                       | Output Styles | Slash Commands | Config Path                         |
| --------------- | ---------------- | ------------ | --- | -------------------------------- | ------------- | -------------- | ----------------------------------- |
| Claude Code     | `claude-code`    | Yes          | Yes | Full (Task tool)                 | Yes           | No             | `~/.claude`                         |
| Cursor          | `cursor`         | Yes          | Yes | Full (native subagents)          | No            | No             | `~/.cursor`                         |
| Codex           | `codex`          | Yes          | Yes | Native multi-agent (default; solo fallback) | No            | No             | `~/.codex`                          |
| Antigravity     | `antigravity`    | Yes (native) | Yes | Solo-agent + Mission Control     | No            | No             | `~/.gemini/antigravity`             |
| Pi              | `pi`             | Yes          | Yes | Full (package-managed subagents) | No            | Yes            | `~/.pi`                             |

`gentle-ai install --scope=workspace` is supported across selected agents for agent-scoped files, not only Claude Code. In workspace scope, Gentle AI writes system prompts, skills, SDD agents, and persona files into the current project root when the agent supports project-local configuration. Global-only integrations, such as package installs or settings that the agent only reads from its global config, remain global by design.

---

## Delegation Models

| Model                 | How It Works                                                                                                                                                                                       | Agents                                                                                                    |
| --------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| **Native multi-agent** | The orchestrator delegates through the agent's native collaboration tools when configured and available, with inline execution as a graceful fallback. | Codex |

### Cursor Native Subagents

Cursor uses its built-in `.cursor/agents/` system. `gentle-ai` writes 10 agent files to `~/.cursor/agents/sdd-{phase}.md` — one per SDD phase. Cursor's Agent auto-delegates to the correct subagent based on the `description` field in each file's YAML frontmatter.

- `sdd-explore` and `sdd-verify` run with `readonly: false` so they can inspect the codebase and execute verification commands
- Each subagent gets its own context window (fresh context, no pollution)
- The orchestrator resolves skill paths from the skill registry and passes exact `SKILL.md` files in the invocation message

### Antigravity + Mission Control

Antigravity is an agent-first platform with built-in sub-agents (Browser, Terminal) managed by Mission Control. However, custom sub-agent creation is not yet available. SDD phases run inline, with Mission Control handling automatic delegation to built-in sub-agents when specialized tooling is needed (e.g., Browser for research during `sdd-explore`).

## SDD Mode Support

| Mode | Claude Code | Codex | Cursor | Antigravity | Pi |
| --- | :---: | :---: | :---: | :---: | :---: |
| SDD orchestrator | Yes | Yes | Yes | Yes | Yes |
| Single-mode SDD | Yes | Yes | Yes | Yes | Yes |
| Multi-mode SDD | — | Yes | — | — | Yes\* |

> \* **Pi multi-mode** is owned by the Pi packages. `gentle-pi` installs SDD agent and chain assets into `.pi/agents/` and `.pi/chains/`; model overrides live in those Pi-managed files or chain steps.

---

## Agent Notes

### Claude Code

- Sub-agents via the native Task tool with isolated context windows
- MCP servers configured as plugins in `~/.claude/mcp/`
- Output styles in `~/.claude/output-styles/`
- System prompt via markdown sections in `~/.claude/CLAUDE.md`

### Cursor

- Native subagents via `~/.cursor/agents/sdd-{phase}.md` (10 files installed by gentle-ai)
- Skills at `~/.cursor/skills/`
- System prompt in `~/.cursor/rules/gentle-ai.mdc`
- MCP config in `~/.cursor/mcp.json`

### Codex

- CLI-native agent with TOML config at `~/.codex/config.toml`
- Skills at `~/.codex/skills/`
- System prompt at `~/.codex/AGENTS.md`
- Engram instruction files at `~/.codex/engram-instructions.md`
- MCP servers (Engram and Context7) are upserted as `[mcp_servers.<name>]` blocks in `~/.codex/config.toml`
- SDD model-selection profiles written as separate files at `~/.codex/<name>.config.toml`. GPT-5.6 defaults require Codex >= 0.144.0 (the separate-file mechanism itself is available since 0.134.0). Select a profile at runtime via `codex --profile <name>`:

  Model and effort defaults vary together by preset. These effort levels are Gentle AI workload policy, not Codex defaults. The carriles split by what the phase actually does: `sdd-strong` phases reason over context delivered to them, `sdd-mid` phases write code in an agentic loop where effort matters more than raw model strength, and `sdd-cheap` phases do structured transcription with short context and verifiable output, so they buy effort instead of a bigger model.

  Every curated preset runs the main orchestrator/session at `medium` effort — it plans, routes and adjudicates rather than doing the delegated work. The orchestrator *model* varies: Low-cost runs it on `gpt-5.6-terra` so a Plus plan can still afford `gpt-5.6-sol` in the strong carril, where the reasoning pays. Custom and legacy state preserve existing top-level settings:

  | Profile | Low-cost | Recommended | Powerful | SDD phases |
  |---------|----------|-------------|----------|------------|
  | Orchestrator | `gpt-5.6-terra` / `medium` | `gpt-5.6-sol` / `medium` | `gpt-5.6-sol` / `medium` | main session |
  | `sdd-strong` | `gpt-5.6-sol` / `medium` | `gpt-5.6-sol` / `medium` | `gpt-5.6-sol` / `xhigh` | explore, propose, design, verify, judge |
  | `sdd-mid` | `gpt-5.6-terra` / `medium` | `gpt-5.6-terra` / `high` | `gpt-5.6-sol` / `high` | apply, fix-agent |
  | `sdd-cheap` | `gpt-5.6-luna` / `high` | `gpt-5.6-luna` / `high` | `gpt-5.6-luna` / `high` | spec, tasks, archive, onboard |

- Explicit saved Codex model assignments are preserved on sync, including older pinned IDs such as `gpt-5.5` or `gpt-5.4-mini`. The narrow exception is the exact former implicit-default tuple (`sdd-strong=gpt-5.5`, `sdd-mid=gpt-5.5`, `sdd-cheap=gpt-5.4-mini`), which sync treats as Recommended and upgrades to the current GPT-5.6 tuple; partial, extended, or otherwise different maps remain custom and unchanged.
- GPT-5.6 `max` reasoning effort and `ultra` mode are intentionally not enabled by this default update. `max` requires confirmed Codex support; `ultra` changes orchestration semantics and needs separate design.
- Multi-agent SDD delegation is enabled by default. gentle-ai writes `features.multi_agent = true` and `agents.max_threads = 4` / `agents.max_depth = 2` into `~/.codex/config.toml`; set `multi_agent = false` in the `[features]` section to opt out. The delegated route requires both the enabled setting and Codex's native `spawn_agent`, `wait_agent`, and `list_agents` tools. If the configuration or tools are unavailable, orchestration gracefully falls back to solo-agent inline execution.
- **Delegation**: Native multi-agent by default, with graceful solo-agent fallback

### Antigravity

- Skills at `~/.gemini/antigravity/skills/` (native Antigravity feature)
- MCP config at `~/.gemini/antigravity/mcp_config.json`
- Mission Control handles built-in sub-agent delegation (Browser, Terminal) automatically
- Settings managed via the IDE's Agent settings UI, not via `settings.json`

### Pi

For the full Pi command and package reference, see [Pi Agent](pi.md).

- **Detection**: gentle-ai detects Pi from the `pi` binary on `PATH` and its config root at `~/.pi`.
- **Install**: Pi must already be installed. gentle-ai then installs the full Pi support stack with:
  - `pi install git:github.com/Alexma03/gentle-pi@custom/main`
  - `pi install npm:gentle-engram@0.1.11`
  - `pi install npm:pi-mcp-adapter`
  - `npm exec --yes --package gentle-engram@0.1.11 -- pi-engram init`
  - `pi install npm:pi-subagents@0.65.0`
- **`gentle-pi` package**: adds the Gentleman harness for Pi: SDD/OpenSpec workflow, strict TDD guidance, safety defaults, `/gentle-ai:*` commands, skill assets, prompts, SDD agents, and SDD chains. On normal `session_start`, it copies project assets into `.pi/agents/`, `.pi/chains/`, and `.pi/gentle-ai/support/` without overwriting local files unless the Pi recovery command uses `--force`. Starting Pi with `pi -ns` skips startup skill loading/hooks, so that automatic refresh does not run in that mode.
- **Package metadata**: this fork installs `git:github.com/Alexma03/gentle-pi@custom/main` ([Alexma03/gentle-pi](https://github.com/Alexma03/gentle-pi)), not the npm registry package.
- **Persona command**: `gentle-pi` owns Pi persona switching through `/gentleman:persona` (`/gentle-ai:persona` remains a compatibility alias). It switches between `gentleman` and `neutral`, saves `.pi/gentle-ai/persona.json`, and may require `/reload` or a new Pi session for the active prompt to refresh.
- **Model assignment command**: `gentle-pi` owns Pi model selection through `/gentleman:models` (`/gentle-ai:models` remains a compatibility alias). It opens a Pi-native modal for project, user, and built-in agents, prioritizes SDD agents, saves `.pi/gentle-ai/models.json`, and applies overrides into `.pi/agents/*.md` or `.pi/settings.json`.
- **`gentle-engram` package**: adds persistent Engram memory for Pi. It captures sessions, exposes Engram MCP tools through `pi-mcp-adapter`, and degrades safely when the local `engram` binary is missing.
- **MCP adapter wiring**: ComponentEngram declares `npm:pi-mcp-adapter` in `.pi/agent/settings.json` packages and adds `pi-mcp-adapter` `^2.6.0` to `.pi/npm/package.json` without removing unrelated user entries. `pi-engram init` owns the Pi Engram MCP config schema and is run during installation.
- CLI precedence is flag, non-empty environment, prior managed state, then `auto`; `auto` never enables by itself, unresolved non-interactive `auto` stays foreground, and the interactive Pi installer prompts only when that preference is unresolved.
- The resolved on/off policy is projected to `~/.pi/gentle-ai/background-subagents.json` as `{"schema":"gentle-pi.background-subagents/v1","policy":"on"|"off"}` (the base directory honors `GENTLE_PI_CONFIG_HOME`); `off` rewrites the policy instead of deleting files, and a file at that path without the managed schema marker is never overwritten.
- **`pi-subagents` package**: NicoBailon's runtime owns child execution and exposes the RPC v1 capability surface consumed by `gentle-pi`.
- **Pi-only flow**: when Pi is the only selected agent, gentle-ai skips persona, ecosystem component selection, and Strict TDD prompts because those behaviors are provided by `gentle-pi`.
