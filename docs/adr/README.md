# Architecture Decision Records

## Shared ADRs (canonical: phonewave)

0001-0005 are reserved. Canonical versions live in `phonewave/docs/adr/`.

| # | Decision | Linear |
|---|----------|--------|
| 0001 | cobra CLI framework adoption | MY-329 |
| 0002 | stdio convention (stdout=data, stderr=logs) | MY-339 |
| 0003 | OpenTelemetry noop-default + OTLP HTTP | — |
| 0004 | D-Mail Schema v1 specification | MY-352, MY-353 |
| 0005 | fsnotify daemon design | — |

## Extended Shared ADRs (S-series, canonical: phonewave)

Canonical versions live in phonewave `docs/adr/`. Referenced here for discoverability.

| # | Decision | Status |
|---|----------|--------|
| S0001 | ~~Logger as root package exception~~ | Superseded by S0005 |
| S0002 | JSONL append-only event sourcing pattern | Accepted |
| S0003 | Three-way approval contract | Accepted |
| S0004 | ~~Layer architecture conventions~~ | Superseded by S0005 |
| S0005 | Root infrastructure pattern and layer conventions | Accepted |
| S0011 | SQLite WAL cooperative model for concurrent CLI | Accepted |
| S0012 | Reference data management pattern | Accepted |
| S0013 | COMMAND naming convention (imperative present tense) | Accepted |
| S0014 | POLICY pattern reference implementation | Accepted |
| S0015 | State directory naming convention | Accepted |
| S0016 | Root package file organization | Accepted |
| S0017 | Aggregate root and use case layer | Accepted |
| S0018 | Event Storming alignment and per-tool applicability | Accepted |
| S0019 | Data persistence boundaries (Linear/GitHub/local) | Accepted |
| S0020 | Accepted cross-tool divergence (default subcommand, storage model) | Accepted |
| S0021 | D-Mail receive-side validation (Postel's Law) | Accepted |

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
