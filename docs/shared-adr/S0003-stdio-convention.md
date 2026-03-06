# S0003. stdio Convention (stdout=data, stderr=logs)

**Date:** 2026-02-23
**Status:** Accepted

## Context

CLI tools in the phonewave ecosystem produce both structured output (data) and
human-readable messages (logs, progress, errors). Mixing these on the same
stream breaks pipeline composability (`tool1 | tool2`) and makes output parsing
unreliable. A cross-tool audit (MY-339) confirmed this separation was already
in practice but not formally documented.

## Decision

Enforce the following stdio convention across all four tools:

1. **stdout**: Machine-readable data only (JSON, YAML, file content).
2. **stderr**: Human-readable messages (logs, progress, warnings, errors).
3. **cobra abstraction**: Use `cmd.OutOrStdout()` for data output and
   `cmd.ErrOrStderr()` for log output. Never use `fmt.Println` or `os.Stdout`
   directly in command implementations.
4. **Structured logger**: Each tool provides a logger that writes to stderr.
   The logger is constructed from `cmd.ErrOrStderr()` and injected into the
   application layer. Implementation varies per tool (e.g., phonewave uses
   package-level functions, amadeus/paintress/sightjack use Logger structs).
5. **TTY separation**: When stdin is consumed by a pipe (e.g., JSON input),
   interactive user input must be obtained from `/dev/tty` (or `CONIN$` on
   Windows) directly, not from stdin. This allows `tool1 | tool2` to work
   while tool2 still prompts the user.
6. **JSON mode stdout protection**: When a structured output mode is active
   (e.g., `--output json`), any streaming output (e.g., LLM response text)
   must be redirected to stderr to keep stdout exclusively for the final
   structured JSON result.

## Consequences

### Positive

- Pipeline composability: `phonewave status --json | jq .endpoints`
- Testable output: cobra's `SetOut`/`SetErr` enable buffer-based assertions
- Consistent user experience across all four tools

### Negative

- Developers must consciously choose the correct stream for each output
- Existing code using direct `fmt.Print` requires migration

### Neutral

- `main.go` outside cobra `Execute()` may use `os.Stderr` directly (e.g., fatal
  startup errors before cobra initializes)
- DI default values (e.g., `dataOut = os.Stdout`) are acceptable as they
  represent the production wiring, not direct usage in command implementations
- TTY separation (`/dev/tty`) is currently used by sightjack (select, discuss).
  Other tools should adopt the same pattern when adding interactive features
  to pipe-compatible commands
- JSON mode stdout protection is currently used by paintress (`--output json`
  redirects Claude streaming to stderr). Other tools should follow this pattern
  when adding structured output modes
