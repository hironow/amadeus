# Handover

**Last updated:** 2026-05-21 (asia/tokyo, Phase 2b finalize)
**Updated by:** Claude Opus 4.7 session

## Current State

`feat/jun15-mcp-pivot` long-lived branch 上で refs/issues/0027
(jun15 MCP pivot v4) の Phase 2b (= amadeus horizontal expansion)
を 8 commit (scaffold + sub-A + sub-B + sub-C) 完了。 paintress
Phase 1 (PR #213, squash-merged at 9b884c6, ADR 0017) + sightjack
Phase 2a (PR #213, squash-merged at e28ed0f, ADR 0018) で確立した
10-commit pattern (= 9 commit + sub-D post-merge fixup) を amadeus
用に adapt し、 `claude --print` exec を全 production path から
削除、 `amadeus mcp` stdio MCP server + `/review-gate` skill + 9-field
D-Mail envelope schema を追加、 amadeus ADR 0026 を発行。

Phase 2b 完了内容:

1. **`.semgrep/jun15-no-headless-llm.yaml`** (= 5 rule, scaffold で
   transitional exclude を設定し sub-B で削除済。 残る exclude は
   `tests/**` のみ = fake-claude binary を呼ぶ test fixture 用)。
2. **`amadeus mcp` MCP server** (`internal/session/mcp_server.go`)
   = JSON-RPC 2.0 stdio、 4 MiB scanner buffer、 Phase 2b MVP として
   `amadeus.ping` / `amadeus.next_review` /
   `amadeus.post_comment` / `amadeus.get_pr_status` を
   advertise + dispatch。 後 3 つは contract 固定 + stub。 7 test
   pass (= ListsAllPhase2bTools / CallsPingTool / RejectsUnknownTool
   / NextReviewStub / PostCommentStub_EchoesPRNumberAndBodyLength /
   GetPRStatusStub_EchoesPRNumber / RejectsUnknownMethod)。
3. **`/review-gate` skill** (= `plugins/amadeus/skills/review-gate/SKILL.md`)
   + plugin README。 `--plugin-dir ./plugins/amadeus` で claude code
   session に load、 `mcp__amadeus__*` tools を allowed-tools に
   宣言。
4. **D-Mail 9-field envelope** (`internal/domain/dmail_envelope.go`)
   = paintress canonical の symmetric copy (= paintress → amadeus
   方向 conflict_notification 系を含む全方向の parse / validate /
   ack を支える)。
   `tests/fixtures/dmail/dmail-2026-06-01T12-00-00Z-ghi789.{yaml,body.md}`
   で fixture pair を配置 + 5 test pass。
5. **sub-A** (= claude_adapter.go + doctor.go の `claude --print`
   invocation を `ErrMCPPivotDeprecated` stub に置換)。
   `ClaudeAdapter.RunDetailed` body 290 行削除、 struct field は
   composition root 互換のため保持。 doctor の inference /
   context-budget check は `Skip` 結果に置換。 doctor_claude.go の
   `extractStreamResult` 関数 + `encoding/json` import も削除。
   既存 2 test (TestRunDoctor_MCPListFails / AllPassWithFakeClaude)
   は expectation を OK → Skip に更新、 streambus_wiring_test の
   2 件に t.Skip 付与。
6. **sub-B** (= semgrep transitional excludes 削除 + deprecated test
   2 件物理削除 + canonical assertion 1 件追加)。 file-level 削除:
   `streambus_wiring_test.go`。 canonical assertion: claude_adapter_test.go の
   `TestClaudeAdapter_RunDetailedReturnsErrMCPPivotDeprecated`
   (`errors.Is` で `session.ErrMCPPivotDeprecated` を検証)。
7. **sub-C** (= 本 commit、 amadeus ADR 0026 起票 + handover finalize)。

## In Progress

- branch: `feat/jun15-mcp-pivot` (= scaffold + 7 commit、 8 commit 目
  が本 sub-C、 sub-D は必要時 post-merge fixup)
- main merge は Phase 2b 完了後の PR 作成 + CI green + squash-merge
  待ち (= paintress / sightjack PR pattern)
- 次 phase: 2c dominator (= phonewave は LLM 非使用のため対象外)

## Next Actions

1. `feat/jun15-mcp-pivot` に PR 作成 (= title: `feat(session): Phase
   2b amadeus jun15 MCP pivot (refs/issues/0027)`)
2. CI を green まで監視 (= sightjack PR #213 と同様、 docs-check と
   test-fail / e2e-fail が発生する可能性あり、 必要なら sub-D fixup)
3. squash-merge 完了後、 Phase 2c dominator 着手
4. cost monitoring: OTel MCP invocation count を amadeus でも計測、
   Anthropic dashboard で credit 0 維持を手動検証

## Known Risks / Blockers

- paintress / sightjack で post-merge sub-D が必要だった patterns:
  docs-check (CLI doc 未生成) + e2e tests (deprecated CLI 経由) →
  amadeus でも同様の fixup が CI で必要になる可能性。
- `amadeus run` / `sync` / `mark-commented` 全 LLM-using subcommand
  が `ErrMCPPivotDeprecated` 返却に倒れたため、 paintress PR
  convergence + auto-merge workflow が claude code session 経由に
  なる (= human-in-the-loop 必須化)。
- `amadeus mcp-config` (legacy `.mcp.json` 管理) と `amadeus mcp`
  (= MCP server) は名称が紛らわしい。 plugin README + skill SKILL.md
  で role 違いを明示済。

## Context the Next Actor Needs

- **canonical plan**: `refs/HTMLification/docs/issues/0027-jun15-mcp-pivot.html`
- **paintress ADR 0017**: `~/tap/paintress/docs/adr/0017-mcp-pivot.md`
- **sightjack ADR 0018**: `~/tap/sightjack/docs/adr/0018-mcp-pivot.md`
- **amadeus ADR 0026**: `docs/adr/0026-mcp-pivot.md` (= 本 phase の
  architectural pin、 paintress/sightjack の symmetric counterpart)
- **paintress 9 commit history**: paintress PR #213 (= 9b884c6)
- **sightjack 10 commit history**: sightjack PR #213 (= e28ed0f)
- **billing boundary 原則**: LLM 発火は常に human-initiated、 daemon
  は route まで、 consume 側は明示 slash command で trigger
- **semgrep gate**: `.semgrep/jun15-no-headless-llm.yaml` 5 rule、
  production path に `permanent` nosemgrep 例外禁止、 残る exclude は
  `tests/**` (fake-claude binary) のみ
- **MCP server tool 命名規約**: `<tool_name>.<verb>` (= dot 区切り、
  paintress の `paintress.ping` / `paintress.next_issue` / sightjack
  の `sightjack.ping` / `sightjack.next_wave` と対称)。 claude code
  側の `mcp__<server>__<tool>` 自動 mapping に対応。

## Relevant Files and Commands

- `docs/adr/0026-mcp-pivot.md` - 本 phase の architectural pin
- `.semgrep/jun15-no-headless-llm.yaml` - billing-boundary gate (5
  rule、 production scope 完全 enforced、 `tests/**` のみ exclude)
- `internal/session/mcp_server.go` - amadeus MCP server (= Phase 2b
  MVP scope、 4 tool stub)
- `internal/session/claude_adapter.go` - `ClaudeAdapter.Run` /
  `RunDetailed` = `ErrMCPPivotDeprecated` stub
- `internal/session/doctor.go` - `claude-inference` /
  `context-budget` = Skip (post jun15 MCP pivot)
- `internal/domain/dmail_envelope.go` - 9-field envelope symmetric
  copy
- `internal/cmd/mcp.go` - `amadeus mcp` cobra subcommand
- `plugins/amadeus/skills/review-gate/SKILL.md` - human-driven
  entry point
- `tests/fixtures/dmail/dmail-2026-06-01T12-00-00Z-ghi789.{yaml,body.md}`
  - synthetic D-Mail contract fixture (paintress → amadeus 方向、
  conflict_notification kind)
- `just lint-go` - golangci-lint v2 (= 0 issues 維持)
- `just semgrep` - semgrep gate (= 0 findings 維持、 75 rules)
- `go test -count=1 -short ./...` - amadeus test suite (= 全 pkg ok)
