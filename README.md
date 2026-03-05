# Amadeus

**A post-merge integrity verification system that measures how far your codebase has diverged from its intended design.**

Amadeus uses [Claude Code](https://docs.anthropic.com/en/docs/claude-code) to evaluate merged changes against ADRs (Architecture Decision Records) and DoDs (Definitions of Done), scoring divergence across four axes and routing corrective D-Mails to downstream tools when the world line drifts too far.

```bash
amadeus check
```

This single command executes a three-phase pipeline:

1. **Phase 1 (Reading Steiner)** — Detect shifts: scan merged PRs or the full codebase for structural changes
2. **Phase 2 (Divergence Meter)** — Measure divergence: Claude evaluates the changes against ADRs and DoDs, scoring four axes 0-100
3. **Phase 3 (D-Mail)** — Route corrections: generate feedback D-Mails routed by severity

## Why "Amadeus"?

The system design is inspired by [Steins;Gate 0](https://en.wikipedia.org/wiki/Steins;Gate_0), a visual novel by MAGES. (2015).

In the story, Amadeus is an AI system that digitizes human memories — preserving a person's knowledge and personality as data that persists beyond the original. The protagonist Okabe possesses Reading Steiner, the ability to detect when the world line has shifted. He uses the Divergence Meter to quantify how far the current timeline has drifted, and sends D-Mails — short messages to the past — to correct the course of history.

This structure maps directly to post-merge integrity verification:

| Game Concept | Amadeus | Design Meaning |
|---|---|---|
| **Amadeus** | This binary | AI that monitors the "memory" of your codebase's integrity |
| **World Line** | Repository state | The current active timeline of the codebase |
| **Reading Steiner** | Phase 1: Shift detection | Detects that the world line has changed (merged PRs, structural shifts) |
| **Divergence Meter** | Phase 2: Scoring | Measures how far the current state diverges from intended design |
| **Divergence Value** | Weighted score (0-100) | Numerical deviation across four axes |
| **D-Mail** | Phase 3: Corrective messages | Short, targeted actions sent to downstream tools to correct the timeline |
| **Attractor Field** | ADRs + DoDs | Design constraints that pull the world line toward convergence |
| **World Line Convergence** | Target state | All axes at low divergence, codebase aligned with architecture |
| **.gate/** | The device | Persistent state directory that tracks readings across checks |

### Three Design Principles

1. **Measure divergence, don't assume it** — Like Okabe's Divergence Meter, quantify deviation with objective scoring rather than gut feelings.
2. **D-Mail must be actionable** — D-Mails in the story are limited to 36 bytes. Keep corrective actions short, targeted, and specific.
3. **Reading Steiner detects shifts, not causes** — Phase 1 only detects that something changed. Phase 2 evaluates what it means. Don't conflate detection with diagnosis.

---

## Game Mechanics

Three Steins;Gate-inspired mechanics control verification quality:

### Full Check Interval (Calibration Cycle)

Most checks are diff-based (fast, focused on recent PRs). After a configurable number of diff checks, a full calibration scan runs — evaluating the entire codebase from zero.

```
Diff checks: ██████████ 10/10 -> Full calibration triggered
                                  (reset counter, score from zero)
```

- **Interval**: configurable in `config.yaml` (default: every 10 checks)
- **Force**: `amadeus check --full` triggers immediately
- **Auto-trigger**: a divergence jump also forces a full scan on the next run

### Divergence Jump (World Line Shift)

When the divergence score changes by more than a configured threshold between consecutive checks, a "divergence jump" is detected — the world line has shifted significantly.

```
Previous: 0.23  ->  Current: 0.45  ->  Delta: 0.22 > 0.15 threshold
                                        DIVERGENCE JUMP DETECTED
```

- Logs a warning with before/after values
- Forces a full calibration on the next run (to re-evaluate from zero)
- Recorded as an OpenTelemetry event on the `divergence_meter` span

### D-Mail Severity Routing

D-Mails are routed based on severity, determined by the weighted divergence score and per-axis overrides:

```
low    (score <= 0.25) -> Auto-sent
medium (score <= 0.50) -> Auto-sent with elevated priority
high   (score >  0.50) -> Auto-sent (receiver handles approval)
```

All D-Mails go directly to `outbox/` + `archive/`. Receiver-side tools (sightjack, paintress) handle their own approval workflows.

- Per-axis overrides can force high severity for critical axes (e.g., ADR integrity > 60 always high)

## Architecture

```
amadeus check
    |
    |  Phase 1: Reading Steiner
    |  +-- Diff mode: scan merged PRs since last check
    |  +-- Full mode: scan entire codebase structure
    |  +-- Output: ShiftReport (significant? merged PRs, diff)
    |
    |  Phase 2: Divergence Meter
    |  +-- Build prompt (diff_check or full_check template)
    |  +-- Claude evaluates against ADRs + DoDs
    |  +-- Parse scores per axis, compute weighted divergence
    |  +-- Detect divergence jumps (delta > threshold)
    |
    |  Phase 3: D-Mail
    |  +-- Generate D-Mails from Claude candidates
    |  +-- Route by severity (all auto-sent to outbox)
    |  +-- Dual-write to outbox/ + archive/
    |
    v
.gate/                  <- Persistent state
    +-- config.yaml           <- Weights, thresholds, intervals
    +-- .run/                 <- Ephemeral state (gitignored)
    |   +-- latest.json       <- Current check state
    |   +-- baseline.json     <- Full calibration baseline
    +-- events/               <- Append-only event log (JSONL, daily rotation)
    +-- outbox/               <- Outgoing D-Mails (gitignored)
    +-- inbox/                <- Incoming D-Mails (gitignored)
    +-- archive/              <- All D-Mails (git-tracked)
```

### Scoring Axes

| Axis | Weight | What It Measures |
|------|--------|-----------------|
| `adr_integrity` | 0.4 | Compliance with Architecture Decision Records |
| `dod_fulfillment` | 0.3 | Definition of Done completion |
| `dependency_integrity` | 0.2 | Dependency graph consistency |
| `implicit_constraints` | 0.1 | Unwritten conventions and patterns |

Weights and thresholds are configurable in `.gate/config.yaml`.

### D-Mail Format

D-Mails use YAML frontmatter + Markdown body, stored as `.md` files:

```yaml
---
name: feedback-001
kind: feedback
description: "ADR-003 violation detected"
issues:
  - MY-42
severity: high
---

# ADR-003 Violation

The auth module violates the JWT requirement specified in ADR-003.
```

**Kinds** (role-based addressing):

| Kind | Producer | Purpose |
|------|----------|---------|
| `feedback` | Amadeus (verifier) | Corrective actions from divergence detection |
| `specification` | Paintress (designer) | Updated baseline expectations |
| `report` | Sightjack (implementer) | Implementation status reports |

D-Mail `.md` files are immutable once written.

## Install

```bash
# Homebrew (macOS / Linux)
brew install hironow/tap/amadeus

# Go install
go install github.com/hironow/amadeus/cmd/amadeus@latest
```

## Setup

```bash
# Initialize .gate/ with default config
amadeus init

# Run divergence check
amadeus check
```

Amadeus creates `.gate/` with config, events, and D-Mail storage automatically.

## Subcommands

| Command | Description |
|---------|-------------|
| `amadeus check` | Execute three-phase divergence check |
| `amadeus init` | Initialize `.gate/` directory with default config |
| `amadeus validate` | Validate `.gate/config.yaml` |
| `amadeus doctor` | Check environment health (git, Claude CLI, config) |
| `amadeus log` | Print check history and D-Mail log |
| `amadeus sync` | Show D-Mail × Issue comment sync status (JSON) |
| `amadeus mark-commented <name> <id>` | Record a D-Mail × Issue pair as commented |
| `amadeus status [path]` | Show amadeus operational status |
| `amadeus clean [path]` | Remove state directory (`.gate/`) |
| `amadeus rebuild [path]` | Rebuild projections from event store |
| `amadeus archive-prune` | Prune old archived D-Mail files |
| `amadeus install-hook` | Install git post-merge hook |
| `amadeus uninstall-hook` | Remove git post-merge hook |
| `amadeus version` | Print version, commit, and build date |
| `amadeus update` | Self-update to the latest release |

## Usage

```bash
# Basic check (diff mode)
amadeus check

# Full calibration (evaluate entire codebase from zero)
amadeus check -f

# Dry run (build prompt only, skip Claude call)
amadeus check -n

# Summary-only output
amadeus check -q

# JSON output (machine-readable, stdout)
amadeus check -j

# Combine short flags
amadeus check -v -j -c /path/to/config.yaml

# Show D-Mail × Issue comment sync status
amadeus sync

# Mark a D-Mail × Issue pair as commented
amadeus mark-commented feedback-001 MY-250

# Mark-commented with JSON output
amadeus mark-commented feedback-001 MY-250 -j

# Prune archived D-Mails older than 90 days (dry run)
amadeus archive-prune -d 90 -n

# JSON output for scripting
amadeus log -j | jq '.dmails[] | select(.severity == "high")'

# Version info as JSON
amadeus version -j

# Check for updates without installing
amadeus update -C
```

## Options

### Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | `.gate/config.yaml` | Config file path |
| `--verbose` | `-v` | `false` | Verbose logging |
| `--output` | `-o` | `text` | Output format: `text` or `json` |
| `--lang` | `-l` | | Output language (`ja`, `en`) |

### check

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dry-run` | `-n` | `false` | Build prompt only, skip Claude |
| `--full` | `-f` | `false` | Force full calibration check |
| `--quiet` | `-q` | `false` | Summary-only output |
| `--json` | `-j` | `false` | Structured JSON output to stdout |

### archive-prune

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--days` | `-d` | `30` | Prune files older than N days |
| `--dry-run` | `-n` | `false` | Show what would be pruned |
| `--yes` | `-y` | `false` | Skip confirmation prompt |

### version / doctor / log

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--json` | `-j` | `false` | JSON output |

### update

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--check` | `-C` | `false` | Check for updates without installing |

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success (no drift / operation completed) |
| `1` | Runtime error |
| `2` | Drift detected (divergence threshold exceeded) |

```bash
amadeus check --quiet
case $? in
  0) echo "clean" ;;
  2) echo "drift detected" >&2 ;;
  *) echo "error" >&2; exit 1 ;;
esac
```

## Configuration

```yaml
# .gate/config.yaml
weights:
  adr_integrity: 0.4
  dod_fulfillment: 0.3
  dependency_integrity: 0.2
  implicit_constraints: 0.1

thresholds:
  low_max: 0.25
  medium_max: 0.5

per_axis_override:
  adr_integrity_force_high: 60
  dod_fulfillment_force_high: 70
  dependency_integrity_force_medium: 80

full_check:
  interval: 10
  on_divergence_jump: 0.15
```

## Tracing (OpenTelemetry)

Amadeus instruments key operations with OpenTelemetry spans and events. Tracing is off by default (noop tracer) and activates when `OTEL_EXPORTER_OTLP_ENDPOINT` is set.

```bash
# Start Jaeger v2 (trace viewer + MCP)
just jaeger

# Run amadeus with tracing enabled
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 amadeus check

# View traces at http://localhost:16686
# MCP endpoint at http://localhost:16687

# Stop Jaeger
just jaeger-down
```

Spans cover: `amadeus.check` (root), `reading_steiner`, `divergence_meter`, `dmail`, and `amadeus.doctor`.

Events: `shift.detected`, `divergence.evaluated`, `divergence.jump`, `dmail.created`, `doctor.check`.

## Development

```bash
# Task runner (just)
just build              # Build binary
just install            # Build and install to /usr/local/bin
just test               # Run all tests
just test-v             # Verbose test output
just test-race          # Tests with race detector
just cover              # Coverage report
just cover-html         # Open coverage in browser
just fmt                # Format code (gofmt)
just vet                # Run go vet
just lint               # fmt check + vet + markdown lint
just lint-md            # Lint markdown files only
just semgrep            # Run semgrep with project cobra rules
just check              # fmt + vet + test (pre-commit check)
just docgen             # Generate CLI reference docs
just clean              # Clean build artifacts
just prek-install       # Install prek hooks (pre-commit + pre-push)
just prek-run           # Run all prek hooks on all files
just jaeger             # Start Jaeger trace viewer (docker)
just jaeger-down        # Stop Jaeger
just release-check      # Validate goreleaser config
just release-snapshot   # Test release locally (snapshot, no upload)
```

## File Structure

```
+-- cmd/amadeus/
|   +-- main.go              CLI entry point
+-- internal/cmd/
|   +-- root.go              Root command, global flags, build metadata
|   +-- check.go             check subcommand
|   +-- archive_prune.go     archive-prune subcommand
|   +-- version.go           version subcommand (text + JSON)
|   +-- update.go            Self-update via GitHub releases
|   +-- doctor.go            doctor subcommand
|   +-- log.go               log subcommand
|   +-- init.go              init subcommand
|   +-- validate.go          validate subcommand
|   +-- sync.go              sync subcommand (JSON output)
|   +-- mark_commented.go    mark-commented subcommand (sync state)
|   +-- hook.go              install-hook / uninstall-hook
+-- internal/usecase/         Use case layer (PolicyEngine + handlers)
+-- internal/session/         I/O orchestration layer
|   +-- amadeus.go            Amadeus orchestrator (RunCheck, PrintLog, PrintSync)
|   +-- projection.go         Projector (event replay to materialized state)
|   +-- reading_steiner.go    Shift detection (diff + full scan)
|   +-- claude.go             DefaultClaudeRunner (subprocess)
|   +-- dmail_io.go           D-Mail file I/O (archive, inbox, outbox)
|   +-- git.go                GitClient (subprocess)
|   +-- source.go             Content collection (ADRs, DoDs, go.mod)
|   +-- state.go              ProjectionStore, InitGateDir
|   +-- sync_io.go            Sync state persistence
|   +-- hook.go               Git hook file management
|   +-- archive_prune.go      Archive file discovery/deletion
+-- internal/eventsource/     Event store infrastructure (JSONL append-only)
+-- internal/domain/          Pure domain functions
+-- internal/tools/docgen/    CLI docs generation
+-- doc.go                    Package declaration (root-zero: all code in internal/)
+-- templates/                AI prompt templates ({en,ja})
|   +-- skills/               D-Mail SKILL.md templates
+-- tests/scenario/           Scenario tests (L1-L4, //go:build scenario)
+-- tests/e2e/                Docker E2E tests (//go:build e2e)
+-- .semgrep/                 Semgrep rules (layer enforcement)
+-- .goreleaser.yaml          Release configuration
+-- .github/workflows/        CI + Release
+-- docker/                   Jaeger v2 for trace viewing
+-- docs/cli/                 Generated CLI reference (Markdown)
```

## The Ecosystem

Amadeus is the third pillar in a three-tool AI development ecosystem:

```
Sightjack (pre-merge)      Paintress (execution)      Amadeus (post-merge)
    |                           |                          |
    |  Issue architecture       |  Autonomous impl         |  Integrity verification
    |  DoD, dependencies        |  Code, tests, PRs        |  Divergence scoring
    |  Wave-by-wave approval    |  Expedition loop         |  D-Mail routing
    |                           |                          |
    v                           v                          v
Linear Issues -----------> Git Repository -----------> .gate/
                                |                          |
                   D-Mail       |         D-Mail           |
                  (report) -----+----> inbox/         outbox/ ----> feedback
                  (specification)                      archive/ (immutable)
```

## What / Why / How

See [docs/conformance.md](docs/conformance.md) for the full conformance table (single source).

## Documentation

- [docs/](docs/README.md) — Full documentation index
- [docs/conformance.md](docs/conformance.md) — What/Why/How conformance table
- [docs/gate-directory.md](docs/gate-directory.md) — `.gate/` directory structure
- [docs/policies.md](docs/policies.md) — Event → Policy mapping
- [docs/otel-backends.md](docs/otel-backends.md) — OTel backend configuration
- [docs/adr/](docs/adr/README.md) — Architecture Decision Records

## Prerequisites

- Go 1.26+
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code)
- [Docker](https://www.docker.com/) (optional, for Jaeger tracing)

## License

Apache License 2.0
See [LICENSE](./LICENSE) for details.
