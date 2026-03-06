# 0018. Parse-Don't-Validate Commands

**Date:** 2026-03-06
**Status:** Accepted

## Context

Domain command types (InitCommand, RunCommand) used a `Validate() []error` method called in the usecase layer. This pattern had several issues:

- Validation was optional and could be forgotten
- Invalid command objects could exist in memory
- Error aggregation boilerplate duplicated across usecase functions
- Domain types did not enforce their own invariants

## Decision

Adopt the Parse-Don't-Validate pattern for all domain command types:

1. Domain primitives (RepoPath, Days) validate in their `New*()` constructors
2. Command types use unexported fields with `New*Command()` constructors
3. Constructors accept only pre-validated domain primitives
4. `Validate() []error` methods are removed entirely
5. Usecase layer receives always-valid commands — no validation needed

A semgrep rule `domain-no-validate-method` prevents reintroduction of `Validate() []error` in the domain layer.

## Consequences

### Positive
- Invalid commands cannot exist — correctness by construction
- Usecase functions are simpler (no validation boilerplate)
- Domain primitives are reusable across command types

### Negative
- Parsing errors surface at cmd layer (further from domain logic)
- More types to maintain (one per validated concept)

### Neutral
- Error messages move from domain validation to primitive constructors
