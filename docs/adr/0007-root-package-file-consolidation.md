# 0007. Root Package File Consolidation

**Date:** 2026-02-25
**Status:** Accepted

## Context

ADR 0011 (Event Sourcing) introduced several micro-files in the root package, each containing fewer than 50 lines of code. While small files are not inherently problematic, these files contained types and functions that naturally belong alongside their primary consumers:

| File | Lines | Contents |
|---|---|---|
| `event_store.go` | 15 | `EventStore` interface |
| `event_payloads.go` | 47 | 8 event payload structs |
| `issue_id.go` | 28 | `ExtractIssueIDs` + regex pattern |
| `divergence_meter.go` | 31 | `MeterResult` + `DivergenceMeter` |

This fragmentation reduced cohesion: developers had to navigate multiple files to understand a single concept (e.g., Event types were split across `event.go`, `event_store.go`, and `event_payloads.go`).

This ADR follows the same rationale as sightjack ADR 0010, adapted for amadeus's flat package structure.

## Decision

Consolidate micro-files into their natural homes based on conceptual affinity:

| Source (deleted) | Destination | Rationale |
|---|---|---|
| `event_store.go` | `event.go` | `EventStore` interface belongs with `Event` type |
| `event_payloads.go` | `event.go` | Payload structs are event-specific types |
| `issue_id.go` | `dmail.go` | `ExtractIssueIDs` is D-Mail issue extraction |
| `divergence_meter.go` | `scoring.go` | `DivergenceMeter` is part of the scoring pipeline |

Test files follow the same consolidation:

| Source (deleted) | Destination |
|---|---|
| `event_payloads_test.go` | `event_test.go` |
| `issue_id_test.go` | `dmail_test.go` |
| `divergence_meter_test.go` | `scoring_test.go` |

This is a purely structural change: no code was added, removed, or modified beyond moving declarations between files. All tests pass before and after consolidation.

## Consequences

### Positive

- Higher cohesion: related types and functions live in the same file
- Reduced file count: 7 fewer files to navigate (4 source + 3 test)
- Easier discoverability: `event.go` is the single source for all event-related types
- Consistent with Tidy First principle of structural changes preceding behavioral changes

### Negative

- Larger individual files (e.g., `event.go` grew from ~30 to ~90 lines), though still well within reasonable bounds
- Slightly larger diffs when modifying consolidated files

### Neutral

- No behavioral changes; all existing tests pass unmodified
- File-level organization continues to follow the convention established in ADR 0014
