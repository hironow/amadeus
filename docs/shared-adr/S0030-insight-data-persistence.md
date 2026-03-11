# S0030. Insight Data Persistence (Supersedes S0017)

**Date:** 2026-03-10
**Status:** Accepted

## Context

S0017 established that all state directory contents are gitignored. However, tools
accumulate semantic value through feedback loops (Lumina patterns, Shibito warnings,
Convergence alerts, Divergence trends) that should persist across sessions and be
shared via git.

This value is environment-independent, contains no absolute paths, and is pure
semantic content — distinct from runtime state (`.run/`), events (`events/`), and
transient D-Mails (`inbox/outbox/archive/`).

## Decision

Add a new persistence category **"insight data"** to the data persistence boundaries:

| Category | Location | Git-tracked | Content |
|----------|----------|-------------|---------|
| State/cache | `.run/` | No | SQLite, runtime logs |
| Events | `events/` | No | JSONL event store |
| D-Mail | `inbox/outbox/archive/` | No | Inter-tool messages |
| Config | `config.yaml` | Varies | Tool settings |
| **Insight data** | **`insights/`** | **Yes** | **Semantic knowledge (what/why/how)** |

### Gitignore Strategy

State dir contents are gitignored individually (not the parent dir), allowing
`insights/` to remain tracked:

    # Tool runtime state — individual ignores (insights/ is git-tracked)
    .expedition/.run/
    .expedition/events/
    .expedition/inbox/
    .expedition/outbox/
    .expedition/archive/
    .expedition/journal/
    .expedition/skills/
    .expedition/config.yaml
    .expedition/.otel.env
    .expedition/.gitignore

### Insight File Rules

- Files use `insight-schema-version: "1"` YAML frontmatter
- Content is environment-independent (no absolute paths, no machine-specific data)
- Atomic writes via temp-file + rename; concurrent access via flock
- Lock file stored in `.run/insights.lock` (gitignored)

## Consequences

### Positive

- Accumulated knowledge persists across sessions and developers via git
- Clear separation: insight data is semantic, not runtime state

### Negative

- Gitignore becomes individual-entry pattern instead of parent-dir pattern
- New state subdirectories require gitignore updates

### Neutral

- Supersedes S0017 — all S0017 rules remain except for the new insights category
- `*.local.*` pattern for env-specific data is unchanged
