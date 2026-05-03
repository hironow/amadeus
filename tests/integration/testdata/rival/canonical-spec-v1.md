---
name: sj-spec-auth-canonical-spec-v1_deadbeef
kind: specification
description: Add session expiry enforcement
dmail-schema-version: "1"
issues:
  - MY-1
wave:
  id: auth:canonical-spec-v1
  steps:
    - id: MY-1
      title: Enforce session expiry in middleware
      description: Update the auth middleware to read the expiry claim and reject expired tokens with a 401.
metadata:
  contract_id: canonical-spec-v1
  contract_revision: "1"
  contract_schema: rival-contract-v1
  idempotency_key: 586ce9526c8d87eb452c06fb6ed6ae0da16db3efa5a5c20caa482d9323412b9c
  supersedes: ""
---

# Contract: Add session expiry enforcement

## Intent

Reject expired sessions at the auth middleware before the request reaches business logic.

- Deliver wave "Add session expiry enforcement" so the implementing tool can act on a single self-contained contract.
- Success means every implementation step under Steps is executed and its acceptance signal is observable in Evidence.

## Domain

- Cluster: auth
- Issues: MY-1
- Command: implementation tool will execute the steps listed below.
- Event: wave was specified by sightjack and ready for implementation.

## Decisions

- No explicit design decision recorded.

## Steps

1. Enforce session expiry in middleware
   - Issue: MY-1
   - Detail: Update the auth middleware to read the expiry claim and reject expired tokens with a 401.
   - Action type: implement

## Boundaries

- Already applied by sightjack [add_dod]: Document expiry enforcement in DoD
- Do not modify issue-management state already applied above.
- Do not expand scope beyond the steps listed in this contract.

## Evidence

- test: just test
- lint: just lint
- Acceptance: Enforce session expiry in middleware

