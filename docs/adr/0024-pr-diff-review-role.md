# 0024. PR Diff Review — Reviewer Role Extension

**Date:** 2026-03-29
**Status:** Accepted
**Supersedes:** [0015](0015-pr-convergence-conductor-role.md) (Decision items 1 and 2)

## Context

ADR-0015 established amadeus as a Conductor for PR convergence with two key constraints:

1. **No LLM for PR analysis** — purely mechanical chain detection
2. **Read-only git/gh access** — no write operations on PRs

However, when `--base main` is used, amadeus evaluates the main branch state against ADRs/DoDs but does NOT read the code diffs of open PRs targeting main. This leads to false positives: amadeus reports ADR violations that PRs have already fixed, because it only sees the current main state — not the pending changes.

The existing PR convergence pipeline (chain detection, conflict analysis) remains valuable but insufficient: it tells paintress about dependency ordering but gives no feedback on whether PR code changes actually comply with ADRs/DoDs.

## Decision

Extend amadeus's role from **Conductor** to **Conductor + Reviewer** for PRs targeting the integration branch:

1. **LLM-based PR diff evaluation**: For each open PR targeting `--base`, amadeus reads the unified diff via `gh pr diff` and evaluates ADR/DoD compliance using Claude (the same Divergence Meter pattern used for post-merge checks). This supersedes ADR-0015 Decision item 1.

2. **Label write access**: amadeus applies `amadeus:reviewed-{head_sha8}` labels to evaluated PRs via `gh pr edit --add-label`. This is the only write operation — amadeus does NOT merge, rebase, close, or modify PR content. This supersedes ADR-0015 Decision item 2.

3. **Commit-aware re-evaluation**: Labels encode the HEAD commit SHA (first 8 chars). When a PR receives new commits, the label no longer matches, triggering re-evaluation. Old labels remain as audit history.

4. **Existing convergence preserved**: Chain detection, conflict analysis, and implementation-feedback D-Mail generation (ADR-0015 items 3-5) remain unchanged.

5. **Per-PR evaluation**: Each PR is evaluated individually (not batched) to manage Claude context limits, enable independent label tracking, and allow partial failure without blocking other PR evaluations.

## Consequences

### Positive

- Eliminates false positives from PRs that have already addressed divergence
- Per-PR feedback gives paintress targeted, actionable guidance
- Commit-aware labels prevent stale reviews after force-push
- Label serves as audit trail (which commit was reviewed, when)
- Same Claude evaluation pattern as post-merge checks — consistent architecture

### Negative

- LLM cost per PR evaluation (bounded by open PR count)
- GitHub label write access required (minimal blast radius — labels only)
- Large PR diffs may approach Claude context limits

### Neutral

- ADR-0015 items 3-5 (integration point model, inbox-driven, separation of concerns) remain in effect
- `amadeus:reviewed-{sha}` label follows the same pattern as sightjack's `sightjack:wave-done`
