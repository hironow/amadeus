# S0045. Omni-session topology and project wiring

**Date:** 2026-06-10
**Status:** Accepted

## Context

After the jun15 MCP pivot (S0044 supersession wave), each tool is an
MCP data plane driven by a human-initiated Claude Code session — but
the pivot never defined how the human runs the assembly line, and the
entry skills had no distribution path (measured zero invocations).
A conformance audit against the June-2026 Claude Code feature set
(refs issue 0032 §5, constraints C1-C6) showed: plugin-manifest
machinery would namespace the skills; project `.claude/skills/` is
auto-discovered; a project-root `.mcp.json` is auto-attached with a
pending-approval flow; `disable-model-invocation: true` mechanically
enforces human-only invocation; tool names should be dot-free because
dot normalization is undocumented.

## Decision

1. **Session topology (decision D3)**: for a single project the
   standard form is one **omni-session** — all relevant tap tools
   attached via one project-root `.mcp.json`, the human invoking the
   role skills (`/sightjack-scan`, `/expedition-next`, `/review-gate`,
   `/nfr-judge`) in sequence. Role-per-session splits are reserved for
   multiplex (1 VM = N projects) operation. Durable handoffs between
   roles stay on the D-Mail protocol regardless of topology.
2. **Project wiring (decision D5(a))**: each tool's `init`
   materializes its entry skill into the target project's
   `.claude/skills/<name>/SKILL.md` from an embedded template, and the
   tool's config generator upserts the project-root `.mcp.json`
   **merge-aware** (sibling entries and foreign keys preserved). No
   plugin manifests; no launch flags required.
3. **Skill frontmatter invariant**: entry skills carry
   `disable-model-invocation: true` (human-invocable only — the
   mechanical jun15 invariant) and reference MCP tools by their
   dot-free canonical names (`mcp__<tool>__<name>`).
4. **MCP server contract**: tool names are dot-free; the initialize
   handshake advertises `instructions` (Tool Search deferred loading
   reads only names + instructions at startup).

## Enforcement inventory

### Entry points

- `{tool} init` (skill materialization into `.claude/skills/`)
- `{tool} mcp-config generate` (or `init` where no mcp-config command
  exists) — project-root `.mcp.json` merge-aware upsert
- Entry skill frontmatter (`disable-model-invocation`, allowed-tools)
- MCP server `initialize` / `tools/list` (dot-free names,
  instructions)

### Persistent / carried data needed at each enforcement point

- Embedded skill template (single source of truth per tool)
- Project-root `.mcp.json` `mcpServers` map (shared across tools —
  merge, never overwrite)
- State-dir `.mcp.json` (isolated `sessions enter` wiring, unchanged)

### Bypass candidates ("where can this go wrong?")

- Hand-editing the root `.mcp.json` (tolerated: upsert preserves
  foreign entries; invalid JSON fails loudly instead of clobbering)
- Hand-copying stale SKILL.md into `.claude/skills/` (next `init`
  overwrites only when the template differs — re-running `init` is
  the repair)
- Launching via `--plugin-dir` with ad-hoc plugin manifests (out of
  contract; skills would namespace differently — not supported)
- A tool registering dotted tool names again (caught by each repo's
  MCP contract tests asserting the dot-free list)

### Tests proving coverage (one per enforcement point)

- Per tool: skill-materialization test (file lands under
  `.claude/skills/`, idempotent re-run)
- Per tool: root `.mcp.json` upsert test (creates when absent;
  preserves sibling entries + foreign keys; idempotent)
- Per tool: `tools/list` contract test pinning dot-free names;
  initialize handshake test pinning `instructions`
- Cross-tool: phonewave scenario L1.5 MCP closed-loop test (scripted
  stdio client through the full designer → courier → implementer →
  reviewer chain)

## Consequences

### Positive

- Zero-flag startup: `init` + `mcp-config generate` make a bare
  `claude` session fully wired — removes the friction that produced
  the zero-invocation baseline.
- One root config = omni-session by default; the topology question is
  pinned instead of implicit.
- Human-only invocation is enforced by the harness, not by prose.

### Negative

- Tools write into the project's `.claude/` and root `.mcp.json` —
  more surface owned by `init` (mitigated by merge-aware writes and
  idempotency).
- Two `.mcp.json` locations (root + state-dir) must be kept
  conceptually distinct (root = discovery, state-dir = isolation).

### Neutral

- Multiplex (Phase β) keeps the option of role-per-session without
  changing the data planes.
