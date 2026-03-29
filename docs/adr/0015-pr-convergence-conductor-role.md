# 0015. PR Convergence — Conductor Role

**Date:** 2026-03-09
**Status:** Superseded by [0024](0024-pr-diff-review-role.md)

## Context

When large numbers of PRs accumulate in a repository (e.g., 17 open PRs with deeply nested dependency chains), the codebase needs a mechanism to detect and converge these PRs. The question is what role amadeus should play in this convergence process.

Three roles were considered:

- **Observer**: Detect and report PR state only
- **Conductor**: Detect PR state, plan convergence, and instruct other tools via D-Mail
- **Executor**: Detect, plan, and directly execute git operations (rebase, merge)

## Decision

amadeus operates as a **Conductor** for PR convergence:

1. **Read-only git/gh access**: amadeus uses `git` and `gh` CLI commands to mechanically detect PR dependency chains, conflicts, and merge order. No LLM is used for PR analysis.

2. **D-Mail as the action mechanism**: When chains or conflicts are detected, amadeus sends `implementation-feedback` D-Mails (one per chain, not per PR) to paintress, which executes the actual rebase/conflict resolution using LLM-based code understanding.

3. **Integration point model**: The current branch is treated as the "integration point". Pre-merge analysis examines PRs targeting this branch. Post-merge divergence checking is optionally enabled via `--base` flag.

4. **Inbox-driven, no polling**: amadeus does not poll GitHub APIs on a timer. D-Mail arrival in the inbox triggers analysis. This leverages the existing D-Mail event-driven architecture and avoids GitHub API rate limit concerns.

5. **Separation of observation and execution**: amadeus handles the "what needs to happen" (chain detection, conflict identification, merge order planning) while paintress handles the "how to do it" (rebase execution, conflict resolution with LLM).

## Consequences

### Positive

- Clear separation of concerns: observation (amadeus) vs. execution (paintress)
- No LLM cost for PR analysis — purely mechanical detection
- No GitHub API rate limit risk — inbox-driven, not polling
- Single D-Mail per chain provides paintress with full context for ordered execution
- Consistent with amadeus's existing role as a verifier, not implementer

### Negative

- Two-tool coordination required for PR convergence (amadeus + paintress + phonewave routing)
- Conflict file detection is limited (gh CLI doesn't provide detailed conflict info; paintress discovers details during resolution)
- No real-time PR monitoring — depends on D-Mail arrival timing

### Neutral

- `amadeus check` is deprecated in favor of `amadeus run` (daemon mode)
- Post-merge divergence checking remains available via `--base` flag
