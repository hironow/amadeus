# amadeus self-improvement loop

## Purpose

`amadeus` is the verifier and diagnosis side of the 4-tool loop.

It sits on the path:

`specification -> implementation -> verification -> correction`

and is responsible for turning a failed or divergent run into structured corrective feedback instead of a raw retry.

## What this tool now does

`amadeus` now treats corrective feedback as an observable loop, not as one-shot feedback generation.

The current implementation does four things:

1. It generates corrective D-Mails with normalized improvement metadata.
2. It preserves rerun correlation so the next report can be tied back to the original corrective thread.
3. It classifies rerun outcome as `resolved` or `failed_again`.
4. It stores provider pause state in coding session metadata using the shared provider-state vocabulary.
5. It can poll Weave feedback into the improvement ledger when Weave environment variables are present.

## Shared corrective metadata

When `amadeus` emits corrective feedback, it can attach structured metadata such as:

- `failure_type`
- `severity`
- `secondary_type`
- `target_agent`
- `recurrence_count`
- `corrective_action`
- `retry_allowed`
- `escalation_reason`
- `correlation_id`
- `trace_id`
- `outcome`

This metadata is meant to be carried forward by rerun-linked reports so later checks can reason about the same corrective thread.

`amadeus` now emits `severity` and explicitly marks escalation-producing corrective D-Mails as `outcome=escalated`.

## Corrective routing behavior

`amadeus` decides whether a corrective path is still retryable or should move toward escalation.

The current rules are intentionally small:

- explicit candidate action is preserved
- repeated recurrence can disable retry
- high-severity cases can move directly to escalation
- previously disabled retry stays disabled on later reruns

This keeps routing behavior inspectable without making `phonewave` responsible for diagnosis.

## Provider pause model

`amadeus` uses a shared provider-state snapshot for Claude/provider availability:

- `active`
- `waiting`
- `degraded`
- `paused`

Those states are persisted into coding session metadata together with:

- `provider_state`
- `provider_reason`
- `provider_retry_budget`
- `provider_resume_at`
- `provider_resume_when`

This makes pause state machine-readable instead of log-only.

## Current scope

This is the implemented slice, not the final architecture.

What is in:

- observable corrective rerun tracking
- small failure taxonomy for corrective routing
- provider pause state snapshots in session metadata
- Weave feedback collector MVP writing normalized entries into `.gate/insights/improvement-loop.md`
- normalized ingestion for feedback / CI / PR / scorer / trace outcome payloads

What is not in yet:

- a separate improvement controller
- long-horizon learning or policy updates
- richer Weave query/filter configuration beyond the default env-driven poller
