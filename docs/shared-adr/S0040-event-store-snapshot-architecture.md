# ADR S0040: Event Store Snapshot Architecture

**Date:** 2026-04-04
**Status:** Accepted

## Context

All 4 tools use append-only JSONL event stores as the source of truth for domain state. Current pain points:

1. **Unbounded growth**: Event files grow linearly with usage. A long-running go-taskboard session generates thousands of events per day.
2. **Full replay cost**: `LoadAll()` replays every event to rebuild projections. Performance degrades as history grows.
3. **Unsafe truncation**: Codex review (Session 8) confirmed truncating event files corrupts event-sourced state — projections, metrics, and checkpoint tracking all depend on complete history.
4. **Layout inconsistency**: Sightjack uses session-partitioned directories (`events/<sessionID>/YYYY-MM-DD.jsonl`); other tools use flat date-based layout.
5. **No managed DB migration path**: The current `EventStore` interface lacks operations needed for Bigtable/Spanner backends (sequence-based reads, pagination).

### Current EventStore Interface (all 4 tools)

```go
type EventStore interface {
    Append(events ...domain.Event) (domain.AppendResult, error)
    LoadAll() ([]domain.Event, domain.LoadResult, error)
    LoadSince(after time.Time) ([]domain.Event, domain.LoadResult, error)
}
```

### Current Event Struct

```go
type Event struct {
    SchemaVersion uint8           `json:"schema_version,omitempty"`
    ID            string          `json:"id"`
    Type          EventType       `json:"type"`
    Timestamp     time.Time       `json:"timestamp"`
    Data          json.RawMessage `json:"data"`
    SessionID     string          `json:"session_id,omitempty"`
    CorrelationID string          `json:"correlation_id,omitempty"`
    CausationID   string          `json:"causation_id,omitempty"`
    AggregateID   string          `json:"aggregate_id,omitempty"`
    AggregateType string          `json:"aggregate_type,omitempty"`
    SeqNr         uint64          `json:"seq_nr,omitempty"`
}
```

SeqNr is already present but not consistently used as a global sequence number. It is set per-aggregate in some tools.

## Decision

Introduce a **Snapshot + Archive** pattern to all 4 tools, extending the existing port interface to support efficient state recovery and future managed DB migration.

### 1. Extended EventStore Interface

```go
type EventStore interface {
    // Write path (unchanged)
    Append(events ...domain.Event) (domain.AppendResult, error)

    // Read paths
    LoadAll() ([]domain.Event, domain.LoadResult, error)           // Existing: full replay
    LoadSince(after time.Time) ([]domain.Event, domain.LoadResult, error) // Existing: time-windowed
    LoadAfterSeqNr(seqNr uint64) ([]domain.Event, domain.LoadResult, error) // NEW: sequence-based

    // Metadata
    LatestSeqNr() (uint64, error) // NEW: highest recorded SeqNr
}
```

**Rationale**: `LoadAfterSeqNr` is the key primitive for snapshot-based recovery. Bigtable/Spanner row keys are naturally ordered by sequence number, making this operation efficient on any backend.

### 2. SnapshotStore Port (new)

```go
// SnapshotStore persists materialized projection state at a known SeqNr.
// Snapshots are an optimization — the system must function without them
// (falling back to full replay via LoadAll).
type SnapshotStore interface {
    // Save persists a snapshot. aggregateType identifies the projection kind.
    Save(ctx context.Context, aggregateType string, seqNr uint64, state []byte) error

    // Load returns the latest snapshot for the given aggregateType.
    // Returns (0, nil, nil) if no snapshot exists.
    Load(ctx context.Context, aggregateType string) (seqNr uint64, state []byte, err error)
}
```

**Key design choices**:
- `aggregateType` (not `aggregateID`) — snapshots are per projection kind, not per entity. E.g., "amadeus.state", "paintress.expedition_progress".
- `state []byte` — projections serialize themselves (JSON). The store is schema-agnostic.
- Graceful degradation — absent snapshot → full replay. Zero-snapshot is valid initial state.

### 3. File-Based Adapters

```
{stateDir}/
  events/
    YYYY-MM-DD.jsonl          <- hot events (recent)
  snapshots/
    {aggregateType}.json      <- latest snapshot per projection
  archive/
    YYYY-MM-DD.jsonl.gz       <- compressed cold events (optional)
```

**FileSnapshotStore**:
- `Save`: atomic write (temp + rename) to `snapshots/{aggregateType}.json`
- `Load`: read + unmarshal header (seqNr) + body (state bytes)
- File format: `{"seq_nr": 12345, "aggregate_type": "...", "timestamp": "...", "state": <raw>}`

**Archive rotation** (future Phase 3):
- Events before the snapshot's SeqNr can be compressed to `archive/`.
- `archive prune` gains `--archive` flag to move pre-snapshot events.
- Full replay still works by reading `archive/*.jsonl.gz` + `events/*.jsonl`.

### 4. Projection Recovery Flow

```
Before (current):
  events = store.LoadAll()           // O(total_events)
  projection.Rebuild(events)

After (with snapshot):
  seqNr, state, _ := snapshots.Load(ctx, "amadeus.state")
  if state != nil {
      projection.Deserialize(state)      // O(1)
      events, _ = store.LoadAfterSeqNr(seqNr)  // O(recent_events)
  } else {
      events, _ = store.LoadAll()        // fallback: full replay
  }
  projection.Apply(events...)
```

### 5. Global SeqNr Assignment (codex review fix)

**Problem identified by codex review**: An in-process atomic counter initialized from `LatestSeqNr()` at startup fails under multi-process concurrent execution (CLAUDE.md requirement: "複数同時起動が普通にあり得る"). Two processes can read the same initial value and emit duplicate SeqNrs.

**Solution**: Use **SQLite WAL** for globally monotonic SeqNr assignment. All 4 tools already maintain a SQLite database in `{stateDir}/.run/` for the transactional outbox. Add a `seq_counter` table:

```sql
CREATE TABLE IF NOT EXISTS seq_counter (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    next_seq INTEGER NOT NULL DEFAULT 1
);
```

SeqNr allocation:
```go
// AllocSeqNr atomically increments and returns the next global SeqNr.
// Safe for concurrent multi-process use via SQLite WAL locking.
func (db *SeqDB) AllocSeqNr(ctx context.Context) (uint64, error) {
    var seq uint64
    err := db.tx(ctx, func(tx *sql.Tx) error {
        return tx.QueryRow(`UPDATE seq_counter SET next_seq = next_seq + 1
                            RETURNING next_seq - 1`).Scan(&seq)
    })
    return seq, err
}
```

This guarantees:
- Globally monotonic (SQLite WAL serializes concurrent writers)
- No gaps in normal operation (single atomic increment)
- Survives process restart (persisted in SQLite)
- Batch allocation possible: `AllocSeqNrBatch(n int)` returns a range

`SessionRecorder.Record()` calls `AllocSeqNr()` before `Append()`.

**LatestSeqNr()**: Now reads from SQLite (`SELECT next_seq - 1 FROM seq_counter`), not from scanning event files. O(1) regardless of event count.

### 6. LoadAfterSeqNr Order Guarantee (codex review fix)

**Problem identified by codex review**: `LoadAfterSeqNr` must specify strict ordering and deduplication semantics, otherwise projection replay is non-deterministic.

**Specification**:
```go
// LoadAfterSeqNr returns all events with SeqNr > afterSeqNr,
// ordered by SeqNr ascending. Within the same SeqNr (should not happen
// with SQLite-allocated SeqNrs, but defensive), events are ordered by
// Timestamp ascending. Duplicate SeqNrs are included (caller handles
// idempotency). Returns (nil, LoadResult{}, nil) if no events found.
LoadAfterSeqNr(afterSeqNr uint64) ([]domain.Event, domain.LoadResult, error)
```

For FileEventStore: scan all event files (all session dirs for sightjack), filter `SeqNr > afterSeqNr`, sort by SeqNr ascending. This is O(files) for scanning but O(matching_events) for the result set — acceptable because snapshots ensure the matching set is small.

### 7. Sightjack Session Layout Alignment

Sightjack's per-session directory layout is preserved but unified under the same interface:
- `LoadAfterSeqNr` scans all session dirs, merges by SeqNr ascending
- SnapshotStore uses `aggregateType` = `"sightjack.session.{sessionID}"` for per-session snapshots
- Global SeqNr via SQLite is shared across sessions (single `.run/` DB)
- Long-term: consider migrating to flat layout for consistency

### 8. Managed DB Migration Path

The extended interface maps cleanly to managed backends:

| Operation | File | Bigtable | Spanner |
|-----------|------|----------|---------|
| Append | JSONL append | row insert (rowkey=seqNr) | INSERT |
| LoadAfterSeqNr | scan + filter + sort | row range scan | WHERE seq_nr > ? ORDER BY seq_nr |
| LatestSeqNr | SQLite query | reverse scan limit 1 | SELECT MAX(seq_nr) |
| AllocSeqNr | SQLite atomic increment | Bigtable atomic increment | Spanner sequence |
| SnapshotStore.Save | atomic file write | single row upsert | UPSERT |
| SnapshotStore.Load | file read | single row read | SELECT ... LIMIT 1 |

No interface changes needed for migration — only new adapter implementations.

### 9. Existing Data Migration: Cutover Marker (codex review fix)

**Problem identified by codex review**: Existing events have per-aggregate SeqNr (not globally monotonic). Introducing a global SQLite-managed SeqNr without addressing legacy data causes SeqNr collisions.

**Solution**: **Cutover marker** approach (not re-numbering):

1. On first run after upgrade, the system performs a **one-time cutover**:
   - Load all existing events via `LoadAll()` (legacy path)
   - Execute a full projection rebuild from these events
   - Save a snapshot at this point with `SeqNr = 0` (special marker: "includes all pre-cutover events")
   - Initialize `seq_counter.next_seq = 1`
   - Emit a `system.cutover` event with `SeqNr = 1` marking the boundary

2. After cutover:
   - All new events get globally monotonic SeqNr >= 1 from SQLite
   - `LoadAfterSeqNr(0)` returns all post-cutover events
   - Snapshot recovery: deserialize snapshot (which includes all pre-cutover state) + replay SeqNr > snapshot.SeqNr

3. Edge cases:
   - **No existing events**: seq_counter starts at 1, no cutover event needed
   - **Cutover already done**: detected by `SELECT next_seq FROM seq_counter` > 0
   - **Idempotent**: re-running cutover on already-cutover data is a no-op (seq_counter exists)

This means:
- Existing event files are **never modified** (immutability preserved)
- Pre-cutover events have SeqNr = 0 or per-aggregate values (ignored by `LoadAfterSeqNr`)
- Post-cutover events have globally monotonic SeqNr >= 1
- The snapshot at cutover captures the full pre-cutover state

## Implementation Phases

### Phase 1: Interface + FileSnapshotStore + Cutover (this ADR)
- Add `LoadAfterSeqNr`, `LatestSeqNr` to EventStore port (all 4 tools)
- Add `SnapshotStore` port (all 4 tools)
- Implement `FileSnapshotStore` in `internal/eventsource/`
- Add `seq_counter` table to existing SQLite in `.run/`
- Make `SessionRecorder` assign global monotonic SeqNr via SQLite
- FileEventStore: implement `LoadAfterSeqNr` and `LatestSeqNr`
- Implement one-time cutover logic (detect + snapshot + initialize counter)
- **No projection changes yet** — `LoadAll()` still works, cutover runs on first boot

### Phase 2: Projection Snapshot Integration
- Each tool's projection implements `Serialize() []byte` / `Deserialize([]byte) error`
- Recovery flow uses snapshot + delta replay
- `{tool} rebuild` command creates a snapshot after full replay
- Automatic snapshot after every N events (configurable, default 500)

### Phase 3: Archive Rotation + Managed DB
- `archive prune --archive` compresses pre-snapshot events
- BigtableEventStore / SpannerEventStore adapters
- GCSSnapshotStore adapter

## Consequences

### Positive
- Bounded replay cost: O(events since snapshot) instead of O(total events)
- Safe storage management: archive rotation without data corruption
- Clean migration path to managed databases
- Backward compatible: absent snapshot = full replay (no breaking change)

### Negative
- Snapshot serialization adds coupling between projection and storage format
- Global SeqNr requires careful initialization on startup
- Two stores to manage (events + snapshots) instead of one

### Neutral
- `LoadAll()` remains available for full audit / debugging
- Existing `archive prune` functionality unchanged
- amadeus's `TrimCheckHistory` remains as a domain-level optimization (orthogonal to snapshots)
