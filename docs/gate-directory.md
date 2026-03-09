# `.gate/` Directory Structure

Amadeus manages all state under `<repo-root>/.gate/`.
This document describes what each directory/file does, who creates it, and how it flows into the check pipeline.

## Directory Tree

```
.gate/
  .gitignore            # auto-managed by InitGateDir
  config.yaml           # weights, thresholds, full check interval
  events/               # append-only event log (JSONL, daily rotation)
    2026-02-25.jsonl
    ...
  history/              # legacy (no longer written to; see ADR-0011)
  archive/              # permanent immutable D-Mail records
    feedback-001.md
    feedback-002.md
    ...
  skills/
    dmail-sendable/
      SKILL.md          # agent skill manifest (phonewave discovery)
    dmail-readable/
      SKILL.md
  inbox/                # incoming d-mails (specifications, reports)
    *.md
  outbox/               # outgoing d-mails (feedback, auto-sent)
    *.md
  .run/                 # ephemeral runtime projections
    latest.json         # projected from check.completed events
    baseline.json       # projected from baseline.updated events
    sync.json           # projected from dmail.commented events
    consumed.json       # projected from inbox.consumed events
```

## Git Tracking Rules

`.gate/.gitignore` (auto-managed by `InitGateDir`):

```
.run/
outbox/
inbox/
.otel.env
events/
```

| Path | Git Status | Reason |
|------|-----------|--------|
| `config.yaml` | Tracked | Project-level scoring configuration |
| `events/` | Ignored | Append-only event log (single source of truth) |
| `archive/` | Tracked | Permanent immutable record of all D-Mails |
| `skills/` | Tracked | Agent capability manifests for phonewave discovery |
| `.run/` | Ignored | Ephemeral projections (rebuildable from events) |
| `outbox/` | Ignored | Transient; courier picks up and delivers |
| `inbox/` | Ignored | Transient; consumed and archived on check |

## Event Sourcing Architecture

All state mutations flow through the `emit()` method, which appends events to the event store and applies them to projections.

### Event Types

| Event Type | Projection Target | Description |
|---|---|---|
| `check.completed` | `.run/latest.json` | Divergence check result |
| `baseline.updated` | `.run/baseline.json` | Full calibration baseline |
| `force_full_next.set` | `.run/latest.json` | Deferred full scan flag |
| `dmail.generated` | `archive/` + `outbox/` | D-Mail creation |
| `inbox.consumed` | `.run/consumed.json` | Inbox D-Mail processed |
| `dmail.commented` | `.run/sync.json` | D-Mail posted as comment |
| `convergence.detected` | (informational only) | Convergence alert |
| `archive.pruned` | `archive/` (file removal) | Archive cleanup |
| `run.started` | (informational only) | Daemon started |
| `run.stopped` | (informational only) | Daemon stopped |
| `pr_convergence.checked` | (informational only) | PR convergence pipeline completed |

### Rebuild

`.run/` projections and `dmail.generated` D-Mails in `archive/` can be regenerated from events:

```bash
amadeus rebuild
```

**Limitations:**

- Inbox-sourced D-Mails (`inbox.consumed` events) contain only metadata, not the full D-Mail content. These files in `archive/` are NOT reconstructed by rebuild.
- `archive.pruned` events may also reference `events/*.jsonl` files for event log pruning.

Auto-rebuild triggers when `.run/latest.json` is missing but events exist.
Auto-rebuild is skipped when `inbox.consumed` events are present (to avoid losing inbox D-Mails) and in `--dry-run` mode.

## Check Pipeline Data Flow

The `amadeus run` command (daemon mode) executes the divergence check pipeline, PR convergence pipeline, and monitors inbox via fsnotify. The legacy `amadeus check` command runs a single divergence check (deprecated).

### Input Sources

| Source | Path | Reader |
|--------|------|--------|
| Previous scores | `.run/latest.json` | `ProjectionStore.LoadLatest()` |
| Calibration baseline | `.run/baseline.json` | `ProjectionStore.LoadBaseline()` |
| Scoring config | `config.yaml` | `LoadConfig()` |
| Inbox D-Mails | `inbox/*.md` | `ProjectionStore.ScanInbox()` |

### Output Events

| Event | Triggered By | Projection |
|-------|-------------|------------|
| `check.completed` | Check finalization | `.run/latest.json` |
| `baseline.updated` | Full check or shift detection | `.run/baseline.json` |
| `dmail.generated` | Divergence above threshold | `archive/` + `outbox/` |
| `inbox.consumed` | Inbox scan | `.run/consumed.json` |

## D-Mail Lifecycle

```
[External tool]          amadeus (daemon)                 [External tool]
     |                      |                              |
     | writes to inbox/     |                              |
     |--------------------->|                              |
     |                      | MonitorInbox (fsnotify)      |
     |                      |   ReceiveDMailFromInbox()    |
     |                      |   parse -> archive/ (copy)   |
     |                      |   remove from inbox/         |
     |                      |   emit(inbox.consumed)       |
     |                      |   dedup via archive filename  |
     |                      |                              |
     |                      | (check runs, D-Mail generated)
     |                      |                              |
     |                      | emit(dmail.generated):       |
     |                      |   all -> archive/ + outbox/  |
     |                      |                              |
     |                      |              reads outbox/   |
     |                      |----------------------------->|
```

In daemon mode (`amadeus run`), inbox is monitored via fsnotify for real-time D-Mail reception. `ReceiveDMailFromInbox` uses archive-based deduplication (filename existence check via `os.Stat`) to ensure idempotent processing. All D-Mails go directly to `outbox/` regardless of severity. Receiver-side tools (sightjack, paintress) handle their own approval workflows.

## D-Mail File Format

```yaml
---
name: design-feedback-001
kind: design-feedback
description: ADR-003 auth dependency violation
issues:
  - MY-310
severity: high
metadata:
  created_at: "2026-02-20T14:30:00Z"
---

PR #120 introduced a direct import from the cart module in the auth
service, violating the dependency direction defined in ADR-003.
```

| Frontmatter Field | Type | Required | Description |
|-------------------|------|----------|-------------|
| `name` | string | Yes | Unique identifier (`{kind}-{NNN}`) |
| `kind` | string | Yes | `design-feedback`, `implementation-feedback`, `specification`, `report`, `convergence`, or `ci-result` |
| `description` | string | Yes | One-line summary |
| `issues` | []string | No | Related Linear issue IDs |
| `severity` | string | No | `high`, `medium`, or `low` |
| `metadata` | map | No | Arbitrary key-value pairs |

## File Creators

| File | Created By | When |
|------|-----------|------|
| `.gate/` dirs | `InitGateDir` | `amadeus init` or first `amadeus check` |
| `.gitignore` | `InitGateDir` | Init (appends missing entries on upgrade) |
| `config.yaml` | `InitGateDir` | Init (only if absent) |
| `skills/*/SKILL.md` | `InitGateDir` | Init (from `embed.FS` templates, only if absent) |
| `events/*.jsonl` | `FileEventStore.Append` | On each state mutation via `emit()` |
| `.run/latest.json` | `Projector.Apply` | After `check.completed` event |
| `.run/baseline.json` | `Projector.Apply` | After `baseline.updated` event |
| `.run/consumed.json` | `Projector.Apply` | After `inbox.consumed` event |
| `.run/sync.json` | `Projector.Apply` | After `dmail.commented` event |
| `archive/*.md` | `Projector.Apply` + `ScanInbox` | D-Mail creation or inbox consumption |
| `outbox/*.md` | `Projector.Apply` | D-Mail creation (all severities) |
| `inbox/*.md` | External tool (courier) | Before check |

## File Movements

| Operation | From | To | Function |
|-----------|------|----|----------|
| Inbox consume | `inbox/{name}.md` | (deleted after copy to archive) | `ScanInbox` |

## Git Hook

`amadeus install-hook` writes to `.git/hooks/post-merge`:

```bash
#!/bin/sh
# >>> amadeus hook — do not edit this section
amadeus check --quiet 2>/dev/null || true
# <<< amadeus hook
```

- Appended to existing hooks (does not overwrite)
- `amadeus uninstall-hook` removes only the marked section
- Hook file created with `0755` permissions

## Legacy Migration

On first run, `InitGateDir` migrates the old `state/` directory:

| Legacy Path | New Path |
|-------------|----------|
| `.gate/state/latest.json` | `.gate/.run/latest.json` |
| `.gate/state/baseline.json` | `.gate/.run/baseline.json` |

Migration is safe: files are only moved if the destination does not exist. Empty `state/` directory is removed after migration.
