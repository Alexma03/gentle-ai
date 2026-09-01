# Review

## MODIFIED Requirements

### Requirement: One ordinary correction transaction
4R MUST permit one correction. Each unit MUST map frozen IDs, record evidence or N/A, and retain rollback. Tasks MUST NOT create authority or budgets.
(Previously: task-level authority was implicit.)

#### Scenario: Shared budget
- GIVEN three fixes address frozen IDs
- WHEN one correction transaction launches
- THEN the fix counter increases once; no extra budget is created.
