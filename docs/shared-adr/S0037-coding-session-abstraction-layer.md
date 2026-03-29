# S0037. AI Coding Session Abstraction Layer

**Date:** 2026-03-30
**Status:** Accepted

## Context

All three Claude-invoking tools (sightjack, paintress, amadeus) execute AI coding CLIs as subprocesses. The provider's native session ID (e.g. Claude Code's `session_id` from stream-json) was captured in OTel spans but never persisted, making it impossible to:

- Resume a specific session by provider ID (`--resume <id>`)
- Track which tool operation produced which provider session
- Abstract over multiple AI coding providers (Claude Code, Codex, Copilot, Gemini CLI, Pi, Kiro)

The AI coding tool landscape is expanding rapidly. A provider-agnostic session management layer enables future provider swaps without rewriting tool internals.

## Decision

Introduce a **provider-agnostic session tracking abstraction** as a shared pattern across all Claude-invoking tools, consisting of:

### Domain Layer (`internal/domain/coding_session.go`)

- `Provider` type with known constants: `claude-code`, `codex`, `copilot`, `gemini-cli`, `pi`, `kiro`
- `SessionStatus`: `running` → `completed` | `failed` | `abandoned`
- `CodingSessionRecord`: ID, ProviderSessionID, Provider, Status, Model, WorkDir, CreatedAt, UpdatedAt, Metadata
- Parse-Don't-Validate constructors for all domain primitives

### Port Layer (`internal/usecase/port/coding_session.go`)

- `DetailedRunner` interface: extends `ClaudeRunner` with `RunDetailed()` returning `RunResult{Text, ProviderSessionID}`
- `CodingSessionStore` interface: Save, Load, FindByProviderSessionID(provider, pid), LatestByProviderSessionID(provider, pid), List, UpdateStatus, Close
- `RunConfig.ResumeSessionID` field + `WithResume(id)` option (mutually exclusive with `WithContinue`)

### Session Layer (`internal/session/`)

- `SQLiteCodingSessionStore`: WAL mode, `busy_timeout=5000`, `MaxOpenConns(1)`, `BEGIN IMMEDIATE` (per S0009)
- `SessionTrackingAdapter`: composes `DetailedRunner` + `CodingSessionStore`, persists session records around each invocation
- `EnterSession`: interactive re-entry via provider CLI with isolation flags, `cmd.Dir` set to `record.WorkDir` (CWD drift prevention)
- `ClaudeAdapter.RunDetailed()`: thin refactor of existing `Run()`, captures `session_id` from stream-json result

### CLI Layer (`internal/cmd/sessions.go`)

- `sessions list [--status X] [--limit N]`: queries session store, outputs table or JSON
- `sessions enter <id> | --provider-id <pid>`: launches provider CLI in interactive mode with `--resume` + isolation flags

### Storage

- SQLite database at `<stateDir>/.run/sessions.db` (alongside existing `outbox.db`)
- Session records are mutable state (status transitions), not domain events — therefore SQLite, not JSONL event store

### Provider Abstraction

- `DetailedRunner` is provider-agnostic; new providers implement the same interface
- Provider-specific resume mechanisms (e.g. `--resume` for Claude, checkpoint for Gemini) are encapsulated in each adapter
- `FindByProviderSessionID` and `LatestByProviderSessionID` require `(provider, pid)` tuple to prevent cross-provider ID collisions

## Consequences

### Positive
- Sessions are queryable and resumable by explicit provider ID
- Provider swap requires only a new adapter implementation, not interface changes
- Session audit trail persisted in SQLite for operational visibility
- Existing `ClaudeRunner` interface preserved; `Run()` is a thin wrapper over `RunDetailed()`

### Negative
- Additional SQLite database per tool per project (minimal disk overhead)
- `sessions enter` bypasses `ClaudeAdapter` (separate code path for interactive mode)

### Neutral
- phonewave does not invoke AI coding CLIs directly but receives this ADR per shared-adr distribution convention
- Pattern is copied (not shared via Go module) per existing cross-tool convention
