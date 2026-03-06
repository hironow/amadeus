# S0005. Root Infrastructure and Layer Conventions

**Date:** 2026-03-07
**Status:** Accepted

## Context

All four tools (phonewave, sightjack, paintress, amadeus) share a common
layered architecture enforced by semgrep rules. Each tool uses the same
seven-layer structure, but the conventions were only documented implicitly
in semgrep rule comments. This ADR formalizes the layer dependency graph
to serve as the single source of truth.

## Decision

Adopt a seven-layer architecture with strict unidirectional dependency rules:

```
cmd -> usecase -> usecase/port -> domain
cmd -> session -> eventsource -> domain
cmd -> platform -> domain
session -> usecase/port
session -> platform
```

### Layer Responsibilities

| Layer | Responsibility |
|-------|---------------|
| cmd | CLI entry point, composition root, dependency wiring |
| usecase | Business logic orchestration, policy engine |
| usecase/port | Interface definitions (ports) for dependency inversion |
| session | Adapter implementations, I/O coordination |
| eventsource | Event persistence (JSONL append-only) |
| platform | Cross-cutting infrastructure (telemetry, logging) |
| domain | Pure types, value objects, validation logic |

### Dependency Rules

- `cmd` may import: usecase, session, usecase/port, platform, domain
- `usecase` may import: usecase/port, domain (session PROHIBITED)
- `session` may import: eventsource, usecase/port, platform, domain
- `eventsource` may import: domain
- `platform` may import: domain
- `domain` may import: nothing (leaf layer, no I/O, no syscalls)

### Enforcement

These rules are enforced via semgrep `layers.yaml` at ERROR severity in all
four tools. Any violation fails CI.

## Consequences

### Positive

- Clear separation of concerns across all four tools
- Dependency inversion between usecase and session via ports
- Domain layer remains pure and testable without I/O dependencies
- Semgrep enforcement prevents accidental layer violations

### Negative

- Additional indirection through port interfaces
- New contributors must understand the layer graph before making changes

### Neutral

- Each tool independently maintains its semgrep rules (consistent with
  the independent-tool design principle)
