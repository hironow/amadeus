# amadeus

## Workflow

- Do NOT use git worktrees (`EnterWorktree`, `isolation: "worktree"`). Work directly on the current branch.

## Repository Structure

- Entry: `cmd/amadeus/main.go` (InitTracer defer + ExitCode with exit 2 for drift)
- CLI: `internal/cmd/` (cobra v1.10.2, `NewRootCommand()` exported for testability)
- Library: root package `amadeus` (check, validate, dmail, resolve, hook, telemetry, logger)
- OTel: `telemetry.go` (noop default + OTLP HTTP exporter)
- Docker: `docker/compose.yaml` + `docker/jaeger-v2-config.yaml` (Jaeger v2)
- Semgrep: `.semgrep/cobra.yaml` (canonical source is phonewave)
- Release: `.goreleaser.yaml`
- E2E: `tests/e2e/compose-e2e.yaml`

## CLI Design

- `cobra.EnableTraverseRunHooks = true` in `init()` (not constructor)
- All commands use `RunE` (not `Run`)
- `--config`, `--verbose`, `--lang` are PersistentFlags on root
- Exit codes: 0 = success, 1 = error, 2 = drift detected
- No default subcommand (requires explicit subcommand)

## Build & Test

```bash
just build           # build with version from git tags
just install         # build + install to /usr/local/bin
just test            # all tests, 300s timeout
just test-race       # with race detector
just test-e2e        # Docker E2E tests
just check           # fmt + vet + test
just semgrep         # cobra semgrep rules
just lint            # vet + markdown lint + gofmt check
just release-check   # validate goreleaser config
```
