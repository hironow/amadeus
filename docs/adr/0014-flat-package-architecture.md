# 0014. Flat Package Architecture Decision

**Date:** 2026-02-25
**Status:** Superseded by [0016](0016-root-package-layer-separation.md)

## Context

sightjack (~22,000 lines) adopted a layered package structure in ADRs 0011 and 0012, splitting its root package into `domain/`, `session/`, `eventsource/`, and moving I/O functions to `internal/`. The question is whether amadeus (~3,500 lines) should follow the same pattern.

This ADR evaluates sightjack ADRs 0011 (ES Layer-First Refactoring) and 0012 (Root Package I/O Cleanup) in the context of amadeus and records the decision.

## Decision

**Do not adopt layered package splitting.** Maintain the flat root package structure.

### Reasons

1. **Scale difference**: amadeus has ~3,500 lines vs sightjack's ~22,000 (roughly 1/6). The import ceremony and package boundary overhead of multi-package architecture outweighs the cohesion benefits at this scale.

2. **`go:embed` constraints**: `state.go` (skill templates) and `claude.go` (prompt templates) use `embed.FS`, which requires the embedded files to be at or below the package directory. These files must remain in the root package.

3. **File-level separation already works**: Pure types and functions (`event.go`, `scoring.go`, `convergence.go`, `config.go`) are naturally separated from I/O adapters (`event_store_file.go`, `projection.go`, `git.go`, `source.go`, `hook.go`, `archive_prune.go`) at the file level.

4. **Change cost**: An estimated 60+ call site updates across 3,500 lines of code would be needed, with no functional benefit.

### File-Level Convention (codified)

Instead of package-level separation, amadeus follows these file-level conventions:

| Category | Files |
|---|---|
| Types + pure functions | `event.go`, `scoring.go`, `convergence.go`, `config.go` (except LoadConfig) |
| I/O adapters | `event_store_file.go`, `projection.go`, `git.go`, `source.go`, `hook.go`, `archive_prune.go` |
| Orchestration | `amadeus.go` (public API entry point) |
| Templates + embed | `state.go`, `claude.go` |

### Re-evaluation Trigger

If amadeus grows beyond ~8,000 lines or introduces a second process (daemon mode), this decision should be re-evaluated.

## Consequences

### Positive
- No unnecessary import ceremony for a small codebase
- All types remain directly accessible without package prefixes
- Developers can understand the full codebase by reading ~15 files
- `go:embed` works without workarounds

### Negative
- File-level conventions are not compiler-enforced (unlike package boundaries)
- If amadeus grows significantly, the flat structure may become harder to navigate

### Neutral
- This decision does not preclude future splitting; it merely defers it until the codebase warrants it
- File-level conventions are documented here and in CLAUDE.md for tooling and human reference
