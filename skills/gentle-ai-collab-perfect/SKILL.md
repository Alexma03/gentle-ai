---
name: gentle-ai-collab-perfect
description: "Trigger: contributing to Gentleman-Programming/gentle-ai or a fork, preparing a PR, splitting review units, or auditing delivery readiness."
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "3.0"
---

# Gentle AI Collaboration

## Purpose

Prepare clear, reviewable contributions without importing public-project ceremony into personal work.

## Rules

1. **Issues are optional.** Never require, create, search, approve, or link a GitHub issue unless the user explicitly asks for that issue operation. Missing issue metadata never blocks implementation, review, or a PR.
2. **Repository facts are authoritative.** Inspect the target branch, protection rules, checks, and templates that actually apply. Do not invent policy from another repository.
3. **Keep changes cohesive.** Split only at natural architecture, domain, interface, risk, verification, or rollback boundaries. Never use changed-line counts as a gate.
4. **Use honest evidence.** State exact checks run, failures, omissions, and remaining risks. Do not claim approval from an RDD receipt or from labels.
5. **Respect ownership.** The user decides delivery. Review findings inform that decision but never silently merge, force-push, or broaden scope.
6. **Use Conventional Commits** and never add `Co-Authored-By` or AI attribution.

## Workflow

1. Confirm repository, target branch, requested scope, and whether delivery is direct or via PR.
2. Map the affected behavior and choose one cohesive work unit or a natural chain.
3. Implement and verify in an isolated worktree.
4. Review the final candidate and report exact evidence.
5. Push or open the PR only when requested. Link an issue only when explicitly requested or supplied by the user.

## Output

Return the selected work-unit boundary, branch and commit, test evidence, PR URL when created, rollback notes, and only genuine technical or repository-policy blockers.
