# Development Experience Vision: Sightjack × Paintress × Amadeus

> The three-tool ecosystem for autonomous software development.
>
> This document describes the target developer experience — what changes, what stays human, and how the development cycle transforms when these tools work together.

---

## 1. The Fundamental Shift

Software development has three phases: **Design** (what to build), **Implementation** (how to build it), and **Verification** (did we build it right). Traditionally, a solo developer performs all three, context-switching constantly.

With this ecosystem, each phase is owned by a dedicated AI agent:

| Phase | Agent | Human Role |
|---|---|---|
| Design | Sightjack (SIREN) | Provide intent. Approve direction. |
| Implementation | Paintress (Expedition 33) | Approve merge. |
| Verification | Amadeus (Steins;Gate) | Resolve HIGH divergence D-Mails. |

The human's role transforms from **doing the work** to **directing and approving the work**. This is not delegation in the traditional sense — it is a structural separation of concerns between human judgment and machine execution.

---

## 2. The Three Transformations

### 2.1 Quality Assurance: From Attention to Architecture

**Before:** Quality depends on the developer's attention span. Tired at 11pm? You miss the edge case. Distracted? You forget the ADR. Quality is as inconsistent as human energy.

**After:** Quality is enforced by structure.

- Sightjack ensures every Issue has explicit DoD and dependency mapping before implementation begins. Ambiguity is eliminated at the source.
- Paintress verifies DoD compliance through its Codex CLI review loop (up to 3 iterations) before submitting a PR. The code meets specification by construction.
- Amadeus verifies system-wide integrity after merge. Cross-PR contradictions, ADR drift, and dependency violations are caught structurally, not by luck.

The human never needs to ask "did I miss something?" The system is designed so that gaps are detected at every boundary.

### 2.2 Human Role: From Worker to Decision Maker

**Before:** The developer writes issues, writes code, writes tests, reviews their own code, checks for regressions, and maintains architectural consistency — all in the same brain, on the same day.

**After:** The developer makes decisions. Specifically, four types:

**What to build:** Provide Sightjack with product intent. "Users need to log in with OAuth." The developer describes the goal, not the implementation. Sightjack translates intent into actionable specifications.

**Is this design right?** Review Sightjack's output — the DoD, the dependency map, the ADR implications. Approve or redirect. This is a 5-minute decision, not a 2-hour design session.

**Is this code acceptable?** Review Paintress's PR. The code already passes its own review loop. The developer confirms it matches their mental model. Merge or request changes.

**Is this divergence real?** When Amadeus flags a HIGH divergence, the developer decides: approve the D-Mail (send correction to Sightjack or Paintress), reject it (record that this pattern is acceptable), or investigate further.

Everything else — the actual writing of specifications, code, tests, and integrity checks — is delegated to the agents.

### 2.3 Development Velocity: From Linear to Parallel

**Before:** Every phase blocks the next. Can't implement until the design is clear. Can't verify until the implementation is done. And there's only one brain doing all of it, sequentially.

**After:** The three agents operate on different Issues simultaneously.

```
Timeline:
  Sightjack:  designing Issue #5    → designing Issue #6    → designing Issue #7
  Paintress:  implementing Issue #3 → implementing Issue #4 → implementing Issue #5
  Amadeus:    verifying merge of #1 → verifying merge of #2 → verifying merge of #3
  Human:      approving outputs as they arrive
```

The developer is no longer the bottleneck in the pipeline. They are the **governor** — reviewing and approving outputs as they flow through the system. The throughput ceiling is no longer "how fast can one person work" but "how fast can one person decide."

---

## 3. A Day in the Life

### Morning: Intent

The developer opens Linear and writes a rough idea:

> "We need rate limiting on the API. Should use token bucket. Redis-backed."

That's it. Three sentences. Sightjack picks it up.

Sightjack analyzes the existing codebase (via `.siren/`), checks ADRs for relevant constraints, maps dependencies to the auth and middleware modules, and produces an 80%-complete Issue with explicit DoD:

> - Token bucket algorithm with configurable rate per API key
> - Redis adapter with fallback to in-memory for local dev
> - Returns 429 with Retry-After header
> - Existing auth middleware integration without breaking current request flow
> - ADR-012 (Redis usage patterns) applies

The developer reads it, adjusts one DoD item, approves. Total time: 10 minutes.

### Midday: Implementation

Paintress picks up the approved Issue and begins implementing. It opens a Git worktree, writes code, runs its internal Codex CLI review loop to verify DoD compliance, iterates up to 3 times, and submits a PR.

The developer is notified. They read the diff, confirm it matches their expectations. Merge. Total time: 15 minutes.

Meanwhile, Sightjack is already designing the next Issue. Paintress is already implementing the one before.

### Afternoon: Verification

Three PRs have merged today. Amadeus fires automatically (merge hook):

```
$ [auto] Reading Steiner triggered — 3 PRs merged

  Divergence: 0.087000 (▲ 0.031)
    ADR Integrity:        0.020 — clean
    DoD Fulfillment:      0.040 — Issue #38 DoD edge case
    Dependency Integrity: 0.022 — minor concern
    Implicit Constraints: 0.005 — clean

  D-Mails:
    ✅ #d-071 [LOW]  error handling pattern drift → sent to Paintress
    ⚠️  #d-072 [MED]  DoD #38 boundary condition → sent, review recommended

  No pending approvals. World line stable.
```

The developer glances at the output. Everything is LOW or MEDIUM. No action needed. Amadeus has already sent the corrections to Paintress and Sightjack via Linear. Total time: 30 seconds.

### Occasionally: Course Correction

Once in a while, Amadeus flags something serious:

```
  🔴 #d-089 [HIGH] ADR-003 auth→payment direct dependency detected
                    → D-Mail to Sightjack drafted, awaiting approval

  ⚡ World Line Convergence Warning:
    auth module has received 5 D-Mails in 3 weeks.
    Consider architectural review.
```

The developer runs `amadeus resolve d-089 --approve`. Sightjack receives the D-Mail and will factor it into future Issue design. The developer also notes the Convergence Warning and decides to schedule a broader architecture review.

This is the developer's highest-value activity: making structural decisions about the system's future, informed by data that Amadeus has collected over time.

---

## 4. What Stays Human

Not everything is delegated. Some responsibilities are inherently human and should remain so:

**Product vision.** "What should this product be?" is a human question. Sightjack can break down an intent into specifications, but the intent itself — the why — comes from the developer.

**Architectural inflection points.** When Amadeus sends a Convergence Alert, the decision to redesign a module or accept the technical debt is a business-level judgment. AI provides the data; the human decides the tradeoff.

**Merge approval.** The act of merging code into main is the human's commitment that "I accept this change into the system." Paintress proposes; the human disposes.

**HIGH divergence resolution.** When the world line diverges significantly, the human decides whether to correct it (approve D-Mail), accept it (reject D-Mail), or investigate deeper. This is the "trigger" decision — just as in Steins;Gate, the weight of changing the world line rests on the human.

**Stakeholder communication.** Explaining technical decisions to non-technical stakeholders, negotiating scope, and managing expectations remain human responsibilities.

### The Principle: Humans Decide, Machines Execute

The boundary is clean. Every human touchpoint is a **decision**, not a **task**:

| Human Action | Type | Time |
|---|---|---|
| Write product intent | Decision: what to build | Minutes |
| Approve Sightjack's specification | Decision: is the design right? | Minutes |
| Merge Paintress's PR | Decision: is the code acceptable? | Minutes |
| Resolve Amadeus's HIGH D-Mail | Decision: is the divergence real? | Minutes |
| Act on Convergence Warning | Decision: restructure or accept debt? | Varies |

No human action is "write code," "run tests," "check for regressions," or "update documentation." Those are execution tasks, handled by agents.

---

## 5. The Feedback Ecosystem

The three tools don't just operate in sequence — they form a learning system.

### 5.1 Upward Feedback (Amadeus → Sightjack)

When Amadeus detects that Sightjack's design was insufficient (Type-S D-Mail), Sightjack learns. The next time Sightjack designs an Issue in the same module, it accounts for the gap that Amadeus found.

Example: Amadeus detects "DoD for Issue #42 didn't account for concurrent access." Sightjack's future DoDs for that module will include concurrency considerations.

### 5.2 Lateral Feedback (Amadeus → Paintress)

When Amadeus detects implementation contradictions (Type-P D-Mail), Paintress learns via Lumina. The next implementation in a related area benefits from the lesson.

Example: Amadeus detects "PR #120 and PR #115 have conflicting session handling." Paintress's Lumina now includes "check for session handling consistency when touching auth module."

### 5.3 Forward Feedback (Sightjack → Paintress)

Sightjack's Shibito (past incidents and lessons) informs Paintress proactively. Issues are designed to prevent known failure patterns.

Example: A past Shibito records "null handling in payment module caused production incident." Sightjack now includes "explicit null handling for payment responses" in every related Issue's DoD.

### 5.4 Historical Feedback (Amadeus → Sightjack Shibito)

Recurring D-Mail patterns detected by Amadeus's World Line Convergence become new Shibito entries. The organization's failure memory grows continuously.

```
Feedback Cycle:

  Sightjack designs → Paintress implements → Amadeus verifies
       ↑                                          |
       |              D-Mail (Type-S)              |
       ←───────────────────────────────────────────┘
       |              D-Mail (Type-P)              |
       |                    ↓                      |
       |              Paintress learns             |
       |                                           |
       ←─── Convergence → new Shibito ────────────┘
```

Over time, this loop produces a system that makes fewer mistakes — not because the AI models improve, but because the **organizational memory** (Lumina, Shibito, ADRs, D-Mail history) becomes richer with every cycle.

---

## 6. Delegation Levels

The ecosystem supports progressive delegation. As trust builds, the human hands off more:

### Level 1: Supervised (Starting Point)

The human reviews every output from every tool:

- Reads and edits every Sightjack specification
- Reviews every Paintress PR in detail
- Reviews every Amadeus D-Mail including LOW

This is appropriate when the tools are new and the human is calibrating trust.

### Level 2: Trust but Verify (Target State)

The human focuses on decisions only:

- Reads Sightjack specifications, approves without heavy editing
- Skims Paintress PRs, merges quickly
- Ignores LOW D-Mails, glances at MEDIUM, decides on HIGH

This is the steady state described in Section 3 ("A Day in the Life").

### Level 3: Exception-Based (Future)

The human is only pulled in for exceptions:

- Sightjack auto-designs from Linear backlog; human reviews weekly
- Paintress auto-merges (with sufficient test coverage as gate)
- Amadeus auto-resolves MEDIUM; human only sees HIGH and Convergence

This requires Phase 2 of Amadeus (production monitoring) and significant accumulated Lumina/Shibito. The system has enough history to self-correct for most cases.

### Level 4: Strategic Only (Aspirational)

The human provides quarterly product direction. The system operates autonomously for day-to-day development. Human intervention is limited to:

- Major product pivots
- Architecture redesign decisions (Convergence Alerts)
- External stakeholder management

This level requires mature AI agent capabilities beyond current (2026) technology, but the architecture is designed to support it.

---

## 7. What This Is Not

To be precise about the vision, it's worth stating what this ecosystem does NOT aim to be:

**Not a "vibe coding" tool.** This is not about generating code from vague prompts. Sightjack's explicit DoD and dependency mapping ensure specifications are precise before implementation begins.

**Not a replacement for product thinking.** The system implements decisions; it does not make product decisions. "What should we build?" remains the human's most important job.

**Not an autonomous deployment system (yet).** Phase 1 requires human merge approval. The system proposes changes; the human commits them to reality. This is by design — the weight of changing the world line rests on the human.

**Not vendor-locked.** While currently built on Claude Code, the architecture (UNIX philosophy, Linear as protocol, game-mechanic-driven workflow) is model-agnostic. The agents could run on any sufficiently capable LLM.

---

## 8. Metrics of Success

How do we know the vision is being achieved?

**Lead time reduction.** Time from "idea in head" to "code in production." Target: 80% reduction from baseline.

**Decision-to-execution ratio.** Percentage of the developer's time spent on decisions vs. execution tasks. Target: >80% decisions, <20% execution.

**D-Mail resolution rate.** Percentage of D-Mails that correctly identify real issues (vs. false positives rejected by the human). Target: >90% accuracy.

**Convergence frequency.** Rate of World Line Convergence warnings. Should decrease over time as Shibito and Lumina mature.

**Cycle independence.** Number of Issues that flow from Sightjack → Paintress → Amadeus without human intervention beyond initial approval and final merge. Target: >70% of Issues.

---

## 9. The Three Tools, Unified

| | Sightjack | Paintress | Amadeus |
|---|---|---|---|
| Origin | SIREN | Expedition 33 | Steins;Gate 0 |
| Mechanic | Sight Jack (see through others' eyes) | Paint (rewrite reality) | Amadeus (memory across world lines) |
| Phase | Pre-implementation | Implementation | Post-integration |
| Input | Rough intent | 80% specification | Merged main |
| Output | Issue with DoD + deps | PR with verified code | D-Mails + Divergence score |
| Local state | `.siren/` | `.expedition/` | `.divergence/` |
| Human touchpoint | Approve specification | Approve merge | Resolve HIGH D-Mail |
| Persistence | Linear | Linear + GitHub | Linear + `.divergence/` |
| Learning | Shibito (failure memory) | Lumina (success/failure context) | D-Mail history + Convergence patterns |

Together, they form a cycle:

```
Human Intent
    ↓
Sightjack ──→ Paintress ──→ main ──→ Amadeus
    ↑                                    |
    └────────── D-Mail feedback ─────────┘
```

The human sits above this cycle, providing intent at the beginning and making decisions at key gates. The cycle runs continuously. The system learns from every rotation.

This is what AI-driven development looks like in 2026: not a single copilot suggesting code, but a **structured team of specialists** — each with a clear responsibility, a shared memory, and a feedback loop that makes the whole system smarter over time.
