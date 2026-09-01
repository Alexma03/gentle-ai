# Retirement


## Requirements

### Requirement: Reversible reported migration
Migration MUST preserve raw selections, backup, mapping reports, and transactional restore; unresolved entries MUST require selection.

#### Scenario: Migration
- GIVEN persisted state contains a retired client
- WHEN migration runs
- THEN raw state, backup, and report remain.

#### Scenario: Restore
- GIVEN a migration backup
- WHEN restore runs
- THEN the prior selection returns unchanged.
