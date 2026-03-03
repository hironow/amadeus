# amadeus

## Workflow

- Do NOT use git worktrees (`EnterWorktree`, `isolation: "worktree"`). Work directly on the current branch.

## Repository Structure (ADR 0016: 3-Layer Separation)

Dependency direction: `internal/cmd` → `internal/usecase` → `internal/session` → `internal/eventsource` → `internal/domain`

### Root package — `doc.go` only (root-zero: all code moved to internal/)

### `internal/domain/` — types, constants, pure functions, go:embed

All former root package contents now reside here (types, interfaces, event/command/policy, go:embed templates, pure functions).

### `internal/eventsource/` — event store infrastructure

- `store_file.go` — FileEventStore (JSONL append-only, implements EventStore)
- `lifecycle.go` — FindExpiredEventFiles, PruneEventFiles (flat `.jsonl` storage, `os.Remove` pruning)
- `path.go` — EventsDir path helper

### `internal/session/` — all filesystem, network, subprocess I/O

- `amadeus.go` — Amadeus orchestrator (RunCheck, PrintLog, PrintSync)
- `projection.go` — Projector (event replay to materialized state)
- `state.go` — ProjectionStore, InitGateDir, Save/Load operations
- `dmail_io.go` — D-Mail file I/O (archive, inbox, outbox, consumed.json)
- `sync_io.go` — sync state persistence
- `git.go` — GitClient (subprocess)
- `reading_steiner.go` — repository state inspection
- `source.go` — content collection (ADRs, DoDs, go.mod)
- `claude.go` — DefaultClaudeRunner (subprocess)
- `hook.go` — git hook file management
- `archive_prune.go` — archive file discovery/deletion

### `internal/cmd/` — cobra CLI commands

- `root.go` — NewRootCommand, PersistentFlags
- `check.go`, `sync.go`, `log.go`, `init.go`, `rebuild.go` — subcommands
- `doctor.go` + `doctor_checks.go` — health check command + all check logic
- `config.go` — loadConfig (unexported)
- `telemetry.go` — initTracer (OTLP HTTP exporter setup, shutdown via cobra.OnFinalize)
- `hook.go`, `archive_prune.go`, `mark_commented.go`, `validate.go`, `update.go`, `version.go`

### Other

- Entry: `cmd/amadeus/main.go` (ExitCode, tracer lifecycle via PersistentPreRunE + cobra.OnFinalize)
- Docker: `docker/compose.yaml` + `docker/jaeger-v2-config.yaml` (Jaeger v2)
- ADR: `docs/adr/` (0006~ amadeus-specific; 0001-0005 phonewave canonical)
- Semgrep: `.semgrep/cobra.yaml` (canonical source is phonewave)
- Release: `.goreleaser.yaml`
- E2E: `tests/e2e/compose-e2e.yaml`

## CLI Design

- `cobra.EnableTraverseRunHooks = true` in `init()` (not constructor)
- All commands use `RunE` (not `Run`)
- `--config`, `--verbose`, `--lang` are PersistentFlags on root
- Exit codes: 0 = success, 1 = error, 2 = drift detected
- Default subcommand: `amadeus [flags] <repo>` prepends `check` via `NeedsDefaultCheck`
- stdio convention (ADR 0002): stdout = machine-readable data (JSON), stderr = human-readable logs

## Test Layout

- Root tests: `*_test.go` colocated (pure function tests only, `package amadeus`)
- Eventsource tests: `internal/eventsource/*_test.go` (event store tests, `package eventsource`)
- Session tests: `internal/session/*_test.go` (I/O tests, `package session`)
- CLI tests: `internal/cmd/*_test.go` (command + doctor check tests, `package cmd`)
- E2E tests: `tests/e2e/` (Docker-based, `//go:build e2e` tag)
    - `tests/e2e/compose-e2e.yaml` — Docker Compose for E2E environment
    - `tests/e2e/fake-claude/` — fixture-based Claude test double (stdin → canned JSON)
    - ClaudeRunner interface: unit tests use `fakeClaudeRunner` DI; E2E uses PATH-level fake binary

## Build & Test

```bash
just build              # build with version from git tags
just install            # build + install to /usr/local/bin
just test               # all tests, 300s timeout
just test-race          # with race detector
just test-e2e           # Docker E2E tests
just test-scenario-min  # L1 scenario test (minimal closed loop)
just test-scenario      # L1+L2 scenario tests (CI default)
just test-scenario-all  # all scenario tests (L1-L4)
just check              # fmt + vet + test
just semgrep            # cobra semgrep rules
just lint               # vet + markdown lint + gofmt check
just release-check      # validate goreleaser config
```
