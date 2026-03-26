# S0035. D-Mail Wave Field Extension (Amends S0005/S0031)

**Date:** 2026-03-26
**Status:** Accepted

## Context

Wave-centric mode requires D-Mails to carry wave and step metadata for
archive-based state projection. The existing D-Mail schema (S0005, amended
by S0031 for context) needs an optional `wave` field.

D-Mail archive/ serves as an append-only event source: specification D-Mails
define waves and steps, report D-Mails mark completion/failure, feedback
D-Mails indicate retry. Wave state is a projection — filter by wave.id and
step to reconstruct progress without editing any D-Mail.

## Decision

Add an optional `wave` field to the D-Mail v1 envelope:

### Specification D-Mail (defines wave + steps)

```yaml
---
dmail-schema-version: "1"
name: spec-auth-w1
kind: specification
description: "Authentication Foundation"
wave:
  id: "auth-w1"
  steps:
    - id: "s1"
      title: "Add JWT middleware"
      acceptance: "Middleware intercepts all /api/* routes"
    - id: "s2"
      title: "Write session store"
      prerequisites: ["s1"]
---
```

### Report D-Mail (marks step completion)

```yaml
---
dmail-schema-version: "1"
name: report-auth-w1-s1
kind: report
description: "Step s1 completed"
wave:
  id: "auth-w1"
  step: "s1"
---
```

### Rules

1. `wave` is optional — omission is valid (backward compatible)
2. Schema version remains "1" (additive extension, per S0031 pattern)
3. Receivers MUST NOT reject unknown fields (S0019 Postel's Law)
4. phonewave relays wave field without interpretation
5. `wave.steps[].id` must be unique within a wave
6. `wave.step` (singular, in report/feedback) references a step from the spec
7. Wave state = projection from filtered D-Mails, never persisted separately
8. D-Mails are immutable — state advances only by appending new D-Mails

### Types

```go
type WaveReference struct {
    ID    string        `yaml:"id"`
    Step  string        `yaml:"step,omitempty"`   // report/feedback
    Steps []WaveStepDef `yaml:"steps,omitempty"`  // specification
}

type WaveStepDef struct {
    ID, Title, Description, Acceptance string
    Targets, Prerequisites []string
}
```

### Projection Logic

```
filter(archive, wave.id) →
  specification → define steps (first spec wins)
  report(severity=low/empty) → step completed
  report(severity=high/medium) → step failed
  feedback → reset failed step to pending (retry)
```

## Consequences

### Positive

- D-Mail archive becomes self-contained event source for wave state
- No separate wave state file needed — projection is deterministic
- Backward compatible — existing D-Mails without wave field work unchanged
- phonewave relay is transparent (unknown YAML fields pass through)

### Negative

- Full archive scan required for projection (acceptable for CLI tool)
- Wave state not queryable without parsing all archive D-Mails

### Neutral

- Amends S0005 (adds field, does not change existing fields)
- TrackingMode flag (`--linear`) determines whether wave field is populated
