# Handover

**Last updated:** 2026-05-21 (asia/tokyo, Phase 2b kickoff)
**Updated by:** Claude Opus 4.7 session

## Current State

`feat/jun15-mcp-pivot` long-lived branch を切り、 refs/issues/0027
(jun15 MCP pivot v4) の Phase 2b (= amadeus horizontal expansion)
を着手。 paintress Phase 1 (PR #213, squash-merged at 9b884c6, ADR
0017) + sightjack Phase 2a (PR #213 amadeus #?, squash-merged at
e28ed0f, ADR 0018) で確立した 10 commit pattern (= 9 commit + sub-D
post-merge fixup) を amadeus 用に copy する。

本 commit (= scaffold) で配置済:

- `.semgrep/jun15-no-headless-llm.yaml`: 5 rule + transitional
  exclude on `internal/session/claude_adapter.go` および
  `internal/session/doctor.go` (= 現状 `claude --print` exec を
  保持しているため、 sub-B で MCP 移行と一緒に削除予定)

## In Progress

- branch: `feat/jun15-mcp-pivot` (= long-lived feature branch、 main
  merge は Phase 2b 全完了後)
- linked issue: `refs/HTMLification/docs/issues/0027-jun15-mcp-pivot.html`
- canonical pattern: paintress ADR 0017 + sightjack ADR 0018 (= LLM
  owner inversion、 Go CLI を MCP server data plane に縮約)
- Phase 2b MVP scope (= refs 0027 §8 を amadeus 用に adapt):
  - [x] feat/jun15-mcp-pivot branch 作成 + scaffold commit (= 本 commit)
  - [ ] MCP server endpoint (= `internal/session/mcp_server.go`) skeleton + `amadeus mcp` cobra subcommand
  - [ ] amadeus.next_review / post_comment / get_pr_status 等の MCP tool **interface fixed + stub**
  - [ ] `/review-gate` slash command の skill definition (= `plugins/amadeus/skills/review-gate/SKILL.md`)
  - [ ] D-Mail envelope schema 参照 (= paintress canonical を symmetric copy)
  - [ ] **sub-A**: `internal/session/claude_adapter.go` + `internal/session/doctor.go` の `claude --print` invocation を deprecate stub に置換
  - [ ] **sub-B**: semgrep transitional exclude 削除 + skipped test 完全削除
  - [ ] **sub-C**: `docs/adr/0026-mcp-pivot.md` 起票 (= amadeus 内 ADR 連番継続) + handover finalize
  - [ ] **sub-D** (post-merge): docs/cli regen + e2e t.Skip

## Next Actions

次 commit で MCP server skeleton 着手:
1. `internal/session/mcp_server.go` を新規実装 (= sightjack ae2e313 を copy + amadeus 用 adapt)
2. `internal/cmd/mcp.go` cobra subcommand
3. root.go に `newMCPCommand()` register
4. test 配置

## Known Risks / Blockers

- amadeus は **reviewer** で auto-merge + conflict D-Mail 生成という
  cross-tool 観点で sightjack/paintress とは異なる責務を持つが、
  LLM 利用箇所 (= claude_adapter.go + doctor.go) は同一構造なので
  9-commit pattern は同じ
- paintress 既存 PR review flow との連携は維持必要 (= `amadeus run`
  が claude code session 経由になるため、 daemon orchestration は
  human-in-the-loop 必須化)

## Context the Next Actor Needs

- **canonical plan**: `refs/HTMLification/docs/issues/0027-jun15-mcp-pivot.html`
- **paintress ADR 0017**: `~/tap/paintress/docs/adr/0017-mcp-pivot.md`
- **sightjack ADR 0018**: `~/tap/sightjack/docs/adr/0018-mcp-pivot.md`
- **paintress 9 commit history**: paintress PR #213 (= 9b884c6)
- **sightjack 10 commit history**: sightjack PR #213 (= e28ed0f)
- **billing boundary 原則**: LLM 発火は常に human-initiated、 daemon は
  route まで、 consume 側は明示 slash command で trigger
- **semgrep gate**: `.semgrep/jun15-no-headless-llm.yaml` 5 rule、
  production path に `permanent` nosemgrep 例外禁止

## Relevant Files and Commands

- `.semgrep/jun15-no-headless-llm.yaml` - billing-boundary gate (5
  rule、 transitional exclude on claude_adapter.go + doctor.go)
- `internal/session/claude_adapter.go` - 現状の LLM invocation entry
  point (= sub-A で deprecate 予定)
- `internal/session/doctor.go` - health check で `claude --print` 利用
  (= sub-B で MCP ping に置換)
- `just lint-go` - golangci-lint v2
- `just semgrep` - semgrep gate (= 0 finding 維持目標)
- `go test -count=1 -short ./internal/...` - amadeus test suite
