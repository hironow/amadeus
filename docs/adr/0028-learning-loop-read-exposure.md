# 0028. Learning-loop read exposure (get_insights)

**Date:** 2026-06-10
**Status:** Accepted

## Context

The S0041 improvement controller — the pre-pivot consumer of the
`.gate/insights/` ledger — retired with the jun15 MCP pivot, leaving
the verifier's learning assets invisible to the Claude Code session
(refs issue 0034). Review history, meanwhile, accumulates live in the
gate event store (review.posted / pr.snapshot.ingested / legacy
check.completed).

## Decision

1. **`get_insights` read-only MCP tool**: returns persisted
   insight-ledger files (parsed by the existing `InsightWriter.Read`)
   plus a live review summary derived from the gate events (reviews
   posted, latest snapshot size/at, pending reviews, latest check
   divergence — reusing the next_review intake projection).
2. **No write path is introduced**; the event store written by
   refresh_reviews / post_comment is the durable learning input.
3. **Empty state is not an error.**
4. **Skill v0.3.1**: `/review-gate` consults `get_insights` before the
   four-axis review (recurring failure classes raise scrutiny on the
   matching axes).

## Consequences

### Positive

- The verifier's loop closes: past corrections shape the next review,
  through an audited read surface; no new persistence or ports.

### Negative

- The insight-file half stays sparse until a future wave adds a
  session-side correction-persistence path (deliberately out of scope).

### Neutral

- Legacy check.completed divergence is surfaced when present (read-only
  compatibility).
