# S0006. fsnotify-based File Watch Daemon

**Date:** 2026-02-23
**Status:** Accepted

## Context

phonewave's core responsibility is delivering D-Mail messages from outbox
directories to the correct inbox directories based on routing rules. This
requires continuous monitoring of file system changes. Polling-based approaches
introduce latency and unnecessary I/O, while inotify/kqueue-based watching
provides near-instant detection.

## Decision

Implement the daemon using fsnotify with the following design:

1. **fsnotify watcher**: Use `github.com/fsnotify/fsnotify` for cross-platform
   file system event monitoring (inotify on Linux, kqueue on macOS).
2. **Startup scan**: On daemon start, scan all outbox directories for
   pre-existing files and deliver them before entering the watch loop. This
   handles files written while the daemon was stopped.
3. **PID file lifecycle**: Write `watch.pid` to the state directory on start,
   remove on graceful shutdown. This enables `phonewave doctor` to check daemon
   health and `kill $(cat watch.pid)` for shutdown.
4. **Delivery log**: Append-only `delivery.log` in the state directory records
   every delivery with timestamp, source, destination, and kind.
5. **Retry with backoff**: Failed deliveries are retried with configurable
   interval and max retries. Files that exhaust retries are moved to a dead
   letter area.
6. **Graceful shutdown**: Context cancellation triggers watcher close, PID file
   removal, and delivery log flush.

## Consequences

### Positive

- Near-instant delivery on file creation (no polling delay)
- Startup scan ensures no messages are lost across daemon restarts
- PID file enables external health checks and clean shutdown

### Negative

- fsnotify has platform-specific behavior (e.g., macOS kqueue has limited
  event granularity compared to Linux inotify)
- Watcher must be added for each outbox directory individually
