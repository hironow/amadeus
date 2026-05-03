# Contract: Add session expiry enforcement

## Intent
- Prevent expired sessions from authorizing API calls (variant A).

## Domain
- Command: validate session for request.

## Decisions
- Enforce expiry in middleware before handler execution.

## Steps
1. Add expiry check to auth middleware.
   - Target: `internal/http/auth_middleware.go`
   - Acceptance: expired sessions return 401.

## Boundaries
- Do not add OAuth or refresh tokens.

## Evidence
- test: just test
