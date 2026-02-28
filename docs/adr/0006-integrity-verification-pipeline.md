# 0006. Integrity Verification Pipeline

**Date:** 2026-02-23
**Status:** Accepted

## Context

amadeus is an integrity verifier that detects drift between architectural
decisions (ADRs, DoDs) and actual implementation. A single monolithic check
would be too expensive to run on every commit. The tool needs a pipeline that
supports both incremental (diff-based) and full (calibration) checks, with
automatic promotion between the two modes.

Additionally, the exit code must communicate the nature of the outcome to
calling processes (CI, git hooks, scripts) without parsing output.

## Decision

Implement a five-phase pipeline (`RunCheck`) with hybrid diff/full calibration
and a tripartite exit code scheme.

### Pipeline Phases

1. **Phase 0 — Inbox consumption**: Scan `.gate/inbox/` for inbound D-Mails
   from external tools. Move consumed files to `archive/` and record in
   `consumed.json`. Skipped in `--dry-run` mode.
2. **Phase 1 — ReadingSteiner**: Detect shifts in the codebase. `DetectShift()`
   for incremental checks (changes since last commit), `DetectShiftFull()` for
   full repository scans. Returns a `ShiftReport` with diffs, merged PRs, and
   codebase structure.
3. **Phase 2 — Claude evaluation + DivergenceMeter**: Build a language-specific
   prompt (diff or full), invoke Claude CLI, parse the structured JSON response,
   and run `DivergenceMeter.ProcessResponse()` to compute the four-axis
   divergence score and severity.
4. **Phase 3 — D-Mail generation**: Create feedback D-Mails from Claude's
   candidates with the computed severity. Route all D-Mails to both `archive/`
   (permanent record) and `outbox/` (for phonewave delivery). No pending state
   exists (removed per MY-359).
5. **Phase 4 — Convergence detection**: Run `AnalyzeConvergence()` on all
   archive D-Mails. Generate convergence D-Mails for HIGH severity alerts.

### Hybrid Diff/Full Calibration

- **Interval-based**: Full check triggers every N diff checks
  (`FullCheck.Interval`, default: 10).
- **Divergence jump promotion**: If `|current - previous| >= OnDivergenceJump`
  (default: 0.15), the next check is promoted to full via `ForceFullNext`.
- **State persistence**: `CheckCountSinceFull` and `ForceFullNext` are persisted
  in `latest.json` across runs.

### Exit Code Tripartite

| Code | Meaning | Trigger |
|------|---------|---------|
| 0 | Success — no drift detected | `err == nil` |
| 1 | Runtime/configuration error | any non-`DriftError` |
| 2 | Drift detected — D-Mails generated | `*DriftError` |

`ExitCode()` uses `errors.As` to distinguish `DriftError` from other errors.
`main.go` maps the return to `os.Exit()` and writes context-specific messages
to stderr ("error:" for code 1, "drift detected:" for code 2).

## Consequences

### Positive

- Incremental checks are fast (diff only); full scans provide periodic calibration
- Automatic promotion ensures full scans happen when divergence shifts significantly
- Exit code 2 enables CI gates (`amadeus check || exit $?`) without output parsing
- Phase 0 inbox consumption enables asynchronous inter-tool communication

### Negative

- Pipeline complexity increases debugging difficulty for multi-phase failures
- Full check depends on Claude CLI availability and response quality
- State persistence across runs requires careful management of `latest.json`
