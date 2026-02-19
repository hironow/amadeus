# Amadeus — Architecture Document

> **Game Mechanic Origin:** Steins;Gate (シュタインズ・ゲート)
>
> **Name Origin:** Amadeus — the AI from Steins;Gate 0 that preserves Kurisu's memories across world lines. Amadeus holds the memory of "how things should be" and compares it against reality.
>
> **Position:** The third pillar alongside Sightjack (SIREN) and Paintress (Expedition 33)
>
> **Naming Pattern:** Sightjack (verb — to see), Paintress (noun — one who paints), Amadeus (proper noun — the AI that remembers)
>
> **Relationship to Sightjack:** Mirror image — Sightjack sees the future (pre-implementation risk), Amadeus sees the past (post-implementation truth)

---

## 1. Core Problem Statement

Sightjack raises Issue resolution to 80%. Paintress autonomously implements those Issues, verifying DoD compliance through its internal Codex CLI review loop (up to 3 iterations). Individual PRs are correct.

**But correctness of parts does not guarantee correctness of the whole.**

When multiple PRs merge into main, the resulting world may be broken in ways no individual PR caused:

- Contradictions between independently correct PRs
- Drift from architectural decisions (ADRs) that were valid when written but no longer hold
- Dependency relationships that Sightjack mapped but Paintress implementations violated in combination
- Accumulated degradation invisible at the PR level

Amadeus detects that the world line has diverged, identifies where in the timeline the divergence originated, and sends corrections back through time to the responsible agent — Sightjack or Paintress.

---

## 2. Design Principles

### 2.1 UNIX Philosophy

Amadeus does one thing: **post-merge world line integrity verification and correction routing.**

It does NOT:

- Review individual PRs (Paintress's internal Codex CLI loop handles this)
- Define specifications or DoD (Sightjack's responsibility)
- Write or modify code (Paintress's responsibility)
- Run tests (Paintress ensures RED → GREEN before PR submission)

### 2.2 Function-First Metaphor Mapping

Every game mechanic maps 1:1 to a functional requirement. The metaphor serves the implementation, not the other way around.

### 2.3 Linear as Universal Protocol

All three tools use Linear as their persistence layer. Inter-tool communication is unified through Linear Issues and Comments. No custom IPC protocol needed.

### 2.4 Execution Foundation: Claude Code

The judgment engine of Amadeus is an AI Agent (Claude Code). All four evaluation axes (ADR Integrity, DoD Fulfillment, Dependency Integrity, Implicit Constraints) require semantic understanding — matching natural language specifications against code, reasoning about cross-PR interactions, and inferring architectural intent. This is fundamentally an LLM task, not a static analysis task.

The scoring model (Section 5) is a framework that structures the Agent's output, not a replacement for it. The Agent performs the reasoning; the scoring model quantifies and routes the results.

This is consistent with the other two tools: Sightjack's core is an LLM analyzing Linear Issues and inferring dependencies. Paintress's core is Claude Code writing and reviewing code. All three tools are AI Agents with game-mechanic-driven workflows layered on top.

---

## 3. Mechanic-to-Function Mapping

| Steins;Gate Mechanic | Function | Description |
|---|---|---|
| Reading Steiner | World Line Shift Detection | The sole ability to perceive that main has changed in ways beyond individual PR scope |
| Divergence Meter | Integrity Scoring | Quantified measurement of how far current main has drifted from Sightjack's intended architecture |
| D-Mail | Targeted Correction Routing | Short, precise feedback sent to the exact point in the timeline (Sightjack or Paintress) where divergence originated |
| World Line Convergence | Structural Problem Detection | Recognition that repeated D-Mails to the same area indicate an unfixable-by-patch architectural issue |

---

## 4. Internal Workflow

### Phase 1: Reading Steiner — World Line Shift Detection

```
Trigger:  merge event on main branch

Input:
  - Pre-merge main snapshot (full codebase state)
  - Merged PR set (diffs)
  - Linear Issues linked to each PR (including Sightjack's DoD)

Process:
  Recognize the semantic delta between "previous world line" and "current world line"
  Not just code diff — structural change comprehension
  Example: "Both auth module and cart module changed simultaneously"

Output:
  - Shift Report: structural summary of what changed and how
  - Impact Radius Map: modules potentially affected by the changes
  - Verdict: shift detected → proceed to Phase 2 / shift minor → log and terminate
```

### Phase 2: Divergence Meter — Integrity Scoring

```
Input:
  - Phase 1 Shift Report + Impact Radius Map
  - Sightjack's ADRs (Architecture Decision Records)
  - Sightjack's DoD set (per-Issue completion criteria)
  - Sightjack's Dependency Map (from Link Navigator)

Process:
  Evaluate how far current main has diverged from "the world Sightjack designed"

  Evaluation Axes:
    a) ADR Integrity:
       Does the codebase still comply with all active ADRs?
       Example: ADR says "use JWT" but session-based auth crept in
    b) DoD Fulfillment:
       Are individual Issue DoDs still satisfied after integration?
       Example: Issue A's DoD passes alone but breaks when Issue B is also present
    c) Dependency Integrity:
       Are Sightjack's defined dependency directions preserved?
       Example: Module A should not depend on Module B, but now it does
    d) Implicit Constraints:
       Naming conventions, error handling patterns, architectural consistency

Output:
  - Divergence Value: 0.000000~ (quantified drift score)
  - Divergence Breakdown: per-axis drift details
  - Verdict: below threshold → log and terminate / above threshold → proceed to Phase 3
```

### Phase 3: D-Mail — Divergence Point Identification & Correction Routing

```
Input:
  - Phase 2 Divergence Breakdown
  - Git history (PR merge order and timeline)
  - Linear Issues + Sightjack's design intent per Issue

Process:
  Identify the "divergence point" on the timeline that caused the drift

  Divergence Point Classification:
    Type-S (Sightjack-originated):
      - ADR premise no longer holds
      - DoD definition was insufficient
      - Dependency relationship was missed
    Type-P (Paintress-originated):
      - DoD was correct but implementation deviated
      - PR was correct in isolation but contradicts other PRs in integration
    Type-SP (Both):
      - Interpretation gap between design intent and implementation

Output:
  D-Mail to Sightjack (via Linear):
    - Comment on existing Issue or new Issue creation
    - Examples:
      "ADR-007 premise has changed. Update required."
      "Undefined dependency between Issue #42 and #58 detected."

  D-Mail to Paintress (via Linear):
    - New Linear Issue for fix (no Sightjack re-specification needed)
    - Examples:
      "PR #120 auth header handling contradicts PR #115 session management."
```

### Phase 4: World Line Convergence — Structural Problem Detection

```
Input:
  - Historical D-Mail log (all past D-Mails)
  - Past incident/bug tickets on Linear

Process:
  Analyze D-Mail patterns for recurring problems
  "Same module received 3+ D-Mails"
  "Same Type-S divergence recurring across different Issues"

Output:
  Convergence Alert to Sightjack (via Linear):
    "Structural problem detected in auth domain.
     D-Mail (patch-level fixes) cannot resolve this.
     Architecture redesign required."

  Recommended Actions:
    - Request new ADR creation
    - List of affected Issues
```

---

## 5. Divergence Scoring Model

### 5.1 Architecture: Two-Layer Design

Internal computation uses practical 0–100 scoring. CLI display converts to Steins;Gate-style 0.000000~ notation. Implementation logic and user-facing metaphor are decoupled.

### 5.2 Evaluation Axes & Weights

| Axis | Weight | Rationale |
|---|---|---|
| ADR Integrity | 40% | Architecture decisions are the highest-level constraints. Violations cascade. |
| DoD Fulfillment | 30% | Completion criteria define correctness. Post-integration gaps indicate systemic issues. |
| Dependency Integrity | 20% | Dependency direction violations create coupling that compounds over time. |
| Implicit Constraints | 10% | Naming, patterns, conventions. Important but lowest blast radius. |

Each axis scores 0–100 independently (0 = full compliance, 100 = full deviation).

### 5.3 Internal Score Calculation

```
Internal Score = (ADR × 0.4) + (DoD × 0.3) + (Dep × 0.2) + (Implicit × 0.1)

Example:
  ADR Integrity:        15/100 × 0.4 = 6.0
  DoD Fulfillment:      20/100 × 0.3 = 6.0
  Dependency Integrity: 10/100 × 0.2 = 2.0
  Implicit Constraints:  5/100 × 0.1 = 0.5
                                       ----
  Internal Score:                      14.5 / 100
```

### 5.4 Display Conversion

```
Divergence Value = Internal Score / 100

Internal Score 14.5 → Divergence 0.145000
Internal Score 0    → Divergence 0.000000 (perfect alignment — Steins;Gate world line)
Internal Score 100  → Divergence 1.000000 (total deviation — world line collapse)
```

### 5.5 D-Mail Severity Thresholds

| Tier | Divergence | Internal Score | Behavior |
|---|---|---|---|
| LOW | < 0.250000 | < 25 | D-Mail auto-sent. No action required. |
| MEDIUM | 0.250000 – 0.499999 | 25 – 49 | D-Mail auto-sent + flagged in CLI output. Review recommended. |
| HIGH | ≥ 0.500000 | ≥ 50 | D-Mail held as draft. `tool resolve <id> --approve/--reject` required. |

These thresholds are initial values. They should be calibrated through operational experience and D-Mail rejection patterns (rejected D-Mails indicate the threshold may be too aggressive).

### 5.6 Per-Axis Severity Override

Even if total Divergence is LOW, a single axis exceeding its own critical threshold escalates severity:

```
ADR Integrity ≥ 60        → force HIGH (regardless of total)
DoD Fulfillment ≥ 70      → force HIGH
Dependency Integrity ≥ 80 → force MEDIUM (minimum)
```

This prevents a dangerous ADR violation from being masked by clean scores on other axes.

### 5.7 CLI Display Example

```
Divergence: 0.145000 (▲ 0.012)
  ADR Integrity:        0.060 [15/100 × 40%]
  DoD Fulfillment:      0.060 [20/100 × 30%]
  Dependency Integrity: 0.020 [10/100 × 20%]
  Implicit Constraints: 0.005 [ 5/100 × 10%]
```

---

## 6. Three-Tool Ecosystem

### 6.1 Timeline Coverage

| Phase | Owner | Input → Output |
|---|---|---|
| Pre-implementation | Sightjack (SIREN) | Ambiguous Issue → 80% resolution ticket |
| During implementation | Paintress (Expedition 33) | Ticket → PR (DoD verified via internal Codex CLI loop) |
| Post-integration (Phase 1) | Amadeus (Steins;Gate) | Merged main → integrity verification + correction routing |
| Production monitoring (Phase 2, future) | Amadeus (Steins;Gate) | Production metrics → continuous observation + auto-response |

### 6.2 Mirror Relationship with Sightjack

Sightjack (SIREN): Borrows the eyes of the living (other Issues, stakeholders) to see risks **before** they materialize.

Amadeus (Steins;Gate): Reads the memories of the diverged world line to understand what went wrong **after** implementation.

Both share the same principle: **seeing what cannot be seen through one's own perspective alone.**

### 6.3 Communication Protocol

All inter-tool communication flows through **Linear**:

```
Sightjack → Paintress:  Linear Issue (80% resolved ticket)
Paintress → GitHub:      PR (linked to Linear Issue)
Amadeus → Sightjack:   Linear Comment or new Issue (D-Mail Type-S)
Amadeus → Paintress:   Linear Issue (D-Mail Type-P)
Amadeus → Sightjack:   Linear Issue (Convergence Alert)
```

No custom protocols. Linear is the shared memory of all three tools.

### 6.4 Feedback Loops

```
Normal Loop (D-Mail):
  merge → Reading Steiner → Divergence Meter → D-Mail → Sightjack/Paintress fixes → merge → ...

Escalation Loop (World Line Convergence):
  D-Mail pattern detected → Convergence Alert → Sightjack redesigns architecture → new ADRs → ...
```

---

## 7. Relationship to Sightjack Components

| Sightjack Component | How Amadeus Uses It |
|---|---|
| ADR (Architecture Decision Records) | Phase 2 evaluates compliance. Phase 3 may request updates via D-Mail. |
| DoD (Definition of Done) | Phase 2 checks post-integration fulfillment. Phase 3 may flag insufficient DoD. |
| Link Navigator (Dependency Map) | Phase 2 verifies dependency direction integrity. Phase 3 may report missing links. |
| Shibito (Past incidents/lessons) | Phase 4 contributes new entries. Recurring D-Mail patterns become new Shibito. |

---

## 8. Relationship to Paintress Components

| Paintress Component | How Amadeus Interacts |
|---|---|
| Lumina (Context/Learning) | D-Mail Type-P becomes Lumina for future implementations |
| Codex CLI Review Loop | Amadeus operates AFTER Paintress's internal loop completes. No overlap. |
| Gradient Gauge (Momentum) | Not directly related. Amadeus is post-hoc, not in-flow. |

---

## 9. Technical Implementation

### 9.1 Execution Foundation

Amadeus runs on Claude Code, consistent with Sightjack and Paintress. All semantic evaluation (ADR compliance, DoD fulfillment, dependency analysis) is performed by the LLM. The scoring model structures the output; the Agent performs the reasoning.

### 9.2 Local State: `.divergence/` Directory

Amadeus stores its state in a dot directory at the repository root, following the same pattern as the other tools:

```
.siren/       — Sightjack (SIREN)
.expedition/  — Paintress (Expedition 33)
.divergence/  — Amadeus (Steins;Gate)
```

This directory is Git-tracked (not in `.gitignore`). Amadeus's memory travels with the repository — consistent with the name origin (an AI that preserves memories across world lines).

#### Directory Structure

```
.divergence/
├── config.yaml           # Weights, thresholds, full check interval
├── state/
│   ├── latest.json       # Latest check result (baseline for next diff check)
│   └── baseline.json     # Latest full check result (calibration reference)
├── history/
│   ├── 2026-02-18T1430.json  # Past check results (amadeus log source)
│   ├── 2026-02-18T1200.json
│   └── ...
└── dmails/
    ├── d-041.json        # Sent D-Mail
    ├── d-042.json        # Sent D-Mail
    └── d-043.json        # pending / approved / rejected
```

#### `config.yaml`

```yaml
weights:
  adr_integrity: 0.4
  dod_fulfillment: 0.3
  dependency_integrity: 0.2
  implicit_constraints: 0.1

thresholds:
  low_max: 0.250000
  medium_max: 0.500000

per_axis_override:
  adr_integrity_force_high: 60
  dod_fulfillment_force_high: 70
  dependency_integrity_force_medium: 80

full_check:
  interval: 10              # Run full check every N diff checks
  on_divergence_jump: 0.15  # Run full check if Divergence moves more than this

check_count_since_full: 0   # Accumulated diff checks since last full
```

#### `state/latest.json`

```json
{
  "checked_at": "2026-02-18T14:30:00Z",
  "commit": "a1b2c3d",
  "type": "diff",
  "divergence": 0.145000,
  "axes": {
    "adr_integrity": { "score": 15, "details": "ADR-003 minor tension" },
    "dod_fulfillment": { "score": 20, "details": "Issue #42 edge case" },
    "dependency_integrity": { "score": 10, "details": "clean" },
    "implicit_constraints": { "score": 5, "details": "naming drift in cart" }
  },
  "prs_evaluated": ["#120", "#122", "#125"]
}
```

#### `dmails/d-043.json`

```json
{
  "id": "d-043",
  "severity": "HIGH",
  "status": "pending",
  "target": "sightjack",
  "type": "Type-S",
  "summary": "ADR-003 auth→cart dependency violation",
  "detail": "PR #120 introduced direct import from cart module in auth service...",
  "linear_issue_id": null,
  "created_at": "2026-02-18T14:30:00Z",
  "resolved_at": null,
  "resolved_action": null,
  "reject_reason": null
}
```

### 9.3 Scan Strategy: Hybrid (Diff + Periodic Full)

Two scan modes, automatically selected:

#### Diff Check (default)

Runs on every `amadeus check` invocation. Evaluates only what changed since last check.

```
Input to Claude Code:
  1. .divergence/state/latest.json (previous scores + context)
  2. PR diffs merged since last check
  3. Linear Issues linked to those PRs (DoD included)
  4. ADRs relevant to touched modules only

Instruction:
  "Given the previous scores as baseline, evaluate how these changes
   affect each axis. Output updated scores and any D-Mails."

Output:
  Updated Divergence value + D-Mails
  Written to .divergence/state/latest.json and .divergence/history/
```

#### Full Check (calibration)

Resets all scores from zero. Triggered automatically when:

- `check_count_since_full` reaches `full_check.interval` (default: 10)
- Divergence value jumps more than `full_check.on_divergence_jump` (default: 0.15) in a single diff check

Also available manually: `amadeus check --full`

```
Input to Claude Code:
  1. Main branch structure (directory tree + key module summaries)
  2. All active ADRs
  3. Recent N Issues with DoDs
  4. Sightjack's dependency map

Instruction:
  "Evaluate the entire codebase against all ADRs and DoDs.
   Score each axis from zero. This is a full calibration."

Output:
  Reset Divergence value + D-Mails
  Written to .divergence/state/baseline.json, latest.json, and history/
```

### 9.4 `amadeus check` Execution Flow

```
1. Read .divergence/config.yaml and .divergence/state/latest.json
2. Determine scan mode:
   - If check_count_since_full >= interval → full check
   - Else → diff check
3. Gather inputs (PR diffs or full codebase, ADRs, DoDs)
4. Invoke Claude Code with appropriate prompt
5. Parse response → Divergence value + axis scores + D-Mail candidates
6. Apply per-axis severity override (Section 5.6)
7. For each D-Mail:
   - LOW → auto-send to Linear, save to .divergence/dmails/
   - MEDIUM → auto-send to Linear, save, flag in CLI output
   - HIGH → save as pending in .divergence/dmails/, prompt for resolve
8. Check if Divergence jump exceeds on_divergence_jump → trigger full if so
9. Update .divergence/state/latest.json
10. Append to .divergence/history/
11. Increment check_count_since_full (or reset to 0 if full check)
12. Print CLI output
```

---

## 10. Evolution Path

### Phase 1 (MVP): Post-Merge Verification

- Trigger: merge events only
- Scope: code-level and architecture-level integrity
- Output: D-Mails via Linear

### Phase 2 (Future): Continuous Production Observation

- Trigger: production metrics (Datadog, Sentry, etc. via MCP)
- Scope: runtime behavior, performance drift, SLO compliance
- Output: D-Mails + auto-remediation requests
- Note: This is where the Sibyl-like capabilities naturally emerge, scoped and grounded in the established mechanic framework

---

## 11. UX Design

### 11.1 Runtime Model

**CLI-based + merge hook (C+A hybrid)**

- Primary: CLI tool invoked manually or via merge webhook
- No daemon process, no persistent server
- Merge hook triggers automatic execution; CLI enables on-demand inspection
- Consistent with Sightjack and Paintress CLI patterns

Steins;Gate alignment: Reading Steiner is an ability Okabe always possesses, not a constantly running machine. It fires automatically when a world line shift occurs (merge hook), but Okabe actively chooses to check the Divergence Meter (manual CLI).

### 11.2 Commands

Three commands only:

```
$ amadeus check                          # Inspect + auto-send D-Mails
$ amadeus resolve <id> --approve         # Approve High Divergence D-Mail
$ amadeus resolve <id> --reject          # Reject D-Mail (records reason)
$ amadeus log                            # View all past checks and D-Mails
```

`check` runs Phase 1 through Phase 4 in a single pass. Internal phase separation does not leak into the CLI surface.

`resolve` has no default action. `--approve` or `--reject` must be explicitly specified. Sending a D-Mail changes the world line — this must be a deliberate act.

`log` consolidates all historical data: past checks, sent D-Mails, rejections, and World Line Convergence warnings.

### 11.3 D-Mail Severity Model

Three tiers based on Divergence contribution:

| Tier | Behavior | Human Action | Steins;Gate Analogy |
|---|---|---|---|
| LOW | Auto-send to Linear | None required | Trivial mail — world line barely shifts |
| MEDIUM | Auto-send + flagged in CLI output | Review recommended | D-Mail sent, noticeable shift, Okabe is aware |
| HIGH | Draft created, held pending | `tool resolve <id>` required | Major world line change — Okabe deliberates before sending |

Rejection reasons are recorded and contribute to Shibito (lessons learned). "This pattern was judged acceptable" becomes future calibration data for Divergence Meter accuracy.

### 11.4 CLI Output

Default output is verbose. All phases rendered in a single pass:

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
    ✅ #d-041 [LOW]  naming inconsistency → sent to Paintress
    ⚠️  #d-042 [MED]  DoD #42 partial gap → sent, review recommended
    🔴 #d-043 [HIGH] ADR-003 auth→cart dep → awaiting approval

  ⚡ World Line Convergence Warning:
    auth module has received 4 D-Mails in the last 2 weeks.
    Consider architectural review.

  1 pending. Run `tool resolve d-043 --approve` or `tool resolve d-043 --reject`
```

Use `--quiet` flag for summary-only output when needed.

### 11.5 Notification Strategy

CLI output only. No Slack, no email, no external notification dependencies.

Merge hook output is written to stdout/stderr of the CI/CD pipeline. Manual `tool check` output goes to terminal. Linear is the persistence layer, not the notification layer.

---

## 12. Open Questions

- [x] ~~Tool name~~ → Amadeus (Section header, Steins;Gate 0 — AI that preserves memory across world lines)
- [x] ~~Trigger mechanism~~ → CLI + merge hook hybrid (Section 11.1)
- [x] ~~D-Mail approval model~~ → 3-tier severity with explicit resolve (Section 11.3)
- [x] ~~Divergence value thresholds~~ → 0.25 / 0.50 with per-axis override (Section 5.5, 5.6)
- [x] ~~Axis weights~~ → 40/30/20/10 (Section 5.2)
- [x] ~~CLI binary name~~ → `amadeus` (Section 11.2)
- [x] ~~Technical implementation~~ → Hybrid scan (diff + periodic full), `.divergence/` local state (Section 9)
- [ ] Scope of Phase 2 expansion and timeline

---

## 13. Appendix: Why Not the Alternatives

### Why not Chronos (Katana ZERO)?
SRE/DevOps concepts and AI elements were not cleanly abstracted. The "precognitive simulation" metaphor conflated too many concerns.

### Why not MAGI (Evangelion)?
Passive consensus system. Does not address the core need: identifying divergence points in the timeline and routing corrections to the past.

### Why not Sibyl (PSYCHO-PASS)?
Correct intuition (continuous observation, autonomous response) but scope was too broad for an initial implementation. Sibyl-like capabilities are the Phase 2 evolution path, not the starting point.

### Why not Obra Dinn / Danganronpa / Papers, Please?
These are "investigate the present" tools. The core requirement is "identify where the timeline diverged and send corrections to the past" — a fundamentally different operation.

### Why not Radiant Historia / Ghost Trick / Zero Escape?
Closer to the correct frame (timeline manipulation), but evaluated after the core insight crystallized: the tool's job is not just "go back in time" but specifically "identify the divergence point and route corrections to the responsible agent (Sightjack or Paintress)." Steins;Gate's D-Mail mechanic maps to this routing function more precisely than any alternative.
