# S0001. Cross-Tool Decision Index

**Date:** 2026-02-23
**Status:** Accepted

## Context

The phonewave ecosystem consists of four CLI tools that share common architectural
decisions: phonewave (courier daemon), sightjack (issue scanner), paintress
(autonomous implementer), and amadeus (integrity verifier). These tools were
developed in parallel and converged on shared patterns through cross-tool review
(MY-329, MY-339).

Recording shared decisions in every repository would create duplication and
divergence risk. A single canonical source with cross-references avoids this.

## Decision

Adopt **Option C (hybrid)** for cross-tool ADR management:

1. **phonewave** holds the canonical version of shared ADRs (0001-0005).
2. Each tool maintains its own `docs/adr/` with independent numbering (0001~).
3. Tool-specific ADRs (0006+) live only in the relevant repository.
4. Cross-references use Linear issue numbers (MY-xxx) as stable identifiers.
5. Each tool includes a copy of this index file (`0000-cross-tool-decisions.md`).

## Cross-Tool ADR Index

| # | Decision | Canonical (phonewave) | Linear (impl) | Linear (decision) |
|---|----------|-----------------------|----------------|-------------------|
| 0001 | cobra CLI framework adoption | `docs/adr/0001-cobra-cli-framework.md` | MY-363 | MY-329 |
| 0002 | stdio convention (stdout=data, stderr=logs) | `docs/adr/0002-stdio-convention.md` | MY-363 | MY-339 |
| 0003 | OpenTelemetry noop-default + OTLP HTTP | `docs/adr/0003-opentelemetry-noop-default.md` | MY-363 | — |
| 0004 | D-Mail Schema v1 specification | `docs/adr/0004-dmail-schema-v1.md` | MY-363 | MY-352, MY-353 |
| 0005 | fsnotify-based file watch daemon | `docs/adr/0005-fsnotify-daemon-design.md` | MY-363 | — |

## Tool-Specific ADR Ranges

| Tool | Repository | 0006+ Scope |
|------|-----------|-------------|
| phonewave | `phonewave` | goreleaser, Docker E2E, signal propagation, config-relative state |
| sightjack | `sightjack` | Unix pipe architecture, convergence gate, fake-Claude E2E, Matrix Navigator |
| paintress | `paintress` | Expedition system, per-worker flag isolation, approval contract |
| amadeus | `amadeus` | Pipeline architecture, scoring system, convergence detection, severity routing |

## Consequences

### Positive

- Single source of truth for shared decisions eliminates drift
- Each tool retains autonomy for tool-specific decisions
- Linear issue numbers provide stable cross-references across repositories

### Negative

- phonewave bears the maintenance burden for shared ADR updates
- Other tools must check phonewave for shared decision changes
