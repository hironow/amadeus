# Architecture Decision Records

amadeus の設計判断を記録する ADR ディレクトリ。

## Cross-Tool ADR (phonewave canonical)

共通 ADR は [phonewave `docs/adr/`](https://github.com/hironow/phonewave/tree/main/docs/adr) が canonical source。
amadeus では 0001-0005 を欠番扱いとし、以下を参照のみとする。

| # | Decision | Canonical | Linear |
|---|----------|-----------|--------|
| 0001 | cobra CLI Framework Adoption | phonewave `0001-cobra-cli-framework.md` | MY-329 |
| 0002 | stdio Convention (stdout=data, stderr=logs) | phonewave `0002-stdio-convention.md` | MY-339 |
| 0003 | OpenTelemetry noop-default + OTLP HTTP | phonewave `0003-opentelemetry-noop-default.md` | MY-363 |
| 0004 | D-Mail Schema v1 Specification | phonewave `0004-dmail-schema-v1.md` | MY-352, MY-353 |
| 0005 | fsnotify-based File Watch Daemon | phonewave `0005-fsnotify-daemon-design.md` | MY-363 |

## amadeus ADR (0006~)

| # | Decision | File |
|---|----------|------|
| 0006 | Integrity Verification Pipeline | [0006-integrity-verification-pipeline.md](0006-integrity-verification-pipeline.md) |
| 0007 | Four-Axis Divergence Scoring | [0007-four-axis-divergence-scoring.md](0007-four-axis-divergence-scoring.md) |
| 0008 | Claude Code as Judgment Engine | [0008-claude-code-judgment-engine.md](0008-claude-code-judgment-engine.md) |
| 0009 | World Line Convergence Detection | [0009-world-line-convergence-detection.md](0009-world-line-convergence-detection.md) |
| 0010 | Gate Directory Structure | [0010-gate-directory-structure.md](0010-gate-directory-structure.md) |
