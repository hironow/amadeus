# Handover

**Last updated:** 2026-10-11 (JST)
**Updated by:** Codex

## Current State

CI and release no longer set up Takumi Guard (5 steps removed). Semgrep
installs from official PyPI through `just install-semgrep` using uv. PR
workflows skip drafts and restart on `ready_for_review`. Go dependencies and
application code are unchanged.

## In Progress

Branch `ci/remove-flatt`: local `just check` passes with `GOTOOLCHAIN=go1.26.1`,
the toolchain declared by `go.mod` and used by CI. PR checks must all succeed
before squash merge; afterward remove the Flatt Actions allowlist entries.

## Next Actions

1. Complete PR CI and squash merge this branch.
2. Remove Flatt from the Actions allowlist while preserving other entries.
3. Confirm the default branch contains no active Flatt integration.

## Known Risks / Blockers

The existing golangci-lint dependency cannot decode Go 1.27 export data on this
host. Local validation uses the declared CI toolchain, Go 1.26.1; compiler and
linter upgrades are outside this registry-removal change.

## Previous handover (2026-06-10; unverified historical draft)

The MCP pivot is reflected throughout the repo: amadeus is a pure data plane
(`amadeus mcp`) reading the `.gate/` event store and PR-status projection and
posting PR comments via `gh`; the headless scoring/waiting-loop daemon is
retired (README). Recent work on `main` aligned MCP pivot wording/docs
(#241–#246), hardened session close handling (#236), kept MCP session wiring
active (`c99ec18`), suppressed pre-existing lint findings (#237–#240), and
migrated e2e tests to testcontainers-go (#233). Last commit: `fa14207`
"docs: add decision queue for human-review items (#248)" on 2026-06-10.

### In Progress

不明 (git 履歴からは判別できず) — no open feature branch is evident in the
shallow clone; the most recent code change is `fix(sessions): keep mcp
session wiring active`.

### Next Actions

1. requester による docs/intent.md ドラフトのレビューと確定
2. Work through the human-review items in `docs/decision-queue.md` (added 2026-06-10, #248)

### Known Risks / Blockers

- `docs/intent.md` / `docs/handover.md` are in `.gitignore`; this PR adds them with `git add -f`. Decide whether to track them or drop the ignore entries.

### Context the Next Actor Needs

- Task runner is `just`; `just check` runs fmt + vet + golangci-lint + semgrep + root-guard + tests + docs-check
- Project-specific semgrep rules live under `.semgrep/`; pre-commit hooks via `.pre-commit-config.yaml`; toolchain pinned in `mise.toml`
- Naming/design concepts come from Steins;Gate 0 (Amadeus, Reading Steiner, Divergence Meter, D-Mail, World Line) — see README before touching domain terms
- `.gate/` layout matters: `config.yaml` (weights/thresholds), `events/` (append-only JSONL, gitignored), `insights/` (git-tracked ledger), `outbox/` / `inbox/` / `archive/` for D-Mails
- `amadeus.post_comment` shells out to `gh pr comment` — `gh` auth is an external dependency
- Releases via GoReleaser; e2e tests use testcontainers-go

### Relevant Files and Commands

- `README.md` — MCP tools, Steins;Gate concept mapping, `.gate/` architecture
- `docs/decision-queue.md` — open human-review items
- `docs/dmail-protocol-conventions.md` and `docs/gate-directory.md` — protocol and state-dir conventions
- `justfile` — `just check` (full gate), `just test`, `just lint`, `just semgrep`
- `cmd/` and `internal/` — CLI entrypoints and core implementation
