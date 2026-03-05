# What / Why / How Conformance

This is the single source of truth for amadeus's purpose, design rationale, and implementation approach.
Referenced from [README.md](../README.md) and [docs/README.md](README.md).

| Aspect | Description |
|--------|-------------|
| **What** | Post-merge integrity verification system that measures codebase divergence from intended design |
| **Why** | Detect architectural drift early and route corrective actions before design debt compounds |
| **How** | Scan merged PRs → Claude evaluates against ADRs/DoDs → score 4 divergence axes → route D-Mails by severity |
| **Input** | Git log (merged PRs), ADRs, DoDs, codebase source |
| **Output** | Divergence scores, corrective D-Mails to downstream tools |
| **Telemetry** | OTel spans: `amadeus.check`, `reading_steiner`, `divergence_meter`, `claude.invoke` (with `claude.model`, `claude.timeout_sec`, `gen_ai.*`) |
| **External Systems** | Claude Code subprocess, Git, OTel exporter (Jaeger/Weave) |

## Layer Architecture

```
cmd              --> usecase, session, usecase/port, platform, domain  (composition root)
usecase          --> usecase/port, domain                              (output port only)
usecase/port     --> domain (+ stdlib)                                 (interface contracts)
session          --> eventsource, usecase/port, platform, domain       (adapter impl)
eventsource      --> domain                                            (event store infra)
platform         --> domain (+ stdlib)                                 (cross-cutting infra)
domain           --> (nothing internal, stdlib only)                   (pure types/logic)
```

Key constraints enforced by semgrep (ERROR severity):
- `usecase --> session` PROHIBITED (must use output port interfaces)
- `cmd --> eventsource` PROHIBITED (ADR S0008)
- `domain` has no I/O, no `context.Context`

Ref: `.semgrep/layers.yaml`, ADR 0017

## Cross-Tool Conformance

All 4 tools (phonewave, sightjack, paintress, amadeus) maintain a What/Why/How conformance table in `docs/conformance.md` with the same structure. This prevents expression drift across README files.
