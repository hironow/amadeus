# S0021. D-Mail Receive-Side Validation (Postel's Law)

**Date:** 2026-03-02
**Status:** Accepted

## Context

Cross-tool gap inventory (2026-03-02) identified that receive-side D-Mail
validation differs from send-side validation across all four tools. On the
send side, each tool validates D-Mails strictly via `ValidateDMail` (checking
name, kind, description, schema version). On the receive side, all tools use
only `ParseDMail`, which checks frontmatter structure but does not validate
schema version or kind values.

This asymmetry was initially flagged as a potential gap (GAP-01-02). After
review, it was determined to be an intentional design choice following
Postel's Law (RFC 793): "Be conservative in what you send, be liberal in
what you accept."

## Decision

Adopt Postel's Law as the explicit D-Mail validation strategy across all
four tools:

- **Send side (conservative):** All outbound D-Mails must pass `ValidateDMail`
  before emission. This ensures every D-Mail leaving a tool conforms to schema
  v1 (valid name, kind, description, schema version).

- **Receive side (liberal):** Inbound D-Mails are parsed via `ParseDMail` only.
  No schema version or kind validation is performed on received D-Mails. This
  allows tools to accept D-Mails from future schema versions or with unknown
  kinds without breaking.

### Per-tool validation paths

| Tool | Send validation | Receive parsing | Location |
|------|----------------|-----------------|----------|
| phonewave | `ExtractDMailKind` (validates schema version + kind) | `ParseDMail` | `delivery.go` |
| sightjack | `ValidateDMail` (before `ComposeDMail`) | `ParseDMail` | `internal/session/dmail.go` |
| paintress | `ValidateDMail` (in `SendDMail`, after schema stamp) | `ParseDMail` | `dmail.go`, `internal/session/dmail.go` |
| amadeus | `ValidateDMail` (before event emission) | `ParseDMail` | `dmail.go`, `internal/session/dmail_io.go` |

## Consequences

### Positive

- Forward-compatible: tools can receive D-Mails from newer schema versions
  without crashing
- Resilient: a single tool upgrading its schema does not break the entire
  cross-tool pipeline
- Consistent: all four tools follow the same send-strict / receive-liberal
  pattern

### Negative

- Invalid D-Mails from external sources may silently enter inbox without
  validation errors
- Schema violations in received D-Mails are only caught when a tool attempts
  to use the specific fields (lazy validation)
