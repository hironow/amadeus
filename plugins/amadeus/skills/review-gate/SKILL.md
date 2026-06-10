---
name: review-gate
description: >-
  Slash command for the amadeus review gate (jun15 MCP pivot). Triggers
  when the user types "/review-gate", asks to "review the next PR via
  amadeus", "run amadeus review-gate", "次の PR をレビューして", or
  "test the amadeus MCP server end-to-end". Drives the amadeus MCP
  server's tools (next_review / post_comment / get_pr_status) from
  inside a human-initiated Claude Code interactive session so inference
  stays on the subscription quota rather than the Agent SDK credit pool
  that gates `claude -p` from 2026-06-15.
version: 0.2.0
argument-hint: "(none) - fetches the next PR awaiting review from amadeus MCP and reviews it"
allowed-tools:
  - Read
  - Edit
  - Write
  - Bash
  - Grep
  - Glob
  - Agent
  - mcp__amadeus__amadeus_ping
  - mcp__amadeus__amadeus_next_review
  - mcp__amadeus__amadeus_post_comment
  - mcp__amadeus__amadeus_get_pr_status
---

# /review-gate — amadeus review gate

Human-initiated entry point. Drives the amadeus MCP server's tools
without ever invoking `claude -p`, so all inference happens inside
this interactive Claude Code session's subscription quota.

## Execution principle: one invocation = one PR review

One `/review-gate` run reviews **exactly one PR**, then stops and
reports back to the human. Do not loop into the next PR automatically —
the human re-invokes the slash command for each unit. This keeps the
feedback loop negative (stable, human-paced) and prevents runaway
sessions.

## Prerequisites

The session was launched with the amadeus MCP server attached:

```bash
claude --mcp-config '{"amadeus":{"command":"amadeus","args":["mcp"]}}'
```

If `amadeus mcp` is not on PATH, build it first:

```bash
cd path/to/amadeus && go build -o ./dist/amadeus ./cmd/amadeus
```

`amadeus mcp` must be started from the project root so it can resolve
the gate dir (review state). The MCP server answers the `initialize`
handshake, then exposes ping / next_review / post_comment /
get_pr_status.

## Workflow

1. **Verify MCP wiring**. Call `mcp__amadeus__amadeus_ping`. The tool
   must return `pong`. If it errors, the MCP server is not attached
   — abort and ask the human to relaunch claude with `--mcp-config`.

2. **Fetch the next PR awaiting review**. Call
   `mcp__amadeus__amadeus_next_review` with no arguments. It returns
   the next PR awaiting review from amadeus's gate state. If no PR is
   pending, surface that and stop — do not invent work.

3. **Review along the four divergence axes**. Read the PR body, diff,
   and any prior review comments, then assess each axis (this is
   amadeus's divergence model — keep the weights in mind when judging
   severity):

   | Axis | Weight | Question |
   |---|---|---|
   | ADR integrity | 40% | Does the change contradict any accepted ADR / shared ADR? |
   | DoD fulfillment | 30% | Are the issue's definition-of-done items actually met (tests, docs, gates)? |
   | Dependency integrity | 20% | Does it respect layer boundaries (semgrep layer rules) and module direction? |
   | Implicit constraints | 10% | Conventions not written as ADRs: stdio discipline, idempotency, naming, neutral wording |

   The judgment happens inside this human-initiated session — no
   `claude -p` invocation.

4. **Post a review comment**. Call
   `mcp__amadeus__amadeus_post_comment` with
   `{"pr_number": ..., "body": "..."}`. Structure the body with one
   short finding-block per axis that has findings (skip clean axes),
   then an overall verdict line. Keep the wording neutral
   (public-repo discipline). The tool posts via the GitHub Comments
   API (`gh pr comment`) and returns
   `{"posted": true, "persistence": "github-comments-api"}`. On error
   the `reason` field surfaces the failure (rate limit / auth) so the
   session can decide whether to retry (at most once).

5. **Check convergence + auto-merge status**. Call
   `mcp__amadeus__amadeus_get_pr_status` with `{"pr_number": ...}` to
   fetch the current convergence + auto-merge state (`convergence` /
   `auto_merge_ready` / `review_count` / `blocking_reviewers`).

6. **Report**. End with: PR number, per-axis findings summary,
   posted-comment confirmation, convergence state, and what the human
   should do next (merge decision / re-invoke for the next PR).

## Failure paths

- **MCP tool error mid-run**: report the tool name and the `reason`
  field, stop. Retry at most once for transient errors (rate limit).
- **PR too large to review meaningfully in one pass**: say so in the
  posted comment (scoped review of the riskiest files + explicit list
  of unreviewed areas) rather than pretending full coverage.
- **Conflicting prior reviews**: surface the conflict to the human
  instead of silently overriding another reviewer's judgment.

## Re-run idempotency

Re-invoking `/review-gate` after a partial run is safe: `next_review`
re-serves the PR until a review is recorded. Before posting, check the
PR's existing comments for a previous run's partial comment to avoid
duplicate posts — extend it with a follow-up comment instead of
repeating it.

## What this skill must NOT do

- Invoke `claude -p`, `claude --print`, the Anthropic Agent SDK, or
  any shell wrapper that does so (= billing boundary). The repo-wide
  semgrep gate (`.semgrep/jun15-no-headless-llm.yaml`) blocks these
  patterns in production code.
- Auto-trigger inference from a SessionStart hook or any other
  non-human-initiated path. The slash command typed by a human is
  the only valid entry to this workflow.
- Emit a D-Mail (e.g., conflict notification to paintress) by writing
  to `outbox/` directly. Direct writes bypass the transactional outbox
  (atomicity / idempotency / OTel audit). An `amadeus.dmail` emission
  tool does not exist yet — that gap is tracked in refs issue 0031;
  until it lands, D-Mail emission is out of scope for this skill.
- Call the GitHub Comments API directly via `curl` / `gh api` in the
  shell — the canonical path is the `amadeus.post_comment` MCP tool
  so the audit trail (= OTel `messaging.*` attrs) stays consistent.

## Done criteria

A `/review-gate` run is complete when, in a real Claude Code session
with the amadeus MCP server attached:

1. `ping` returns `pong` (handshake + tool dispatch verified).
2. `next_review` returns the next PR (or signals none pending).
3. The PR is reviewed along the four divergence axes.
4. `post_comment` returns `posted: true` /
   `persistence: "github-comments-api"`, and `get_pr_status` reflects
   the updated review state.
5. The closing report (PR / axes / convergence / next step) is
   delivered to the human.

## Related

- Canonical plan: `http://localhost:8765/docs/archive/0027-jun15-mcp-pivot.html` (refs)
- refs restructure + skill review: `http://localhost:8765/docs/issues/0030-refs-attic-restructure.html`
- D-Mail emission tool gap: `http://localhost:8765/docs/issues/0031-mcp-tool-surface-gaps.html`
- Pattern reference: amadeus ADR 0026 (`docs/adr/0026-mcp-pivot.md`)
- Divergence axes: amadeus ADR 0002 (`docs/adr/0002-four-axis-divergence-scoring.md`)
- Mechanical gate (semgrep rules): `.semgrep/jun15-no-headless-llm.yaml`
- D-Mail 9-field schema: `internal/domain/dmail_envelope.go`
