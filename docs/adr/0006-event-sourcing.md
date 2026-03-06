# 0006. Event Sourcing for State Management

**Date:** 2026-02-25
**Status:** Accepted

## Context

amadeus managed state through direct file writes scattered across multiple methods (SaveLatest, SaveHistory, SaveBaseline, SaveDMail, MarkCommented). History was stored as individual JSON files in `history/`, with each check writing a separate timestamped file. This approach had several problems:

- State mutations were implicit and hard to trace
- No single source of truth existed; `history/` and `.run/` could become inconsistent
- Rebuilding state from scratch was impossible without re-running all checks
- Adding new derived state required new write paths in multiple locations

The AWS prescriptive guidance Event Sourcing pattern provided a proven model for addressing these issues in append-only, file-based systems.

This decision partially supersedes [0010 (Gate Directory Structure)](0010-gate-directory-structure.md): `history/` is removed and replaced by `events/`. All other aspects of 0010 (`.run/`, `outbox/`, `inbox/`, `archive/`, `skills/`, `.gitignore` policy) remain unchanged.

## Decision

Adopt Event Sourcing with eager projection:

1. **Event Store** (`events/*.jsonl`): Append-only JSONL files with daily rotation serve as the single source of truth. Each event has an envelope (`id`, `type`, `timestamp`, `data`). Event IDs use UUID v4 (`google/uuid`).

   JSONL was chosen over SQLite or a single JSON file because: (a) append-only writes require no read-modify-write cycle, (b) daily file rotation keeps individual files small and greppable, (c) JSONL files are git-trackable plain text, consistent with how amadeus already stores archive and config.

2. **Projector**: A dedicated component that applies events to update materialized projection files in `.run/` and `archive/`. Handles all event types: `check.completed`, `baseline.updated`, `force_full_next.set`, `dmail.generated`, `inbox.consumed`, `dmail.commented`, `convergence.detected`, `archive.pruned`.

3. **Eager Projection + Lazy Rebuild**: The CLI writes events AND immediately updates projections on each command run. A `rebuild` command replays all events to regenerate projections from scratch if needed.

   Eager projection was chosen over pure lazy (rebuild-on-read) because amadeus is a CLI tool where every invocation must be fast. Full event replay on every `amadeus log` or `amadeus sync` read would add unacceptable latency as the event store grows. The trade-off is slightly more disk I/O on writes, which is acceptable since writes (check runs) are infrequent.

4. **emit() pattern**: A single `Amadeus.emit()` method appends to the event store and applies to projections. All state mutations flow through this method.

5. **history/ removed, replaced by events/**: The `check.completed` events in the event store replace the legacy `history/` directory. `loadCheckHistory()` extracts CheckResults from events. `InitGateDir` no longer creates `history/`.

## Consequences

### Positive

- Single source of truth: all state changes are traceable events
- Full rebuild capability: delete `.run/`, run `amadeus rebuild`, state is restored
- Auto-rebuild: if projections are missing but events exist, they are rebuilt automatically
- Forward compatibility: unknown event types are ignored by the projector (`default: return nil`)
- Audit trail: the event log captures the complete history of all mutations

### Negative

- Slightly more disk I/O: events are written in addition to projections (eager projection)
- Event store grows indefinitely (no compaction implemented yet)
- All CLI commands now construct EventStore + Projector, adding initialization overhead

### Neutral

- No package restructuring: Event Sourcing components live in the root `amadeus` package alongside existing code, matching the flat package design
- `events/` is git-tracked (same policy as the former `history/`), keeping the audit trail in version control
