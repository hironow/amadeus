# S0044. Provider-Adapter Substrate Removal

**Date:** 2026-05-25
**Status:** Accepted

## Context

S0037 introduced a provider-agnostic AI coding session abstraction — a headless
"provider adapter" substrate that wrapped an AI coding CLI as a subprocess,
persisted session metadata around each invocation, retried with exponential
backoff, and recorded circuit-breaker state from stderr. It was built when the
three Claude-invoking tools (sightjack, paintress, amadeus) drove the provider
CLI themselves through a Go-side runner pipeline.

The jun15 MCP pivot retired those headless pipelines. After Phase 1, all three
tools are pure MCP data-planes: the AI coding CLI is the MCP *client* and the
tool exposes state through an MCP server. No tool drives the provider runner
in-process any more, so the entire provider-adapter substrate became orphaned:

- `internal/session/session_tracking_adapter.go` (`SessionTrackingAdapter`)
- `internal/session/provider_adapter_config.go` (`ProviderAdapterConfig`)
- `internal/session/provider_telemetry.go` (`providerStateSpanAttrs`)
- `internal/session/retry_runner.go` (`RetryRunner`)
- `internal/usecase/port/provider_runner.go` (`ProviderRunner` / `RunConfig` /
  `RunOption` / `ApplyOptions` / `With*`)
- the `RunResult` + `DetailedRunner` half of
  `internal/usecase/port/coding_session.go`
- the circuit-breaker block in the per-tool `claude.go` / `paintress.go`
  (`sharedCircuitBreaker`, `SetCircuitBreaker`, `recordCircuitBreaker`,
  `currentProviderState`)

paintress additionally carried a vestigial headless path
(`session/issues.go` `FetchIssuesViaMCP`, `session/project_ops_adapter.go`,
`port.ProjectOps`, and the `fetch_issues` prompt fixture) whose only consumer
was the removed runner; `cmd/mcp.go` already passed a `nil` runner.

The verifiable contract that survives is the session *store* surface used by
the `sessions` command: `CodingSessionStore` + `ListSessionOpts` in the port,
backed by `SQLiteCodingSessionStore`, plus the LIVE session-enter / mcp-config /
stream-normalizer surface (S0037 companion files that are still in use).

## Decision

Remove the orphaned provider-adapter substrate from all three tools and trim the
`coding_session` port to the store/list surface that `sessions` still uses:

1. Delete the substrate files listed above (byte-identical across the three
   tools for the canonical-locked ones).
2. Trim `internal/usecase/port/coding_session.go` to keep only
   `ListSessionOpts` + `CodingSessionStore`; drop `RunResult` + `DetailedRunner`
   and the now-unused `io` / `RunOption` dependency.
3. Trim the per-tool circuit-breaker file: amadeus and paintress delete it
   entirely (fully dead); sightjack keeps only the `newCmd` / `lookPath`
   doctor helpers that `doctor` still uses.
4. paintress only: delete the vestigial `FetchIssuesViaMCP` path
   (`issues.go`, `project_ops_adapter.go`, `port.ProjectOps`, `fetch_issues`
   fixture).
5. Drop the four deleted canonical entries from
   `check_substrate_drift.sh` and re-pin the new normalized hashes for the
   trimmed `coding_session.go` and the superseded `S0037-current-contract.md`.

This ADR supersedes both S0037 documents
(`S0037-coding-session-abstraction-layer.md` decision rationale and
`S0037-current-contract.md` active contract).

## Consequences

### Positive

- The three tools carry no dead headless-runner code; the session surface
  matches the MCP data-plane reality.
- The canonical drift lock shrinks to the surface that is actually live,
  reducing copy-sync burden across tools.
- `sessions enter` / `sessions list` continue to work unchanged via the
  retained `CodingSessionStore` surface.

### Negative

- Re-introducing an in-process provider runner (e.g. a future non-MCP
  provider) would require rebuilding a runner abstraction from scratch rather
  than extending the removed substrate. The git history (S0037 + the deleted
  files) is the reference if that need returns.

### Neutral

- phonewave never invoked AI coding CLIs and is unaffected; it receives this
  ADR per the shared-adr distribution convention.
- dominator is out of scope: it never carried the S0037 provider-adapter
  substrate (NFR-judge persona, reduced canonical subset).
- The removal-record ADR is intentionally NOT added to the canonical drift
  lock; it is a one-time decision document, written byte-identical across the
  three tools for ADR correctness only.
