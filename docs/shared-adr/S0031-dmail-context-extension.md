# S0031. D-Mail Context Extension (Amends S0005)

**Date:** 2026-03-10
**Status:** Accepted

## Context

S0005 defined the D-Mail v1 envelope with fixed fields. Tools need to attach
contextual insight summaries to D-Mails for cross-tool knowledge propagation
without introducing side-channel file reads.

## Decision

Add an optional `context` field to the D-Mail v1 envelope:

```yaml
---
dmail-schema-version: "1"
name: spec-001
kind: specification
description: Implementation specification
context:
  insights:
    - source: ".expedition/insights/lumina.md"
      summary: "auth module CI不安定、stacked PR注意"
---
```

### Rules

1. `context` is optional — omission is valid
2. `context.insights` is a list of `{source, summary}` pairs
3. Schema version remains "1" (additive, backward-compatible)
4. Receivers MUST NOT reject unknown context fields (S0019 Postel's Law)
5. phonewave relays context without interpretation

### Kind Validation Update

Add to the known kind set (already implemented, not yet in S0005):
- `implementation-feedback`
- `design-feedback`

## Consequences

### Positive
- Cross-tool insight propagation via D-Mail contract (no side-channel reads)
- Backward-compatible — existing parsers ignore unknown fields

### Negative
- All tools' D-Mail parsers need `context` field in struct

### Neutral
- Amends S0005 (adds field, does not change existing fields)
