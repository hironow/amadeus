# S0025. Event Delivery Guarantee Levels

**Date:** 2026-03-02
**Status:** Accepted

## Context

R4-04 requested reconsideration of fire-and-forget event delivery guarantees.

Current state across 4 tools:

- phonewave/paintress: emitEvent errors logged, never propagated (fire-and-forget)
- sightjack/amadeus: event errors propagated to caller (critical path)

Events are consumed by:

- amadeus Projector: materializes 8 event types into projections (Rebuild capable)
- sightjack: LoadAllEventsAcrossSessions for cross-session reporting
- paintress: gommage reads events for cleanup decisions

FileEventStore.Append() provides fsync durability. The fire-and-forget
semantics exist at the caller level, not the store level.

Silent event emission failures cause:

- READ MODEL staleness (projection misses event)
- Policy non-execution (WHEN event THEN command never fires)
- Observable via gap in JSONL sequence (detectable but not prevented)

## Decision

Define three event criticality levels. Each tool classifies its events:

**Level 1 — Critical (error propagated, operation fails)**
Events whose loss corrupts system state. Currently used by:

- amadeus: EventCheckCompleted, EventDMailGenerated (projection + outbox)
- sightjack: All session events (session recorder propagates)

**Level 2 — Important (error logged as Warning, metric incremented)**
Events whose loss degrades observability but does not corrupt state.
Failures are visible via OTel counter `{tool}.event.emit_error.total`.

- phonewave: RecordDeliveryEvent, RecordFailureEvent
- paintress: EventExpeditionCompleted

**Level 3 — Observational (error logged as Debug, best-effort)**
Events whose loss is acceptable. Purely diagnostic.

- phonewave: RecordScanEvent, RecordRetryEvent
- paintress: EventCascadeStarted, EventWorkerDispatched
- amadeus: EventForceFullNextSet

No changes to existing error propagation behavior for Level 1 (already correct).
Level 2 events gain OTel error counter (implemented via REMAIN-01 metrics).
Level 3 events remain as-is.

Implementation diff for Level 2:

- Add `{tool}.event.emit_error.total` counter (Int64Counter, attributes: event_type)
- Increment counter in existing error-logging code path
- No behavioral change to caller (still fire-and-forget)

Note on phonewave: DaemonSession.Record* methods exist and are tested,
but are not yet wired into the Daemon execution path (usecase/daemon.go
line 85: `_ = ds`). The phonewave Level 2 counter will be added when
DaemonSession is integrated into the daemon's event handling flow.

## Consequences

### Positive

- Explicit classification prevents accidental downgrade of critical events
- OTel metrics make Level 2 failures observable without blocking operations
- Rebuild() capability in amadeus provides recovery path for projection gaps

### Negative

- Level 2 metric adds minimal overhead to event emission hot path

### Neutral

- Classification is documented, not enforced by type system (pragmatic choice)
- Semgrep rule could enforce Level 1 error propagation in future
