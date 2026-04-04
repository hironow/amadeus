# S0039. Harness Evolution Loop

**Date:** 2026-04-04
**Status:** Accepted

## Context

S0038 introduced the harness layer with three sub-packages (filter, verifier,
policy) representing the LLM-dependence spectrum. This establishes the
structural foundation, but does not describe how harness logic *evolves* over
time or how the same spectrum applies to the code that the tools develop.

AutoHarness (arxiv 2603.03329v1) demonstrates that a harness can mature from
LLM-dependent to fully deterministic. Our 4-tool pipeline (sightjack →
paintress → amadeus → phonewave) has a unique property: the tools themselves
contain harness logic, AND they produce harness-equivalent artifacts (semgrep
rules, type constraints, CI checks) in the repositories they develop.

## Decision

Adopt two parallel harness evolution loops, both following the same
filter → verifier → policy spectrum.

### Loop 1: Internal Harness Evolution

Each tool's own harness functions evolve from LLM-dependent to deterministic.

```
harness/filter (v1)     harness/verifier (v2)     harness/policy (v3)
LLM decides             LLM output validated      Code decides
    |                        |                        |
    +--- pattern found ----->+--- rule solidified ---->+
```

**Trigger**: When a filter-layer prompt consistently produces the same
classification (e.g., Lumina in paintress accumulates success/failure
patterns), extract the pattern as a verifier rule, then as a policy function.

**Tracking**: Physical file movement from `filter/` to `verifier/` to
`policy/` is visible in git history. The `version` field in YAML prompts
tracks optimization iterations.

**Examples**:
- paintress issue selection: prompt → priority rules → GradientGauge policy
- amadeus ADR compliance: prompt → divergence axes → semgrep rule
- sightjack wave unlock: prompt → prerequisite check → EvaluateUnlocks policy

### Loop 2: Target Repository Harness Evolution

The tools produce harness-equivalent artifacts in the repositories they develop.
These artifacts also evolve along the same spectrum.

```
sightjack (filter)      amadeus (verifier)       CI/semgrep (policy)
"follow ADR"            "ADR divergence: 0.4"    "layer-domain-no-import-X"
    |                        |                        |
    +--- spec D-Mail ------->+--- feedback D-Mail ---->+
                             |                        ^
                             +--- pattern repeated -->+
                                  (semgrep rule gen)
```

**Phase A (filter)**: sightjack instructs paintress via specification D-Mail
to follow ADRs and DoDs. The instruction is an LLM prompt — the target repo
has no enforcement mechanism yet.

**Phase B (verifier)**: amadeus evaluates merged code against ADRs using Claude.
When divergence is detected, feedback D-Mails are generated. The enforcement
exists but requires LLM judgment.

**Phase C (policy)**: Patterns that amadeus detects repeatedly (e.g., "domain
imports session", "missing error handling in handler") are converted to semgrep
rules in the target repo's `.semgrep/` directory. CI now enforces the rule
deterministically — no LLM needed.

### The Recursive Property

These two loops reinforce each other:

1. As Loop 1 matures (tool harness evolves), the tools make better decisions
   with less LLM dependency → fewer false positives, more precise D-Mails.

2. As Loop 2 matures (target repo gets more semgrep rules), amadeus finds
   fewer divergences → the tools can focus on higher-level architectural
   compliance that hasn't been codified yet.

3. Loop 2's semgrep rule generation can itself evolve via Loop 1:
   initially, sightjack's specification says "generate semgrep rules for
   repeated violations" (filter). Later, amadeus automatically proposes
   semgrep rules when it detects ≥3 identical violations (verifier → policy).

### Feedback Direction

This system uses **negative feedback** (stabilizing):
- More policy rules → fewer violations detected → fewer D-Mails → less LLM usage
- The system converges toward a state where LLM is needed only for novel,
  unprecedented situations

This is desirable (per CLAUDE.md Perspective): the system becomes more
predictable and manageable over time, with LLM reserved for genuinely
creative decisions.

## Consequences

### Positive
- Tools become more reliable as harness matures (fewer LLM-dependent decisions)
- Target repositories become more self-enforcing (semgrep rules accumulate)
- Cost reduction: LLM calls decrease as policy functions replace prompts
- Auditability: git history shows which decisions graduated from filter to policy

### Negative
- Risk of premature policy crystallization (codifying wrong patterns)
- semgrep rule accumulation needs periodic review (rules may become stale)
- The evolution loop requires human oversight to validate policy promotions

### Neutral
- The speed of evolution depends on the volume and diversity of development work
- phonewave is not directly involved in either loop (courier only)
- GEPA optimization (Phase 3) accelerates Loop 1 by systematically improving prompts
