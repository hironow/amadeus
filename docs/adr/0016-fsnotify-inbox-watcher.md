# 0016. fsnotify Inbox Watcher for Run Daemon

**Date:** 2026-03-09
**Status:** Accepted

## Context

amadeus's `run` command operates as a long-running daemon that monitors the inbox directory for D-Mail files. The original implementation used synchronous polling (`ScanInbox` + `time.Sleep(PollInterval)`) inherited from the one-shot `check` command.

sightjack and paintress both use `github.com/fsnotify/fsnotify` for real-time inbox monitoring. amadeus was the only tool still using polling, creating an inconsistency in the D-Mail reception pattern across the TAP ecosystem.

The polling approach had two drawbacks:

1. Latency: D-Mails were only detected at poll intervals (default 5 seconds)
2. Wasted CPU cycles: repeated directory scans even when inbox was empty

## Decision

Replace the polling loop with an fsnotify-based `MonitorInbox` function that returns a `<-chan domain.DMail`. This follows the sightjack channel-based pattern (as opposed to paintress's callback-based pattern) because amadeus's `Run` loop is already structured around `select`, making channel integration natural.

The implementation uses a two-phase approach:

1. **Synchronous drain**: Read existing inbox files before starting the watch goroutine
2. **Async watch**: fsnotify goroutine delivers new arrivals via channel

`ReceiveDMailFromInbox` handles single-file processing with archive-based dedup, shared by both phases.

The existing `ScanInbox` method on `ProjectionStore` is preserved for `RunCheck` (one-shot command) but is no longer used by the `Run` daemon.

`PollInterval` and `DefaultPollInterval` are removed from `RunOptions`.

An `InboxCh` field on the `Amadeus` struct allows test injection of D-Mails without filesystem setup.

## Consequences

### Positive

- Unified D-Mail reception pattern across sightjack, paintress, and amadeus (all fsnotify)
- Real-time D-Mail detection (sub-second latency vs 5-second polling)
- No wasted CPU cycles on empty inbox scans
- Cleaner Run loop (select on channel vs poll-sleep)

### Negative

- fsnotify dependency added (already used by sightjack and paintress)
- Platform-specific behavior: fsnotify relies on OS-level file watchers (inotify on Linux, kqueue on macOS, ReadDirectoryChangesW on Windows)

### Neutral

- `ScanInbox` remains for one-shot `check` command — no breaking change there
- Partial-write resilience: if a Create event fires before the file is fully written, parse fails and the file stays in inbox; subsequent Write events re-trigger processing
