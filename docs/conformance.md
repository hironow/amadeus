# What / Why / How Conformance

This is the single source of truth for amadeus's purpose, design rationale, and implementation approach.
Referenced from [README.md](../README.md) and [docs/README.md](README.md).

| Aspect | Description |
|--------|-------------|
| **What** | MCP server + data plane for post-merge integrity review: serves the gate event store / PR-status projection and posts PR review comments |
| **Why** | Give a human-initiated claude-code review session a stable, idempotent data plane over the gate state, without a headless LLM daemon |
| **How** | `amadeus mcp` serves MCP tools (`next_review` reads the gate event store, `get_pr_status` reads the PR-status projection, `post_comment` posts a PR comment via `gh` CLI); data-plane commands (`log` / `sync` / `mark-commented` / `rebuild` / ...) operate on the same event store. The LLM review fires from the claude-code session via the `/review-gate` skill (`plugins/amadeus/skills/review-gate/SKILL.md`; `.gate/skills/` holds D-Mail routing manifests); divergence scoring + D-Mail generation + the waiting-loop daemon are retired. |
| **Input** | Gate event store (recorded checks), PR-status projection, MCP tool arguments; PR comment targets (via `gh` CLI) |
| **Output** | MCP tool responses (latest check / divergence reading / PR status), PR review comments posted to GitHub, insight ledger files (`insights/divergence.md`, `insights/convergence.md`) when present in the event history |
| **Telemetry** | OTel spans on command roots (e.g. `amadeus.doctor`) and MCP tool handlers; `context_budget.*` attributes: `context_budget.tools`, `context_budget.skills`, `context_budget.plugins`, `context_budget.mcp_servers`, `context_budget.hook_bytes`, `context_budget.estimated_tokens` |
| **External Systems** | Git, `gh` CLI (PR reading + comment posting), OTel exporter (Jaeger/Weave), claude-code session (MCP client) |

## Layer Architecture

```
cmd              --> usecase, session, usecase/port, platform, harness, domain  (composition root)
usecase          --> usecase/port, harness, domain                              (output port only)
usecase/port     --> domain (+ stdlib)                                          (interface contracts)
session          --> eventsource, usecase/port, platform, harness, domain       (adapter impl)
harness          --> domain                                                     (LLM mediation facade)
  harness/policy   --> domain                                                   (deterministic decisions, no LLM)
  harness/verifier --> domain, harness/policy                                   (validation rules, no LLM)
  harness/filter   --> domain, harness/verifier, harness/policy                 (LLM action spaces: prompts, response schemas)
eventsource      --> domain                                                     (event persistence adapter)
platform         --> domain (+ stdlib)                                          (cross-cutting infra)
domain           --> (nothing internal, stdlib only)                             (pure types/logic)
```

`harness` mediates between the LLM and the task environment. It is the single import surface for all decision, validation, and specification logic. The facade (`harness.go`) re-exports symbols from three sub-packages ordered by LLM-dependence:

- **`harness/policy`** — Deterministic decisions with no LLM involvement (merge readiness, PR convergence reports, merge method selection)
- **`harness/verifier`** — Validation rules with no LLM involvement (D-Mail schema validation, provider error classification)
- **`harness/filter`** — LLM action spaces: externalized YAML prompts via `PromptRegistry`, response schemas, and GEPA prompt optimization scaffold

Callers import `harness` (the facade), not the sub-packages directly. Semgrep rules in `.semgrep/layers-harness.yaml` enforce the internal dependency order.

`eventsource` is the event persistence adapter based on the [AWS Event Sourcing pattern](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/event-sourcing.html).
Its responsibility is limited to append, load, and replay of domain events.
Event store implementation MUST NOT exist outside `internal/eventsource`.
`session` uses `eventsource` as a client but does not implement event persistence itself.

Key constraints enforced by semgrep (ERROR severity):

- `usecase --> session` PROHIBITED (must use output port interfaces)
- `cmd --> eventsource` PROHIBITED (ADR S0008)
- `domain --> harness` PROHIBITED (domain is pure types/logic)
- `eventsource --> harness` PROHIBITED
- `harness/policy` must not import `harness/verifier` or `harness/filter` (most independent layer)
- `domain` has no I/O, no `context.Context`

Ref: `.semgrep/layers.yaml`, `.semgrep/layers-harness.yaml`, `refs/opsx/semgrep-layer-contract.md`, ADR S0007

## Domain Primitives & Parse-Don't-Validate

Domain command types use the Parse-Don't-Validate pattern:

- Domain primitives (`RepoPath`, `Days`) validate in `New*()` constructors — invalid values are rejected at parse time
- Command types use unexported fields with `New*Command()` constructors that accept only pre-validated primitives
- Commands are always-valid by construction — no `Validate() []error` methods exist
- Usecase layer receives always-valid commands with no validation boilerplate
- Semgrep rule `domain-no-validate-method` prevents reintroduction of `Validate() []error`

Ref: `.semgrep/layers.yaml`, ADR S0029

## MCP Pivot Boundary

Amadeus does not own model inference, score divergence with a headless model call, or run a D-Mail waiting-loop daemon. LLM review is owned by a human-initiated Claude Code session attached to `amadeus mcp`.

- `amadeus mcp` implements the MCP lifecycle (`initialize`, `notifications/initialized`, `tools/list`, `tools/call`) over stdio.
- `refresh_reviews` ingests the GitHub open-PR list (EventPRSnapshotIngested; reviewer write path, refs issue 0032).
- `next_review` serves the review intake contract: oldest snapshot PR without a posted review (review.posted ledger), legacy check fallback.
- `get_pr_status` reads per-PR status from the projection.
- `post_comment` posts via the wired `gh`-backed comment poster and records review.posted.
- `dmail` emits producer-kind D-Mails through the transactional outbox — the only sanctioned emission path.
- Data-plane commands (`log`, `sync`, `mark-commented`, `rebuild`, `status`) operate on local state without invoking an LLM.

Ref: ADR 0026, `internal/session/mcp_server.go`

## Harness Inventory (Track A)

amadeus harness sub-packages and their current function count:

| Sub-package | Functions | Role |
|-------------|-----------|------|
| `harness/policy` | `DetermineCorrectionDecision`, `CorrectiveTargetAgentForFailure`, `DetectOwnerLoop`, `providerStateGate`, `EvaluateExhaustion`, `RunGuard` | Deterministic decisions (routing, exhaustion, run locking) |
| `harness/verifier` | Provider error classification, D-Mail schema validation | Validation rules |
| `harness/filter` | `PromptRegistry`, response schemas, GEPA scaffold | LLM action spaces |

Ref: ADR S0038, S0039, `refs/opsx/semgrep-layer-contract.md`

## Improvement Controller (Track D3/F)

The improvement controller (Weave feedback collector, normalized signal store, corrective policy generation) resides in amadeus session layer. This is an intentional placement decision (ADR S0041).

Key components:

- `session/improvement_collector.go` — Weave feedback polling + normalization
- `session/improvement_signal_store.go` — SQLite sink + outcome transition + aggregation
- `session/improvement_weave_source.go` — HTTP adapter for Weave API

Ref: ADR S0041

## Cross-Tool Conformance

All 4 tools (phonewave, sightjack, paintress, amadeus) maintain a What/Why/How conformance table in `docs/conformance.md` with the same structure. This prevents expression drift across README files.
