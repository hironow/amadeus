# Rival Contract v1 (amadeus — drift controller)

amadeus is the **drift controller** for Rival Contract v1. It projects
the current revision of every contract from the D-Mail archive, feeds
contract context into the Claude divergence-check prompt, and emits
canonical corrective D-Mails when the implementation drifts from
contract.

The full cross-tool plan lives at
[`refs/plans/2026-05-03-rival-contract-v1.md`](../../refs/plans/2026-05-03-rival-contract-v1.md).

## What it is

A Rival Contract v1 is the canonical Markdown body of a `kind: specification`
D-Mail produced by sightjack. amadeus treats the chain of revisions
(linked by `metadata.supersedes`) as the source of truth for what the
implementation should look like.

amadeus does not produce Rival Contracts. It cites them and asks the
producer (sightjack) to amend them when reality diverges.

## Where the controller lives

| Concern | File |
|---------|------|
| Archive projection (current revision per contract) | `internal/session/rival_contract_archive.go` |
| Pure parser shared with sj/pt | `internal/harness/policy/rival_contract.go` |
| Drift prompt context builder | `internal/session/prompt_builder.go` |
| Corrective D-Mail body shapes | `internal/session/rival_contract_correction.go` |
| D-Mail emitter wiring | `internal/session/amadeus_dmail.go` |

## Archive projection: ProjectCurrentContracts

`ProjectCurrentContracts` walks the D-Mail archive and returns, per
`contract_id`, the highest-revision specification D-Mail that is not
itself superseded. The function is pure: no I/O, no LLM, deterministic.

Selection rules:

1. Group all `kind: specification` D-Mails with
   `metadata.contract_schema = rival-contract-v1` by `contract_id`.
2. Within each group, pick the entry with the largest
   `contract_revision`.
3. A contract whose D-Mail name appears in another contract's
   `supersedes` field is excluded from the current set (it has been
   superseded by a later revision).

When the input is ambiguous, the function does **not** guess. It returns
the contract in a `[]ContractConflict` slice with a structured `Reason`
so callers can emit feedback rather than silently picking one.

## Conflict handling

Two cases produce a `ContractConflict` rather than a `CurrentContract`:

- **Same-revision conflict** — two D-Mails share the same
  `contract_id` and the same `contract_revision`. amadeus emits a
  corrective D-Mail asking sightjack to issue an explicit revision
  bump.
- **Invalid supersedes** — a `supersedes` field references a D-Mail
  name that does not exist in the archive, or a revision that is not
  strictly less than the current. amadeus reports the dangling pointer.

amadeus never invents a revision. It surfaces the inconsistency and
lets the producer fix it.

## Drift prompt context

When amadeus performs a divergence check (diff or full), it injects the
projected current contract into the Claude prompt. The prompt section
provides:

- `## Intent` — what the contract aims to deliver
- `## Domain` — domain terms and ownership for entity-level reasoning
- `## Decisions` — chosen approach and trade-offs (so the model does
  not propose a rejected alternative)
- `## Boundaries` — guardrails for what must not be changed
- `## Evidence` — acceptance signals to verify against

`## Steps` is omitted from the drift context because amadeus checks
delivered code against decisions and boundaries, not against an ordered
implementation plan.

## Corrective D-Mail body shapes

When drift is detected, amadeus emits feedback in two canonical shapes:

- `## Violated Contract` — appended to a `kind: implementation-feedback`
  D-Mail aimed at paintress. Lists the specific contract clauses that
  were violated and quotes the offending code/PR locations.
- `## Contract Amendments` — appended to a `kind: design-feedback`
  D-Mail aimed at sightjack. Proposes textual edits to the contract
  when the implementation has uncovered information that justifies a
  contract change rather than a code change.

The amendment shape is the input to sightjack's amendment loop
described in [sightjack/docs/rival-contract-v1.md](../../sightjack/docs/rival-contract-v1.md).

## Routing: canonical D-Mail v1 kinds only

amadeus emits only canonical D-Mail v1 kinds on the Rival Contract
path:

- `specification` — produced by sightjack (amadeus only consumes)
- `report` — convergence/divergence summary
- `design-feedback` — to sightjack (contract amendments)
- `implementation-feedback` — to paintress (contract violations)
- `convergence` — successful merge confirmation

Non-canonical kinds are intentionally never emitted by the Rival
Contract path. dominator's Phase 4 fix removed the legacy
non-canonical kind across the system; amadeus mirrors that
discipline.

## Cross-tool reference

| Tool | Role | Doc |
|------|------|-----|
| sightjack | producer | [sightjack/docs/rival-contract-v1.md](../../sightjack/docs/rival-contract-v1.md) |
| paintress | consumer | [paintress/docs/rival-contract-v1.md](../../paintress/docs/rival-contract-v1.md) |
| amadeus | drift controller (you are here) | this file |
| dominator | NFR judge | [dominator/docs/rival-contract-v1.md](../../dominator/docs/rival-contract-v1.md) |

## Required metadata read from each contract

```yaml
metadata:
  contract_schema: rival-contract-v1
  contract_id: "<stable work-unit id>"
  contract_revision: "1"
  supersedes: ""
```

`contract_schema = rival-contract-v1` is the gate that activates the
drift controller. Specifications without this schema marker are not
projected as contracts.

## Plan reference

- [`refs/plans/2026-05-03-rival-contract-v1.md`](../../refs/plans/2026-05-03-rival-contract-v1.md) — full design, phase plan, risks
- [`refs/scripts/check_rival_contract_docs.sh`](../../refs/scripts/check_rival_contract_docs.sh) — gap-check enforcement

## v1.1 additions

Rival Contract v1.1 is a purely additive minor extension. The schema name
remains `rival-contract-v1`. amadeus gains a new optional branch in the
divergence prompt that activates when the projected current contract
carries `metadata.domain_style: event-sourced`. The corrective D-Mail
shapes (`## Violated Contract`, `## Contract Amendments`) are unchanged.

Plan: [`refs/plans/2026-05-03-rival-contract-v1-1-extensions.md`](../../refs/plans/2026-05-03-rival-contract-v1-1-extensions.md).

### `metadata.domain_style` accepted by the parser

`ParseRivalContractMetadata` accepts an OPTIONAL `domain_style` key with
exactly three enumerated values: `event-sourced`, `generic`, `mixed`.
Unknown values are rejected. A missing key parses as the empty string and
is treated as `generic` by amadeus (no behavior change vs v1).

The parser never infers `domain_style` from ADRs, environment variables,
or any other side channel. The metadata map is the only signal.

### Divergence prompt glossary preamble

When the current contract for a divergence check carries
`metadata.domain_style == "event-sourced"`, the prompt builder injects an
event-sourcing glossary preamble (Command / Event / Read Model /
Aggregate / Policy) into the divergence prompt context. Both `diff` and
`full` divergence prompt paths share the same branch via
`renderEventSourcedGlossarySection` in
`internal/session/prompt_builder.go`. The preamble has Japanese (`ja`)
and English (`en`) variants matching the surrounding locale.

When the current contract has no `domain_style`, or carries `generic` /
`mixed`, the divergence prompt is bit-identical to the v1 surface. The
divergence scoring function and corrective D-Mail emission are not
affected by this branch.

### Canonical projection: ProjectCurrentContracts

`ProjectCurrentContracts` is canonical here in amadeus
(`internal/harness/policy/rival_contract.go`). sightjack v1.1 added an
internal copy of this function to its own parser package so that the
producer-side REASONS Canvas export subcommand (`--wave <id>` mode)
resolves the current revision deterministically using the same selection
rules. A regression
test in sightjack (`TestProjectCurrentContracts_BehavesLikeAmadeus`)
enforces parity. amadeus remains the source of truth; the sj copy is a
controlled duplicate.

### What amadeus does NOT do

- amadeus never SETS `domain_style`. The producer (sightjack) is the
  only writer.
- amadeus does not invoke the producer-side REASONS Canvas export
  subcommand. That projection is a sightjack-only tool; the drift loop
  has no need for it.
- The corrective D-Mail body shapes (`## Violated Contract`,
  `## Contract Amendments`) are unchanged from v1; v1.1 only adds prompt
  context, not new corrective shapes.

### Backward compatibility

Legacy v1 D-Mails (no `domain_style` key) produce a divergence prompt
that is bit-identical to the v1 prompt. The v1.1 branch is opt-in purely
through producer-emitted metadata. Tools that haven't been upgraded
continue to work unchanged.

## v1.2 additions — integration test coverage

Rival Contract v1.2 is a test-only minor revision. The schema name
remains `rival-contract-v1` and no production code path changed.
amadeus gains both consumer-side round-trip integration coverage and a
white-box amendment-emit golden test that produces the source-of-truth
fixture sightjack consumes from disk.

Plan: [`refs/plans/2026-05-03-rival-contract-v1-2-integration-e2e.md`](../../refs/plans/2026-05-03-rival-contract-v1-2-integration-e2e.md).

### Consumer round-trip integration

`tests/integration/rival_contract_roundtrip_test.go` reads three
committed fixtures and exercises amadeus's parser end-to-end:

| Fixture | Asserts |
|---|---|
| `tests/integration/testdata/rival/canonical-spec-v1.md` | byte-identical copy of sj's produced `canonical-spec-v1.md`; am parses it via `ParseRivalContractBody` + `ParseRivalContractMetadata` and the result matches the canonical Go struct expectation |
| `tests/integration/testdata/rival/legacy-spec.md` | legacy v1 (no `domain_style`) gracefully parses without rejecting metadata |
| `tests/integration/testdata/rival/event-sourced-v1.md` | a v1.1 D-Mail with `metadata.domain_style: event-sourced` parses correctly |

Three integration tests total. A regression in sj's `ComposeSpecification`
breaks amadeus's roundtrip test; a regression in am's parser breaks the
same test. Cross-tool drift is caught either way.

### Amendment emit white-box golden (am is source of truth)

`internal/session/rival_amendment_emit_integration_test.go` is a
white-box `package session` test that calls amadeus's real corrective
body composer (the path that wires `composeCorrectiveBodyWithContract`
inside `internal/session/amadeus_dmail.go`'s emit flow). The emitted
body is written byte-stable to
`internal/session/testdata/rival/cross-tool/amadeus-emitted-correction.md`
and asserted against a committed golden.

The white-box placement is deliberate: amadeus's emit functions remain
**unexported**. Putting the test inside `package session` exercises the
real production path without forcing an API boundary change. The
emitted golden is the **source of truth** — sightjack commits a
byte-identical copy under
`sightjack/tests/integration/testdata/rival/cross-tool/amadeus-emitted-correction.md`,
and `refs/scripts/check_rival_amendment_fixture.sh` enforces byte
identity (wired into `just gap-check-rival-contract`).

Two tests in this file:

- `TestRivalAmendmentEmit_AppendsContractAmendmentsSection_WritesGolden`
  (golden diff against the committed `amadeus-emitted-correction.md`,
  with the standard `-update` flag pattern for maintenance).
- `TestRivalAmendmentEmit_BulletGrammarStable` (asserts the bullet
  grammar amadeus emits remains stable against accidental rewording).

### Test inventory delta (am only)

| Phase | Tests added (am) |
|---|---|
| 1.2A — consumer round-trip | 3 integration tests |
| 1.2B — amendment emit white-box | 2 white-box tests |

Total: 5 cross-component tests added in v1.2 for amadeus. Cross-side
gap-check enforces byte-identity am golden ↔ sj copy; the cycle is
provably consistent through the on-disk D-Mail format.

### What did NOT change

- Schema (still `rival-contract-v1`; v1 invariants 1-13 maintained).
- The corrective D-Mail body shapes (`## Violated Contract`,
  `## Contract Amendments`) — unchanged from v1; v1.1 added
  `domain_style` prompt context only, v1.2 adds tests only.
- `ProjectCurrentContracts` selection rules.
- The divergence prompt surface.
- Any production code path. The amendment emit test is white-box and
  requires no exporting of unexported emit functions.

v1.2 is purely additive test code and gap-check guards.
