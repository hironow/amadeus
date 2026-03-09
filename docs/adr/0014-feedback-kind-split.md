# 0014. Split feedback D-Mail kind into design-feedback and implementation-feedback

**Date:** 2026-03-08
**Status:** Accepted

## Context

The `feedback` D-Mail kind was multi-purpose: amadeus sent both design-level
issues (ADR violations, dependency structure) and implementation-level issues
(DoD gaps, convention violations) through the same kind. Because phonewave
routes by kind, both sightjack and paintress received identical feedback
regardless of relevance, adding noise and wasting processing.

## Decision

Replace `feedback` with two new kinds: `design-feedback` and
`implementation-feedback`. Amadeus classifies each D-Mail candidate using
dual-signal classification: qualitative (Claude AI category field) and
quantitative (4-axis weighted score comparison). When both signals agree,
one D-Mail is emitted to the appropriate target; when they disagree, two
D-Mails are emitted (safe fallback to both targets).

Schema version remains "1" (additive change, all tools deployed simultaneously).

## Consequences

### Positive

- Sightjack receives only design-relevant feedback for wave planning
- Paintress receives only implementation-relevant feedback for code fixes
- Reduced noise in each tool's inbox

### Negative

- Disagreement between AI and rule-based classification emits duplicate D-Mails
- Old `feedback` kind removed without backward compatibility

### Neutral

- phonewave routing logic unchanged (SKILL.md-driven auto-derivation handles new kinds)
