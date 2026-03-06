# 0011. Root Package Layer Separation

**Date:** 2026-02-26
**Status:** Accepted

## Context

ADR 0014 (Flat Package Architecture) chose to keep amadeus as a flat root
package based on its ~3,500-line size. Since then, the codebase has grown and
the other three tools (phonewave ADR 0010, sightjack ADR 0011-0012, paintress
ADR 0013) have all adopted 2-layer separation. Maintaining amadeus as the sole
exception creates cognitive overhead when working across tools.

ADR 0014's file-level convention (types vs I/O adapters) already acknowledged
the separation need conceptually. ADR 0015 moved LoadConfig to `internal/cmd/`
as a first step. This ADR completes the migration by moving all remaining I/O
to `internal/session/`.

Go prohibits circular imports: since `internal/session` imports root for types,
root cannot import `internal/session`. This constraint requires I/O to move
atomically per file.

## Decision

Separate the root package into two layers, superseding ADR 0014:

- **Root `amadeus`**: types, constants, pure functions, `go:embed`, logger. No runtime I/O.
- **`internal/session`**: all filesystem, network, and subprocess I/O operations.

This is a 2-layer architecture (not sightjack's 3-layer) because a separate
domain layer is YAGNI for amadeus's size.

`logger.go` stays in root as an intentional exception (shared ADR S0001).
`telemetry.go` stays in root as initialization-time infrastructure.

Dependency direction: `internal/cmd` → `internal/session` → `amadeus` (root).

### Files that stay in root

| File | Reason |
|---|---|
| `config.go` | Pure types + validation (LoadConfig already in cmd per ADR 0015) |
| `convergence.go` | Pure algorithm |
| `event.go` | Domain model types + validation |
| `scoring.go` | Pure calculation logic |
| `logger.go` | Infrastructure exception (S0001) |
| `telemetry.go` | Initialization-time infrastructure |
| `claude.go` | go:embed templates + type definitions + pure parsing (split: I/O moves) |
| `state.go` | go:embed skill templates + type definitions (split: I/O moves) |
| `amadeus.go` | Core types + pure methods (split: I/O orchestration moves) |

### Files that move to internal/session/

| File | Content |
|---|---|
| `archive_prune.go` | Archive file discovery/deletion |
| `dmail.go` | D-Mail file I/O (archive, inbox, outbox, consumed.json) |
| `event_store_file.go` | JSONL event persistence |
| `git.go` | Git subprocess operations |
| `hook.go` | Git hook file management |
| `projection.go` | Event projection to materialized state |
| `reading_steiner.go` | Repository state inspection |
| `source.go` | Content collection (ADRs, DoDs, go.mod) |
| `sync.go` | Sync state persistence |

### Files that move to internal/cmd/

| File | Content |
|---|---|
| `doctor.go` | CLI-specific health checks (already partially in cmd) |

### Files requiring split

| File | Root (types/pure) | Session (I/O) |
|---|---|---|
| `amadeus.go` | DriftError, ExitCode, Amadeus type, pure methods | RunCheck, emit, autoRebuildIfNeeded, saveConvergenceDMails |
| `claude.go` | Types, go:embed, parsing, prompt building | defaultClaudeRunner.Run (subprocess) |
| `state.go` | Types, go:embed, constants | InitGateDir, SaveLatest, SaveBaseline, LoadLatest |

## Consequences

### Positive

- Consistent 2-layer pattern across all four tools
- Root package safe to import without pulling in I/O dependencies
- Clear separation of concerns: types vs operations
- Tests split naturally: pure function tests in root, I/O tests in session

### Negative

- Large migration effort (~14 structural commits)
- `internal/cmd` now imports two packages instead of one
- `logger.go` and `telemetry.go` exceptions break pure types-only rule

### Neutral

- go:embed files must remain at or below root package directory
- Split files require careful interface design at boundary
