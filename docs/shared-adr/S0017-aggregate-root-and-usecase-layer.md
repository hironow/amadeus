# S0017. Aggregate Root and Use Case Layer

**Date:** 2026-02-28
**Status:** Accepted

## Context

P0-P3 established event sourcing, 3-layer separation (cmd → session →
eventsource → root), semgrep enforcement, typed events, POLICY registries,
READ MODEL projections, and transactional outbox across all 4 tools.

However, root package types remain anemic — data-only structs with no
domain behavior. Session layers (900-5200 LOC) act as god objects that
orchestrate everything: COMMAND validation, domain logic, event emission,
I/O coordination, and policy evaluation.

CLAUDE.md Event Storming principles require:

- COMMAND → Aggregate.Handle() → EVENT
- Use case orchestrates Aggregate + external systems
- Aggregate enforces domain invariants

Current conformance: EVENT 90%, COMMAND 5%, POLICY 40%, READ MODEL 60%,
AGGREGATE 30%, EXTERNAL SYSTEM 95%.

## Decision

### 1. Aggregate Root Types in Root Package

Each tool defines aggregate types in the root package that own domain state
and enforce invariants. Aggregates produce events as return values (no side
effects). State is hydrated from existing projections (SessionState,
CheckResult), not from a new persistence mechanism.

| Tool | Aggregate | Key Invariant |
|------|-----------|---------------|
| amadeus | CheckAggregate | CheckCount ↔ ForceFullNext consistency |
| sightjack | SessionAggregate, WaveAggregate | Wave unlock prerequisites |
| paintress | ExpeditionAggregate | Gommage threshold ↔ consecutive failures |
| phonewave | DeliveryAggregate (thin) | Delivery event emission single point |

### 2. Use Case Layer (`internal/usecase/`)

A new `internal/usecase/` layer sits between cmd and session:

```
cmd -> usecase -> session -> eventsource -> root
```

Use cases:

- Receive COMMAND from cmd layer
- Validate via COMMAND.Validate()
- Invoke Aggregate methods to produce events
- Persist events via session/eventsource
- Dispatch POLICY handlers

Session layer retains only I/O adapter responsibilities (Claude subprocess,
git operations, file I/O, HTTP, SQLite).

### 3. Semgrep Layer Enforcement

Three new rules added to each tool's `.semgrep/layers.yaml`:

- `layer-usecase-no-import-cmd` — usecase must not import cmd
- `layer-usecase-no-import-eventsource` — usecase accesses stores via session factories
- `layer-session-no-import-usecase` — prevents reverse dependency

### 4. POLICY Dispatch Engine

Each tool's usecase layer includes a PolicyEngine that connects the existing
Policy registry (type + trigger + action) to actual handler execution:

```
emit(event) → PolicyEngine.Dispatch(event) → registered handlers
```

### 5. COMMAND → Aggregate End-to-End Connection

Each tool connects at least one primary path in P4:

- amadeus: ExecuteCheckCommand → CheckAggregate.RecordCheck() → events
- sightjack: ApplyWaveCommand → WaveAggregate.Approve() → events
- paintress: RunExpeditionCommand → ExpeditionAggregate.StartExpedition() → events
- phonewave: RunDaemonCommand → DeliveryAggregate.RecordDelivery() → events

Secondary COMMAND paths are connected in P5.

## Consequences

### Positive

- Domain invariants enforced in one place (aggregate), not scattered across session
- Session god objects decomposed into focused I/O adapters
- COMMAND → Aggregate → EVENT loop matches Event Storming model
- Semgrep prevents layer violations at CI time
- PolicyEngine enables reactive automation (WHEN event THEN command)

### Negative

- 4-layer architecture adds one indirection level
- Refactoring 4 tools simultaneously is high-effort (~24 tasks)
- phonewave aggregate is thin due to daemon-in-root-package constraint

### Neutral

- Existing CLI interface (flags, output, exit codes) is unchanged
- Event store format is unchanged — aggregates hydrate from existing projections
- phonewave root → session separation deferred to P5
