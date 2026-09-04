---
name: issue-creation
description: "Trigger: explicit request to create, edit, search, label, close, or comment on a GitHub issue. Perform only the requested issue operation."
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "3.0"
---

# GitHub Issue Operations

## Activation

Load this skill only when the user explicitly requests a GitHub issue operation. A bug report, implementation task, review, failure, or PR does not by itself authorize issue search or mutation.

## Rules

- Issues are optional tracking artifacts, never prerequisites for implementation, review, pull requests, or delivery.
- Confirm the target repository and requested operation from current context.
- Read-only issue searches are allowed only when the user asked to find or inspect issues.
- Create, edit, label, comment on, close, or reopen an issue only when the user explicitly requested that exact mutation.
- Never infer approval authority from labels, issue state, task text, or model output.
- Sanitize credentials, private paths, hostnames, source contents, and unrelated project details before publishing.
- Perform one bounded mutation, read it back from GitHub, and report the resulting URL and state.
- If authentication, permission, target identity, or mutation outcome is uncertain, stop issue operations without blocking unrelated technical work.

## Output

Return the repository, operation, issue URL or number when known, read-back result, and any issue-specific blocker. Never convert a missing issue into a development blocker.
