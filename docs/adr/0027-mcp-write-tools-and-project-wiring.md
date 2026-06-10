# 0027. MCP write tools and project wiring (reviewer write-path restoration)

**Date:** 2026-06-10
**Status:** Accepted

## Context

The jun15 MCP pivot (ADR 0026) retired the headless check pipeline but
rebuilt only the read side of the MCP surface. Verified on 2026-06-10
(refs issue 0032): nothing fed the gate event store, so `next_review`
served frozen pre-pivot check state; D-Mail emission had no sanctioned
path; and the entry skill had no distribution mechanism (zero
invocations to date). Decision D2(a) chose on-demand GitHub ingestion
over reviving a daemon. Claude Code conformance constraints C1-C6
(refs issue 0032 §5) bound the design.

## Decision

1. **Dot-free tool names** (C1): `amadeus.ping` → `ping` etc.
2. **`refresh_reviews`**: on-demand ingest of the GitHub open-PR list
   via the existing `GhPRReader` (narrow `OpenPRLister` port),
   appending a new `EventPRSnapshotIngested` — a snapshot of PR
   identity only, deliberately NOT a `check.completed` (no fake
   divergence values).
3. **Review intake contract**: `post_comment` records
   `EventReviewPosted` on success (best-effort ledger entry);
   `next_review` serves the oldest snapshot PR without a posted
   review, signals `none_pending`, and falls back to the legacy
   `check.completed` read model when no snapshot exists.
4. **`dmail` emission tool**: typed D-Mail v1 subset built by
   `domain.NewProducedDMail` (producer kinds: design-feedback /
   implementation-feedback / convergence), staged and flushed through
   the existing transactional outbox.
5. **Project wiring** (C4/C5, decision D5(a)): `init` materializes the
   entry skill into the project's `.claude/skills/`; `mcp-config
   generate` upserts the project-root `.mcp.json` merge-aware. The
   canonical-locked `mcp_config.go` stays byte-identical (the upsert
   lives in `claude_wiring.go`, wired from cmd).
6. **`instructions` in the initialize handshake** (C6).

## Consequences

### Positive

- The reviewer loop functions end-to-end without a daemon: refresh →
  next → review → post → (feedback d-mail), with the queue state
  durable in the gate ledger.
- Emission cannot bypass atomicity; review bookkeeping cannot drift
  from what was actually posted.

### Negative

- The review queue is only as fresh as the last refresh_reviews call
  (accepted: on-demand is the negative-feedback design).
- post_comment's ledger entry is best-effort; a failed append needs a
  refresh cycle rather than a re-post.

### Neutral

- Legacy check.completed events remain readable (fallback path) until
  a future cleanup.
