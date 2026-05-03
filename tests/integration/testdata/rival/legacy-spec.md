---
name: sj-spec-billing-legacy-v1_cafef00d
kind: specification
description: Legacy v1 contract without v1.1 metadata
dmail-schema-version: "1"
issues:
  - MY-100
wave:
  id: billing:legacy-v1
  steps:
    - id: MY-100
      title: Refund expired credits
      description: Implement automatic refund for credits past their expiry window.
metadata:
  contract_id: legacy-v1
  contract_revision: "1"
  contract_schema: rival-contract-v1
  idempotency_key: 0000000000000000000000000000000000000000000000000000000000000001
  supersedes: ""
---

# Contract: Refund expired credits

## Intent

Issue refunds when credits expire so users are not silently charged for unused balance.

- Deliver wave "Refund expired credits" so the implementing tool can act on a single self-contained contract.
- Success means every implementation step under Steps is executed and its acceptance signal is observable in Evidence.

## Domain

- Cluster: billing
- Issues: MY-100
- Command: implementation tool will execute the steps listed below.
- Event: wave was specified by sightjack and ready for implementation.

## Decisions

- No explicit design decision recorded.

## Steps

1. Refund expired credits
   - Issue: MY-100
   - Detail: Implement automatic refund when a credit's expiry timestamp has passed.
   - Action type: implement

## Boundaries

- Do not modify issue-management state already applied above.
- Do not expand scope beyond the steps listed in this contract.

## Evidence

- test: just test
- lint: just lint
- Acceptance: Refund expired credits
