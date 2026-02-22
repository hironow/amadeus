# stdio Convention

Amadeus follows the Unix convention of separating machine-readable data from human-readable diagnostics across standard streams.

## Stream Assignment

| Stream | Purpose | Implementation |
|--------|---------|----------------|
| **stdout** | Machine-readable output (JSON, check results) | `Amadeus.DataOut` |
| **stderr** | Human-readable progress, logs, errors | `Amadeus.Logger` |
| **stdin** | Prompt input to Claude CLI subprocess only | `runClaude()` internal |

The core Amadeus library does not read from stdin. Some CLI commands (such as `archive-prune`) optionally read from stdin for confirmations.

## Cobra Wiring

All cobra subcommands MUST use cobra's stream accessors:

```go
logger := amadeus.NewLogger(cmd.ErrOrStderr(), verbose)
a := &amadeus.Amadeus{
    DataOut: cmd.OutOrStdout(),
    Logger:  logger,
}
```

Rules:

- Use `cmd.OutOrStdout()` for `DataOut` — never `os.Stdout` directly
- Use `cmd.ErrOrStderr()` for `Logger` — never `os.Stderr` directly
- This enables cobra's `cmd.SetOut()` / `cmd.SetErr()` for testing

### Exceptions

Direct `os.Stderr` is acceptable only where cobra's `cmd` is unavailable:

| Location | Reason |
|----------|--------|
| `cmd/amadeus/main.go` | Error handling after `root.ExecuteContext()` returns |
| `internal/tools/docgen/main.go` | Standalone tool outside cobra |

## Library Layer

The `Amadeus` struct accepts `DataOut io.Writer` and `Logger *Logger` via dependency injection. This makes the library layer stream-agnostic — callers decide where output goes.

In tests, both are wired to `bytes.Buffer`:

```go
a := &Amadeus{
    DataOut: &dataBuf,
    Logger:  NewLogger(&logBuf, false),
}
```

## Pipeline Compatibility

The stream separation ensures correct behavior in Unix pipelines:

```bash
amadeus check --json | jq '.axes'    # stdout = JSON only
amadeus check --json 2>/dev/null     # suppress stderr logs
amadeus check --json 2>check.log     # split logs to file
```
