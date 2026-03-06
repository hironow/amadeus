# S0014. Root Package File Organization

**Date:** 2026-02-28
**Status:** Accepted

## Context

Each tool's root package (`package {toolname}`) contains types, constants,
pure functions, go:embed resources, and port interfaces. As tools grow, the
number of root files increases (phonewave: 15, amadeus: 16, paintress: 12,
sightjack: 11). A consistent organization pattern improves navigability.

## Decision

Root packages follow these file categories. Not all tools require every
category, but files that exist follow this naming convention.

### Required Files

| File | Content |
|------|---------|
| `{tool}.go` or `types.go` | Primary types and constants |
| `config.go` | Config struct, DefaultConfig, ValidateConfig (pure) |
| `event.go` | Event envelope, EventType constants, EventStore interface |
| `interfaces.go` | All port interfaces (Approver, Notifier, OutboxStore, etc.) |

### Optional Files

| File | Content |
|------|---------|
| `command.go` | Typed COMMAND objects (when introduced) |
| `policy.go` | Policy type and registry |
| `logger.go` | Structured logger (root infrastructure per S0007) |
| `telemetry.go` | Tracer (noop default, root infrastructure per S0007) |
| `metrics.go` | OTel metrics instruments |
| `state.go` | State constants and path helpers |
| `init.go` | go:embed templates, install/render functions |
| `dmail.go` | D-Mail types, ParseDMail, MarshalDMail (pure) |

### Tool-Specific Domain Files

Tools may have additional files for domain-specific pure logic:

- phonewave: `daemon.go`, `delivery.go`, `scanner.go`, `router.go`
- sightjack: `prompt.go`
- paintress: `expedition.go`, `flag.go`, `gradient.go`, `lumina.go`
- amadeus: `convergence.go`, `scoring.go`, `claude.go`, `sync.go`, `git.go`

### Rules

1. Root package files contain only: types, constants, pure functions, go:embed, interfaces
2. No I/O operations (filesystem, network, subprocess) in root package
3. Port interfaces go in `interfaces.go`, not scattered across domain files
4. Logger and telemetry are root infrastructure (per S0007), not session layer

## Consequences

### Positive

- Consistent file discovery across tools
- Clear separation between ports (interfaces.go) and domain types
- New contributors can predict where to find specific constructs

### Negative

- Some tools have a large number of root files (16 for amadeus)

### Neutral

- The exact set of domain files varies per tool, reflecting different problem domains
