# CLI Completion Design: resolve, log, --quiet

**Goal:** Complete the three-command CLI surface defined in the architecture document (Section 11.2).

**Scope:** Local file operations only. No Linear integration (deferred to a separate feature).

---

## 1. `amadeus resolve <id> --approve / --reject`

Resolves a HIGH-severity D-Mail held in pending status.

### New Functions

- `StateStore.LoadDMail(id string) (DMail, error)` — reads `dmails/<id>.json`
- `StateStore.LoadAllDMails() ([]DMail, error)` — reads all files in `dmails/`, returns sorted by ID
- `Amadeus.ResolveDMail(id string, action string, reason string) error` — validates and updates D-Mail status

### Validation Rules

- D-Mail must exist (file not found → error)
- D-Mail status must be `pending` (already resolved → error)
- Exactly one of `--approve` or `--reject` must be specified (no default action, per architecture doc)
- `--reject` requires `--reason` string (empty reason → error)

### State Changes on Resolve

- `status` → `approved` or `rejected`
- `resolved_at` → current UTC timestamp
- `resolved_action` → `"approve"` or `"reject"`
- `reject_reason` → provided reason (reject only)
- File overwritten in place (`dmails/<id>.json`)

### CLI Output

```
  D-Mail d-043 approved.
  ADR-003 auth→cart dep → approved at 2026-02-20T14:30:00Z
```

### CLI Flags

```
amadeus resolve <id> --approve
amadeus resolve <id> --reject --reason "reason text"
```

---

## 2. `amadeus log`

Displays all past check results and D-Mail history.

### New Functions

- `StateStore.LoadHistory() ([]CheckResult, error)` — reads all files in `history/`, returns sorted by CheckedAt descending
- Reuses `StateStore.LoadAllDMails()` from resolve

### Output Format

```
  History:
    2026-02-20T14:30  a1b2c3d  diff  0.145000 (+0.012000)  1 D-Mail
    2026-02-20T12:00  e4f5g6h  diff  0.133000 (+0.000000)  0 D-Mails
    2026-02-19T10:00  i7j8k9l  full  0.133000 (baseline)   2 D-Mails

  D-Mails:
    d-043  [HIGH] pending    ADR-003 auth→cart dep → sightjack
    d-042  [MED]  sent       DoD #42 partial gap → paintress
    d-041  [LOW]  sent       naming inconsistency → paintress
```

### Design Decisions

- History entries sorted newest-first (descending by CheckedAt)
- D-Mails sorted by ID (ascending)
- Delta shown as `(baseline)` for full checks, `(+X.XXXXXX)` for diffs
- No `--limit` flag (YAGNI)

---

## 3. `--quiet` Flag

Summary-only output for `amadeus check`.

### Output Format

Single line:

```
  0.145000 (+0.012000) 1 D-Mail (1 pending)
```

### Design Decisions

- Applies to `check` command only
- Contains: divergence value, delta, total D-Mail count, pending count
- Exit code remains 0 regardless of pending D-Mails (YAGNI)

---

## CLI Routing Changes

`cmd/amadeus/main.go` switch statement expands:

```
check   → runCheck(...)
resolve → runResolve(...)
log     → runLog(...)
```

Each subcommand gets its own `flag.NewFlagSet` following the existing pattern.

`--quiet` is added to the check subcommand's FlagSet.
