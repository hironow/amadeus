# S0022. CLI Argument Design Decisions

**Date:** 2026-03-02
**Status:** Superseded by [S0028](S0028-cli-argument-design-actual.md)

## Context

Two deferred items requested CLI argument redesign:

- R4-02: Unify phonewave's variadic path args with other tools
- R4-03: Reconsider paintress's required path arg (default-to-cwd)

Current designs:

- phonewave: `run [paths...]` — variadic (multi-directory watch)
- sightjack: `run <path>` — single required (code review scope)
- paintress: `run <path>` — single required (expedition target)
- amadeus: `check [path]` — optional, defaults to cwd (drift detection)

Each design reflects the tool's operational semantics:

- phonewave watches multiple directories simultaneously → variadic natural
- sightjack/paintress operate on explicit scope → required prevents accidental runs
- amadeus checks drift in current project → cwd default appropriate

Cross-tool unification would force:

- phonewave: lose multi-directory capability, or
- sightjack/paintress: accept cwd default (dangerous implicit behavior)
- amadeus: require explicit path (unnecessary friction for its use case)

## Decision

Maintain per-tool argument design. CLI ergonomics are tool-specific, not
cross-tool. Each tool's argument shape is optimized for its use case.

paintress retains required path to prevent accidental expeditions in wrong
directories. No UX feedback has requested default-to-cwd behavior.

## Consequences

### Positive

- Each tool has optimal UX for its specific use case
- No regression in existing user workflows
- Explicit path requirement prevents costly mistakes (paintress, sightjack)

### Negative

- New users must learn per-tool argument conventions
- No single --help pattern covers all tools

### Neutral

- This decision can be revisited if user feedback demands unification
- ADR serves as guideline for future tool argument design
