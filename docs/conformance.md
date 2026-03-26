# What / Why / How Conformance

This is the single source of truth for amadeus's purpose, design rationale, and implementation approach.
Referenced from [README.md](../README.md) and [docs/README.md](README.md).

| Aspect | Description |
|--------|-------------|
| **What** | Post-merge integrity verification system that measures codebase divergence from intended design |
| **Why** | Detect architectural drift early and route corrective actions before design debt compounds |
| **How** | Scan merged PRs → Claude evaluates against ADRs/DoDs → score 4 divergence axes → route D-Mails by severity → enter D-Mail waiting loop (fsnotify inbox/ watch, re-check on arrival); with `--base`: additionally run PR convergence pipeline via `gh` CLI |
| **Input** | Git log (merged PRs), ADRs, DoDs, codebase source, inbox D-Mails; with `--base`: open PR state (via `gh` CLI) |
| **Output** | Divergence scores, corrective D-Mails (design-feedback / implementation-feedback from divergence scoring, works with or without `--base`) to downstream tools, insight ledger files (`insights/divergence.md` with LLM-enriched How, `insights/convergence.md` with archive-enriched Why); with `--base`: additionally PR convergence reports |
| **Telemetry** | OTel spans: `amadeus.run`, `reading_steiner`, `divergence_meter`, `claude.invoke` (with `claude.model`, `claude.timeout_sec`, `gen_ai.*`); `context_budget.*` attributes: `context_budget.tools`, `context_budget.skills`, `context_budget.plugins`, `context_budget.mcp_servers`, `context_budget.hook_bytes`, `context_budget.estimated_tokens` |
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

Ref: `.semgrep/layers.yaml`, ADR S0007

## Domain Primitives & Parse-Don't-Validate

Domain command types use the Parse-Don't-Validate pattern:

- Domain primitives (`RepoPath`, `Days`) validate in `New*()` constructors — invalid values are rejected at parse time
- Command types use unexported fields with `New*Command()` constructors that accept only pre-validated primitives
- Commands are always-valid by construction — no `Validate() []error` methods exist
- Usecase layer receives always-valid commands with no validation boilerplate
- Semgrep rule `domain-no-validate-method` prevents reintroduction of `Validate() []error`

Ref: `.semgrep/layers.yaml`, ADR S0029

## Tracking Mode (Wave vs Linear)

- **Wave mode** (default, `--linear` not set): `checkLinearMCP` in doctor is skipped (status: SKIP, "wave mode"). D-Mail wave field is accepted but not yet used for divergence scoring.
- **Linear mode** (`--linear`): Existing behavior — `checkLinearMCP` validates Linear MCP connection.
- `runDoctor` receives `TrackingMode` parameter to conditionally run mode-specific checks.

Ref: ADR S0035, `internal/cmd/doctor_checks.go`

## Cross-Tool Conformance

All 4 tools (phonewave, sightjack, paintress, amadeus) maintain a What/Why/How conformance table in `docs/conformance.md` with the same structure. This prevents expression drift across README files.
