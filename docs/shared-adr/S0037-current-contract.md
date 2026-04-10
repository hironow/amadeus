# AI Coding Substrate: Current Contract (post-v1)

**Date:** 2026-04-11
**Status:** Active
**Companion ADR:** S0037 (session abstraction decision rationale)

This document freezes the current implementation contract for the AI coding substrate
shared across sightjack, paintress, and amadeus. It is the reference for provider
expansion and copy-sync verification.

## Constructor / Retry / Ownership Matrix

| Aspect | sightjack | paintress | amadeus |
|--------|-----------|-----------|---------|
| Tracked Constructor | `NewTrackedRunner` (exported) | `NewTrackedRunner` (exported) | `claudeRunner()` (lazy singleton) |
| Once Constructor | `NewOnceRunner` (exported) | N/A (expedition-level retry) | N/A |
| Retry in Tracked Path | Yes (RetryRunner) | No (expedition-level) | No (check-cycle-level) |
| Store Ownership | Caller-owned | Instance-owned (CloseRunner) | Instance-owned (CloseRunner) |
| Close Mechanism | Caller scope | `Paintress.CloseRunner()` | `Amadeus.CloseRunner()` |

## Sessions CLI Contract

Resolution order: `--path` → `--config` → `cwd`

Config loading: missing file → default (graceful), malformed YAML → error (fail-fast), empty `claude_cmd` → default.

Canonical errors:
- `"state directory not found: %s (run '<tool> init' first)"`
- `"session %s has no provider session ID"`
- `"session %s has no work directory recorded"`

## Drift Checker Canonical Surface

### Exact-sync production files (checksum verified):
- `internal/domain/coding_session.go`
- `internal/session/session_tracking_adapter.go`
- `internal/session/coding_session_store.go`
- `internal/platform/stream_normalizer.go`
- `internal/harness/policy/run_guard.go`
- `internal/session/session_enter.go`
- `internal/session/mcp_config.go`
- `internal/session/provider_telemetry.go`
- `internal/usecase/port/coding_session.go`
- `docs/shared-adr/S0037-coding-session-abstraction-layer.md`

### Exact-sync test files:
- `internal/session/session_enter_test.go`
- `internal/session/mcp_config_test.go`

### Structural check (signature existence):
- `internal/cmd/sessions_resolve.go` — `resolveSessionsDir` signature

## CMD Test Canonical Case Set

### sessions_enter_test.go:
- ByRecordID
- ByProviderID
- ByConfigFlag
- ByPathFlag
- NoWorkDir
- ConfigBaseIsRepoRoot

### sessions_resolve_test.go:
- PathFlag
- ConfigFlag (tools with --config)
- CwdFallback
- PathOverridesConfig (tools with --config)
- MissingStateDir
- ErrorMessageFormat

## Provider Expansion Checklist

When adding a new provider:
1. Add `Provider` constant to `internal/domain/coding_session.go`
2. Implement `DetailedRunner` interface in session adapter
3. Add resume mechanism in `internal/session/session_enter.go`
4. Add stream normalization in `internal/platform/stream_normalizer.go`
5. Add MCP config isolation in `internal/session/mcp_config.go`
6. Add provider telemetry attributes in `internal/session/provider_telemetry.go`
7. Update `scripts/check-substrate-drift.sh` checksums
8. Run `just substrate-drift-check` on all 3 tools
