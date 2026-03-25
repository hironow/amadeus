# S0034. Session-Usecase Boundary Clarification

**Date:** 2026-03-25
**Status:** Accepted

## Context

Cross-tool audit (2026-03-25) identified that session layers across sightjack,
paintress, and amadeus had accumulated COMMAND orchestration and POLICY dispatch
logic that belongs in the usecase layer. The root cause was that session structs
(Paintress, Amadeus, Session) were acting as pseudo-usecase layers, handling
business logic alongside I/O adaptation.

Specific violations found and migrated:
- amadeus: `CollectRepeatedViolations`, `AnalyzeDivergenceTrend` (pure domain logic in session)
- amadeus: `runPreMergePipeline` (COMMAND orchestration in session)
- sightjack: `StructuralErrors`, `StallEscalationBody` (pure domain logic in session)
- paintress: `triagePreFlightDMails` (POLICY dispatch in session)
- paintress: `handleFeedbackAction` (POLICY dispatch in session)

## Decision

### Session Layer Responsibilities (MUST)

Session layer is **strictly I/O adaptation**:
- Filesystem operations (read, write, rename, mkdir)
- Subprocess execution (Claude CLI, git, shell commands)
- Network I/O (HTTP, fsnotify)
- OTel span creation and attribute recording
- SQLite transactional outbox (Stage/Flush)

### Session Layer Prohibitions (MUST NOT)

Session layer MUST NOT contain:
- COMMAND dispatch logic (switch on action types, kind-based routing)
- POLICY evaluation (retry counting, escalation thresholds, max retries)
- Pure domain computation (scoring, trend analysis, string formatting)
- Business rule enforcement (cycle budgets, convergence detection)

### Usecase Layer Responsibilities

Usecase layer owns:
- COMMAND → Aggregate → EVENT orchestration
- POLICY registration and dispatch (PolicyEngine)
- Business logic that coordinates domain types with port interfaces
- Retry/escalation/triage state machines

### Migration Pattern

When session logic must move to usecase:

1. Define port interface in `usecase/port/` (e.g., `PreFlightTriager`, `PRPipelineRunner`)
2. Implement interface in `usecase/` package
3. Add `Set*` method to `ExpeditionRunner`/`Orchestrator` port
4. Session method becomes thin delegation to injected port (nil-safe fallback)
5. `cmd/` composition root wires usecase implementation into session

### Enforcement

- Semgrep rule `session-no-action-dispatch` (ERROR) prevents `switch dm.Action` in session
- Existing `layer-usecase-no-import-session` prevents reverse dependency
- Existing `layer-session-no-import-usecase` enforces port-only access

## Consequences

### Positive

- Session layers reduced to pure I/O adapters (testable with real processes)
- Usecase layer owns all business logic (testable with port mocks)
- Port interfaces enable dependency inversion without layer violations
- Semgrep rules prevent regression

### Negative

- More port interfaces and Set* methods add indirection
- Session tests for migrated logic need test doubles that replicate usecase behavior
- cmd composition root wiring becomes more complex

### Neutral

- phonewave was already compliant (reference implementation)
- RunReviewGate exception is documented and intentional
