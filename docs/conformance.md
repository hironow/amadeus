# What / Why / How Conformance

This is the single source of truth for amadeus's purpose, design rationale, and implementation approach.
Referenced from [README.md](../README.md) and [docs/README.md](README.md).

| Aspect | Description |
|--------|-------------|
| **What** | Post-merge integrity verification system that measures codebase divergence from intended design |
| **Why** | Detect architectural drift early and route corrective actions before design debt compounds |
| **How** | Scan merged PRs → Claude evaluates against ADRs/DoDs → score 4 divergence axes → route D-Mails by severity; PR convergence pipeline scores open PRs; fsnotify watches inbox for real-time D-Mail reception |
| **Input** | Git log (merged PRs), ADRs, DoDs, codebase source, open PR state (via `gh` CLI), inbox D-Mails |
| **Output** | Divergence scores, PR convergence reports, corrective D-Mails (design-feedback / implementation-feedback) to downstream tools, insight ledger files (`insights/divergence.md` with LLM-enriched How, `insights/convergence.md` with archive-enriched Why) |
| **Telemetry** | OTel spans: `amadeus.run`, `reading_steiner`, `divergence_meter`, `claude.invoke` (with `claude.model`, `claude.timeout_sec`, `gen_ai.*`) |
| **External Systems** | Claude Code subprocess, Git, `gh` CLI (PR reading), OTel exporter (Jaeger/Weave), fsnotify (inbox watcher) |

## Layer Architecture

```
cmd              --> usecase, session, usecase/port, platform, domain  (composition root)
usecase          --> usecase/port, domain                              (output port only)
usecase/port     --> domain (+ stdlib)                                 (interface contracts)
session          --> eventsource, usecase/port, platform, domain       (adapter impl)
eventsource      --> domain                                            (event persistence adapter)
platform         --> domain (+ stdlib)                                 (cross-cutting infra)
domain           --> (nothing internal, stdlib only)                   (pure types/logic)
```

`eventsource` is the event persistence adapter based on the [AWS Event Sourcing pattern](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/event-sourcing.html).
Its responsibility is limited to append, load, and replay of domain events.
Event store implementation MUST NOT exist outside `internal/eventsource`.
`session` uses `eventsource` as a client but does not implement event persistence itself.

Key constraints enforced by semgrep (ERROR severity):

- `usecase --> session` PROHIBITED (must use output port interfaces)
- `cmd --> eventsource` PROHIBITED (ADR S0008)
- `domain` has no I/O, no `context.Context`

Ref: `.semgrep/layers.yaml`, ADR 0017

## Domain Primitives & Parse-Don't-Validate

Domain command types use the Parse-Don't-Validate pattern:

- Domain primitives (`RepoPath`, `Days`) validate in `New*()` constructors — invalid values are rejected at parse time
- Command types use unexported fields with `New*Command()` constructors that accept only pre-validated primitives
- Commands are always-valid by construction — no `Validate() []error` methods exist
- Usecase layer receives always-valid commands with no validation boilerplate
- Semgrep rule `domain-no-validate-method` prevents reintroduction of `Validate() []error`

Ref: `.semgrep/layers.yaml`, ADR 0018

## Cross-Tool Conformance

All 4 tools (phonewave, sightjack, paintress, amadeus) maintain a What/Why/How conformance table in `docs/conformance.md` with the same structure. This prevents expression drift across README files.
