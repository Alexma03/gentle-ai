# Lifecycle


## Requirements

### Requirement: Transactional operations
Lifecycle operations MUST be transactional, preserve manifests, and roll back mutations. Complex candidates MUST retain user-owned immutable review without task authority.

#### Scenario: Failure
- GIVEN multiple managed-path mutations
- WHEN one fails
- THEN prior state and manifest are restored.

#### Scenario: Final candidate
- GIVEN a complex final candidate and enabled review
- WHEN review runs
- THEN one immutable candidate transaction runs.
