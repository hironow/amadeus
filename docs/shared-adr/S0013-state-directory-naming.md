# S0013. State Directory Naming Convention

**Date:** 2026-02-28
**Status:** Accepted

## Context

Each of the 4 CLI tools creates a hidden state directory in the user's project
root to store runtime state, event logs, D-Mail lifecycle data, and SQLite
databases. These directories use tool-specific names rather than a shared
naming pattern.

The naming divergence is intentional: each tool has a distinct identity and
user-facing brand, so the state directory name reflects the tool's domain
concept rather than a generic convention.

## Decision

Each tool uses its own state directory name. This is intentional and not
a deviation requiring unification.

| Tool | State Directory | Domain Concept |
|------|----------------|----------------|
| phonewave | `.phonewave/` | Phone Microwave (name subject) |
| sightjack | `.siren/` | Siren (alert system) |
| paintress | `.expedition/` | Expedition (exploration journey) |
| amadeus | `.gate/` | Gate (convergence checkpoint) |

### Common Internal Structure

Despite different root names, all state directories follow the same internal
structure (per ADR 0009):

```
.{tool}/
  config.yaml          # tool configuration
  inbox/               # incoming D-Mail
  outbox/              # outgoing D-Mail (transactional outbox)
  archive/             # processed D-Mail archive
  events/              # JSONL event store files
  .run/                # SQLite databases (WAL mode)
```

### Configuration

The state directory path is derived from the `--config` flag path
(ADR 0009: config-relative state directory). Tools never use `os.Getwd()`
to determine state location.

## Consequences

### Positive

- Each tool has a clear, recognizable state directory
- No naming collisions when multiple tools operate on the same repository
- Internal structure consistency enables cross-tool knowledge transfer

### Negative

- New contributors must learn each tool's directory name individually

### Neutral

- The `.gitignore` pattern differs per tool but follows the same structure
