# S0005. D-Mail Schema v1 Specification

**Date:** 2026-02-23
**Status:** Accepted

## Context

D-Mail is the inter-tool message format used for asynchronous communication
between endpoints in the phonewave ecosystem. The initial implementation used
top-level YAML frontmatter fields (`produces`, `consumes`) in SKILL.md files,
which conflicted with the Claude Code skills-ref specification where these
fields have different semantics.

MY-352 (D-Mail decoupling) and MY-353 (JSON Schema v1) established the need
for a versioned schema that nests D-Mail capabilities under `metadata` to avoid
conflicts with the host skill format.

## Decision

Adopt D-Mail Schema v1 with the following specification:

1. **Schema version**: `dmail-schema-version: "1"` is required under `metadata`
   in SKILL.md frontmatter.
2. **Capability nesting**: `produces` and `consumes` arrays are placed under
   `metadata`, not at the top level.
3. **Top-level rejection**: Top-level `produces`/`consumes` without
   `dmail-schema-version` is rejected with an error.
4. **Mixed format rejection**: Top-level capabilities coexisting with `metadata`
   containing `dmail-schema-version` is rejected.
5. **Kind validation**: Each capability's `kind` field is validated against the
   set of known kinds (specification, report, feedback, convergence).
6. **D-Mail envelope**: Message files in outbox/inbox use the same
   `dmail-schema-version: "1"` header with `name`, `kind`, and `description`.

### SKILL.md Example

```yaml
---
name: "dmail-sendable"
description: "Produces D-Mail messages"
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  produces:
    - kind: specification
      description: "Issue specification ready for implementation"
---
```

### D-Mail Envelope Example

```yaml
---
dmail-schema-version: "1"
name: spec-001
kind: specification
description: Implementation specification
---

# Content here
```

## Consequences

### Positive

- No conflict with Claude Code skills-ref specification
- Explicit versioning enables future schema evolution
- Strict validation catches misconfiguration early (fail-fast)

### Negative

- Existing SKILL.md files required migration from top-level to metadata format
- Additional nesting adds verbosity to the YAML structure

### Neutral

- Only version "1" is currently supported; future versions will require a new ADR
- Some tools generate internal D-Mails (e.g., amadeus `convergence` kind) that
  are written directly to archive/ without passing through the phonewave routing
  pipeline. These still use the v1 envelope format for consistency
