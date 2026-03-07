# S0025. RDRA Gap Resolution — D-Mail Protocol Extension

**Date:** 2026-03-02
**Status:** Accepted

## Context

RDRA audit identified 4 gaps in the D-Mail ecosystem:

1. GAP-1-01: Rework loop protocol undefined — no explicit action field
   for retry/escalate/resolve decisions in feedback D-Mails
2. GAP-1-02: CI results intake undefined — no D-Mail kind for CI results
3. GAP-1-03: Priority/scheduling policy weak — priority not propagated
   through D-Mail protocol or used for ordering
4. GAP-1-04: Operations dashboard missing — only phonewave has status

## Decision

### Protocol Extensions (schema v1 — no version bump)

Per ADR S0021 (Postel's Law), new optional fields do not require a
schema version bump. All existing parsers tolerate unknown YAML fields.

**New optional frontmatter fields:**

- `action` (string): Rework directive in feedback D-Mails.
  Values: `retry` (re-attempt), `escalate` (human attention needed),
  `resolve` (issue complete). Empty = no directive (backward compatible).
  Set by: amadeus. Consumed by: paintress.

- `priority` (int): Issue priority propagated through D-Mails.
  Values: 0 (unset), 1 (urgent), 2 (high), 3 (medium), 4 (low).
  Matches Linear API convention. Set by: any tool with Linear integration
  (currently paintress). Consumed by: paintress for issue ordering.

**New D-Mail kind:**

- `ci-result`: CI/CD execution results. Protocol definition only —
  actual CI bridge is out of scope.

### Rework Loop Contract (GAP-1-01)

amadeus → (feedback with action) → phonewave → paintress

1. amadeus sets `action` based on evaluation severity
2. paintress reads `action` from matched feedback D-Mails
3. Retry counting: per-issue attempt count in session (in-memory,
   keyed by sorted Issues slice; empty Issues skips tracking)
4. Max retries exceeded → automatic escalation (config: MaxRetries)
5. Default: High→escalate, Medium→retry, Low→resolve

### Priority Scheduling (GAP-1-03)

paintress fetches issues with priority from Linear API.

1. paintress sorts fetched issues by priority (urgent first)
2. Priority 0 (unset) sorted last
3. D-Mail `priority` field available for future cross-tool propagation

### Status Commands (GAP-1-04)

Each tool gains a `status` subcommand:

- Human-readable text to stdout
- Reads local state (event store + filesystem)
- Follows phonewave's existing StatusReport pattern

## Consequences

### Positive

- Rework decisions become explicit protocol contracts
- Priority propagation enables reproducible scheduling
- CI result kind enables future CI integration
- Status commands provide per-tool operational visibility

### Negative

- D-Mail struct divergence increases (new golden files needed)
- Retry counting adds session-level state to paintress

### Neutral

- Schema stays at v1 (no breaking change)
- CI bridge deferred (protocol only)
- Cross-tool dashboard aggregation not included
