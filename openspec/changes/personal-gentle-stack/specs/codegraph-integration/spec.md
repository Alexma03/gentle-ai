# CodeGraph


## Requirements

### Requirement: Managed CodeGraph
System MUST manage CodeGraph lifecycle, paths, backups, and Pi sync cross-platform.

#### Scenario: Enabled
- GIVEN CodeGraph is enabled
- WHEN install or backup runs
- THEN paths enter the manifest and Pi divergence is reported.

#### Scenario: Unavailable
- GIVEN CodeGraph is unavailable
- WHEN doctor runs
- THEN degradation is reported; clients are unchanged.
