# Amadeus MVP Design

**Date:** 2026-02-19
**Status:** Approved

## Decisions

| Item | Decision |
|---|---|
| Language | Go (consistent with Sightjack and Paintress) |
| MVP Scope | Core Engine: Phase 1-3 + `check` command + `.divergence/` state |
| Claude Invocation | Claude CLI as external process (`claude -p --output-format json`) |
| Execution Context | Run from within target repository root |
| Linear Integration | Via Claude Code MCP tools (no Go API client) |

## Architecture: Flat Package + Phase-Oriented Files (Approach A)

### Source Layout

```
amadeus/
├── cmd/amadeus/
│   └── main.go                  # CLI entry point, flag parsing, subcommand dispatch
├── amadeus.go                   # Orchestrator: Phase 1→2→3 control flow
├── reading_steiner.go           # Phase 1: World Line Shift Detection
├── divergence_meter.go          # Phase 2: Integrity Scoring (Claude invocation)
├── dmail.go                     # Phase 3: D-Mail generation + severity routing
├── scoring.go                   # Divergence scoring model (pure functions, no I/O)
├── state.go                     # .divergence/ directory read/write
├── config.go                    # config.yaml parsing + defaults
├── claude.go                    # Claude CLI invocation wrapper
├── git.go                       # Git operations (log, diff, commit hash)
├── logger.go                    # Color-coded, level-based logging
├── templates/
│   ├── diff_check.md.tmpl       # Diff check prompt template
│   └── full_check.md.tmpl       # Full check prompt template
├── justfile
├── go.mod
└── *_test.go                    # Test files adjacent to source
```

### Target Repository State (`.divergence/`)

```
<target-repo>/.divergence/
├── config.yaml
├── state/
│   ├── latest.json
│   └── baseline.json
├── history/
│   └── {timestamp}.json
└── dmails/
    └── d-{NNN}.json
```

## Core Data Models

### Scoring (pure logic, no I/O)

- `Axis`: enum (`adr_integrity`, `dod_fulfillment`, `dependency_integrity`, `implicit_constraints`)
- `AxisScore`: per-axis score (0-100) with details string
- `DivergenceResult`: weighted total (0.000000~1.000000), axes map, severity, override flag
- `Severity`: `LOW` / `MEDIUM` / `HIGH`

### Scoring Functions

- `CalcDivergence(axes, weights) → DivergenceResult`: weighted sum calculation
- `DetermineSeverity(result, thresholds) → Severity`: threshold + per-axis override
- `FormatDivergence(internal) → string`: 6-digit fixed-point display

### Weights and Thresholds

| Axis | Weight |
|---|---|
| ADR Integrity | 40% |
| DoD Fulfillment | 30% |
| Dependency Integrity | 20% |
| Implicit Constraints | 10% |

| Tier | Divergence Range | Behavior |
|---|---|---|
| LOW | < 0.250000 | Auto-sent, no action required |
| MEDIUM | 0.250000 - 0.499999 | Auto-sent + flagged in CLI |
| HIGH | >= 0.500000 | Pending, requires `resolve` |

Per-axis overrides: ADR >= 60 → force HIGH, DoD >= 70 → force HIGH, Dep >= 80 → force MEDIUM.

### D-Mail

- `DMail`: id, severity, status, target (sightjack/paintress), type (Type-S/P/SP), summary, detail, timestamps
- `DMailStatus`: `pending` / `sent` / `approved` / `rejected`
- MVP: saved to `.divergence/dmails/` as JSON. Linear sending via Claude Code MCP.

### Check Result

- `CheckResult`: timestamp, commit hash, type (diff/full), divergence value, axes, evaluated PRs, generated D-Mail IDs

## Execution Flow

### `amadeus check`

1. Load config from `.divergence/config.yaml`
2. Load previous state from `.divergence/state/latest.json`
3. Determine scan mode: `--full` flag OR `check_count_since_full >= interval` → full; else → diff
4. **Phase 1 (Reading Steiner)**: Gather git info (merged PRs, diffs) or full codebase summary
5. If shift is not significant → log and exit
6. **Phase 2 (Divergence Meter)**: Build prompt from template, invoke Claude CLI, parse JSON response, calculate scores via `scoring.go` pure functions
7. **Phase 3 (D-Mail)**: Extract D-Mail candidates from Claude response, apply severity routing
8. Save: `state/latest.json`, `history/{timestamp}.json`, `dmails/d-{NNN}.json`, update `check_count_since_full`
9. Check divergence jump → suggest `--full` if threshold exceeded
10. Print CLI output

### Claude Invocation

Single Claude call handles both Phase 2 and Phase 3. Prompt includes instructions for axis scoring and D-Mail candidate generation. Claude Code uses Linear MCP tools for LOW/MEDIUM D-Mail sending.

```bash
echo "<prompt>" | claude -p --output-format json --model opus
```

### CLI Commands (MVP)

```bash
amadeus check              # diff check (default)
amadeus check --full       # full calibration check
amadeus check --dry-run    # generate prompt only, no Claude execution

# Global flags
-c, --config <path>        # config path (default: .divergence/config.yaml)
-v, --verbose              # verbose logging
--dry-run                  # skip Claude execution
```

### CLI Output Format

```
$ amadeus check

  Reading Steiner: 3 PRs merged since last check
    PR #120 (auth-header-refactor) merged 2h ago
    PR #122 (cart-session-update) merged 1h ago
    PR #125 (payment-timeout-fix) merged 30m ago

  Divergence: 0.048291 (▲ 0.012)
    ADR Integrity:        0.031 — ADR-003 partial violation
    DoD Fulfillment:      0.012 — Issue #42 DoD gap
    Dependency Integrity: 0.005 — minor concern
    Implicit Constraints: 0.000 — clean

  D-Mails:
    [LOW]  #d-041 naming inconsistency → sent to Paintress
    [MED]  #d-042 DoD #42 partial gap → sent, review recommended
    [HIGH] #d-043 ADR-003 auth→cart dep → awaiting approval

  1 pending. Run `amadeus resolve d-043 --approve` or `--reject`
```

## TDD Implementation Order

| Step | File | Focus | Dependencies |
|---|---|---|---|
| 1 | scoring.go | Pure scoring functions | None |
| 2 | config.go | YAML parsing, defaults | None |
| 3 | state.go | `.divergence/` management | None |
| 4 | dmail.go | D-Mail model, ID generation, routing | scoring.go |
| 5 | git.go | Git operations | None (exec) |
| 6 | claude.go | Prompt building, response parsing | None (exec) |
| 7 | reading_steiner.go | Phase 1: shift detection | git.go |
| 8 | divergence_meter.go | Phase 2: scoring orchestration | claude.go, scoring.go |
| 9 | amadeus.go | Orchestrator | All above |
| 10 | cmd/amadeus/main.go | CLI integration | amadeus.go |

## Excluded from MVP

| Feature | Reason |
|---|---|
| `amadeus resolve` | After `check` stabilizes |
| `amadeus log` | Read `history/` JSON directly for now |
| Phase 4 (Convergence) | Needs accumulated D-Mail history |
| merge hook | After CLI stabilizes |
| Retry / backoff | After Claude CLI stability confirmed |
| i18n | English only, add later |
