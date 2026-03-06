# S0026. CLI Argument Design (Actual Implementation)

**Date:** 2026-03-03
**Status:** Accepted
**Supersedes:** [S0024](S0024-cli-argument-design-decisions.md)

## Context

S0024 documented intended CLI argument designs, but three of five contracts
diverged from actual implementation:

- phonewave `run`: S0024 said `[paths...]` (variadic), implementation is `NoArgs`
- sightjack `run`/`scan`: S0024 said `<path>` (required), implementation is `[path]` (optional, defaults to cwd)

paintress `run <repo-path>` (required) and amadeus `check [path]` (optional)
matched S0024 and remain unchanged.

## Decision

Document the actual implemented CLI argument contracts. Each tool's primary
subcommand uses the argument shape that matches its operational semantics:

| Tool | Subcommand | Args | Constraint | Rationale |
|------|-----------|------|------------|-----------|
| phonewave | `run` | (none) | `NoArgs` | Daemon reads config for watch directories; no positional args needed |
| sightjack | `scan [path]` | optional | `MaximumNArgs(1)` | Defaults to cwd; explicit path optional for convenience |
| sightjack | `run [path]` | optional | `MaximumNArgs(1)` | Same as scan (convergence mode) |
| paintress | `run <repo-path>` | required | `ExactArgs(1)` | Prevents accidental expeditions in wrong directory |
| amadeus | `check [path]` | optional | `MaximumNArgs(1)` | Drift detection in current project; cwd default appropriate |

### Design Rationale

- **phonewave** takes no positional args because it is a daemon that reads
  watch directories from its config file. Multi-directory watching is a config
  concern, not a CLI argument concern.
- **sightjack** defaults to cwd because scan/run targets the current repository,
  and requiring an explicit path adds friction without preventing mistakes
  (the user is already in the repository).
- **paintress** retains required path because expeditions execute code changes
  and an accidental run in the wrong directory is destructive.
- **amadeus** defaults to cwd for the same reason as sightjack — integrity
  checks are read-only and safe to run in the current directory.

## Consequences

### Positive

- ADR now matches actual implementation — no doc/code mismatch
- Each tool's ergonomics are optimized for its risk profile (destructive vs read-only)

### Negative

- No single argument pattern across all tools
- New users must learn per-tool conventions

### Neutral

- phonewave's config-based approach and paintress's explicit path are the two
  poles; sightjack and amadeus share the optional-with-cwd-default middle ground
