# amadeus claude code plugin (jun15 MCP pivot)

**Status:** Phase 2b in progress (= MCP server stub + skill skeleton).
Production target for the post-2026-06-15 architecture where claude
code interactive sessions own LLM inference and the amadeus Go CLI
exposes its review queue + GitHub orchestration as an MCP server.
Pattern referenced from paintress Phase 1 (ADR 0017) + sightjack
Phase 2a (ADR 0018).

## Layout

```
plugins/amadeus/
├── README.md                       # this file
└── skills/
    └── review-gate/SKILL.md        # /review-gate slash command
```

Subsequent commits on `feat/jun15-mcp-pivot` add:

- `agents/reviewer.md` — long-running PR review agent (post-stub)
- `skills/check-inbox/SKILL.md` — explicit D-Mail consume entry point
- `hooks/` — non-LLM hooks only (e.g. stderr-only review-queue depth notice)

## Loading the plugin

```bash
claude \
  --plugin-dir ./plugins/amadeus \
  --mcp-config '{"amadeus":{"command":"amadeus","args":["mcp"]}}'
```

The `--plugin-dir` flag registers the skills; the `--mcp-config` flag
attaches the amadeus MCP server (`amadeus mcp` subcommand) so the
skill's `mcp__amadeus__*` tools resolve.

## Phase 2b MVP scope

Only `/review-gate` is wired. The slash command calls the amadeus
MCP server's stub tools (amadeus.ping, amadeus.next_review,
amadeus.post_comment, amadeus.get_pr_status) and surfaces the stub
contract to the human. Real domain wiring lands in subsequent
commits on `feat/jun15-mcp-pivot`.

## Distinct from `amadeus mcp-config`

`amadeus mcp-config` (legacy) manages the `.mcp.json` config consumed
by the embedded claude_adapter. `amadeus mcp` (this plugin) is the
**server** consumed by claude code itself. The two have different roles
and coexist during the pivot transition.
