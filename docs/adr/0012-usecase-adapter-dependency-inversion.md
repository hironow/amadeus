# 0012. Usecase-Adapter Dependency Inversion

**Date:** 2026-03-05
**Status:** Accepted

## Context

The usecase layer directly imported the session layer to call infrastructure functions
(check runner, store factories, config loading). This created a tight coupling:
usecase depended on concrete session implementations rather than abstractions.

The hexagonal architecture principle (port-adapter pattern) requires that inner layers
depend only on interfaces, not on outer-layer implementations. The cmd layer should act
as the composition root, wiring concrete session implementations to usecase-defined
output port interfaces.

Prior to this change, the dependency graph was:

```
cmd → usecase → session (direct import)
```

This violated the Dependency Inversion Principle: high-level policy (usecase) depended
on low-level detail (session infrastructure).

## Decision

Invert the usecase→session dependency using output port interfaces:

1. **usecase depends only on `usecase/port` interfaces** — no session import allowed.
2. **session implements port interfaces** as adapter structs (e.g., `CheckRunnerAdapter`).
3. **cmd acts as composition root** — creates session adapters and injects them into usecase functions.
4. **Pure passthrough functions eliminated** — usecase functions that only delegated to session are deleted; cmd calls session directly.

Post-inversion dependency graph:

```
cmd → usecase      (business logic invocation)
cmd → session      (composition root wiring)
cmd → usecase/port (type references)
usecase → usecase/port (output port interfaces only)
session → usecase/port (adapter implementation)
```

Enforced by semgrep rule `layer-usecase-no-import-session` (ERROR severity).

### amadeus-specific changes

- `CheckRunner` port interface abstracts the check execution workflow
- `session.NewCheckRunnerAdapter()` factory creates the concrete adapter
- `usecase.RunCheck()` receives `port.CheckRunner`, retaining only
  aggregate management, PolicyEngine creation, and delegation
- Pure passthrough functions (rebuild, init, etc.) deleted; cmd calls session directly

## Consequences

### Positive

- usecase layer is fully decoupled from infrastructure — testable with null objects
- Dependency direction matches hexagonal architecture intent
- Pure passthrough elimination reduces indirection and code volume
- semgrep rule prevents regression

### Negative

- cmd layer has more wiring code (composition root responsibility)
- Port interface additions require coordination between usecase/port and session

### Neutral

- Test files (`*_test.go`) are exempt from the session import prohibition
  to allow integration-style tests that verify adapter behavior
