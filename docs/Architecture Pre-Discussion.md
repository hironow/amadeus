# Third Tool — Architecture Document (Working Title: TBD)

> **Game Mechanic Origin:** Steins;Gate (シュタインズ・ゲート)
>
> **Position:** The third pillar alongside Sightjack (SIREN) and Paintress (Expedition 33)
>
> **Relationship to Sightjack:** Mirror image — Sightjack sees the future (pre-implementation risk), this tool sees the past (post-implementation truth)

---

## 1. Core Problem Statement

Sightjack raises Issue resolution to 80%. Paintress autonomously implements those Issues, verifying DoD compliance through its internal Codex CLI review loop (up to 3 iterations). Individual PRs are correct.

**But correctness of parts does not guarantee correctness of the whole.**

When multiple PRs merge into main, the resulting world may be broken in ways no individual PR caused:

- Contradictions between independently correct PRs
- Drift from architectural decisions (ADRs) that were valid when written but no longer hold
- Dependency relationships that Sightjack mapped but Paintress implementations violated in combination
- Accumulated degradation invisible at the PR level

This tool detects that the world line has diverged, identifies where in the timeline the divergence originated, and sends corrections back through time to the responsible agent — Sightjack or Paintress.

---

## 2. Design Principles

### 2.1 UNIX Philosophy

This tool does one thing: **post-merge world line integrity verification and correction routing.**

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

The judgment engine of this tool is an AI Agent (Claude Code). All four evaluation axes (ADR Integrity, DoD Fulfillment, Dependency Integrity, Implicit Constraints) require semantic understanding — matching natural language specifications against code, reasoning about cross-PR interactions, and inferring architectural intent. This is fundamentally an LLM task, not a static analysis task.

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
| Post-integration (Phase 1) | This Tool (Steins;Gate) | Merged main → integrity verification + correction routing |
| Production monitoring (Phase 2, future) | This Tool (Steins;Gate) | Production metrics → continuous observation + auto-response |

### 6.2 Mirror Relationship with Sightjack

Sightjack (SIREN): Borrows the eyes of the living (other Issues, stakeholders) to see risks **before** they materialize.

This Tool (Steins;Gate): Reads the memories of the diverged world line to understand what went wrong **after** implementation.

Both share the same principle: **seeing what cannot be seen through one's own perspective alone.**

### 6.3 Communication Protocol

All inter-tool communication flows through **Linear**:

```
Sightjack → Paintress:  Linear Issue (80% resolved ticket)
Paintress → GitHub:      PR (linked to Linear Issue)
This Tool → Sightjack:   Linear Comment or new Issue (D-Mail Type-S)
This Tool → Paintress:   Linear Issue (D-Mail Type-P)
This Tool → Sightjack:   Linear Issue (Convergence Alert)
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

| Sightjack Component | How This Tool Uses It |
|---|---|
| ADR (Architecture Decision Records) | Phase 2 evaluates compliance. Phase 3 may request updates via D-Mail. |
| DoD (Definition of Done) | Phase 2 checks post-integration fulfillment. Phase 3 may flag insufficient DoD. |
| Link Navigator (Dependency Map) | Phase 2 verifies dependency direction integrity. Phase 3 may report missing links. |
| Shibito (Past incidents/lessons) | Phase 4 contributes new entries. Recurring D-Mail patterns become new Shibito. |

---

## 8. Relationship to Paintress Components

| Paintress Component | How This Tool Interacts |
|---|---|
| Lumina (Context/Learning) | D-Mail Type-P becomes Lumina for future implementations |
| Codex CLI Review Loop | This tool operates AFTER Paintress's internal loop completes. No overlap. |
| Gradient Gauge (Momentum) | Not directly related. This tool is post-hoc, not in-flow. |

---

## 9. Evolution Path

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

## 10. UX Design

### 10.1 Runtime Model

**CLI-based + merge hook (C+A hybrid)**

- Primary: CLI tool invoked manually or via merge webhook
- No daemon process, no persistent server
- Merge hook triggers automatic execution; CLI enables on-demand inspection
- Consistent with Sightjack and Paintress CLI patterns

Steins;Gate alignment: Reading Steiner is an ability Okabe always possesses, not a constantly running machine. It fires automatically when a world line shift occurs (merge hook), but Okabe actively chooses to check the Divergence Meter (manual CLI).

### 10.2 Commands

Three commands only:

```
$ tool check                          # Inspect + auto-send D-Mails
$ tool resolve <id> --approve         # Approve High Divergence D-Mail
$ tool resolve <id> --reject          # Reject D-Mail (records reason)
$ tool log                            # View all past checks and D-Mails
```

`check` runs Phase 1 through Phase 4 in a single pass. Internal phase separation does not leak into the CLI surface.

`resolve` has no default action. `--approve` or `--reject` must be explicitly specified. Sending a D-Mail changes the world line — this must be a deliberate act.

`log` consolidates all historical data: past checks, sent D-Mails, rejections, and World Line Convergence warnings.

### 10.3 D-Mail Severity Model

Three tiers based on Divergence contribution:

| Tier | Behavior | Human Action | Steins;Gate Analogy |
|---|---|---|---|
| LOW | Auto-send to Linear | None required | Trivial mail — world line barely shifts |
| MEDIUM | Auto-send + flagged in CLI output | Review recommended | D-Mail sent, noticeable shift, Okabe is aware |
| HIGH | Draft created, held pending | `tool resolve <id>` required | Major world line change — Okabe deliberates before sending |

Rejection reasons are recorded and contribute to Shibito (lessons learned). "This pattern was judged acceptable" becomes future calibration data for Divergence Meter accuracy.

### 10.4 CLI Output

Default output is verbose. All phases rendered in a single pass:

```
$ tool check

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

### 10.5 Notification Strategy

CLI output only. No Slack, no email, no external notification dependencies.

Merge hook output is written to stdout/stderr of the CI/CD pipeline. Manual `tool check` output goes to terminal. Linear is the persistence layer, not the notification layer.

---

## 11. Open Questions

- [ ] Tool name (candidates: Reading Steiner, Divergence, Amadeus, or other)
- [x] ~~Trigger mechanism~~ → CLI + merge hook hybrid (Section 10.1)
- [x] ~~D-Mail approval model~~ → 3-tier severity with explicit resolve (Section 10.3)
- [x] ~~Divergence value thresholds~~ → 0.25 / 0.50 with per-axis override (Section 5.5, 5.6)
- [x] ~~Axis weights~~ → 40/30/20/10 (Section 5.2)
- [ ] Technical implementation: how to snapshot and compare "world line states" efficiently
- [ ] Scope of Phase 2 expansion and timeline
- [ ] CLI binary name (depends on tool name decision)

---

## 12. Appendix: Why Not the Alternatives

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
