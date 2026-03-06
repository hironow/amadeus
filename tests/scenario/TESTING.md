# amadeus scenario tests

## Prerequisites

- Go 1.26+ (all 4 repos must use the same toolchain)
- Sibling repos at the same parent directory:
  - `phonewave/`, `sightjack/`, `paintress/`, `amadeus/`
  - Override with env vars: `PHONEWAVE_REPO`, `SIGHTJACK_REPO`, `PAINTRESS_REPO`, `AMADEUS_REPO`

## Running

```bash
# L1 minimal (single closed loop, ~12s)
just test-scenario-min

# L2 small (~14s)
just test-scenario-small

# L3 middle (~60s)
just test-scenario-middle

# L4 hard (~45s)
just test-scenario-hard

# L1+L2 (CI default)
just test-scenario

# All scenario tests (nightly)
just test-scenario-all
```

Or directly with `go test`:

```bash
go test -tags scenario ./tests/scenario/ -run TestScenario_L1 -count=1 -v -timeout=120s
```

## Test levels

| Level | Test | Focus |
|-------|------|-------|
| L1 | `TestScenario_L1_Minimal` | Single closed loop: report → check → feedback |
| L2 | `TestScenario_L2_Small` | Multi-issue, divergence scoring |
| L3 | `TestScenario_L3_Middle` | Convergence detection, multi-check |
| L4 | `TestScenario_L4_Hard` | Fault injection, recovery |
| - | `TestScenario_ApproveCmdPath` | `--approve-cmd` / `--notify-cmd` hooks (human-on-the-loop) |

## Human-on-the-loop

amadeus uses `--approve-cmd` and `--notify-cmd` flags to integrate external
approval gates. `TestScenario_ApproveCmdPath` verifies:

- CmdApprover fires when a report D-Mail arrives for checking
- CmdNotifier fires before approval is requested
- Approval exit code 0 = approve, non-zero = deny

amadeus does not use go-expect PTY interaction (unlike sightjack). The `check`
command is non-interactive; all approval is handled via `--approve-cmd`
(external command) or `--auto-approve`.

## Build tag

All scenario tests use `//go:build scenario`. They are excluded from regular
`go test ./...` runs and require `-tags scenario`.

## Troubleshooting

### `compile: version "go1.26.0" does not match go tool version "go1.19.3"`

GOROOT or GOTOOLDIR points to a different Go installation than the `go` binary in PATH.

```bash
go version
go tool compile -V
go env GOROOT
go env GOTOOLDIR

# Fix (mise users)
unset GOROOT GOTOOLDIR
mise install go
mise reshim
```

All 4 repos pin `go = "1.26"` in `mise.toml` and `go 1.26` in `go.mod`.
