# S0013. COMMAND Naming Convention

**Date:** 2026-02-28
**Status:** Accepted

## Context

The 4-tool ecosystem uses Event Storming terminology (EVENT, COMMAND, POLICY,
READ MODEL, AGGREGATE, EXTERNAL SYSTEM) as documented in the root CLAUDE.md.

EVENTs already follow a consistent past-tense naming convention across all tools
(e.g., `ScanCompleted`, `DeliveryFailed`, `ExpeditionStarted`, `CheckDriftDetected`).
However, COMMAND naming has no formal convention. As typed COMMAND objects are
introduced (replacing implicit cobra handler invocations), a consistent naming
rule is needed.

## Decision

- **EVENT** names use past tense (did / -ed): describes what happened.
  Examples: `ScanCompleted`, `DeliveryFailed`, `CheckDriftDetected`

- **COMMAND** names use imperative present tense (will / -する): describes intent.
  Examples: `StartScan`, `DeliverDMail`, `RunCheck`, `StartExpedition`

- COMMAND type names follow the pattern `{Verb}{Noun}` (e.g., `StartScan`, not `ScanStart`).

- EVENT type names follow the pattern `{Noun}{PastVerb}` (e.g., `ScanCompleted`, not `CompletedScan`).

## Consequences

### Positive

- Clear distinction between intent (COMMAND) and fact (EVENT) at the type level
- Consistent naming across all 4 tools
- Reads naturally in POLICY definitions: "WHEN ScanCompleted THEN DeliverDMail"

### Negative

- Existing EventType constants already use `{Noun}{PastVerb}`; new COMMAND constants
  must use `{Verb}{Noun}` which is a different word order

### Neutral

- This convention aligns with standard Event Storming practice
