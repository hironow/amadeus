# Architecture Decision Records

## Shared ADRs (canonical: phonewave)

0001-0005 are reserved. Canonical versions live in [phonewave docs/adr/](https://github.com/hironow/phonewave/tree/main/docs/adr).

| # | Decision | Linear |
|---|----------|--------|
| [0001](https://github.com/hironow/phonewave/blob/main/docs/adr/0001-cobra-cli-framework.md) | cobra CLI Framework Adoption | MY-329 |
| [0002](https://github.com/hironow/phonewave/blob/main/docs/adr/0002-stdio-convention.md) | stdio Convention (stdout=data, stderr=logs) | MY-339 |
| [0003](https://github.com/hironow/phonewave/blob/main/docs/adr/0003-opentelemetry-noop-default.md) | OpenTelemetry noop-default + OTLP HTTP | — |
| [0004](https://github.com/hironow/phonewave/blob/main/docs/adr/0004-dmail-schema-v1.md) | D-Mail Schema v1 Specification | MY-352, MY-353 |
| [0005](https://github.com/hironow/phonewave/blob/main/docs/adr/0005-fsnotify-daemon-design.md) | fsnotify-based File Watch Daemon | — |

## Extended Shared ADRs (S-series, canonical: phonewave)

Canonical versions live in [phonewave docs/adr/](https://github.com/hironow/phonewave/tree/main/docs/adr). Referenced here for discoverability.

| # | Decision | Status |
|---|----------|--------|
| [S0001](https://github.com/hironow/phonewave/blob/main/docs/adr/S0001-logger-root-package-exception.md) | ~~Logger as root package exception~~ | Superseded by S0005 |
| [S0002](https://github.com/hironow/phonewave/blob/main/docs/adr/S0002-event-sourcing-jsonl-pattern.md) | JSONL append-only event sourcing pattern | Accepted |
| [S0003](https://github.com/hironow/phonewave/blob/main/docs/adr/S0003-three-way-approval-contract.md) | Three-way approval contract | Accepted |
| [S0004](https://github.com/hironow/phonewave/blob/main/docs/adr/S0004-layer-architecture-conventions.md) | ~~Layer architecture conventions~~ | Superseded by S0005 |
| [S0005](https://github.com/hironow/phonewave/blob/main/docs/adr/S0005-root-infrastructure-and-layer-conventions.md) | Root infrastructure pattern and layer conventions | Accepted |
| S0011 | SQLite WAL cooperative model for concurrent CLI | Accepted |
| S0012 | Reference data management pattern | Accepted |
| S0013 | COMMAND naming convention (imperative present tense) | Accepted |
| S0014 | POLICY pattern reference implementation | Accepted |
| S0015 | State directory naming convention | Accepted |
| S0016 | Root package file organization | Accepted |
| S0017 | Aggregate root and use case layer | Accepted |
| S0018 | Event Storming alignment and per-tool applicability | Accepted |
| S0019 | Data persistence boundaries (Linear/GitHub/local) | Accepted |

## amadeus-specific ADRs

| # | Decision | Linear |
|---|----------|--------|
| [0006](0006-integrity-verification-pipeline.md) | Integrity Verification Pipeline | MY-366 |
| [0007](0007-four-axis-divergence-scoring.md) | Four-Axis Divergence Scoring | MY-366 |
| [0008](0008-claude-code-judgment-engine.md) | Claude Code as Judgment Engine | MY-366 |
| [0009](0009-world-line-convergence-detection.md) | World Line Convergence Detection | MY-366 |
| [0010](0010-gate-directory-structure.md) | Gate Directory Structure | MY-366 |
| [0011](0011-event-sourcing.md) | Event Sourcing for State Management | — |
| [0012](0012-root-package-file-consolidation.md) | Root Package File Consolidation | — |
| [0013](0013-event-validation-and-lifecycle.md) | Event Validation and Lifecycle Management | — |
| [0014](0014-flat-package-architecture.md) | Flat Package Architecture Decision | — |
| [0015](0015-load-config-to-cmd.md) | Move LoadConfig to internal/cmd | — |
| [0016](0016-root-package-layer-separation.md) | Root package layer separation | — |
