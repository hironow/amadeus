# 0001. cobra CLI Framework Adoption

**Date:** 2026-02-23
**Status:** Accepted

## Context

All four tools (phonewave, sightjack, paintress, amadeus) need a CLI framework
that supports subcommands, persistent flags, and testable command construction.
The tools were initially developed with ad-hoc flag parsing, which led to
inconsistent flag handling and difficult-to-test entry points.

## Decision

Adopt cobra v1.10.2 as the standard CLI framework across all four tools with
the following conventions:

1. **`RunE` over `Run`**: All commands use `RunE` to propagate errors to
   `main.go` for unified exit code handling.
2. **`EnableTraverseRunHooks`**: Set in `init()` (not the constructor) to
   ensure `PersistentPreRunE` fires on all subcommands.
3. **Exported constructor**: `NewRootCommand()` is exported from `internal/cmd/`
   for testability without `os.Exit`.
4. **Persistent flags on root**: Common flags (e.g., `--verbose`) are
   `PersistentFlags` available to all subcommands. Each tool defines its own
   set of tool-specific persistent flags (e.g., `--config`, `--lang`).
5. **POSIX short aliases**: Single-character aliases (`-v`, `-c`, etc.) follow
   POSIX conventions for all persistent flags (MY-334).
6. **Convention enforcement**: Each tool enforces these conventions through
   appropriate mechanisms (e.g., semgrep rules, linter configuration,
   pre-commit hooks). phonewave maintains `.semgrep/cobra.yaml` as the
   canonical rule set.

## Consequences

### Positive

- Consistent CLI behavior across all four tools
- Testable command construction without process execution
- POSIX short aliases improve usability for experienced CLI users
- Convention enforcement prevents regression to prohibited patterns

### Negative

- cobra dependency must be kept in sync across repositories
- Enforcement mechanisms differ per tool (semgrep, go vet, pre-commit hooks)
