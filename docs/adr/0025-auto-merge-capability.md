# 0025. Auto-merge capability for convergent PRs

**Date:** 2026-03-30
**Status:** Accepted

## Context

After reaching pre-convergence in go-taskboard 4-tool parallel operation,
all remaining open PRs required manual merging. amadeus already had PR
dependency chain analysis (ADR-0015), PR diff review (ADR-0024), and
4-axis divergence scoring. Adding auto-merge closes the loop from
detection → review → merge without human intervention.

The merge capability is inherently high-risk (irreversible side effect),
so it requires multiple safety guards.

## Decision

amadeus can auto-merge PRs when ALL of the following are true:

1. `--base` flag is set (daemon mode)
2. `--no-merge` is NOT set (default: merge enabled)
3. The most recent `runPostMergeCheck` returned no `DriftError` (world line is not diverged)
4. Each individual PR passes merge preconditions:
   - `mergeStateStatus == "CLEAN"` (CI passes, branch protection satisfied)
   - `reviewDecision == "APPROVED"` or no reviewers assigned
   - `mergeable == "MERGEABLE"` (no conflicts)
   - `amadeus:reviewed-{sha}` label exists (amadeus has reviewed the PR)

Merge strategy depends on chain position:
- **Chain root/middle** (has dependent PRs): `gh pr merge --merge` (preserve commit hash so dependents don't need rebase)
- **Chain leaf / standalone**: `gh pr merge --squash --delete-branch` (clean history)

Chain root/middle PRs are merged WITHOUT `--delete-branch` to prevent
breaking dependent PRs whose base branch would disappear.

Supersedes ADR-0015 item 2 (read-only git/gh access).

## Consequences

### Positive
- go-taskboard convergence can complete without human intervention
- Merge order is automatically correct (root-first via BFS chain)
- Chain-aware strategy prevents rebase cascades

### Negative
- Requires write access to GitHub (merge permission)
- Risk of merging a PR that should not be merged (mitigated by DriftError guard + 4 preconditions)
- `--merge` for chain PRs produces merge commits instead of clean squash history
