# Handover

**Last updated:** 2026-05-25 (asia/tokyo, r4 phase1 headless-pipeline excision)
**Updated by:** Claude Opus 4.7 session

## Current State

jun15 MCP pivot (refs/issues/0027) **全 phase 完了 + archive 入り**、
**かつ 0028 cross-tool semgrep gate 強化も完了**。
Phase 2b で確立した 9-commit pattern を起点に、 Phase 3 real impl と
Phase 4 follow-up #3 で amadeus の MCP server-first architecture が
main merged。

amadeus 固有の jun15 landmark:

- ADR 0026 (= `docs/adr/0026-mcp-pivot.md`) で architectural pin 固定
- 4 MCP tool 全 real impl (= ping / next_review / post_comment /
  get_pr_status)
- Phase 4 #3 (PR #217 `21fdd9d`): `amadeus.post_comment` を preview-only
  → 実 GitHub Comments API post に昇格。 `port.CommentPoster` narrow
  port 新設、 `GhPRWriter.PostComment` を `gh pr comment <num> --body
  <body>` で実装、 cmd composition root が `GhPRWriter(cwd)` を
  `WithCommentPoster` 経由で session に注入。 persistence=
  'github-comments-api' / error → reason surface / nil → preview-only
- 0028 (PR #219 `48d3a9b`): cross-tool symmetric regression prevention。
  新 semgrep rule `jun15-no-print-flag-literal-go` を 5 ツール全てに
  追加 (= dynamic args spread を catch する regex rule)。 amadeus の
  ClaudeAdapter は Phase 2b で既に stub 済なので **production code 変更
  なし、 future regression 防止のみ**
- `.semgrep/jun15-no-headless-llm.yaml` **6 rule** (= base 5 + 0028 で
  `jun15-no-print-flag-literal-go` 追加) で headless LLM 経路 + dynamic
  args spread を permanent block
- **r4 phase1 (2026-05-25)**: 旧 headless pipeline を完全 excise。
  `run` / `install-hook` / `uninstall-hook` command 削除、 `RunCheck` /
  `Run` daemon / divergence scoring / D-Mail generation / insights /
  stall handler / review-gate / PR pre-merge pipeline (= auto-merge) を
  source ごと削除。 `claude_adapter.go` (旧 stub) も削除。 amadeus =
  pure MCP data plane + sessions + data-plane commands。
  `internal/session/retry_runner.go` は phase-1 survivor として温存
  (= locked `provider_telemetry.go` の参照保持)。
- `/review-gate` skill が claude code session 経由の唯一の review-driving 経路
- LLM 発火は human-initiated 維持: post_comment は MCP tool call 時のみ
  `gh pr comment` を実行 (= adapter wired でも自動 post なし)

## In Progress

なし。 jun15 MCP pivot に関する作業は完了し refs 0027 は archive (=
`tap/refs/HTMLification/docs/archive/0027-jun15-mcp-pivot.html`)。

## Next Actions

なし (= Phase 4 #1-#4 全完了)。 後続作業候補は別 issue で fork:

1. Phase 3 cost (c) Anthropic dashboard credit 0 verify (= 2026-06-15
   launch 以降の operational evidence)

## Known Risks / Blockers

- `amadeus run` / `install-hook` / `uninstall-hook` は **削除済** (=
  r4 phase1)。 既存 scheduler / CI で wrap していた job は
  `/review-gate` skill 経由 + `amadeus mcp` data plane に書き換え必要
- `gh pr comment` 失敗時は MCP tool response の `reason` field で
  surface (= rate limit / auth error 等を session が retry 判定可能)

## Context the Next Actor Needs

- **canonical plan archive**: `tap/refs/HTMLification/docs/archive/0027-jun15-mcp-pivot.html`
- **post-mortem**: `tap/refs/HTMLification/lessons/0027-jun15-mcp-pivot-post-mortem.html`
- **billing boundary 原則**: LLM 発火は常に human-initiated、 daemon は route まで
- **semgrep gate**: `.semgrep/jun15-no-headless-llm.yaml` 6 rule (= 0028 で
  `jun15-no-print-flag-literal-go` 追加)、 production path に `permanent`
  nosemgrep 例外禁止
- **port-adapter 境界**: `port.CommentPoster` (1-method narrow port) +
  `port.GitHubPRWriter` (5-method full port) の二重 implementation pattern
  により、 MCP layer は full PR writer 依存を避けつつ post_comment 経路を提供

## Relevant Files and Commands

- `docs/adr/0026-mcp-pivot.md` - architectural pin
- `.semgrep/jun15-no-headless-llm.yaml` - billing-boundary gate (6 rule、 0028 で dynamic args spread catch rule 追加)
- `internal/session/mcp_server.go` - amadeus MCP server (4 tool real impl)
- `internal/session/gh_pr_writer.go` - `GhPRWriter.PostComment` (= `gh pr comment <num> --body <body>`)
- `internal/usecase/port/port.go` - `GitHubPRWriter` extended + `CommentPoster` narrow port 新設
- `internal/cmd/mcp.go` - `amadeus mcp` cobra subcommand (= `GhPRWriter(cwd)` を CommentPoster として注入)
- `plugins/amadeus/skills/review-gate/SKILL.md` - human-driven entry point
- `just lint` - golangci-lint v2 + markdownlint (0 issues 維持)
- `just semgrep` - semgrep gate (0 findings 維持)
- `go test -count=1 ./...` - amadeus test suite
