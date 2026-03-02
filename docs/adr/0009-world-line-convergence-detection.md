# 0009. World Line Convergence Detection

**Date:** 2026-02-23
**Status:** Accepted

## Context

When multiple D-Mails target the same area within a short time window, it
signals a structural issue that individual feedback items cannot convey. For
example, three separate feedback D-Mails all pointing to `auth/session.go`
within two weeks suggests a systemic problem rather than isolated incidents.

Detecting this pattern — "world line convergence" — and escalating it as a
distinct signal prevents repeated low-severity feedback from masking a
high-severity structural concern.

## Decision

Implement window-based convergence detection in Phase 4 of the verification
pipeline (`AnalyzeConvergence`).

### Algorithm

1. **Time window**: Filter archive D-Mails by `metadata.created_at` within
   `now - WindowDays` (default: 14 days).
2. **Self-exclusion**: Skip D-Mails with `kind: convergence` to prevent
   cascading self-referential detection.
3. **Target grouping**: Group remaining D-Mails by their `targets` field.
   Track `firstSeen` and `lastSeen` timestamps per target.
4. **Threshold evaluation**:
   - `count >= Threshold` (default: 3) -> MEDIUM severity alert
   - `count >= Threshold * 2` -> HIGH severity alert
5. **D-Mail generation**: Only HIGH severity alerts produce convergence D-Mails
   (`GenerateConvergenceDMails`). These are written to archive with
   `kind: convergence` and `metadata.convergence_for` containing the related
   D-Mail names.

### Configuration

```yaml
convergence:
  window_days: 14   # Rolling time window
  threshold: 3      # Minimum D-Mails to trigger alert
```

### Output

`ConvergenceAlert` struct captures the detection result:

- `Target`: The repeatedly-hit area
- `Count`: Number of D-Mails in the window
- `DMails`: Names of contributing D-Mails
- `Severity`: MEDIUM or HIGH
- `FirstSeen` / `LastSeen`: Time range of the convergence

## Consequences

### Positive

- Detects systemic issues that individual D-Mails cannot convey
- Severity escalation (MEDIUM -> HIGH at 2x threshold) reflects increasing urgency
- Self-exclusion prevents infinite convergence-on-convergence loops
- Convergence D-Mails enter the normal routing pipeline (archive + outbox)

### Negative

- Detection depends on accurate `metadata.created_at` timestamps
- Window-based approach may miss slow-building convergence outside the window
- Threshold tuning requires empirical observation per project
