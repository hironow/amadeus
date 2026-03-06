# S0018. Event Storming Alignment

**Date:** 2026-03-01
**Status:** Accepted

## Context

parent CLAUDE.md (L155-175) defines Event Storming as the foundational process model
for all 4 tools (phonewave, sightjack, paintress, amadeus). The six elements are:

| Element | Definition | Naming |
|---------|-----------|--------|
| EVENT | State change in system/business | Past tense (did, -した) |
| COMMAND | Input to system (human-issued) | Present tense (will, -する) |
| POLICY | Automatic trigger: WHEN [EVENT] THEN [COMMAND] | - |
| READ MODEL | Reports/screens; built from EVENTs | - |
| AGGREGATE | Domain model between COMMAND and EVENT | - |
| EXTERNAL SYSTEM | Called by use case (LLM, Git, etc.) | - |

Process model:

```
Human --> READ MODEL --> COMMAND --> AGGREGATE --> EVENT
                                                    |
                                              POLICY (auto)
                                                    |
                                                    v
                                                 COMMAND --> ...
```

Each tool has varying applicability of these elements due to different domain concerns.

## Decision

### Element Applicability per Tool

| Element | phonewave | sightjack | paintress | amadeus |
|---------|-----------|-----------|-----------|---------|
| EVENT | Yes (delivery scope) | Yes (session scope) | Yes (expedition scope) | Yes (check scope) |
| COMMAND | Yes (typed structs) | Yes (typed structs) | Yes (typed structs) | Yes (typed structs) |
| POLICY | Yes (routing rules + PolicyEngine) | Yes (PolicyEngine) | Yes (PolicyEngine) | Yes (PolicyEngine) |
| READ MODEL | N/A (daemon) | Yes (ProjectState) | Yes (ProjectState) | Yes (Projector) |
| AGGREGATE | N/A (transport) | Anemic (pure functions) | Anemic (pure functions) | Anemic (pure functions) |
| EXTERNAL SYSTEM | N/A | Yes (Claude, Git) | Yes (Claude, Git, Notifier) | Yes (Claude, Git) |

### Permitted Variances

1. **phonewave** is a daemon/router (transport layer). It does not generate AI output,
   so READ MODEL and AGGREGATE are not applicable. This is intentional, not a deficiency.

2. **AGGREGATE** remains anemic (pure functions) across all tools. Full DDD Aggregate Root
   is deferred (see T-deferred-items.md R4) until domain complexity exceeds pure-function approach.

3. **EVENT scope** varies by tool: phonewave uses delivery scope, others use session/expedition/check scope.
   This per-tool variance is acceptable (see T-deferred-items.md A2).

### POLICY Dispatch Design

PolicyEngine follows best-effort dispatch:

- Handler failures are logged but do not stop the primary operation
- Handlers are registered at startup via `PolicyEngine.Register(eventType, handler)`
- Multiple handlers per event type are supported
- Dispatch is synchronous within the current goroutine
- No guaranteed ordering between handlers for the same event

### COMMAND Design (S0013)

All COMMAND types are defined in root package `command.go` with `Validate() []error`.
See ADR S0013 for naming convention details.

## Consequences

### Positive

- Clear documentation of which Event Storming elements apply to each tool
- Explicit acceptance of variance rather than forcing uniform application
- PolicyEngine provides extensible WHEN-THEN mechanism
- COMMAND types enable type-safe validation and testing

### Negative

- Anemic domain model limits compile-time invariant checking (deferred R4)
- Per-tool EVENT scope variance may complicate future cross-tool event correlation (deferred A2)

### Neutral

- phonewave's N/A for READ MODEL/AGGREGATE is architectural, not a gap to fill
- PolicyEngine handlers are best-effort; critical paths use direct calls
