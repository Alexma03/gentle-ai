# Profile


## Requirements

### Requirement: Five-client registry
The registry MUST expose only Claude Code, Codex, Cursor, Google Antigravity, and Pi on Linux, macOS, and Windows; retired IDs MUST NOT be selectable.

#### Scenario: Catalog
- GIVEN any OS
- WHEN the catalog renders
- THEN five clients appear; retired IDs are rejected.

### Requirement: Thin adapters
Policy, state, dependencies, and safety MUST be neutral; adapters MAY project client configuration/capabilities.

#### Scenario: Projection
- GIVEN a retained client
- WHEN an install plan is built
- THEN policy is identical; only projection varies.
