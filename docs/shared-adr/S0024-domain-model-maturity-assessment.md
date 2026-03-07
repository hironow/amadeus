# S0024. Domain Model Maturity Assessment

**Date:** 2026-03-02
**Status:** Accepted

## Context

Four deferred items requested architectural evolution:

- R4-05: Dynamic budget management (replace static budget)
- R4-06: Formal Aggregate Root DDD (replace thin aggregates)
- R4-07: ES semantics unification (delivery scope vs session scope)
- R4-08: amadeus interface count expansion plan

### R4-05: Budget Management

Current static budget provides negative feedback (stabilizing):

- Fixed review/fix budget prevents runaway resource consumption
- SuccessRate() pure functions provide historical analysis (READ MODEL)
- Dynamic budget would create positive feedback loop (destabilizing)
- CLAUDE.md explicitly values negative feedback for system stability

### R4-06: Aggregate Root

Current model: thin/anemic aggregates with session-layer orchestration.
Stress points identified but manageable:

- State mutation ordering in session god objects
- Event replay vulnerability (no aggregate version guard)
- Pure-function domain model (root package) handles computation

Migration trigger conditions (NOT yet met):

- Domain logic requires cross-entity invariant enforcement
- State mutation bugs appear due to ordering issues
- Event replay produces inconsistent state

### R4-07: ES Semantics

phonewave uses delivery-scoped events (per-file lifecycle).
sightjack uses session-scoped events (per-review lifecycle).
This variance is intentional per ADR S0018 (Event Storming Alignment).

Cross-tool event correlation is achieved via D-Mail idempotency key
(SHA256 of name+kind+description+body), not via event scope unification.

### R4-08: amadeus Interfaces

amadeus has 9 interfaces proportional to its domain requirements
(Linear API, GitHub API, code analysis, D-Mail, projections, etc.).
This count is correct, not excessive.

Expansion trigger: new external system integration requiring
a new port in the hexagonal architecture.
Expansion pattern: follow existing interface + adapter pattern.

## Decision

All four items are assessed as "not needed at this time":

- R4-05: Static budget is the correct design (negative feedback)
- R4-06: Thin aggregates are sufficient (migration triggers not met)
- R4-07: Scope variance is intentional (cross-tool correlation via D-Mail)
- R4-08: Interface count is proportional (expand on demand)

## Consequences

### Positive

- No unnecessary architectural complexity
- System maintains negative feedback stability
- Clear trigger conditions for future reassessment

### Negative

- Session god objects remain (technical debt, manageable)
- No formal aggregate protection against state mutation bugs

### Neutral

- Each item has explicit trigger conditions for revisiting
- This ADR supersedes the deferred status in T-deferred-items.md
