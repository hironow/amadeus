# Contract: Add session expiry enforcement

## Intent
- Prevent expired sessions from authorizing API calls.
- Success means expired sessions return 401 and active sessions continue to work.

## Domain
- Command: validate session for request.
- Event: session validation failed.
- Read model: auth middleware sees session status and expiry timestamp.

## Decisions
- Enforce expiry in middleware before handler execution.
- Reuse the existing session repository instead of adding a cache.

## Steps
1. Add expiry check to auth middleware.
   - Target: `internal/http/auth_middleware.go`
   - Acceptance: expired sessions return 401.
2. Add unit tests for active, expired, and missing sessions.
   - Target: `tests/unit/auth_middleware_test.go`
   - Acceptance: all three cases are covered.

## Boundaries
- Do not add OAuth, refresh tokens, or background cleanup.
- Do not change session table shape.
- Preserve existing error response format.

## Evidence
- test: just test
- lint: just lint
- nfr.p95_latency_ms: <= 200
- Add a regression test for expired sessions.
