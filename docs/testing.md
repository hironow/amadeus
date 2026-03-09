# Testing Strategy

## Test Layers

| Layer | Directory | Build Tag | Dependencies | CI |
|-------|-----------|-----------|-------------|-----|
| Unit | `internal/*/` | none | none | always |
| Integration | `tests/integration/` | none | SQLite | always |
| Scenario | `tests/scenario/` | `scenario` | fake-claude, fake-gh, all 4 tool binaries | CI default (L1+L2) |
| E2E | `tests/e2e/` | `e2e` | Docker, real services | manual / nightly |

## Unit Tests

- Located in `internal/*/` alongside production code
- No build tags required
- Minimize mock usage; prefer real code
- Run: `go test ./internal/... -count=1`

## Integration Tests

- Located in `tests/integration/`
- Test component interactions with real SQLite
- Includes fsnotify inbox watcher tests and daemon lifecycle tests
- Run: `go test ./tests/integration/... -count=1`

## Scenario Tests

- Located in `tests/scenario/`
- Build tag: `//go:build scenario`
- Requires all 4 sibling tool repos at the same parent directory
- TestMain builds all 4 binaries + fake-claude + fake-gh
- Override sibling paths with env vars: `PHONEWAVE_REPO`, `SIGHTJACK_REPO`, `PAINTRESS_REPO`, `AMADEUS_REPO`

### Test Levels

| Level | Focus | Timeout |
|-------|-------|---------|
| L1 | Single closed loop | 120s |
| L2 | Multi-issue scenarios | 180s |
| L3 | Concurrent operations | 300s |
| L4 | Fault injection, recovery | 600s |

Run: `just test-scenario` (L1+L2) or `just test-scenario-all`

## E2E Tests

- Located in `tests/e2e/`
- Build tag: `//go:build e2e`
- Docker compose based (`tests/e2e/compose-e2e.yaml`)
- All dependencies must be real — mocks are strictly prohibited
- Run: `just test-e2e` (requires Docker)

## Public API Test Policy

Unit tests prefer **external test packages** (`package xxx_test`) over white-box packages (`package xxx`). External tests exercise only the public API surface, which:

- Validates the API contract that external consumers depend on
- Catches accidental API breakage through compilation
- Permits internal refactoring without test changes
- Reduces coupling between tests and implementation details

White-box tests (`package xxx`) are reserved for cases that require access to unexported symbols (e.g., testing internal state machines, concurrency internals). Bridge constructors in `export_test.go` files expose specific unexported symbols for external tests when needed.

### CI Enforcement

The `package-audit` CI job enforces minimum external test ratios:

| Scope | Threshold |
|-------|-----------|
| `internal/` | >= 70% |
| `internal/session/` | >= 75% |

Run locally: `just test-package-audit`

### White-Box Test Rationale

Every same-package test file (`package xxx`, not `package xxx_test`) must include a `// white-box-reason:` comment immediately after the package declaration, explaining why public API testing is insufficient.

Format: `// white-box-reason: <concise reason referencing unexported symbols>`

The `package-audit` CI job and `just test-package-rationale-audit` enforce this requirement. New same-package test files without the comment will fail CI.

## Quality Command Contract

### Local Commands

| Command | Purpose | Dependencies |
|---------|---------|-------------|
| `just lint` | Full lint pass | vet, semgrep, root-guard, nosemgrep-audit, lint-md |
| `just check` | Pre-commit gate | fmt, vet, semgrep, root-guard, nosemgrep-audit, test, docs-check |
| `just semgrep` | Semgrep ERROR rules | semgrep |
| `just nosemgrep-audit` | Validate nosemgrep tags | grep/awk |
| `just semgrep-test` | Test semgrep rules against fixtures | semgrep |

### CI Jobs

| Job | Steps |
|-----|-------|
| `semgrep` | `just semgrep` + `just nosemgrep-audit` + `just semgrep-test` |
| `package-audit` | threshold check (inline) + `just test-package-rationale-audit` |
| `test` | build + vet + test + race |
| `docs-check` | docgen + dead links + vocabulary |

### Failure Workflow

1. `just lint` fails locally: fix the issue before committing.
2. `just nosemgrep-audit` fails: add `[permanent]` or `[expires: YYYY-MM-DD]` tag to the nosemgrep annotation.
3. `just semgrep` fails: fix the code or add a tagged nosemgrep annotation if false positive.

## Running Tests

```bash
# Unit + integration (default CI)
just test

# Scenario tests (L1+L2, CI default)
just test-scenario

# E2E (requires Docker)
just test-e2e

# All semgrep rules
just semgrep
just semgrep-test
just semgrep-warnings
```
