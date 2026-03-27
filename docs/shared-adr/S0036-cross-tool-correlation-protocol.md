# S0036. Cross-Tool CorrelationID Assignment Protocol

**Date:** 2026-03-27
**Status:** Accepted

## Context

Phase 1 (S0033) added AggregateID/SeqNr to all 4 tools. Phase 2 (S03) unified
CorrelationID and CausationID across all 4 tools and ported SessionRecorder to
paintress and amadeus.

However, without a shared assignment convention, each tool could assign
CorrelationID differently (sightjack uses sessionID, phonewave has no canonical
rule). Cross-tool event tracing requires a shared protocol so that a single
`grep` across event stores reconstructs the full lifecycle chain.

## Decision

### CorrelationID Assignment Rules

**RULE 1: D-Mail-triggered work uses D-Mail name as CorrelationID.**

When a tool starts a new unit of work triggered by an inbox D-Mail,
`CorrelationID` MUST be set to the D-Mail name (the filename without extension).
This is passed as the `sessionID` parameter to `NewSessionRecorder`.

**RULE 2: CorrelationID propagates unchanged.**

All events in the same unit of work share the same CorrelationID. The
SessionRecorder enforces this automatically.

**RULE 3: CausationID chains to the previous event.**

CausationID is set to the ID of the immediately preceding event in the same
unit of work. The SessionRecorder manages this via its `prevID` field.

### Example Chain

```
sightjack  specification_sent     CorrelationID="spec-auth-w1"  CausationID=""
paintress  expedition.started     CorrelationID="spec-auth-w1"  CausationID=<ev1.ID>
paintress  expedition.completed   CorrelationID="spec-auth-w1"  CausationID=<ev2.ID>
amadeus    check.completed        CorrelationID="spec-auth-w1"  CausationID=<ev3.ID>
```

### Cross-Tool Query

This protocol makes cross-tool causal chains queryable without a central index:

```bash
grep '"correlation_id":"spec-auth-w1"' .siren/events/*.jsonl
grep '"correlation_id":"spec-auth-w1"' .expedition/events/*.jsonl
grep '"correlation_id":"spec-auth-w1"' .gate/events/*.jsonl
```

### Non-D-Mail Work

For work not triggered by a D-Mail (e.g., CLI-initiated runs), the tool SHOULD
generate a unique CorrelationID (e.g., `run-{timestamp}` or UUID). The
SessionRecorder accepts any string as sessionID.

## Consequences

### Positive
- Full lifecycle traceability across 4 tools with filesystem-native `grep`
- No central database or distributed tracing backend required
- Compatible with OTel trace correlation when OTLP is configured
- D-Mail name as correlation key is human-readable and content-addressed

### Negative
- Requires all tools to use SessionRecorder (or equivalent) consistently
- D-Mail naming must remain unique (already enforced by S0031)

### Neutral
- phonewave (courier) does not generate its own CorrelationID; it preserves
  whatever the sender tool set in the D-Mail metadata
