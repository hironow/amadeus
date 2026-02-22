# 0007. Four-Axis Divergence Scoring

**Date:** 2026-02-23
**Status:** Accepted

## Context

Measuring "how much has the implementation drifted from the design?" requires a
structured, multi-dimensional approach. A single divergence number cannot capture
the distinct failure modes: an ADR violation is qualitatively different from a
missing DoD item or a broken dependency constraint. The scoring system must
produce both a composite value for severity classification and per-axis detail
for actionable feedback.

## Decision

Adopt a four-axis weighted scoring system with configurable thresholds and
per-axis override rules.

### Four Axes

| Axis | Key | Default Weight | Measures |
|------|-----|---------------|----------|
| ADR Integrity | `adr_integrity` | 0.4 (40%) | Compliance with Architecture Decision Records |
| DoD Fulfillment | `dod_fulfillment` | 0.3 (30%) | Completion of Definition of Done criteria |
| Dependency Integrity | `dependency_integrity` | 0.2 (20%) | Correctness of dependency relationships |
| Implicit Constraints | `implicit_constraints` | 0.1 (10%) | Adherence to unstated but expected conventions |

### Scoring Pipeline

1. **Claude evaluation**: Claude scores each axis 0-100 with a detail explanation
   (`AxisScore` struct with `Score` and `Details`).
2. **`CalcDivergence()`**: Weighted sum of axis scores divided by 100 produces
   a 0.0-1.0 divergence value.
   ```
   internal = ADR*0.4 + DoD*0.3 + Dep*0.2 + Implicit*0.1
   divergence = internal / 100.0
   ```
3. **`DetermineSeverity()`**: Maps the divergence value to a severity tier using
   configurable thresholds, then applies per-axis overrides.

### Severity Classification

| Divergence Range | Severity |
|-----------------|----------|
| 0.00 - 0.25 | LOW |
| 0.25 - 0.50 | MEDIUM |
| 0.50+ | HIGH |

### Per-Axis Overrides

Critical single-axis scores can escalate severity regardless of the composite
value:

| Override | Threshold | Effect |
|----------|-----------|--------|
| `adr_integrity_force_high` | 60 | Forces HIGH if ADR score >= 60 |
| `dod_fulfillment_force_high` | 70 | Forces HIGH if DoD score >= 70 |
| `dependency_integrity_force_medium` | 80 | Forces MEDIUM (min) if Dep score >= 80 |

The `Overridden` flag in `DivergenceResult` tracks whether an override was applied.

### DivergenceMeter

`DivergenceMeter` bridges Claude's output and the scoring engine:
`ClaudeResponse` -> `CalcDivergence()` -> `DetermineSeverity()` -> `MeterResult`.
This separation allows the scoring logic to be tested independently of Claude.

## Consequences

### Positive
- Multi-axis scoring provides actionable feedback (which axis drifted most)
- Configurable weights allow projects to emphasize different concerns
- Per-axis overrides catch critical single-point failures that composite scores miss
- Scoring is deterministic given the same axis scores (testable without Claude)

### Negative
- Weight tuning requires empirical calibration per project
- Per-axis override thresholds are somewhat arbitrary initial values
- Claude's axis scores are subjective and may vary between runs
