# Handover

**Last updated:** 2026-06-10 (JST)
**Updated by:** claude (AI draft from git history — review before trusting)

## Current State

The MCP pivot is reflected throughout the repo: amadeus is a pure data plane
(`amadeus mcp`) reading the `.gate/` event store and PR-status projection and
posting PR comments via `gh`; the headless scoring/waiting-loop daemon is
retired (README). Recent work on `main` aligned MCP pivot wording/docs
(#241–#246), hardened session close handling (#236), kept MCP session wiring
active (`c99ec18`), suppressed pre-existing lint findings (#237–#240), and
migrated e2e tests to testcontainers-go (#233). Last commit: `fa14207`
"docs: add decision queue for human-review items (#248)" on 2026-06-10.

## In Progress

不明 (git 履歴からは判別できず) — no open feature branch is evident in the
shallow clone; the most recent code change is `fix(sessions): keep mcp
session wiring active`.

## Next Actions

1. requester による docs/intent.md ドラフトのレビューと確定
2. Work through the human-review items in `docs/decision-queue.md` (added 2026-06-10, #248)

## Known Risks / Blockers

- `docs/intent.md` / `docs/handover.md` are in `.gitignore`; this PR adds them with `git add -f`. Decide whether to track them or drop the ignore entries.

## Context the Next Actor Needs

- Task runner is `just`; `just check` runs fmt + vet + golangci-lint + semgrep + root-guard + tests + docs-check
- Project-specific semgrep rules live under `.semgrep/`; pre-commit hooks via `.pre-commit-config.yaml`; toolchain pinned in `mise.toml`
- Naming/design concepts come from Steins;Gate 0 (Amadeus, Reading Steiner, Divergence Meter, D-Mail, World Line) — see README before touching domain terms
- `.gate/` layout matters: `config.yaml` (weights/thresholds), `events/` (append-only JSONL, gitignored), `insights/` (git-tracked ledger), `outbox/` / `inbox/` / `archive/` for D-Mails
- `amadeus.post_comment` shells out to `gh pr comment` — `gh` auth is an external dependency
- Releases via GoReleaser; e2e tests use testcontainers-go

## Relevant Files and Commands

- `README.md` — MCP tools, Steins;Gate concept mapping, `.gate/` architecture
- `docs/decision-queue.md` — open human-review items
- `docs/dmail-protocol-conventions.md` and `docs/gate-directory.md` — protocol and state-dir conventions
- `justfile` — `just check` (full gate), `just test`, `just lint`, `just semgrep`
- `cmd/` and `internal/` — CLI entrypoints and core implementation
