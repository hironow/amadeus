---
name: sj-spec-checkout-event-sourced-v1_b1b2b3b4
kind: specification
description: Event-sourced v1.1 contract carrying domain_style metadata
dmail-schema-version: "1"
issues:
  - MY-200
wave:
  id: checkout:event-sourced-v1
  steps:
    - id: MY-200
      title: Project order totals from order events
      description: Add an OrderTotalsView projection that consumes OrderPlaced and OrderRefunded events.
metadata:
  contract_id: event-sourced-v1
  contract_revision: "1"
  contract_schema: rival-contract-v1
  domain_style: event-sourced
  idempotency_key: 0000000000000000000000000000000000000000000000000000000000000002
  supersedes: ""
---

# Contract: Project order totals from order events

## Intent

Build the OrderTotalsView read model from the canonical OrderPlaced and OrderRefunded events.

- Deliver wave "Project order totals from order events" so the implementing tool can act on a single self-contained contract.
- Success means every implementation step under Steps is executed and its acceptance signal is observable in Evidence.

## Domain

- Cluster: checkout
- Issues: MY-200
- Command: ProjectOrderTotals.
- Event: OrderPlaced, OrderRefunded.
- Read model: OrderTotalsView (per-customer running totals).
- Aggregate: Order.

## Decisions

- Project from immutable events; never mutate order rows directly.

## Steps

1. Project order totals from order events
   - Issue: MY-200
   - Detail: Add OrderTotalsView projection that folds OrderPlaced (+) and OrderRefunded (-) into per-customer running totals.
   - Action type: implement

## Boundaries

- Do not bypass the aggregate by mutating order state directly.
- Do not modify issue-management state already applied above.
- Do not expand scope beyond the steps listed in this contract.

## Evidence

- test: just test
- lint: just lint
- Acceptance: Project order totals from order events
