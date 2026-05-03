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
