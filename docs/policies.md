# Policy Engine

PolicyEngine dispatches domain events to registered handlers (best-effort, fire-and-forget).
Errors are logged (if logger is non-nil) but never propagated — `Dispatch()` always returns nil.

## Location

- Engine: `internal/usecase/policy.go`
- Handlers: `internal/usecase/policy_handlers.go`
- Policy definitions: `internal/domain/policy.go`
- Registration: `internal/usecase/check.go` → `registerCheckPolicies()`

## Event → Handler Mapping

| Policy Name | WHEN [EVENT] | THEN [COMMAND] | Side Effects |
|---|---|---|---|
| CheckCompletedGenerateDMail | check.completed | GenerateDMail | Log (Info) + Desktop notification (5s timeout) |
| ConvergenceDetectedNotify | convergence.detected | NotifyConvergence | Log (Debug) |
| InboxConsumedUpdateProjection | inbox.consumed | UpdateProjection | Log (Debug) |
| DMailGeneratedFlushOutbox | dmail.generated | FlushOutbox | Log (Debug) |

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

ConvergenceDetectedNotify, InboxConsumedUpdateProjection,
and DMailGeneratedFlushOutbox are logging-only placeholders.
