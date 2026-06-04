---
name: review-gate
description: >-
  Slash command for the amadeus review gate (refs/issues/0027 jun15 MCP
  pivot). Triggers when the user types "/review-gate", asks to
  "review the next PR via amadeus", "run amadeus review-gate", or
  "test the amadeus MCP server end-to-end". Drives the amadeus MCP
  server's tools (next_review / post_comment / get_pr_status) from
  inside a human-initiated Claude Code interactive session so inference
  stays on the subscription quota rather than the Agent SDK credit pool
  that gates `claude -p` from 2026-06-15.
version: 0.1.0
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
   pending, surface that and stop.

3. **Inspect the PR diff**. Read the PR body, diff, and any prior
   review comments. Plan the review using the session's human-driven
   judgment (= no claude -p invocation).

4. **Post a review comment**. Call
   `mcp__amadeus__amadeus_post_comment` with
   `{"pr_number": ..., "body": "..."}`. The tool posts the comment via
   the GitHub Comments API (`gh pr comment`) and returns
   `{"posted": true, "persistence": "github-comments-api"}`. On error
   the `reason` field surfaces the failure (rate limit / auth) so the
   session can decide whether to retry.

5. **Check convergence + auto-merge status**. Call
   `mcp__amadeus__amadeus_get_pr_status` with `{"pr_number": ...}` to
   fetch the current convergence + auto-merge state (`convergence` /
   `auto_merge_ready` / `review_count` / `blocking_reviewers`).

## What this skill must NOT do

- Invoke `claude -p`, `claude --print`, the Anthropic Agent SDK, or
  any shell wrapper that does so (= refs/issues/0027 §5 billing
  boundary). The repo-wide semgrep gate
  (`.semgrep/jun15-no-headless-llm.yaml`) blocks these patterns in
  production code.
- Auto-trigger inference from a SessionStart hook or any other
  non-human-initiated path. The slash command typed by a human is
  the only valid entry to this workflow.
- Emit a D-Mail (e.g., conflict notification to paintress) by writing
  to `outbox/` directly. D-Mail emission is not exposed as an MCP tool
  in this skill's tool set. The D-Mail 9-field schema is fixed in
  refs 0027 §8.
- Call the GitHub Comments API directly via `curl` / `gh api` in the
  shell — the canonical path is the `amadeus.post_comment` MCP tool
  so the audit trail (= OTel `messaging.*` attrs) stays consistent.

## Done criteria

A `/review-gate` run is complete when, in a real Claude Code session
with the amadeus MCP server attached:

1. `ping` returns `pong` (handshake + tool dispatch verified).
2. `next_review` returns the next PR (or signals none pending).
3. The PR is reviewed.
4. `post_comment` returns `posted: true` /
   `persistence: "github-comments-api"`, and `get_pr_status` reflects
   the updated review state.

## Related

- Canonical plan: `refs/HTMLification/docs/archive/0027-jun15-mcp-pivot.html`
- Pattern reference: amadeus ADR 0026 (`~/tap/amadeus/docs/adr/0026-mcp-pivot.md`)
- Billing boundary table: refs 0027 §5
- Mechanical gate (semgrep rules): refs 0027 §6 + `.semgrep/jun15-no-headless-llm.yaml`
- D-Mail 9-field schema: refs 0027 §8 + `internal/domain/dmail_envelope.go`
