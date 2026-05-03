# Contract: Add session expiry enforcement

## Intent
- Prevent expired sessions from authorizing API calls (variant B).

## Domain
- Command: validate session for request.

## Decisions
- Enforce expiry at the handler level instead of middleware.

## Steps
1. Add expiry check inside the handler entry point.
   - Target: `internal/http/handler.go`
   - Acceptance: expired sessions return 401.

## Boundaries
- Do not add OAuth or refresh tokens.

## Evidence
- test: just test
