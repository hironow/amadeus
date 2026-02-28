# 0013. Event Validation and Lifecycle Management

**Date:** 2026-02-25
**Status:** Accepted

## Context

After introducing Event Sourcing (ADR 0011), the `FileEventStore.Append()` method accepted any `Event` struct without validation. This meant events with empty Type, zero Timestamp, nil Data, or empty ID could be persisted to the JSONL event log, potentially corrupting the event stream.

Additionally, the event store did not call `fsync` after writes, leaving events vulnerable to loss during process crashes or power failures — the OS write buffer could contain un-flushed data.

Finally, the `archive-prune` CLI command only pruned `.md` files in `archive/`, leaving old `.jsonl` event files to accumulate indefinitely in `events/`.

This ADR follows the same rationale as sightjack ADR 0009, adapted for amadeus's simpler single-process CLI context.

## Decision

### A. Event Validation at Append Level

Add `ValidateEvent(Event) error` that checks:

- ID is non-empty
- Type is non-empty
- Timestamp is non-zero
- Data is non-nil and non-empty

`FileEventStore.Append()` validates all events in a batch **before** any writes. If any event is invalid, the entire batch is rejected with no file mutations.

### B. Per-file fsync

After writing all events to a daily `.jsonl` file and before `Close()`, call `f.Sync()` to flush the OS buffer to disk. This provides crash durability at the cost of slightly increased write latency (acceptable for amadeus's low event throughput of ~5 events/day).

### C. Event File Lifecycle Management

Add `FindExpiredEventFiles(eventsDir, maxAge)` that scans for `.jsonl` files older than `maxAge`, returning `[]PruneCandidate` (reusing the same type as archive pruning).

Integrate event file pruning into the existing `archive-prune` CLI command so a single invocation cleans both `archive/` and `events/`.

### Explicitly Not Adopted (from sightjack ADR 0009)

| Feature | Reason |
|---|---|
| Sequence monotonicity | amadeus uses UUID + timestamp ordering, no session-based sequence numbers |
| Snapshots | Event count is ~5/day; full replay is < 1ms |
| Schema evolution / upcasting | Not needed pre-release (ADR 0011) |
| CQRS | Single process, single projection |
| Correlation/Causation IDs | Single process CLI, no distributed tracing needed |

## Consequences

### Positive

- Invalid events are caught at write time, preventing event stream corruption
- fsync ensures events survive process crashes and power failures
- `archive-prune` now manages the full lifecycle of both archive and event files
- `PruneCandidate` type is reused, keeping the abstraction minimal

### Negative

- fsync adds ~1ms latency per file write (negligible for CLI tool)
- Batch-level validation means one bad event rejects the entire batch (desired behavior for atomicity)

### Neutral

- `ValidateEvent` checks structural validity only (non-empty fields), not semantic validity (e.g., valid EventType values)
- Event file pruning uses the same `--days` threshold as archive pruning
