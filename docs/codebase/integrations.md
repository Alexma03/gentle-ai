# Integrations

[Back to Codebase Guide](../CODEBASE-GUIDE.md)

Gentle-AI integration code should stay thin: adapters describe where and how an agent accepts configuration; components decide what managed content to inject.

## Agent integration map

| Integration area | Source owner | Purpose |
|---|---|---|
| Agent IDs and config roots | `internal/model/types.go`, `internal/catalog/agents.go` | Declare supported agent names and roots. |
| Adapter strategies | `internal/agents/<agent>/` | Return path, MCP strategy, prompt strategy, and capabilities. |
| SDD assets | `internal/assets/<agent>/`, `internal/components/sdd/` | Install orchestrators, sub-agent prompts, and commands. |
| Engram MCP | `internal/components/engram/` | Add external Engram MCP server entries. |
| Context7 MCP | `internal/components/mcp/` | Add documentation MCP server entries. |
| Skills | `internal/components/skills/`, `internal/assets/skills/` | Copy curated skill files. |
| Skill registry | `internal/skillregistry/`, `internal/app/` | Refresh or list `.atl/skill-registry.md` entries. |
| Community tools | `internal/components/communitytool/` | Orchestrate community tool installation plus managed guidance/config/MCP reconciliation; do not own external runtime implementation. |

## Setup boundaries

| Boundary | Rule |
|---|---|
| Agent discovery | Detect config roots or binaries through system/adapters; do not hard-code in UI screens. |
| MCP wiring | Use adapter MCP strategy instead of custom JSON writes in feature code. |
| Prompt injection | Use component/filemerge helpers to preserve user content when strategy requires it. |
| Community tool orchestration | Keep install commands, generated guidance/config, and MCP reconciliation thin and traceable to the selected external tool. |

## Thin plugin principle

```text
TUI selection
  -> append external package name or managed local plugin path to plugin array
```

## Contributor checklist

- [ ] Add or update an adapter before adding special cases to components.
- [ ] Keep component behavior reusable across agents.
- [ ] Add golden tests when generated config changes.
- [ ] Update [Agents](../agents.md) for user-visible agent capabilities.
- [ ] Keep optional community integrations thin and reversible.

## Navigation

Previous: [Dashboard](dashboard.md) | Next: [Maintainer playbook](maintainer-playbook.md)
