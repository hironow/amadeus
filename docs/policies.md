# Policy Engine

PolicyEngine dispatches domain events to registered handlers (best-effort, fire-and-forget).
Errors are logged (if logger is non-nil) but never propagated — `Dispatch()` always returns nil.

## Location

- Engine: `internal/usecase/policy.go` (implements `port.EventDispatcher`)
- Handlers: `internal/usecase/policy_handlers.go` → `registerCheckPolicies()`
- Policy definitions: `internal/domain/policy.go`

## Post jun15 MCP pivot: handlers preserved but unwired

The headless check pipeline that wired these handlers was retired with the MCP
pivot (ADR 0026). `registerCheckPolicies()` has **no production caller today**
(tests exercise it via `export_test.go`); the table below documents the
declarative WHEN/THEN intent. The reactions are driven by the human-initiated
Claude Code session via the `/review-gate` skill and the amadeus MCP tools.

## Event → Handler Mapping

| Policy Name | WHEN [EVENT] | THEN [COMMAND] | Side Effects |
|---|---|---|---|
| CheckCompletedGenerateDMail | check.completed | GenerateDMail | Log (Info) + Desktop Notify + Metrics |
| ConvergenceDetectedNotify | convergence.detected | NotifyConvergence | Log (Info) + Desktop Notify + Metrics |
| InboxConsumedUpdateProjection | inbox.consumed | UpdateProjection | Log (Debug) + Metrics |
| DMailGeneratedFlushOutbox | dmail.generated | FlushOutbox | Log (Debug) + Metrics |

Note: `run.started`, `run.stopped`, and `pr.convergence.checked` events are informational (no policy handlers). They are emitted for observability and event store completeness.

## Event Payload Format

| Event | Payload Type | Fields |
|---|---|---|
| check.completed | `domain.CheckCompletedData` | `Result.Divergence`, `Result.Commit` |
| convergence.detected | (none) | uses `event.Type` |
| inbox.consumed | (none) | uses `event.Type` |
| dmail.generated | (none) | uses `event.Type` |

## Dispatch Guarantee

Best-effort (at-most-once). Handler failures are silently logged.
No retry, no dead-letter queue, no error propagation to callers.

## Skeleton Handlers

InboxConsumedUpdateProjection and DMailGeneratedFlushOutbox are observation-only placeholders
(Debug log + Metrics, no notification).
