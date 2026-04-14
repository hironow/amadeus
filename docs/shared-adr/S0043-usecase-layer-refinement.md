# S0043. Usecase Layer Refinement

**Date:** 2026-04-15
**Status:** Accepted

## Context

Cross-tool audit (2026-04-14) found that usecase layers accumulated three
categories of code beyond their ADR S0015/S0034 mandate:

1. **Pass-through functions** — `RunInit()` that just forwarded to
   `port.InitRunner.InitProject()` with no orchestration logic
2. **Setup boilerplate** — `RunSession()`, `RunCheck()`, `SetupAndRunDaemon()`
   that created aggregates, wired PolicyEngine, built EventEmitter, then
   delegated to session runners with no business decisions
3. **Inconsistent emit() contracts** — sightjack's emit() was void
   (best-effort), other tools returned errors; phonewave lacked
   CorrelationID/CausationID enrichment

These patterns obscured the legitimate usecase content (orchestration,
policy dispatch, event emission) behind composition-root boilerplate.

## Decision

### 1. Pass-through Prohibition

Functions that only delegate to a single port method with no branching,
state, or transformation are prohibited in usecase/. cmd/ calls session
adapters directly for such cases (e.g., `adapter.InitProject()`).

### 2. Composition Root in cmd/

Aggregate creation, PolicyEngine construction, policy handler registration,
and EventEmitter wiring are composition-root concerns. usecase/ exports
factory functions (`BuildSessionEmitter`, `PrepareExpeditionRunner`,
`BuildCheckEmitter`, `PrepareDaemonRunner`) that cmd/ calls before
delegating to session runners.

Pattern:
```go
// cmd/run.go (composition root)
emitter := usecase.BuildSessionEmitter(ctx, store, logger, ...)
return runner.RunSession(ctx, cfg, ..., emitter, logger)
```

### 3. emit() Returns Error

All tools' `emit()` methods return `error`. Store and SeqNr errors propagate
to callers. Dispatch errors remain best-effort (logged, not returned).
Callers decide whether to treat store errors as fatal or best-effort.

### 4. Metadata Enrichment Consistency

All EventEmitter implementations enrich events with:
- `CorrelationID` — session/expedition/check/delivery ID
- `CausationID` — previous event ID (causation chain)
- `SeqNr` — global sequence number (when allocator is available)

### 5. Usecase Retains

- EventEmitter (aggregate wrap + persistence + dispatch) — ADR S0015
- PolicyEngine + handler registration — ADR S0015
- Orchestration (review gate, triage, PR convergence, etc.) — ADR S0034

## Consequences

### Positive

- usecase/ contains only orchestration + application infrastructure
- cmd/ is the explicit composition root (Go idiom)
- emit() contract is consistent across all 4 tools
- Event metadata is uniform (enables cross-tool event correlation)

### Negative

- cmd/ RunE functions are longer (absorb setup boilerplate)
- Factory functions in usecase/ must be kept in sync with session runner APIs

### Neutral

- Existing semgrep rules remain unchanged (layer boundaries still enforced)
- Port interfaces preserved (InitRunner, EventStore, etc.)
