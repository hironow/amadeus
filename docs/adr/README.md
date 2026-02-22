# Architecture Decision Records

amadeus の設計判断を記録する ADR ディレクトリ。

## Cross-Tool ADR (phonewave canonical)

共通 ADR は [phonewave `docs/adr/`](https://github.com/hironow/phonewave/tree/main/docs/adr) が canonical source。
amadeus では 0001-0005 を欠番扱いとし、以下を参照のみとする。

| # | Decision | Linear |
|---|----------|--------|
| [0001](https://github.com/hironow/phonewave/blob/main/docs/adr/0001-cobra-cli-framework.md) | cobra CLI Framework Adoption | MY-329 |
| [0002](https://github.com/hironow/phonewave/blob/main/docs/adr/0002-stdio-convention.md) | stdio Convention (stdout=data, stderr=logs) | MY-339 |
| [0003](https://github.com/hironow/phonewave/blob/main/docs/adr/0003-opentelemetry-noop-default.md) | OpenTelemetry noop-default + OTLP HTTP | MY-363 |
| [0004](https://github.com/hironow/phonewave/blob/main/docs/adr/0004-dmail-schema-v1.md) | D-Mail Schema v1 Specification | MY-352, MY-353 |
| [0005](https://github.com/hironow/phonewave/blob/main/docs/adr/0005-fsnotify-daemon-design.md) | fsnotify-based File Watch Daemon | MY-363 |

## amadeus ADR (0006~)

| # | Decision | Linear |
|---|----------|--------|
| [0006](0006-integrity-verification-pipeline.md) | Integrity Verification Pipeline | MY-366 |
| [0007](0007-four-axis-divergence-scoring.md) | Four-Axis Divergence Scoring | MY-366 |
| [0008](0008-claude-code-judgment-engine.md) | Claude Code as Judgment Engine | MY-366 |
| [0009](0009-world-line-convergence-detection.md) | World Line Convergence Detection | MY-366 |
| [0010](0010-gate-directory-structure.md) | Gate Directory Structure | MY-366 |
