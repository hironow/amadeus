# Shared Architecture Decision Records

Cross-tool decisions that apply to all four tools: phonewave, sightjack, paintress, amadeus.

## Canonical Location

This directory (`docs/shared-adr/`) contains shared ADRs that apply across all four tools.
Each tool maintains its own copy in `docs/shared-adr/`. ADR IDs are referenced in `docs/adr/README.md`, `.semgrep/layers.yaml`, and `docs/conformance.md`.

## Shared ADRs

| # | Decision | Status |
|---|----------|--------|
| [S0001](S0001-cross-tool-decisions.md) | Cross-Tool Decision Index | Accepted |
| [S0002](S0002-cobra-cli-framework.md) | cobra CLI framework adoption | Accepted |
| [S0003](S0003-stdio-convention.md) | stdio convention (stdout=data, stderr=logs) | Accepted |
| [S0004](S0004-opentelemetry-noop-default.md) | OpenTelemetry noop-default + OTLP HTTP | Accepted |
| [S0005](S0005-dmail-schema-v1.md) | D-Mail Schema v1 specification | Accepted |
| [S0006](S0006-fsnotify-daemon-design.md) | fsnotify-based file watch daemon | Accepted |
| [S0007](S0007-root-infrastructure-and-layer-conventions.md) | Root infrastructure and layer conventions | Accepted |
| [S0008](S0008-cmd-eventsource-import-prohibition.md) | cmd-eventsource import prohibition | Accepted |
| [S0009](S0009-sqlite-wal-cooperative-model.md) | SQLite WAL cooperative model for concurrent CLI | Accepted |
| [S0010](S0010-reference-data-management.md) | Reference data management pattern | Accepted |
| [S0011](S0011-command-naming-convention.md) | COMMAND naming convention (imperative present tense) | Accepted |
| [S0012](S0012-policy-pattern-reference.md) | POLICY pattern reference implementation | Accepted |
| [S0013](S0013-state-directory-naming.md) | State directory naming convention | Accepted |
| [S0014](S0014-root-package-organization.md) | Root package file organization | Accepted |
| [S0015](S0015-aggregate-root-and-usecase-layer.md) | Aggregate root and use case layer | Accepted |
| [S0016](S0016-event-storming-alignment.md) | Event Storming alignment and per-tool applicability | Accepted |
| [S0017](S0017-data-persistence-boundaries.md) | ~~Data persistence boundaries (Linear/GitHub/local)~~ | Superseded by S0030 |
| [S0018](S0018-accepted-cross-tool-divergence.md) | Accepted cross-tool divergence (default subcommand, storage model) | Accepted |
| [S0019](S0019-dmail-receive-side-postel-law.md) | D-Mail receive-side validation (Postel's Law) | Accepted |
| [S0020](S0020-otel-metrics-design.md) | OTel Metrics Design | Accepted |
| [S0021](S0021-cross-tool-contract-testing.md) | Cross-Tool Contract Testing | Accepted |
| [S0022](S0022-cli-argument-design-decisions.md) | ~~CLI Argument Design Decisions~~ | Superseded by S0026 |
| [S0023](S0023-event-delivery-guarantee-levels.md) | Event Delivery Guarantee Levels | Accepted |
| [S0024](S0024-domain-model-maturity-assessment.md) | Domain Model Maturity Assessment | Accepted |
| [S0025](S0025-rdra-gap-resolution.md) | RDRA Gap Resolution — D-Mail Protocol Extension | Accepted |
| [S0026](S0026-cli-argument-design-actual.md) | CLI Argument Design (Actual Implementation) — supersedes S0022 | Accepted |
| [S0027](S0027-otel-env-file-backend-config.md) | OTel env-file backend configuration | Accepted |
| [S0028](S0028-usecase-adapter-dependency-inversion.md) | Usecase-adapter dependency inversion | Accepted |
| [S0029](S0029-parse-dont-validate-commands.md) | Parse-don't-validate commands | Accepted |
| [S0030](S0030-insight-data-persistence.md) | Insight Data Persistence — supersedes S0017 | Accepted |
| [S0031](S0031-dmail-context-extension.md) | D-Mail Context Extension — amends S0005 | Accepted |
| [S0032](S0032-cvd-friendly-signal-color-palette.md) | CVD-Friendly Signal Color Palette | Accepted |
| [S0033](S0033-loop-safety-audit-2026-03.md) | Loop Safety Audit (2026-03) | Accepted |
| [S0034](S0034-session-usecase-boundary-clarification.md) | Session-Usecase Boundary Clarification | Accepted |
| [S0035](S0035-dmail-wave-field-extension.md) | D-Mail Wave Field Extension | Accepted |
| [S0036](S0036-cross-tool-correlation-protocol.md) | Cross-Tool Correlation Protocol | Accepted |
| [S0037](S0037-coding-session-abstraction-layer.md) | AI Coding Session Abstraction Layer | Accepted |
| [S0038](S0038-harness-layer.md) | Harness Layer (filter/verifier/policy) | Accepted |
| [S0039](S0039-harness-evolution-loop.md) | Harness Evolution Loop | Accepted |
| [S0040](S0040-event-store-snapshot-architecture.md) | Event Store Snapshot Architecture | Accepted |
| [S0041](S0041-improvement-controller-in-amadeus.md) | Improvement Controller Placement (in amadeus) | Accepted |
