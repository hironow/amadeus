# Intent

**Last updated:** 2026-06-10
**Requester:** hironow
**Status:** DRAFT — AI が README / git 履歴から起草。requester 未確認
**Work unit:** amadeus — MCP server + data plane for post-merge divergence review

## Goal

Provide a pure data-plane Go CLI (`amadeus mcp`) that reads the `.gate/` event
store and PR-status projection over MCP and posts review comments to GitHub
PRs via `gh`, so that a human-initiated Claude Code session (the LLM owner
after the MCP pivot) can drive post-merge divergence review through the
amadeus MCP tools.

## Success Criteria

- `just check` passes (fmt, vet, golangci-lint, semgrep, root-guard, tests, docs-check) — quality gate defined in the justfile and wired into CI under `.github/`
- The four MCP tools documented in README respond as described: `amadeus.ping`, `amadeus.next_review`, `amadeus.get_pr_status`, `amadeus.post_comment` (covered by e2e tests incl. the MCP tools-list handshake test, migrated to testcontainers-go)
- Product-level success criteria beyond these mechanical gates: 未定義 — Open Questions 参照

## Scope

### In scope

- MCP server / data plane: reading the gate event store (latest check, divergence reading, PRs evaluated) and the PR-status projection
- Posting review comments to PRs via the GitHub Comments API (`gh pr comment`)
- Supporting data-plane commands (log / sync / mark-commented / status / rebuild / ...)

### Out of scope (Non-goals)

- Divergence scoring, D-Mail generation, and the headless waiting-loop daemon — explicitly retired per README after the MCP pivot
- Running the Steins;Gate-inspired scoring mechanics inside the binary — scores remain readable from the event store but the review is driven from the claude-code session

## Constraints

- Go module; lint/semgrep/test gates enforced via justfile recipes and pre-commit hooks (`.golangci.yaml`, `.semgrep/`, `.pre-commit-config.yaml`)
- Part of the D-Mail protocol ecosystem (Verifier role, `.gate/` endpoint) alongside sightjack / paintress / phonewave; produces `design-feedback` / `implementation-feedback` / `convergence` D-Mails and consumes `report` D-Mails
- Requires the `gh` CLI for `amadeus.post_comment`
- Released via GoReleaser (`.goreleaser.yaml`) and distributed through the `hironow/homebrew-tap` cask

## Open Questions

- [ ] requester による本ドラフトのレビュー
- [ ] Product-level success criteria for the MCP pivot (when is the data-plane scope "done"?) — not stated in README or docs
- [ ] Deadlines or milestone targets — none found in the repo
- [ ] `docs/intent.md` and `docs/handover.md` are listed in `.gitignore` — was that intentional, and should this PR keep them tracked or should the ignore entries be removed?
- [ ] Disposition of the items in `docs/decision-queue.md` (added 2026-06-10, #248) — which are still open?
