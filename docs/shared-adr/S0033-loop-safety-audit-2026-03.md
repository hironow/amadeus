# S0033. Loop Safety Audit (2026-03)

**Date:** 2026-03-25
**Status:** Accepted

## Context

parent CLAUDE.md Perspective section explicitly requests discovery of infinite loops
and problematic loop patterns across all 4 tools (phonewave, sightjack, paintress,
amadeus). A comprehensive audit was conducted on 2026-03-25.

## Decision

Document the audit results. All loop patterns across all 4 tools are confirmed SAFE.
No remediation required.

### Audit Methodology

1. Search for `for {` (infinite loops) — verify exit conditions
2. Search for `for range` on channels — verify channel closure
3. Search for retry loops — verify max retry limits and backoff caps
4. Search for fsnotify event loops — verify context cancellation
5. Search for ticker-based loops — verify cleanup
6. Search for goroutine spawning — verify bounded concurrency (pond/v2)
7. Search for recursive calls — verify termination

### Loop Pattern Categories Found

#### 1. Daemon Event Loops (phonewave, amadeus, sightjack)

Pattern: `for { select { case <-ctx.Done(): ...; case <-eventCh: ... } }`

| Tool | Location | Exit Conditions |
|------|----------|----------------|
| phonewave | `session/daemon_runner.go` | ctx.Done(), eventCh closed, workerDone |
| amadeus | `session/run.go` | ctx.Done(), inboxCh closed (!ok) |
| sightjack | `session/loop.go` | User quit, WaitTimeout (max 24h cap) |

**Verdict**: All SAFE — context cancellation as primary exit signal.

#### 2. Bounded Concurrency (sightjack, paintress)

Pattern: `pond.Pool` with `StopAndWait()`

| Tool | Location | Safeguard |
|------|----------|-----------|
| sightjack | `session/scanner.go:RunParallel` | pool.StopAndWait(), panic recovery inside Submit |
| paintress | Lumina extraction | pond/v2 ResultPool, bounded workers |

**Verdict**: All SAFE — explicit pool cleanup, no goroutine leaks.

#### 3. Retry Loops (paintress, phonewave)

Pattern: `for attempt := 1; attempt <= maxAttempts; attempt++`

| Tool | Location | Safeguards |
|------|----------|-----------|
| paintress | `session/retry_runner.go` | maxAttempts cap, context timeout, shift capped at 30 |
| phonewave | `domain/retry_backoff.go` | Max 32x base, RecordFailure() caps exponential growth |

**Verdict**: All SAFE — three-layer safeguards (max attempts, context timeout, backoff cap).

#### 4. fsnotify Watchers (paintress, amadeus, sightjack)

Pattern: Goroutine with `for { select { case <-ctx.Done(): case event, ok := <-watcher.Events: } }`

| Tool | Location | Safeguards |
|------|----------|-----------|
| paintress | `session/inbox_watcher.go`, `session/flag_watcher.go` | ctx.Done(), !ok check, defer watcher.Close() |
| amadeus | `session/inbox_watcher.go` | ctx.Done(), !ok check, defer close(ch) |
| sightjack | Flag watcher | Same pattern |

**Verdict**: All SAFE — proper channel checks, deferred cleanup.

#### 5. Expedition Loop (paintress)

Pattern: `for { if ctx.Err() != nil { return } ... if exp >= MaxExpeditions { return } }`

| Tool | Location | Safeguards |
|------|----------|-----------|
| paintress | `session/paintress_expedition.go` | ctx cancellation, MaxExpeditions cap, 5 explicit exit paths, 10s cooldown between expeditions |

**Verdict**: SAFE — bounded by config, multiple exit paths.

#### 6. Interactive Input Loops (sightjack)

Pattern: `for { scanner.Scan() ... }`

| Tool | Location | Safeguards |
|------|----------|-----------|
| sightjack | `cmd/run.go` | ErrQuit break, valid choice break |

**Verdict**: SAFE — user-driven termination.

### Edge Cases Verified

| Edge Case | Finding |
|-----------|---------|
| Multiple workers on single channel | phonewave: single worker goroutine, safe |
| Recursive calls | paintress review_loop: bounded by budget/3-cycle max |
| Channel cleanup on panic | sightjack RunParallel: defer recover() inside Submit |
| Timer leaks | All timers properly stopped (deferred) |
| Goroutine leaks | All watchers defer-close; pond uses StopAndWait() |
| Exponential backoff unbounded | phonewave caps at 32x base, paintress caps shift at 30 |

## Consequences

### Positive

- All loop patterns confirmed safe with multiple safeguards
- No remediation work required
- Audit methodology documented for future reviews

### Negative

- None

### Neutral

- This audit is a point-in-time snapshot; new loops added in future must follow
  established patterns (context cancellation, bounded iteration, channel closure)
