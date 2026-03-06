# S0008. cmd-eventsource Import Prohibition

**Date:** 2026-03-07
**Status:** Accepted

## Context

The composition root (`internal/cmd/`) wires dependencies together. In the
original codebase, some cmd files directly imported eventsource to create
event stores and pass them to session constructors. This created a shortcut
that bypassed the intended layer boundaries: cmd should delegate I/O setup
to the session layer rather than reaching into eventsource directly.

With the port adapter pattern (ADR S0030) and dependency inversion in place,
cmd creates session adapters that internally manage their own eventsource
dependencies. There is no longer any reason for cmd to import eventsource.

## Decision

Prohibit `internal/cmd/` from importing `internal/eventsource/` in all four
tools. This is enforced by a semgrep rule (`layer-cmd-no-import-eventsource`)
at ERROR severity.

### Rationale

1. **Layer boundary preservation**: cmd is the composition root for wiring
   usecase ports to session adapters. It should not reach into implementation
   details of those adapters.
2. **Encapsulation**: Session adapters own the eventsource lifecycle. Moving
   event store creation into session ensures that storage details do not leak
   into the CLI layer.
3. **Consistency**: All other layer prohibitions (e.g., usecase cannot import
   session) follow the same principle of unidirectional dependencies.

## Consequences

### Positive

- cmd layer remains focused on CLI argument parsing and dependency wiring
- Eventsource implementation details are fully encapsulated in session layer
- Consistent with the dependency inversion pattern (ADR S0030)

### Negative

- Session constructors must accept configuration parameters rather than
  pre-built event stores (minor API change)

### Neutral

- The semgrep rule is part of `layers.yaml` which is already maintained
  in all four tools
