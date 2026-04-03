# S0038. Harness Layer

**Date:** 2026-04-04
**Status:** Accepted
**Supersedes:** S0007 (adds harness/ to layer conventions)

## Context

LLM-powered tools (sightjack, amadeus, paintress) interleave three distinct
concerns in the domain layer: deterministic decisions, output validation, and
LLM prompt specification. This makes it impossible to tell which logic depends
on LLM behavior and which is purely algorithmic.

AutoHarness (arxiv 2603.03329v1) formalizes the "Harness as Policy" spectrum:
a harness mediates between an LLM and the task environment, and as harness
logic matures it progresses from LLM-dependent to fully deterministic.

## Decision

Introduce `internal/harness/` as a standard layer in all 4 tools. The harness
layer sits between domain (pure types) and session (I/O adapters), decomposed
into three sub-packages along the LLM-dependence spectrum.

### Structure

```
internal/harness/
  harness.go          Facade — single import surface for all callers
  policy/             Deterministic decisions (no LLM, no I/O)
  verifier/           Validation rules (no LLM, no I/O)
  filter/             LLM action space: PromptRegistry + YAML prompts
    prompts/*.yaml    Externalized prompt templates (GEPA optimize ready)
```

### Facade Pattern

External callers import only `internal/harness`:

```go
import "github.com/hironow/{tool}/internal/harness"

harness.IsPipelinePR(pr)
harness.ValidateDMail(dmail)
harness.MustDefaultPromptRegistry()
```

Sub-packages (policy, verifier, filter) are implementation details.

### Dependency Direction

```
cmd -> usecase -> port <- session
         |                   |
    +----+-------------------+----+
    |         HARNESS             |
    |  filter -> verifier -> policy |
    |              |              |
    |           domain            |
    +----+-------------------+----+
         |                   |
    eventsource          platform
```

| Package | May import | Must NOT import |
|---------|-----------|-----------------|
| harness/policy | domain | harness/verifier, harness/filter, usecase, session, cmd, platform, eventsource |
| harness/verifier | domain, harness/policy | harness/filter, usecase, session, cmd, platform, eventsource |
| harness/filter | domain, harness/verifier, harness/policy | usecase, session, cmd, platform, eventsource |

Enforced by semgrep (5 rules, no exceptions including tests).

### What Stays in domain/

| Condition | Location |
|-----------|----------|
| Aggregate internal state methods | domain/ |
| Type constructors (NewPRState, etc.) | domain/ |
| ValidateEvent (eventsource boundary) | domain/ |
| Worker coordination (IssueClaimRegistry) | domain/ |

### PromptRegistry

YAML-based prompt management following the PromptRegistry pattern:

```yaml
name: divergence_meter
version: 1
description: "Evaluate merged code against ADRs/DoDs"
variables:
  - name: diff
    description: "git diff content"
template: |
  You are amadeus...
  {diff}
  {#if is_full_check}Full scan mode{#else}Diff mode{/if}
```

Features:
- `embed.FS` for compile-time embedding
- `{key}` placeholder expansion
- `{#if key}...{#else}...{/if}` conditionals (truthy: non-empty, not "false")
- Version field for GEPA optimization tracking
- `Must*` variants (panic on impossible embed.FS errors)

### Tool-Specific Scope

| Tool | policy/ | verifier/ | filter/ |
|------|---------|-----------|---------|
| amadeus | merge, convergence, pipeline | D-Mail, provider error | 6 YAML prompts |
| sightjack | wave (43 funcs), scan, config, review | wave validation, provider error | 21 YAML prompts |
| paintress | gradient, reserve, retry, wave, strategy | D-Mail, review, provider error | 14 YAML prompts |
| phonewave | routing, dedup | D-Mail schema | N/A (no LLM) |

### Evolution Path

```
filter (LLM-dependent) -> verifier (validation) -> policy (deterministic)
```

When a decision matures from "ask LLM" to "code can decide", the function
physically moves from filter/ to policy/. Trackable via git history.

## Consequences

### Positive
- LLM-dependence spectrum is visible in package structure
- Prompts are externalized and GEPA-optimizable
- Semgrep enforces layer boundaries (no exceptions)
- Clear litmus test for where logic belongs

### Negative
- Additional layer adds import path complexity
- Facade re-exports require maintenance when functions are added
- Go text/template conditionals must be pre-rendered for simple {key} expansion

### Neutral
- phonewave gets harness/ for consistency even though it has no filter/
- Existing PolicyEngine (WHEN/THEN in usecase/) is unaffected (different concept)
