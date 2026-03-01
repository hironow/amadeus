---
dmail-schema-version: "1"
name: feedback-001
kind: feedback
description: ADR integrity violation in authentication module
issues:
    - MY-301
severity: high
metadata:
    created_at: "2026-03-01T10:00:00Z"
    idempotency_key: placeholder
---

ADR 0003 mandates JWT-based authentication, but the recent PR #42 introduces
session-cookie based auth without updating the ADR. This creates a divergence
between documented architecture and implementation.
