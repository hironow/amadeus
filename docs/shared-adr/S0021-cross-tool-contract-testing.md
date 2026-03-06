# S0021. Cross-Tool Contract Testing

**Date:** 2026-03-02
**Status:** Accepted

## Context

The 4-tool ecosystem (phonewave, sightjack, paintress, amadeus) communicates
via D-Mail files — YAML frontmatter + Markdown body. Each tool has its own
D-Mail parser (`ParseDMail` / `parseDMailFrontmatter`) and validator
(`ValidateDMail` / `ExtractDMailKind`).

Without cross-tool testing, schema drift between parsers is invisible until
runtime. Known divergences discovered during implementation:

- **ValidateDMail return type**: phonewave returns `(string, error)`, sightjack
  returns `error`, paintress returns `error`, amadeus returns `[]string`
- **Kind enum validation**: phonewave/sightjack/amadeus validate kind against
  the schema v1 enum, but paintress does not
- **Targets field**: only amadeus has `Targets []string` in its DMail struct
- **Severity type**: amadeus uses typed `Severity`, others use `string`

These divergences are acceptable per ADR S0020 (Accepted Cross-Tool Divergence)
and ADR S0021 (Postel's Law), but must be documented and monitored.

## Decision

Implement golden file contract tests with two layers:

1. **Central contract suite** (`phonewave/tests/contract/`) — standalone Go
   module with zero tool dependencies. Contains:
   - Postel-liberal parser (more permissive than any tool's parser)
   - 9 golden files covering all 4 tools + edge cases
   - 7 test groups: parse compatibility, required fields, idempotency key,
     round-trip, Postel edge cases, targets field, JSON Schema validation

2. **Per-tool mirror tests** (`*/contract_test.go`, `//go:build contract`) —
   each tool parses the same golden files with its own parser, verifying
   cross-tool compatibility at the actual parser level.

Golden files are copied (not symlinked) to each tool's `testdata/contract/`
directory to maintain independence between repositories.

## Consequences

### Positive

- Schema drift is caught at test time, not runtime
- Each tool's parser compatibility is verified against all producers
- JSON Schema validation ensures tool output matches the formal schema
- Postel edge cases document the liberal/strict boundary explicitly
- Known divergences (like paintress missing kind validation) are documented
  in test comments rather than silently ignored

### Negative

- Golden files are duplicated across 4 repos (maintenance cost)
- Adding a new golden file requires updating all 4 repos
- `//go:build contract` tests don't run by default (`-tags=contract` required)

### Neutral

- The central contract suite lives in phonewave (canonical ADR/schema source)
- Per-tool mirror tests use `//go:build contract` to avoid slowing regular test runs
