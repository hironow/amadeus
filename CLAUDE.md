# amadeus

## Workflow

- Do NOT use git worktrees (`EnterWorktree`, `isolation: "worktree"`). Work directly on the current branch.

## Repository Structure (ADR 0016: 2-Layer Separation)

Dependency direction: `internal/cmd` → `internal/session` → `amadeus` (root)

### Root package `amadeus` — types, constants, pure functions, go:embed
- `amadeus.go` — DriftError, ExitCode, CheckOptions
- `config.go` — Config type, DefaultConfig, ValidateConfig (LoadConfig is in cmd)
- `convergence.go` — pure convergence algorithm
- `dmail.go` — DMail types, ParseDMail, MarshalDMail, ValidateDMail (pure)
- `event.go` — Event envelope, EventType constants, NewEvent
- `scoring.go` — pure scoring calculation
- `state.go` — CheckType, CheckResult, SkillTemplateFS (go:embed)
- `sync.go` — SyncState, CommentRecord, PendingComment types
- `claude.go` — ClaudeRunner interface, go:embed templates, prompt building (pure)
- `logger.go` — structured logger (root exception per S0001)
- `telemetry.go` — Tracer, InitTracer (noop default + OTLP HTTP exporter)

### `internal/session/` — all filesystem, network, subprocess I/O
- `amadeus.go` — Amadeus orchestrator (RunCheck, PrintLog, PrintSync)
- `event_store_file.go` — FileEventStore (JSONL append-only)
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
- `hook.go`, `archive_prune.go`, `mark_commented.go`, `validate.go`, `update.go`, `version.go`

### Other
- Entry: `cmd/amadeus/main.go` (InitTracer + ExitCode)
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
- No default subcommand (requires explicit subcommand)
- stdio convention (ADR 0002): stdout = machine-readable data (JSON), stderr = human-readable logs

## Test Layout

- Root tests: `*_test.go` colocated (pure function tests only, `package amadeus`)
- Session tests: `internal/session/*_test.go` (I/O tests, `package session`)
- CLI tests: `internal/cmd/*_test.go` (command + doctor check tests, `package cmd`)
- E2E tests: `tests/e2e/` (Docker-based, `//go:build e2e` tag)
  - `tests/e2e/compose-e2e.yaml` — Docker Compose for E2E environment
  - `tests/e2e/fake-claude/` — fixture-based Claude test double (stdin → canned JSON)
  - ClaudeRunner interface: unit tests use `fakeClaudeRunner` DI; E2E uses PATH-level fake binary

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
