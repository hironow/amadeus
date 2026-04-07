# S0042. Cross-Tool Contract Testing v2 — Unified Layout

**Date:** 2026-04-07
**Status:** Accepted
**Supersedes:** S0021

## Context

S0021 established the cross-tool contract testing strategy with a central
suite in phonewave and per-tool mirror tests. Since then, several
improvements have been made that the original ADR does not reflect:

- Golden file count grew from 9 to 13 (added corrective-feedback.md,
  sightjack-report.md, ci-result.md, sightjack-spec-priority.md)
- Per-tool golden file location unified from `internal/*/testdata/contract/`
  to `tests/contract/testdata/golden/` across all 4 tools
- A sync script (`scripts/sync-contract-golden.sh`) was introduced in
  phonewave to rsync golden files to sibling tools, replacing manual copies
- Per-tool parsers unified: amadeus, sightjack, and paintress all use
  `domain.ParseDMail()` (phonewave uses `contract.Parse()` as the
  Postel-liberal reference parser)
- Validator return type divergences documented in S0021 still exist:
  amadeus returns `[]string`, paintress/sightjack return `error`,
  phonewave uses `ExtractDMailKind` returning `(DMailKind, error)`.
  Contract tests deliberately test parse compatibility only, not
  validator API shape (per ADR S0020 accepted divergence)
- Corrective metadata round-trip tests added to contract and scenario layers

## Decision

Supersede S0021 with the following updated contract testing layout:

1. **Central contract suite** (`phonewave/tests/contract/`) — standalone Go
   module with zero tool dependencies. Contains:
   - Postel-liberal parser (`contract.Parse`) more permissive than any tool
   - 13 golden files covering all 7 D-Mail kinds + edge cases
   - JSON Schema validation (`dmail-frontmatter.v1.schema.json`)
   - `ParseFrontmatterMap` for generic YAML access
   - `IdempotencyKey` for content-based dedup verification

2. **Per-tool mirror tests** (`{tool}/tests/contract/contract_test.go`,
   `//go:build contract`) — each tool parses golden files with its own
   `domain.ParseDMail()`, verifying cross-tool compatibility at the actual
   parser level. Tests include:
   - Parse all golden files successfully
   - Reject unknown kinds and future schema versions (strict validation)
   - Corrective metadata round-trip (CorrectionMetadataFromMap)

3. **Sync mechanism** (`phonewave/scripts/sync-contract-golden.sh`) —
   rsync with `--delete` from phonewave canonical to amadeus, sightjack,
   and paintress. Prevents golden file drift between repos.

4. **Golden file location** — `tests/contract/testdata/golden/` in all
   4 tools. The `internal/` prefix is no longer used for contract tests.

## Consequences

### Positive

- Single sync script prevents golden file drift (was manual copy in S0021)
- Unified parser API (`domain.ParseDMail`) simplifies per-tool tests
- Corrective metadata coverage ensures improvement loop fidelity
- 13 golden files cover all 7 D-Mail kinds including edge cases

### Negative

- Golden files remain duplicated across 4 repos (sync script mitigates)
- `//go:build contract` tests still require explicit `-tags=contract`

### Neutral

- phonewave remains the canonical golden file owner (courier role)
- phonewave uses `contract.Parse()` intentionally (Postel reference, not
  domain parser) — this is by design, not a divergence
