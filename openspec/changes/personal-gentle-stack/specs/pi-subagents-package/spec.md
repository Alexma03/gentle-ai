# Pi


## Requirements

### Requirement: Canonical package
Pi MUST install only `npm:pi-subagents`, reject j0k3r/Tintinweb dependencies, and verify versioned RPC capabilities.

#### Scenario: Supported
- GIVEN Pi is selected
- WHEN installation verifies
- THEN only the package is installed and capabilities reported.

#### Scenario: Retired
- GIVEN a retired package reference
- WHEN resolution runs
- THEN it is rejected and remains uninstalled.
