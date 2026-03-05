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

## Cross-Tool Conformance

All 4 tools (phonewave, sightjack, paintress, amadeus) maintain a What/Why/How conformance table in `docs/conformance.md` with the same structure. This prevents expression drift across README files.
