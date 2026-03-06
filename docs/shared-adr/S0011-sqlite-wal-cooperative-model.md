# S0011. SQLite WAL Cooperative Model

**Date:** 2026-02-28
**Status:** Accepted

## Context

All four tools (phonewave, sightjack, paintress, amadeus) use SQLite for the
transactional outbox pattern and error stores. Multiple CLI instances may run
concurrently against the same database (e.g. concurrent `sightjack run` in
different terminals, or `phonewave run` alongside `phonewave status`).

SQLite's default journal mode (DELETE) serializes all access, which causes
`database is locked` errors under concurrent load. We need a concurrency
model that allows:

1. Multiple readers to proceed without blocking
2. Serialized writes with reasonable contention tolerance
3. No data loss under concurrent Stage/Flush operations
4. Compatibility with the at-least-once + idempotent design (root CLAUDE.md)

## Decision

Adopt WAL (Write-Ahead Logging) mode with the following cooperative settings
as the Phase 1 standard for all SQLite databases:

```sql
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA synchronous=NORMAL;
```

Go-level settings:

```go
db.SetMaxOpenConns(1)
```

Write transactions use `BEGIN IMMEDIATE` to acquire the write lock eagerly,
avoiding upgrade-deadlocks between concurrent writers.

### Why These Settings

- **WAL mode**: Concurrent readers do not block writers; writers do not block
  readers. Only one writer at a time, which matches our CLI usage pattern.
- **busy_timeout=5000**: If the write lock is held by another process, the
  caller retries internally for up to 5 seconds before returning
  `SQLITE_BUSY`. This is sufficient for CLI workloads where transactions
  complete in milliseconds.
- **SYNCHRONOUS=NORMAL**: In WAL mode, NORMAL provides durability guarantees
  sufficient for our use case (crash may lose the last WAL commit, but
  at-least-once + idempotent design tolerates this).
- **SetMaxOpenConns(1)**: Prevents connection pool contention within a single
  process. Each CLI instance holds exactly one connection.
- **BEGIN IMMEDIATE**: Acquires the write lock at transaction start rather
  than at first write statement, preventing SQLITE_BUSY mid-transaction.

## Consequences

### Positive

- Concurrent CLI instances coexist without `database is locked` errors
- Read operations (status, log) never block write operations (Stage, Flush)
- Concurrent test suites in all 4 tools pass reliably (verified: phonewave,
  sightjack, amadeus concurrent outbox tests)
- Simple configuration — no external lock manager or coordinator needed

### Negative

- WAL mode creates `-wal` and `-shm` sidecar files alongside the database
- Only one writer at a time per database file (acceptable for CLI tools)
- `busy_timeout=5000` means a blocked writer may wait up to 5 seconds
  before failing (acceptable for interactive CLI, not suitable for
  high-throughput services)

### Neutral

- Each tool independently configures these pragmas in its SQLite open
  function — no shared library (consistent with the independent-tool design)
- At-least-once delivery means duplicate Stage is possible; idempotent Flush
  handles this via `INSERT OR IGNORE` semantics
