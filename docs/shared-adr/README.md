# Shared Architecture Decision Records

Cross-tool decisions that apply to all four tools: phonewave, sightjack, paintress, amadeus.

## Canonical Location

This directory (`docs/shared-adr/`) contains shared ADRs that apply across all four tools.
Each tool maintains its own copy in `docs/shared-adr/`. ADR IDs are referenced in `docs/adr/README.md`, `.semgrep/layers.yaml`, and `docs/conformance.md`.
`docs/adr/README.md`, `.semgrep/layers.yaml`, and `docs/conformance.md`.

## Initial Shared ADRs (0000-0005)

| # | Decision |
|---|----------|
| [0000](0000-cross-tool-decisions.md) | Cross-Tool Decision Index |
| [0001](0001-cobra-cli-framework.md) | cobra CLI framework adoption |
| [0002](0002-stdio-convention.md) | stdio convention (stdout=data, stderr=logs) |
| [0003](0003-opentelemetry-noop-default.md) | OpenTelemetry noop-default + OTLP HTTP |
| [0004](0004-dmail-schema-v1.md) | D-Mail Schema v1 specification |
| [0005](0005-fsnotify-daemon-design.md) | fsnotify-based file watch daemon |

## S-series Shared ADRs

| # | Decision | Status |
|---|----------|--------|
| [S0005](S0005-root-infrastructure-and-layer-conventions.md) | Root infrastructure and layer conventions | Accepted |
| [S0008](S0008-cmd-eventsource-import-prohibition.md) | cmd-eventsource import prohibition | Accepted |
| [S0011](S0011-sqlite-wal-cooperative-model.md) | SQLite WAL cooperative model for concurrent CLI | Accepted |
| [S0012](S0012-reference-data-management.md) | Reference data management pattern | Accepted |
| [S0013](S0013-command-naming-convention.md) | COMMAND naming convention (imperative present tense) | Accepted |
| [S0014](S0014-policy-pattern-reference.md) | POLICY pattern reference implementation | Accepted |
| [S0015](S0015-state-directory-naming.md) | State directory naming convention | Accepted |
| [S0016](S0016-root-package-organization.md) | Root package file organization | Accepted |
| [S0017](S0017-aggregate-root-and-usecase-layer.md) | Aggregate root and use case layer | Accepted |
| [S0018](S0018-event-storming-alignment.md) | Event Storming alignment and per-tool applicability | Accepted |
| [S0019](S0019-data-persistence-boundaries.md) | Data persistence boundaries (Linear/GitHub/local) | Accepted |
| [S0020](S0020-accepted-cross-tool-divergence.md) | Accepted cross-tool divergence (default subcommand, storage model) | Accepted |
| [S0021](S0021-dmail-receive-side-postel-law.md) | D-Mail receive-side validation (Postel's Law) | Accepted |
| [S0022](S0022-otel-metrics-design.md) | OTel Metrics Design | Accepted |
| [S0023](S0023-cross-tool-contract-testing.md) | Cross-Tool Contract Testing | Accepted |
| [S0024](S0024-cli-argument-design-decisions.md) | ~~CLI Argument Design Decisions~~ | Superseded by S0028 |
| [S0025](S0025-event-delivery-guarantee-levels.md) | Event Delivery Guarantee Levels | Accepted |
| [S0026](S0026-domain-model-maturity-assessment.md) | Domain Model Maturity Assessment | Accepted |
| [S0027](S0027-rdra-gap-resolution.md) | RDRA Gap Resolution — D-Mail Protocol Extension | Accepted |
| [S0028](S0028-cli-argument-design-actual.md) | CLI Argument Design (Actual Implementation) — supersedes S0024 | Accepted |
| [S0029](S0029-otel-env-file-backend-config.md) | OTel env-file backend configuration | Accepted |
| [S0030](S0030-usecase-adapter-dependency-inversion.md) | Usecase-adapter dependency inversion | Accepted |
| [S0031](S0031-parse-dont-validate-commands.md) | Parse-don't-validate commands | Accepted |
