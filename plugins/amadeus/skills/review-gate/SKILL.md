---
name: review-gate
description: >-
  Phase 2b slash command for the amadeus jun15 MCP pivot
  (refs/issues/0027). Triggers when the user types "/review-gate",
  asks to "review the next PR via amadeus", "run amadeus review-gate",
  or "test the amadeus MCP server end-to-end". Drives the amadeus
  MCP server's stub tools (next_review / post_comment / get_pr_status)
  from inside a human-initiated claude code interactive session so
  inference stays on the subscription quota rather than the Agent SDK
  credit pool that gates `claude -p` from 2026-06-15.
version: 0.1.0
argument-hint: "(none) - fetches the next PR awaiting review from amadeus MCP and surfaces the stub contract"
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

# /review-gate — amadeus MCP pivot Phase 2b

Human-initiated entry point. Drives the amadeus MCP server's tools
without ever invoking `claude -p`, so all inference happens inside
this interactive claude code session's subscription quota.

## Prerequisites

The session was launched with the amadeus MCP server attached:

```bash
claude --mcp-config '{"amadeus":{"command":"amadeus","args":["mcp"]}}'
```

If `amadeus mcp` is not on PATH, build it first:

```bash
cd path/to/amadeus && go build -o ./dist/amadeus ./cmd/amadeus
```

## Workflow

1. **Verify MCP wiring**. Call `mcp__amadeus__amadeus_ping`. The tool
   must return `pong`. If it errors, the MCP server is not attached
   — abort and ask the human to relaunch claude with `--mcp-config`.

2. **Fetch the next PR awaiting review**. Call
   `mcp__amadeus__amadeus_next_review` with no arguments. During
   Phase 2b the response is a stub:

   ```json
   {
     "stub": true,
     "pr": null,
     "reason": "phase-2b-mvp: real implementation lands when ...",
     "contract": {"pr_number": "integer", "owner": "string", "repo": "string", "title": "string", "branch": "string", "status": "string"}
   }
   ```

   While `stub == true`, **do NOT proceed to comment posting or
   auto-merge**. Surface the contract descriptor so the human can
   verify the shape, and stop. Real wiring lands in a subsequent
   commit on the `feat/jun15-mcp-pivot` branch.

3. **(Post-stub) Inspect the PR diff**. Read the PR body, diff, and
   any prior review comments. Plan the review using the human-driven
   judgment (= no claude -p invocation).

4. **(Post-stub) Post a review comment**. Call
   `mcp__amadeus__amadeus_post_comment` with
   `{"pr_number": ..., "body": "..."}`. Phase 2b stub echoes the
   pr_number and body length with `posted: false` to signal no
   side-effect.

5. **(Post-stub) Check convergence + auto-merge status**. Call
   `mcp__amadeus__amadeus_get_pr_status` with `{"pr_number": ...}`
   to fetch the current convergence + auto-merge state. Phase 2b
   stub echoes pr_number with a contract descriptor for the real
   shape (`convergence` / `auto_merge_ready` / `review_count` /
   `blocking_reviewers`).

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
  to `outbox/` directly. The `amadeus.emit_dmail` MCP tool ships in
  a later commit; that tool encapsulates the transactional outbox
  + the 9-field schema fixed in refs 0027 §8.
- Call the GitHub Comments API directly via `curl` / `gh api` in the
  shell — the canonical path is the `amadeus.post_comment` MCP tool
  so the audit trail (= OTel `messaging.*` attrs) stays consistent.

## Phase 2b MVP exit criteria

This skill is considered Phase 2b MVP complete when:

1. Calling `/review-gate` in a real claude code session with the
   amadeus MCP server attached returns the stub responses from
   steps 1-2 without error.
2. The `claude_adapter.go` and `doctor.go` `claude --print`
   invocations are removed and the semgrep transitional excludes
   on those two files are deleted (= the final commit on the
   `feat/jun15-mcp-pivot` branch flips the lint gate from advisory
   to enforced).

## Related

- Canonical plan: `refs/HTMLification/docs/issues/0027-jun15-mcp-pivot.html`
- Pattern reference:
  - paintress ADR 0017 (`~/tap/paintress/docs/adr/0017-mcp-pivot.md`)
  - sightjack ADR 0018 (`~/tap/sightjack/docs/adr/0018-mcp-pivot.md`)
- Billing boundary table: refs 0027 §5
- Mechanical gate (semgrep rules): refs 0027 §6 + `.semgrep/jun15-no-headless-llm.yaml`
- D-Mail 9-field schema: refs 0027 §8 + `internal/domain/dmail_envelope.go`
