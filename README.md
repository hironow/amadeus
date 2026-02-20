# Amadeus

**A post-merge integrity verification system that measures how far your codebase has diverged from its intended design.**

Amadeus uses [Claude Code](https://docs.anthropic.com/en/docs/claude-code) to evaluate merged changes against ADRs (Architecture Decision Records) and DoDs (Definitions of Done), scoring divergence across four axes and routing corrective D-Mails to downstream tools when the world line drifts too far.

```bash
amadeus check
```

This single command executes a three-phase pipeline:

1. **Phase 1 (Reading Steiner)** — Detect shifts: scan merged PRs or the full codebase for structural changes
2. **Phase 2 (Divergence Meter)** — Measure divergence: Claude evaluates the changes against ADRs and DoDs, scoring four axes 0-100
3. **Phase 3 (D-Mail)** — Route corrections: generate targeted messages to Sightjack or Paintress based on severity

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
| **.divergence/** | The device | Persistent state directory that tracks readings across checks |

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
LOW    (score <= 0.25) -> Auto-sent to target tool (Sightjack/Paintress)
MEDIUM (score <= 0.50) -> Auto-sent with elevated priority
HIGH   (score >  0.50) -> Held as PENDING, requires human approval
```

- **`amadeus resolve <id> --approve`** — approve a pending D-Mail
- **`amadeus resolve <id> --reject --reason "..."`** — reject with reason
- Per-axis overrides can force HIGH severity for critical axes (e.g., ADR integrity > 60 always HIGH)

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
    |  +-- Route by severity (auto-send or hold for approval)
    |  +-- Persist to .divergence/dmails/
    |
    v
.divergence/                  <- Persistent state
    +-- config.yaml           <- Weights, thresholds, intervals
    +-- state/latest.json     <- Current check state
    +-- history/              <- Historical check results
    +-- dmails/               <- D-Mail messages
```

### Scoring Axes

| Axis | Weight | What It Measures |
|------|--------|-----------------|
| `adr_integrity` | 0.4 | Compliance with Architecture Decision Records |
| `dod_fulfillment` | 0.3 | Definition of Done completion |
| `dependency_integrity` | 0.2 | Dependency graph consistency |
| `implicit_constraints` | 0.1 | Unwritten conventions and patterns |

Weights and thresholds are configurable in `.divergence/config.yaml`.

### D-Mail Targets

| Target | Downstream Tool | Purpose |
|--------|----------------|---------|
| `sightjack` | [Sightjack](https://github.com/hironow/sightjack) | Issue architecture: DoD gaps, dependency ordering |
| `paintress` | [Paintress](https://github.com/hironow/paintress) | Autonomous implementation: code fixes, test additions |

## Setup

```bash
# Build and install
just install

# Initialize .divergence/ with default config
amadeus init

# Check environment health
amadeus doctor

# Run divergence check
amadeus check
```

Amadeus creates `.divergence/` with config, state, history, and D-Mail storage automatically.

## Subcommands

| Command | Description |
|---------|-------------|
| `amadeus check` | Execute three-phase divergence check |
| `amadeus init` | Initialize `.divergence/` directory with default config |
| `amadeus doctor` | Check environment health (git, Claude CLI, config, Linear MCP) |
| `amadeus resolve <id>` | Approve or reject a pending D-Mail |
| `amadeus log` | Print check history and D-Mail log |
| `amadeus sync` | Show D-Mails awaiting Linear linking |
| `amadeus link <dmail-id> <issue-id>` | Link a D-Mail to a Linear issue |
| `amadeus --version` | Show version and exit |

## Usage

```bash
# Basic check (diff mode)
amadeus check

# Full calibration (evaluate entire codebase from zero)
amadeus check --full

# Dry run (build prompt only, skip Claude call)
amadeus check --dry-run

# Summary-only output
amadeus check --quiet

# Verbose logging
amadeus check --verbose

# Custom config path
amadeus check --config /path/to/config.yaml

# Approve a pending D-Mail
amadeus resolve DM-001 --approve

# Reject a pending D-Mail
amadeus resolve DM-001 --reject --reason "Not applicable to current sprint"

# Link D-Mail to Linear issue
amadeus link DM-001 MY-123
```

## Options

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | `.divergence/config.yaml` | Config file path |
| `--verbose` | `-v` | `false` | Verbose logging |
| `--dry-run` | | `false` | Build prompt only, skip Claude |
| `--full` | | `false` | Force full calibration check |
| `--quiet` | `-q` | `false` | Summary-only output |
| `--version` | | | Show version and exit |

## Configuration

```yaml
# .divergence/config.yaml
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
# Start Jaeger
docker run -d --name jaeger \
  -p 16686:16686 -p 4318:4318 \
  jaegertracing/all-in-one:latest

# Run with tracing enabled
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 amadeus check

# View traces at http://localhost:16686
```

Spans cover: `amadeus.check` (root), `reading_steiner`, `divergence_meter`, `dmail`, `amadeus.resolve`, and `amadeus.doctor`.

Events: `shift.detected`, `divergence.evaluated`, `divergence.jump`, `dmail.created`, `dmail.resolved`, `doctor.check`.

## Development

```bash
just build          # Build binary
just install        # Build and install to /usr/local/bin
just test           # Run all tests
just test-v         # Verbose test output
just test-race      # Tests with race detector
just cover          # Coverage report
just cover-html     # Open coverage in browser
just fmt            # Format code (gofmt)
just vet            # Run go vet
just lint           # fmt check + vet + markdown lint
just check          # fmt + vet + test (pre-commit check)
just clean          # Clean build artifacts
```

## File Structure

```
+-- cmd/amadeus/
|   +-- main.go              CLI entry point + flag parsing
+-- amadeus.go               Main orchestrator (three-phase pipeline)
+-- reading_steiner.go       Phase 1: Shift detection (diff + full scan)
+-- divergence_meter.go      Phase 2: Scoring bridge (Claude -> scores)
+-- claude.go                Claude CLI integration + prompt rendering
+-- scoring.go               Divergence scoring (weights, thresholds, severity)
+-- dmail.go                 D-Mail model (status, target, routing)
+-- config.go                Configuration loader (.divergence/config.yaml)
+-- state.go                 State persistence (.divergence/)
+-- git.go                   Git client (merged PRs, diffs, HEAD)
+-- doctor.go                Environment health checks
+-- logger.go                Leveled logging
+-- telemetry.go             OpenTelemetry tracer setup
+-- *_test.go                Tests
+-- templates/
|   +-- diff_check.md.tmpl   Diff-based check prompt
|   +-- full_check.md.tmpl   Full calibration prompt
+-- justfile                 Task runner
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
Linear Issues -----------> Git Repository -----------> .divergence/
                                                           |
                                              D-Mail ------+-----> Sightjack
                                              (feedback)   +-----> Paintress
```

## Prerequisites

- Go 1.25+
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code)
- [Linear MCP Server](https://github.com/anthropics/model-context-protocol) configured for Claude (for `amadeus doctor` Linear check and `amadeus link`)
- [Docker](https://www.docker.com/) (optional, for Jaeger tracing)

## License

Apache License 2.0
See [LICENSE](./LICENSE) for details.
