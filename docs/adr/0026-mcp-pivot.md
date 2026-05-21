# 0026. MCP pivot: claude code session owns LLM, amadeus Go CLI is MCP server data plane

**Date:** 2026-05-21
**Status:** Accepted

## Context

Starting 2026-06-15, Anthropic Claude Code subscription plans (Pro,
Max 5x, Max 20x) bill `claude -p` and Agent SDK usage against a
separate monthly Agent SDK credit pool ($20 / $100 / $200) that is
disjoint from the interactive usage quota. The previous amadeus
Go CLI architecture invoked `claude --print` as an `exec.Command`
subprocess on every review / sync / convergence / auto-merge step
via `internal/session/claude_adapter.go`, plus a separate
`claude --print --max-turns 1 "1+1="` health probe in
`internal/session/doctor.go`. Every production run after 2026-06-15
would draw on credit pool capacity that is not sized for autonomous
PR-review loops.

We surveyed every technically plausible way to keep the existing
control flow running off the interactive quota:

- PTY automation (`creack/pty`, `expect`), tmux `send-keys` +
  `capture-pane`, Remote Control protocol use, and TTY spoofing via
  `script(1)` all violate the Anthropic Acceptable Use Policy clause
  on bypassing product-imposed restrictions, regardless of intent.
- `--output-format`, `--input-format`, `--fallback-model`,
  `--max-budget-usd`, and the rest of the structured-output flag set
  are documented as `--print`-only, so even a successful TTY-spoof
  would still degrade to TUI scraping for any automation-grade
  output.

We also considered keeping the existing Go CLI architecture and
swapping the auth path to a direct Anthropic API key or a third-party
provider (Bedrock / Vertex / Foundry). This works technically but
abandons subscription billing entirely and shifts amadeus to a
per-token cost model with no upper bound, which is the opposite of
the design goal that motivated subscription onboarding.

The refs/issues/0027 plan synthesised these constraints into a single
direction: invert the LLM ownership. paintress Phase 1 (ADR 0017)
established the canonical 9-commit pattern; sightjack Phase 2a (ADR
0018) confirmed the cross-repo symmetry; amadeus Phase 2b applies
the same pattern adapted to the review / sync / convergence /
auto-merge pipeline.

## Decision

amadeus relinquishes the LLM owner role. From this commit forward,
the architecture is:

1. **Human-initiated claude code interactive session is the LLM
   owner.** All inference happens inside the session's subscription
   quota. No production code path may invoke `claude --print`, import
   the Anthropic Agent SDK, read `ANTHROPIC_API_KEY`, or otherwise
   call the API outside the active session.
2. **amadeus Go CLI exposes an MCP server (`amadeus mcp`).** The
   server speaks JSON-RPC 2.0 over stdio and registers tools that
   wrap the existing data plane (event sourcing, review queue,
   convergence projection, D-Mail outbox/inbox, OTel instrumentation).
   The session loads the server via `--mcp-config`. This is distinct
   from the existing `amadeus mcp-config` subcommand, which manages
   the legacy `.mcp.json` consumed by the now-deprecated
   `claude_adapter`.
3. **A claude code skill drives the workflow.** The `/review-gate`
   slash command (under `plugins/amadeus/skills/review-gate/SKILL.md`)
   is the only sanctioned entry point. Hooks may emit human-readable
   notices on stderr but must not auto-trigger LLM calls and must not
   surface inbox payloads on stdout (which the official hooks docs
   feed into the session's context).
4. **D-Mail cross-tool messaging follows the paintress canonical
   schema.** `internal/domain/dmail_envelope.go` is a symmetric
   re-implementation of paintress's canonical 9-field envelope so
   amadeus can decode inbox messages (e.g., conflict notifications
   from paintress, review-state queries from sightjack) without
   depending on paintress's or sightjack's import graph. The legacy
   D-Mail format used by current convergence / auto-merge flows
   coexists during the pivot transition; a future ADR (post-MCP-pivot)
   reconciles them.
5. **A semgrep gate enforces the boundary.** The rule set in
   `.semgrep/jun15-no-headless-llm.yaml` blocks every executable
   path that could re-introduce `claude --print`, the Agent SDK,
   `ANTHROPIC_API_KEY`, or shell-wrapped variants thereof. `permanent`
   nosemgrep exemptions on this rule are not allowed in production
   paths; the only legitimate exclusion is `tests/**` for the
   fake-claude binary used in test fixtures.

The Go CLI keeps its event sourcing, transactional outbox, OTel
spans, review queue projection, convergence/auto-merge state, and
domain language. What it loses is the right to spawn a subprocess
that calls the model.

## Enforcement inventory

### Entry points

- `cmd/amadeus` Cobra subcommands that previously drove inference:
  `run`, `sync`, `mark-commented`, plus the orchestration commands
  that wrap them.
- `internal/session/claude_adapter.ClaudeAdapter.Run` /
  `RunDetailed` — the unified provider runner that prior to this ADR
  built a prompt, exec'd `claude --print`, parsed stream-json from
  stdout, wired the OTel span emitter and the StreamBus subscriber,
  and persisted raw events to `.run/claude-logs/`.
- `internal/session/doctor.go` — the `claude --print --max-turns 1
  "1+1="` health probe plus the `context-budget` check that read its
  stream output.
- Any future code that wants to call the model from outside an
  interactive session (Agent SDK, GitHub Actions, third-party SDK
  apps).

### Persistent data carried into the new path

- `internal/domain/dmail_envelope.go` (9-field `DMailEnvelope`) pins
  the cross-tool message schema (`message_id`, `source_tool`,
  `target_tool`, `kind`, `body_path`, `created_at`, `seen_at`,
  `ack_at`, `idempotency_key`). The on-disk layout is
  `inbox/<message_id>.yaml` + `inbox/<message_id>.body.md`.
- `amadeus.next_review` / `amadeus.post_comment` /
  `amadeus.get_pr_status` MCP tools will expose the existing review
  queue projection, GitHub Comments API client, and convergence +
  auto-merge state as MCP resources. Phase 2b ships these as stubs
  that surface the contract descriptor; real wiring lands in a
  follow-up commit.
- OTel span attributes (`gen_ai.*`, `messaging.*`) continue to flow
  through the MCP server, so the trace topology that previously
  spanned `claude --print` invocations now spans MCP tool calls.

### Bypass candidates

- Direct `exec.Command("claude", "--print", ...)` from Go code —
  blocked by `jun15-no-claude-print-exec-go`.
- Shell wrappers (`bash -lc "claude --print ..."`, `sh -c`, `just`
  recipes, `scripts/*.sh`) — blocked by
  `jun15-no-claude-print-shell-wrapper` and the literal-scan rule.
- Anthropic SDK imports in Go (`github.com/anthropics/anthropic-sdk-go`)
  — blocked by `jun15-no-anthropic-sdk-import-go`.
- `ANTHROPIC_API_KEY` env reads — blocked by
  `jun15-no-anthropic-api-key-read`.
- `SessionStart` / `PreToolUse` hooks that stream inbox content on
  stdout — blocked by the documentation convention (`stderr only`)
  and the `type: prompt` hook prohibition.
- Future `--bare`-mode invocations of `claude` from outside the
  session — covered by the shell-wrapper rule.
- Direct `gh api` / `curl` calls to the GitHub Comments API from
  skills — discouraged by the skill SKILL.md `must NOT do` section
  in favour of the canonical `amadeus.post_comment` MCP tool, which
  preserves the audit trail (`messaging.*` OTel attrs).

### Tests proving coverage

- `internal/session/mcp_server_test.go` — seven tests prove the
  `amadeus mcp` stdio server advertises all four Phase 2b tools
  (`amadeus.ping` + `next_review` + `post_comment` +
  `get_pr_status`), dispatches each correctly, and returns the
  JSON-RPC `-32601` error for unknown tools / methods.
- `internal/session/claude_adapter_test.go::TestClaudeAdapter_RunDetailedReturnsErrMCPPivotDeprecated`
  — one canonical test proves `ClaudeAdapter.Run` / `RunDetailed`
  return `session.ErrMCPPivotDeprecated` via `errors.Is`. The
  existing `TestClaudeAdapter_ImplementsProviderRunner` is kept
  because it asserts the port interface contract.
- `internal/domain/dmail_envelope_test.go` — five tests cover the
  YAML schema (paintress -> amadeus direction), required-field
  validation, idempotency-key dedup, ack semantics, and the
  `inbox/<id>.yaml` + `body.md` file pair.
- `internal/session/doctor_checks_test.go::TestRunDoctor_MCPListFails_InferenceStillRuns`
  and `TestRunDoctor_AllPassWithFakeClaude` — updated to expect
  `Skip` with a `(post jun15 MCP pivot, refs/issues/0027)` reason
  on the `claude-inference` check.
- `just semgrep` — 75 rules, 0 findings, including the five
  `jun15-no-headless-llm` gate rules with no production-path
  exclusions (only `tests/**` remains for the fake-claude binary).

## Consequences

### Positive

- Subscription billing keeps paying for all amadeus LLM use after
  2026-06-15. Credit pool consumption from amadeus is zero by
  construction.
- The Acceptable Use Policy boundary is honoured: every model call
  is human-initiated inside an interactive session.
- The Go CLI's domain plane (event sourcing, SQLite outbox, OTel,
  review queue, convergence + auto-merge state, D-Mail semantics)
  survives intact and is now exposed via a stable MCP contract that
  other tools can adopt.
- The semgrep gate makes the boundary mechanical; future contributors
  cannot silently re-introduce headless LLM calls.
- The 9-field `DMailEnvelope` adopted from paintress unifies the
  cross-tool message format across paintress / sightjack / amadeus
  and any future consumer.

### Negative

- `amadeus run`, `amadeus sync`, and any subcommand that depends on
  `ClaudeAdapter.Run` now return `ErrMCPPivotDeprecated`. Operators
  must launch a claude code session and invoke `/review-gate`
  manually. Schedulers and CI jobs that wrapped any of these
  subcommands no longer work without that human-in-the-loop step.
- Multi-tool parallel orchestration loses the easy concurrency story
  that came with independent Go processes. A single interactive
  session is the natural unit of work.
- 2 test functions that exercised the legacy `RunDetailed` stream
  wiring (`TestStreamBusWiring_AdapterEmitsExactlyOneSessionEnd` /
  `TestStreamBusWiring_SessionEndInheritsParentTrace`) were retired
  in sub-B. Phase 2 follow-up commits must reintroduce equivalent
  coverage against the MCP server contract.
- The `claude-inference` and `context-budget` doctor checks no longer
  prove headless inference works. A follow-up commit will rewire
  them to probe the MCP server's `amadeus.ping`.

### Neutral

- `ClaudeAdapter` struct fields (`ClaudeCmd`, `Model`, `TimeoutSec`,
  `Logger`, `ToolName`, `StreamBus`, `NewCmd`, `CancelFunc`) are
  retained even though `Run` / `RunDetailed` no longer read them,
  because the composition root (`cmd` package) still constructs the
  adapter and the Phase 2 MCP server tools are expected to reuse
  parts of the wiring (StreamBus, OTel attrs).
- The existing `amadeus mcp-config` subcommand is kept; it manages
  the legacy `.mcp.json` and may be retired once the MCP server path
  is fully wired in a follow-up phase.
- The `tests/fixtures/dmail/dmail-2026-06-01T12-00-00Z-ghi789.{yaml,body.md}`
  fixture pair carries the paintress → amadeus direction of the
  symmetric 9-field envelope; paintress's `sightjack → paintress`
  and sightjack's `paintress → sightjack` fixtures cover the other
  directions of the same contract.

## References

- refs/issues/0027 — canonical plan including all four codex review
  rounds, the billing boundary table, the mechanical gate, the MVP
  scope reduction, the hook context-injection warning, and the
  D-Mail schema fixation.
- paintress ADR 0017 — canonical 9-commit pattern that originated
  the LLM owner inversion.
- sightjack ADR 0018 — Phase 2a confirmation of the cross-repo
  symmetry; this ADR applies the same pattern to amadeus.
- Local ADRs 0010 (load-config-to-cmd), 0011 (root-package-layer-separation),
  0012 (usecase-adapter-dependency-inversion), 0013 (parse-don't-validate),
  0014 (feedback-kind-split), 0015 (pr-convergence-conductor-role),
  0016 (fsnotify-inbox-watcher), 0018 (claude-cli-stream-json-requires-verbose),
  0024 (pr-diff-review-role), 0025 (auto-merge-capability) — the
  architectural layers this ADR preserves.
- <https://code.claude.com/docs/en/headless> — 2026-06-15 credit
  pool change announcement and `--bare` mode documentation.
- <https://support.claude.com/en/articles/15036540> — per-plan
  credit allocation table.
