# GGA

## REMOVED Requirements

### Requirement: PowerShell Shim Asset
(Reason: GGA is retired.) (Migration: retain through backup.)

#### Scenario: Removed
- GIVEN GGA retired; WHEN assets reconcile; THEN shim absent.

### Requirement: Windows Install Step
(Reason: GGA is not installable.) (Migration: restore backup on rollback.)

#### Scenario: Removed
- GIVEN Windows; WHEN catalog installs; THEN no GGA step runs.

### Requirement: Non-Windows Systems Unaffected
(Reason: GGA lifecycle is removed everywhere.) (Migration: None.)

#### Scenario: Removed
- GIVEN any OS; WHEN lifecycle runs; THEN GGA unavailable.
