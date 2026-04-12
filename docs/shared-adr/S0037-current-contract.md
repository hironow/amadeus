# AI Coding Substrate: Current Contract

**Date:** 2026-04-12
**Status:** Active
**Companion ADR:** S0037 (session abstraction decision rationale)
**Authority:** This is the machine-gated active source (`tap/scripts/check_substrate_drift.sh`).
`S0037-coding-session-abstraction-layer.md` is the historical decision rationale (Accepted ADR, not gated).

This document freezes the current implementation contract for the AI coding substrate
shared across sightjack, paintress, and amadeus. It is the reference for provider
expansion and copy-sync verification.

## Factory Contract

### Canonical Helper (copy-synced)

All 3 tools share `WrapWithSessionTracking(runner, baseDir, provider, logger) → (ProviderRunner, *SQLiteCodingSessionStore)`:
- Adds session persistence to a DetailedRunner
- Best-effort: returns `(runner, nil)` when store cannot be opened
- Caller MUST nil-check store before calling `store.Close()`

### Provider Onboarding Pattern

To add a new provider, follow this single pattern:

1. Create an adapter implementing `DetailedRunner`
2. Call `WrapWithSessionTracking(adapter, baseDir, provider, logger)` to get `(ProviderRunner, *Store)`
3. Optionally wrap with `RetryRunner` if role policy requires it (sightjack only)

### ProviderAdapterConfig (copy-synced)

All tools define `ProviderAdapterConfig` with the same shape:

```go
type ProviderAdapterConfig struct {
    Cmd        string // provider CLI command (e.g. "claude")
    Model      string // model name (e.g. "opus")
    TimeoutSec int    // per-invocation timeout (0 = context deadline only)
    BaseDir    string // repository root (state dir parent)
    ToolName   string // tool identifier for stream events
}
```

### Constructor Matrix

| Aspect | sightjack | paintress | amadeus |
|--------|-----------|-----------|---------|
| Tracked Constructor | `NewTrackedRunner(pac, rc, logger)` | `NewTrackedRunner(pac, logger)` | `NewTrackedRunner(pac, logger)` |
| Once Constructor | `NewOnceRunner(pac, logger)` | N/A (expedition-level retry) | N/A |
| Retry in Tracked Path | Yes (RetryRunner via RetryConfig) | No (expedition-level) | No (check-cycle-level) |
| Return Type | `(ProviderRunner, *Store)` | `(ProviderRunner, *Store)` | `(ProviderRunner, *Store)` |
| Store Close | `if store != nil { defer store.Close() }` | `Paintress.CloseRunner()` | `Amadeus.CloseRunner()` |

### Role-Specific Policies

- **Retry placement**: sightjack wraps with RetryRunner inside NewTrackedRunner (extra `RetryConfig` param). paintress/amadeus manage retry at expedition/check-cycle level.
- **Lazy singleton**: amadeus uses `claudeRunner()` (sync.Once) that delegates to `NewTrackedRunner`. Store is instance-owned.
- **NewOnceRunner**: sightjack-only, for side-effect-safe operations (wave apply, classify). Takes `ProviderAdapterConfig` only (same as paintress/amadeus `NewTrackedRunner`).

### RetryRunner.Run Output Contract

On error, `Run` returns `(lastOutput, err)` — partial output from the last attempt is preserved.
Callers MUST NOT assume output is empty when err != nil.

### Telemetry Naming

Provider-generic spans: `provider.invoke`, `provider.model`, `provider.timeout_sec`.
Provider-generic events: `provider.retry`, `provider.blocked`.
Provider-generic event attributes: `provider.error`, `provider.attempt`.
Provider-generic init attributes: `provider.init.model`, `provider.init.mcp_servers`,
`provider.init.tools_count`, `provider.init.skills_count`, `provider.init.plugins_count`.

Future providers add provider-specific child attributes under the same span hierarchy.

### Operator-Facing Schema Boundary

Config key `claude_cmd` (YAML) / `ClaudeCmd` (Go field) is a legacy seam tied to the
current sole provider (Claude). It maps to `ProviderAdapterConfig.Cmd` at the session layer.

When a second provider is onboarded:
1. Introduce `provider_cmd` as the generic key
2. Retain `claude_cmd` as an alias for backwards compatibility
3. Config validation accepts either key

Until then, `claude_cmd` is the canonical operator-facing config key.
Session-layer code (`ProviderAdapterConfig.Cmd`, `EnterConfig.ProviderCmd`) is already generic.

### Claude-Specific Runtime Seams

The following session-layer files are Claude CLI implementation-specific.
They are NOT part of the provider-generic canonical surface.
Second provider onboarding requires replacing or extending these:

| File | Seam | What to replace |
|------|------|-----------------|
| `session_enter.go` | `buildIsolationFlags()` | Provider-specific subprocess flags |
| `session_enter.go` | `ClaudeSettingsPath/Exists` | Provider settings discovery |
| `session_enter.go` | `ResolveMCPConfigPath` | Provider MCP/tool config path |
| `claude_adapter.go` | `ClaudeAdapter` struct | Provider CLI invocation + stream parsing |
| `mcp_config.go` | `ResolveMCPConfigPath` | MCP config path resolution |
| `doctor_claude.go` | `checkClaudeAuth` | Provider auth/health check |

Generic surface (`EnterConfig.ProviderCmd`, `ProviderRunner` interface,
`ProviderAdapterConfig`) is stable — only the implementations listed above
are Claude-specific.

## Sessions CLI Contract

Resolution order: `--path` → `--config` → `cwd`

Config loading: missing file → default (graceful), malformed YAML → error (fail-fast), empty `claude_cmd` → default.

Canonical errors:
- `"state directory not found: %s (run '<tool> init' first)"`
- `"session %s has no provider session ID"`
- `"session %s has no work directory recorded"`

## Drift Checker Canonical Surface

All gap detection runs from `tap/justfile`:
- `just gap-check-ai-coding` — canonical checksum gate
- `just gap-check-ai-sync` — exploratory diff (not a gate)

### Exact-sync production files (checksum verified):
- `internal/domain/coding_session.go`
- `internal/session/session_tracking_adapter.go`
- `internal/session/coding_session_store.go`
- `internal/session/provider_adapter_config.go` (struct only — helpers are tool-specific)
- `internal/platform/stream_normalizer.go`
- `internal/harness/policy/run_guard.go`
- `internal/session/session_enter.go`
- `internal/session/mcp_config.go`
- `internal/session/provider_telemetry.go`
- `internal/usecase/port/coding_session.go`
- `internal/usecase/port/provider_runner.go`
- `docs/shared-adr/S0037-current-contract.md` (this file)

Note: `S0037-coding-session-abstraction-layer.md` is the historical decision rationale (Accepted ADR). It is NOT checksum-gated.

### Exact-sync test files:
- `internal/session/session_enter_test.go`
- `internal/session/mcp_config_test.go`

### Tool-specific (NOT checksum-gated):
- `internal/session/provider_adapter_helpers.go` — assembly helpers (intentionally divergent per tool)
- `internal/session/provider_adapter_config_test.go` — field omission guard tests (test respective helper)

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
3. Call `WrapWithSessionTracking` to compose with session persistence
4. Add resume mechanism in `internal/session/session_enter.go`
5. Add stream normalization in `internal/platform/stream_normalizer.go`
6. Add MCP config isolation in `internal/session/mcp_config.go`
7. Add provider telemetry attributes in `internal/session/provider_telemetry.go`
8. Replace or extend Claude-specific runtime seams (see "Claude-Specific Runtime Seams" section)
9. Update `tap/scripts/check_substrate_drift.sh` checksums
10. Run `cd tap && just gap-check-ai-coding` on all 3 tools
