# Amadeus

**An MCP server + data plane for post-merge divergence review: it reads the gate event store and PR-status projection, and posts review comments to GitHub PRs.**

Following the MCP pivot, LLM ownership moved to a human-initiated [Claude Code](https://docs.anthropic.com/en/docs/claude-code) session. Amadeus the Go CLI is now a pure data plane: it serves the gate event store and PR-status projection over MCP, posts review comments to GitHub PRs via the `gh` CLI, and provides the supporting data-plane commands. The divergence scoring, D-Mail generation, and the headless waiting-loop daemon have been retired — the LLM-driven review is now firing from the claude-code session via the `/review-gate` skill and the amadeus MCP tools (materialized into the project's `.claude/skills/` by `amadeus init`; canonical source `internal/platform/templates/claude-skills/review-gate/SKILL.md`; `.gate/skills/` holds the D-Mail routing manifests for phonewave discovery).

```bash
amadeus mcp
```

`amadeus mcp` starts the MCP server. Its tools expose:

- `ping` — liveness probe
- `refresh_reviews` — ingest the GitHub open-PR list into the gate event store (on-demand, reviewer write path; refs issue 0032)
- `next_review` — serve the oldest un-reviewed PR from the latest snapshot (review intake), falling back to the legacy check read model
- `get_pr_status` — read the PR-status projection for a given PR
- `post_comment` — post a review comment via `gh pr comment` and record review.posted in the gate ledger
- `dmail` — emit a design-feedback / implementation-feedback / convergence D-Mail via the transactional outbox (refs issue 0031)
- `get_insights` — read the learning loop: insight-ledger files + live review summary from the gate events (refs issue 0034)

The terms below (Reading Steiner, Divergence Meter, D-Mail, World Line) describe the conceptual model preserved in the gate event store and read models. They are no longer produced by a headless amadeus daemon; the event store is populated as reviews are recorded from the claude-code session.

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

The Steins;Gate-inspired scoring mechanics (full-check calibration interval,
divergence jump, capability boundary detection, D-Mail severity routing,
divergence trend analysis) were properties of the headless check pipeline,
which has been retired with the MCP pivot. The scoring weights and thresholds
still live in `.gate/config.yaml` and the recorded scores remain readable in
the gate event store via `amadeus log` and the `next_review` MCP tool,
but amadeus no longer runs the scoring itself — the review is driven from the
claude-code session.

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
claude-code session (LLM owner, human-initiated)
    |
    |  drives review via amadeus MCP tools
    v
amadeus mcp  (MCP server / data plane)
    |
    |  amadeus.next_review   -> read gate event store (latest check, divergence, PRs)
    |  amadeus.get_pr_status -> read PR-status projection for a PR
    |  amadeus.post_comment  -> post review comment to PR (gh pr comment)
    |
    |  data-plane commands: log / sync / mark-commented / status / rebuild / ...
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
    +-- archive/              <- Permanent D-Mail audit trail (git-tracked)
    |   +-- index.jsonl       <- Archive index (JSONL, metadata of pruned/existing .md files)
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
name: am-feedback-001_c5b8e2a1
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

D-Mail `.md` files are immutable once written. Each D-Mail carries a `idempotency_key` (SHA-256 hex hash of name + issues + severity) for deduplication. Validation rejects empty bodies, path traversal in targets, absolute target paths, duplicate targets, and self-referencing targets. `ParseDMailStrict` additionally rejects unknown frontmatter fields.

## Scope

**What Amadeus does:**

- Serve the gate event store and PR-status projection over MCP (`amadeus.next_review`, `amadeus.get_pr_status`)
- Post review comments to GitHub PRs via the `gh` CLI (`amadeus.post_comment`)
- Provide interactive Claude Code coding sessions (`sessions list` / `sessions enter`)
- Track check history with append-only event logs and rebuild projections (`log`, `rebuild`)
- Record D-Mail × Issue comment sync state (`sync`, `mark-commented`)
- Prune archived D-Mails and inspect dead letters (`archive-prune`, `dead-letters`)
- Generate Claude Code MCP wiring (`mcp-config generate`)

**What Amadeus does NOT do (retired with the MCP pivot):**

- Run a headless divergence-check daemon or D-Mail waiting loop
- Score divergence with a headless Claude call (the LLM review now fires from the Claude Code session)
- Generate or emit corrective D-Mails autonomously
- Run a PR convergence / auto-merge pipeline
- Implement fixes automatically
- Modify `.gate/` state externally (all operations are idempotent and local)

## Setup

```bash
# Homebrew (WIP — tap may not be published yet)
brew install hironow/tap/amadeus

# Or build from source
just install

# Initialize .gate/ with default config
amadeus init

# Generate claude-code MCP wiring
amadeus mcp-config generate

# Upgrade existing installation (regenerate SKILL.md, .gitignore)
amadeus init --force

# Start the MCP server (data plane for the claude-code review session)
amadeus mcp
```

Amadeus creates `.gate/` with config, events, and D-Mail storage automatically.

## Subcommands

Running `amadeus` without a subcommand prints usage; there is no default run loop.

| Command | Description |
|---------|-------------|
| `mcp` | Start the MCP server (data plane: next_review / get_pr_status / post_comment) |
| `init` | Initialize `.gate/` directory |
| `doctor` | Check environment health |
| `config show` / `config set` | View or update configuration |
| `validate` | Validate config file |
| `log` | Print check history and D-Mail log |
| `sync` | Show D-Mail × Issue comment sync status |
| `mark-commented` | Record a D-Mail × Issue pair as commented |
| `sessions list` / `sessions enter` | List / enter interactive claude-code coding sessions |
| `status` | Show operational status |
| `clean` | Remove state directory |
| `rebuild` | Rebuild projections from event store |
| `archive-prune` | Prune old archived D-Mail files |
| `dead-letters` | Inspect / purge dead-letter D-Mails |
| `mcp-config generate` | Generate `.mcp.json` and `.claude/settings.json` for the claude-code session |
| `improvement-stats` | Show improvement-signal statistics |
| `dashboard` | Cross-repo divergence dashboard |
| `version` | Print version info |
| `update` | Self-update to the latest release |

All commands accept an optional `[path]` argument (defaults to cwd). For flags, examples, and full reference per subcommand, see [docs/cli/](docs/cli/).

## Quick Start

```bash
amadeus init                    # set up .gate/
amadeus mcp-config generate     # claude-code MCP wiring
amadeus mcp                     # start MCP server (data plane)
amadeus log                     # print recorded check history
amadeus sessions list           # list interactive coding sessions
```

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success (operation completed) |
| `1` | Runtime error |

## Configuration

```yaml
# .gate/config.yaml
lang: ja
claude_cmd: claude
model: opus
timeout_sec: 1980
idle_timeout: 30m  # legacy waiting-phase timeout config (retained for compat; the waiting loop is retired)

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
  implicit_constraints_force_medium: 0  # disabled by default

full_check:
  interval: 10
  on_divergence_jump: 0.15
  max_result_history: 100  # max check results retained during event replay

baseline_staleness:
  max_age_days: 0  # 0 = disabled; set to e.g. 7 to auto-promote stale baselines
```

## Tracing (OpenTelemetry)

Amadeus instruments key operations with OpenTelemetry spans and events. Tracing is off by default (noop tracer) and activates when `OTEL_EXPORTER_OTLP_ENDPOINT` is set.

```bash
# Start Jaeger v2 (trace viewer + MCP)
just jaeger

# Run amadeus with tracing enabled
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 amadeus mcp

# View traces at http://localhost:16686
# MCP endpoint at http://localhost:16687

# Stop Jaeger
just jaeger-down
```

Spans cover the command roots (e.g. `amadeus.doctor`) and the MCP tool handlers.

## Development

All code lives in `internal/` (Go convention). The `internal/harness/` layer mediates between the LLM and the task environment, with sub-packages `policy/` (deterministic decisions), `verifier/` (validation rules), and `filter/` (externalized YAML prompts via PromptRegistry). See [docs/conformance.md](docs/conformance.md) for full layer architecture and directory responsibilities. Run `just --list` for available tasks.

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
Issue Source ------------> Git Repository -----------> .gate/
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
- [docs/testing.md](docs/testing.md) — Test strategy and conventions
- [docs/adr/](docs/adr/README.md) — Architecture Decision Records
- [docs/shared-adr/](docs/shared-adr/README.md) — Cross-tool shared ADRs

## Prerequisites

- Go 1.26+
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code)
- [GitHub CLI (`gh`)](https://cli.github.com/) (required for `amadeus.post_comment` PR review comments)
- [Docker](https://www.docker.com/) (optional, for Jaeger tracing)

Run `amadeus doctor` to verify all prerequisites (multiple checks including git-remote, gh CLI, and fsnotify availability).

## License

Apache License 2.0
See [LICENSE](./LICENSE) for details.
