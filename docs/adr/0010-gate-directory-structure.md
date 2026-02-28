# 0010. Gate Directory Structure

**Date:** 2026-02-23
**Status:** Superseded by [0011]

## Context

amadeus needs a well-defined directory structure to manage runtime state,
D-Mail lifecycle, and skill definitions. The structure must support git
tracking for persistent records (archive, history) while excluding ephemeral
state (runtime files, transit queues) from version control.

The `pending/` and `rejected/` directories were part of the original design
for manual D-Mail triage but were removed in MY-359 (severity routing
simplification). All D-Mails now route directly to outbox.

## Decision

Use `.gate/` as the root directory with the following layout, managed by
`InitGateDir()`.

### Directory Layout

```
.gate/
  .run/                    # Runtime state (gitignored)
    latest.json            # Current check state
    baseline.json          # Full check baseline
    consumed.json          # Inbox consumption log
  history/                 # Timestamped check results (tracked)
    2026-02-23T150405.json
  outbox/                  # Outbound D-Mails for phonewave (gitignored)
  inbox/                   # Inbound D-Mails from external tools (gitignored)
  archive/                 # Permanent D-Mail record (tracked)
  skills/                  # Skill definitions (tracked)
    dmail-sendable/SKILL.md
    dmail-readable/SKILL.md
  config.yaml              # Configuration file (tracked)
  .gitignore               # Manages tracked vs ephemeral separation
```

### Git Tracking Policy

| Directory | Tracked | Reason |
|-----------|---------|--------|
| `.run/` | No | Ephemeral runtime state, machine-specific |
| `history/` | Yes | Audit trail of all checks |
| `outbox/` | No | Transit queue, consumed by phonewave |
| `inbox/` | No | Transit queue, consumed during Phase 0 |
| `archive/` | Yes | Permanent record of all D-Mails |
| `skills/` | Yes | Skill definitions are part of the project |

`.gitignore` entries: `.run/`, `outbox/`, `inbox/`

### Skill Templates

Default `SKILL.md` files are embedded via `//go:embed templates/skills/*/SKILL.md`
and written on `init` if they do not exist. This ensures new projects have
valid D-Mail skill definitions without manual setup.

### Configuration

`config.yaml` is generated with `DefaultConfig()` defaults on first `init`.
Uses YAML format with sections for `lang`, `weights`, `thresholds`,
`per_axis_override`, `full_check`, and `convergence`.

### State Migration

Legacy `state/` directory (v0.0.11) is automatically migrated to `.run/`
(v0.0.12) by `migrateLegacyState()`. Files are moved only if the destination
does not exist, preventing accidental overwrites.

## Consequences

### Positive
- Clear separation between tracked (audit) and ephemeral (runtime) data
- `archive/` in git provides a complete D-Mail history without external storage
- Embedded skill templates ensure consistent initial setup across projects
- Automatic legacy migration handles version upgrades transparently

### Negative
- `.gate/` directory adds project-level state that some teams may find intrusive
- Gitignore management adds complexity to `InitGateDir()`
- Embedded templates increase binary size (marginally)
