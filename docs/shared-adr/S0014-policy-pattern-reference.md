# S0014. POLICY Pattern Reference Implementation

**Date:** 2026-02-28
**Status:** Accepted

## Context

The 4-tool ecosystem uses Event Storming terminology where POLICY represents
an automatic reactive rule: "WHEN [EVENT] THEN [COMMAND]". In P1-17, explicit
`Policy` types and registries were added to sightjack, paintress, and amadeus
to document these implicit behaviors. However, no reference implementation
exists showing how to evolve from implicit policies to a dispatch engine.

phonewave's daemon (daemon.go) + delivery (delivery.go) + scanner (scanner.go)
form the most mature example of the POLICY pattern in the ecosystem, even though
it uses implicit code flow rather than explicit Policy types.

## Decision

Document phonewave's existing implicit POLICY implementations as the reference
for future dispatch engine work across all 4 tools.

### Implicit Policies in phonewave

| Policy | Trigger (EVENT) | Action (COMMAND) | Implementation |
|--------|-----------------|------------------|----------------|
| FileCreatedDeliverDMail | fsnotify.Create on outbox/ | DeliverData | daemon.go:handleEvent → delivery.go:DeliverData |
| DeliveryFailedRecordError | DeliverData returns error | RecordFailure | daemon.go:handleEvent error path → error_store.go |
| ScanCompletedUpdateRoutes | Manual scan invocation | UpdateRoutes | scanner.go:ScanEndpoints → daemon route refresh |

### Pattern Structure

```
fsnotify.Event (filesystem trigger)
    |
    v
daemon.handleEvent()        ← implicit POLICY: WHEN FileCreated THEN Deliver
    |
    v
delivery.DeliverData()      ← COMMAND execution (pure routing logic)
    |
    +---> success: file delivered to inboxes
    |
    +---> failure: error_store.RecordFailure()  ← implicit POLICY: WHEN DeliveryFailed THEN RecordError
```

### Evolution Path (P2+ dispatch engine)

1. Extract implicit policies into explicit `Policy` structs (matching sj/pt/am P1-17 pattern)
2. Create a `PolicyDispatcher` that maps EventType → handler function
3. Replace direct function calls in daemon.handleEvent with dispatcher invocations
4. Enable policy composition: multiple policies can react to the same event

## Consequences

### Positive

- Clear reference for other tools implementing dispatch engines
- Documents the implicit-to-explicit evolution path
- Validates that the existing implicit pattern is correct

### Negative

- phonewave's implicit pattern works well; making it explicit adds indirection

### Neutral

- Other tools can reference this ADR when implementing their dispatch engines
