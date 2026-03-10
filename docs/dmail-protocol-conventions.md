# D-Mail Protocol Conventions

This document defines cross-tool conventions for D-Mail filename uniqueness and archive retention.
Ratified via MY-320 / MY-321 comment threads (all 4 tools confirmed).

## Filename Uniqueness Convention (v1.1)

### 1. Filename Format

D-Mail filenames MUST follow the pattern:

```
{prefix}-{identifier}.md
```

- **prefix**: Tool-specific kind abbreviation (see Section 2)
- **identifier**: Sanitized context identifier (wave key, sequential number, etc.)
- Allowed characters: lowercase ASCII alphanumeric (`a-z`, `0-9`), hyphen (`-`), underscore (`_`)
- The complete filename MUST be unique across all d-mails (MUST)

### 1-bis. Context Field (v1, per S0031)

D-Mail envelopes support an optional `context` field for cross-tool insight propagation:

```yaml
---
dmail-schema-version: "1"
name: spec-auth-w1
kind: specification
description: Auth wave 1 specification
context:
  insights:
    - source: ".siren/insights/shibito.md"
      summary: "Token circular dependency risk in auth module"
    - source: ".siren/insights/strictness.md"
      summary: "auth cluster estimated lockdown"
---
```

- `context` is optional — omission is valid
- `context.insights` is a list of `{source, summary}` pairs
- Schema version remains "1" (additive, backward-compatible)
- Receivers MUST NOT reject unknown context fields (Postel's Law, S0019)
- phonewave relays context without interpretation

### 2. Namespace Separation (Kind Prefix)

Each tool uses a prefix derived from the d-mail kind it produces.
SKILL.md routing tables ensure most prefixes are exclusive per tool.

| Tool | Kind | Prefix | Identifier | Example |
|------|------|--------|------------|---------|
| sightjack | specification | `spec` | sanitized wave key | `spec-auth-w1.md` |
| sightjack | report | `report` | sanitized wave key | `report-auth-w1.md` |
| paintress | report | `report` | lowercase issue ID | `report-my-42.md` |
| amadeus | design-feedback | `feedback` | sequential number (zero-padded) | `feedback-001.md` |
| amadeus | implementation-feedback | `feedback` | sequential number (zero-padded) | `feedback-002.md` |
| amadeus | convergence | `conv` | sequential number (zero-padded) | `conv-001.md` |

### 2-bis. Shared Kind Prefix

When multiple tools produce the same kind (e.g. sightjack and paintress both produce `report`),
prefix alone does not guarantee cross-tool uniqueness.

In such cases, each tool MUST ensure **structural divergence in the identifier**:

- sightjack: `report-{cluster}-w{N}` — wave key derived (always contains `-w` + digit)
- paintress: `report-{team}-{issue_number}` — Linear issue ID derived

Each tool is responsible for ensuring its identifier pattern does not overlap with other tools
sharing the same prefix.

**Future safety measure (not implemented, YAGNI)**: Tool-name prefix (`sj-report-*`, `pt-report-*`)
for complete namespace isolation. Not needed as long as identifier patterns remain structurally distinct.

### 3. Within-Tool Uniqueness

Each tool is responsible for uniqueness within its own output:

- **sightjack**: Wave key (`ClusterName:ID`) is a composite key, unique per session.
- **amadeus**: Sequential numbering (`kind-NNN`). Scans archive/ for max number + 1.
- **paintress**: Issue ID is unique per Linear issue. Retry produces the same filename (dedup via archive).

### 4. Collision Semantics

Filename collision is **undefined behavior**. The protocol prohibits producing two d-mails
with identical filenames.

- Senders MUST ensure uniqueness (MUST)
- Consumer-side dedup (archive-based `os.Stat` check) is a safety net, NOT the primary
  uniqueness guarantee
- Courier (phonewave) `atomicWrite` results in last-write-wins, but this path is unreachable
  when uniqueness is guaranteed

## Archive Retention Policy

### 5. Retention Rules

- **Default retention**: Indefinite (no automatic expiration)
- **Manual pruning**: Available via each tool's CLI subcommand (e.g. `sightjack archive-prune`)
- **Retention criterion**: File modification time > N days (default: 30)
- **Compression**: Not applied (individual .md files, git-tracked for diff visibility)
- **Automatic pruning**: Not implemented (premature optimization)

### 6. Per-Tool Archive Usage

| Tool | Archive Usage | Pruning Impact |
|------|--------------|----------------|
| sightjack | Dedup (`os.Stat` existence check only) | Safe — only needed within active session |
| amadeus | Sequential numbering, display, dedup | Safe — empty archive resets to `kind-001` |
| paintress | Historical record for contextual memory | Low risk — future memory features may need longer retention |
| phonewave | Not used | N/A |

### 7. Pruning Implementation

Each tool implements `archive-prune` as a CLI subcommand.

Usage (sightjack example):

```bash
# Dry-run (default): list expired files
sightjack archive-prune

# With custom retention days
sightjack archive-prune --days 90

# Execute deletion
sightjack archive-prune --execute

# Execute with custom days
sightjack archive-prune --days 90 --execute
```

Rationale:

- Archive paths are tool-specific (`.siren/archive/`, `.expedition/archive/`, `.gate/archive/`)
- End users only have CLI binaries, not justfiles
- Default dry-run ensures safety — `--execute` flag required for deletion

Requirements:

- Default behavior: dry-run (list expired files to stdout)
- `--execute` flag for actual deletion
- `--days N` for configurable retention (default: 30)
- Target only `archive/*.md` files (never touch inbox/ or outbox/)
