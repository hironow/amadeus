---
dmail-schema-version: "1"
name: corrective-fb-001
kind: implementation-feedback
description: Corrective feedback with improvement metadata
issues:
    - AUTH-301
severity: high
action: escalate
metadata:
    routing_mode: escalate
    target_agent: sightjack
    provider_state: active
    correlation_id: corr-abc-123
    owner_history: amadeus,sightjack
    trace_id: trace-xyz-789
    failure_type: scope_violation
    idempotency_key: placeholder
    created_at: "2026-04-07T12:00:00Z"
---

Authentication module scope violation detected.
Escalating to sightjack for design-level review.
