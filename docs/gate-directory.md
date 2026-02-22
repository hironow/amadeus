# `.gate/` Directory Structure

Amadeus manages all state under `<repo-root>/.gate/`.
This document describes what each directory/file does, who creates it, and how it flows into the check pipeline.

## Directory Tree

```
.gate/
  .gitignore            # auto-managed by InitGateDir
  config.yaml           # weights, thresholds, full check interval
  history/
    2026-02-20T143005.json    # timestamped check result
    2026-02-20T143005_1.json  # collision suffix if same second
    ...
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
  .run/                 # ephemeral runtime data
    latest.json         # latest check result (baseline for next diff)
    baseline.json       # latest full check result (calibration)
    consumed.json       # processed inbox D-Mails log
```

## Git Tracking Rules

`.gate/.gitignore` (auto-managed by `InitGateDir`):

```
.run/
outbox/
inbox/
```

| Path | Git Status | Reason |
|------|-----------|--------|
| `config.yaml` | Tracked | Project-level scoring configuration |
| `history/` | Tracked | Audit trail of check results over time |
| `archive/` | Tracked | Permanent immutable record of all D-Mails |
| `skills/` | Tracked | Agent capability manifests for phonewave discovery |
| `.run/` | Ignored | Ephemeral runtime state (latest, baseline) |
| `outbox/` | Ignored | Transient; courier picks up and delivers |
| `inbox/` | Ignored | Transient; consumed and archived on check |

## Check Pipeline Data Flow

The `amadeus check` command reads state, evaluates divergence, and routes D-Mails.

### Input Sources

| Source | Path | Reader |
|--------|------|--------|
| Previous scores | `.run/latest.json` | `StateStore.LoadLatest()` |
| Calibration baseline | `.run/baseline.json` | `StateStore.LoadBaseline()` |
| Scoring config | `config.yaml` | `LoadConfig()` |
| Inbox D-Mails | `inbox/*.md` | `StateStore.ScanInbox()` |

### Output Destinations

| Output | Path | Writer |
|--------|------|--------|
| Updated scores | `.run/latest.json` | `StateStore.SaveLatest()` |
| History record | `history/{timestamp}.json` | `StateStore.SaveHistory()` |
| D-Mail (all severities) | `archive/` + `outbox/` | `StateStore.SaveDMail()` |
| Consumed log | `.run/consumed.json` | `StateStore.SaveConsumed()` |

## D-Mail Lifecycle

```
[External tool]          amadeus                      [External tool]
     |                      |                              |
     | writes to inbox/     |                              |
     |--------------------->|                              |
     |                      | ScanInbox()                  |
     |                      |   parse -> archive/ (copy)   |
     |                      |   remove from inbox/         |
     |                      |   SaveConsumed()             |
     |                      |                              |
     |                      | (check runs, D-Mail generated)
     |                      |                              |
     |                      | SaveDMail():                 |
     |                      |   all -> archive/ + outbox/  |
     |                      |                              |
     |                      |              reads outbox/   |
     |                      |----------------------------->|
```

All D-Mails go directly to `outbox/` regardless of severity. Receiver-side tools (sightjack, paintress) handle their own approval workflows.

## D-Mail File Format

```yaml
---
name: feedback-001
kind: feedback
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
| `kind` | string | Yes | `feedback`, `specification`, or `report` |
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
| `history/{ts}.json` | `StateStore.SaveHistory` | After each check |
| `.run/latest.json` | `StateStore.SaveLatest` | After each check |
| `.run/baseline.json` | `StateStore.SaveBaseline` | After each full check |
| `.run/consumed.json` | `StateStore.SaveConsumed` | On inbox consumption during check |
| `archive/*.md` | `StateStore.SaveDMail` + `ScanInbox` | D-Mail creation or inbox consumption |
| `outbox/*.md` | `StateStore.SaveDMail` | D-Mail creation (all severities) |
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
