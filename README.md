# Amadeus

**A post-merge divergence meter that scores design drift against ADRs and DoDs, then routes corrective D-Mails when the codebase shifts too far.**

Amadeus uses [Claude Code](https://docs.anthropic.com/en/docs/claude-code) to evaluate merged changes against ADRs (Architecture Decision Records) and DoDs (Definitions of Done), scoring divergence across four axes and routing corrective D-Mails to downstream tools when the world line drifts too far.

```bash
amadeus run
```

This command starts a daemon that continuously executes a three-phase pipeline and monitors the inbox for incoming D-Mails via fsnotify:

1. **Phase 1 (Reading Steiner)** — Detect shifts: scan merged PRs or the full codebase for structural changes
2. **Phase 2 (Divergence Meter)** — Measure divergence: Claude evaluates the changes against ADRs and DoDs, scoring four axes 0-100
3. **Phase 3 (D-Mail)** — Route corrections: generate design-feedback / implementation-feedback D-Mails routed by severity
4. **PR Convergence** — Read open PR state via `gh` CLI, build PRChain, generate PRConvergenceReport D-Mails
5. **Inbox Watcher** — Monitor inbox/ via fsnotify for real-time D-Mail reception with archive-based dedup

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
- **Force**: `amadeus run --full` triggers immediately
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

## D-Mail Protocol

Amadeus is the verifier in the D-Mail protocol ecosystem:

| Tool | Role | Endpoint |
|------|------|----------|
| **sightjack** | Designer / Protocol spec owner | `.siren/` |
| **paintress** | Implementer | `.expedition/` |
| **amadeus** | Verifier | `.gate/` |
| **phonewave** | Courier / Coordinator | (no endpoint — routes between others) |

Amadeus produces corrective D-Mails (`design-feedback`, `implementation-feedback`, `convergence`) and consumes `report` D-Mails. SKILL.md files in `.gate/skills/` declare produces/consumes routing for phonewave discovery.

## Architecture

```
amadeus run (daemon)
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
    |  PR Convergence Pipeline
    |  +-- GhPRReader: read open PR state via gh CLI
    |  +-- Build PRChain from PRState list
    |  +-- Generate PRConvergenceReport
    |  +-- Emit convergence D-Mail to outbox/
    |
    |  Inbox Watcher (fsnotify)
    |  +-- MonitorInbox: watch inbox/ for new files
    |  +-- ReceiveDMailFromInbox: parse, dedup, archive
    |
    v
.gate/                  <- Persistent state
    +-- config.yaml           <- Weights, thresholds, intervals
    +-- .run/                 <- Ephemeral state (gitignored)
    |   +-- latest.json       <- Current check state
    |   +-- baseline.json     <- Full calibration baseline
    |   +-- insights.lock     <- Flock for concurrent insight writes
    +-- events/               <- Append-only event log (JSONL, daily rotation, gitignored)
    +-- insights/             <- Semantic insight ledger (git-tracked, per ADR S0030)
    |   +-- divergence.md     <- Divergence insights (How enriched with Claude reasoning)
    |   +-- convergence.md    <- Convergence insights (Why enriched from archive D-Mails)
    +-- outbox/               <- Outgoing D-Mails (gitignored)
    +-- inbox/                <- Incoming D-Mails (gitignored)
    +-- archive/              <- All D-Mails (gitignored)
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
name: design-feedback-001
kind: design-feedback
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
| `design-feedback` | Amadeus (verifier) | Design-level corrective actions from divergence detection |
| `implementation-feedback` | Amadeus (verifier) | Implementation-level corrective actions |
| `convergence` | Amadeus (verifier) | PR convergence reports |
| `specification` | Sightjack (designer) | Architecture specifications for implementation |
| `report` | Paintress (implementer) | Implementation completion reports |
| `ci-result` | CI/CD pipeline | CI/CD pipeline integration results |

> **BREAKING**: The former `kind: feedback` has been split into `kind: design-feedback` and `kind: implementation-feedback`. Run `amadeus init --force` to regenerate SKILL.md files. `amadeus doctor` detects the deprecated kind and guides remediation.

D-Mails may include an optional `context` field (per ADR S0031) containing insight summaries from the Insight Ledger, providing receivers with semantic context about the divergence or convergence state.

D-Mail `.md` files are immutable once written.

## Scope

**What Amadeus does:**

- Detect structural shifts in merged PRs or full codebase scans (Reading Steiner)
- Score divergence across four weighted axes using Claude evaluation (Divergence Meter)
- Route corrective D-Mails (design-feedback / implementation-feedback) by severity to downstream tools
- Run PR convergence pipeline: read open PR state, build PRChain, generate convergence reports
- Monitor inbox via fsnotify for real-time D-Mail reception with archive-based dedup
- Track check history with append-only event logs

**What Amadeus does NOT do:**

- Implement fixes automatically (only detects drift and routes D-Mails)
- Approve or merge PRs (uses external approval gates)
- Store full PR content (stores references, diffs, and scores only)
- Modify `.gate/` state externally (all operations are idempotent and local)

## Setup

```bash
# Homebrew (WIP — tap may not be published yet)
brew install hironow/tap/amadeus

# Or build from source
just install

# Initialize .gate/ with default config
amadeus init

# Upgrade existing installation (regenerate SKILL.md, .gitignore)
amadeus init --force

# Run daemon (divergence check + PR convergence + inbox watcher)
amadeus run
```

Amadeus creates `.gate/` with config, events, and D-Mail storage automatically.

## Subcommands

| Command | Description |
|---------|-------------|
| `amadeus run` | Daemon mode: divergence check + PR convergence + fsnotify inbox watcher |
| `amadeus init` | Initialize `.gate/` directory with default config (`--force` to regenerate) |
| `amadeus config show` | Show current configuration values |
| `amadeus config set` | Update configuration values (e.g., `amadeus config set lang en`) |
| `amadeus validate` | Validate `.gate/config.yaml` |
| `amadeus doctor` | Check environment health (multiple checks: git, git-remote, gh, claude, config, fsnotify, etc.) |
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

All commands accept an optional `[path]` argument. When omitted, the current working directory is used.

```bash
# Daemon mode (divergence check + PR convergence + inbox watcher)
amadeus run

# Daemon with full calibration
amadeus run -f

# Daemon with dry run (build prompt only, skip Claude call)
amadeus run -n

# Show/set configuration
amadeus config show
amadeus config set lang en
amadeus config set full_check.interval 20

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

For full flag reference per subcommand, see [docs/cli/](docs/cli/).

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success (no drift / operation completed) |
| `1` | Runtime error |
| `2` | Drift detected (divergence threshold exceeded) |

```bash
amadeus run --quiet
case $? in
  0) echo "clean" ;;
  2) echo "drift detected" >&2 ;;
  *) echo "error" >&2; exit 1 ;;
esac
```

## Configuration

```yaml
# .gate/config.yaml
lang: ja
claude_cmd: claude
model: opus
timeout_sec: 1980

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
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 amadeus run

# View traces at http://localhost:16686
# MCP endpoint at http://localhost:16687

# Stop Jaeger
just jaeger-down
```

Spans cover: `amadeus.run` (daemon root), `reading_steiner`, `divergence_meter`, `dmail`, `pr_convergence`, and `amadeus.doctor`.

Events: `shift.detected`, `divergence.evaluated`, `divergence.jump`, `dmail.created`, `doctor.check`, `run.started`, `run.stopped`, `pr_convergence.checked`.

## Development

All code lives in `internal/` (Go convention). See [docs/conformance.md](docs/conformance.md) for layer architecture and directory responsibilities. Run `just --list` for available tasks.

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
                  (report) -----+----> inbox/         outbox/ ----> design-feedback
                  (specification)                               ----> implementation-feedback
                                                       archive/ (immutable)
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
- [GitHub CLI (`gh`)](https://cli.github.com/) (required for PR convergence pipeline)
- [Docker](https://www.docker.com/) (optional, for Jaeger tracing)

Run `amadeus doctor` to verify all prerequisites (multiple checks including git-remote, gh CLI, and fsnotify availability).

## License

Apache License 2.0
See [LICENSE](./LICENSE) for details.
