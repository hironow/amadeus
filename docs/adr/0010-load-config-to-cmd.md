# 0010. Move LoadConfig to internal/cmd

**Date:** 2026-02-25
**Status:** Accepted

## Context

ADR 0014 (Flat Package Architecture) established file-level conventions for
the amadeus root package. The convention table lists `config.go` as a
"Types + pure functions" file with the parenthetical "(except LoadConfig)".
However, `LoadConfig` — which performs `os.ReadFile` + `yaml.Unmarshal` —
remained in the root package, creating a documented-but-unresolved gap between
the convention and the implementation.

All four callers of `LoadConfig` are in `internal/cmd/` (check.go, sync.go,
log.go, validate.go). The fifth usage is in `doctor.go`'s `checkConfig`
function, which already performs `os.Stat` before calling `LoadConfig`,
making `LoadConfig`'s `ErrNotExist` fallback redundant in that context.

## Decision

Move `LoadConfig` from the root `config.go` to `internal/cmd/config.go` as
an unexported `loadConfig` function.

1. **`internal/cmd/config.go`**: Contains `loadConfig(path string) (amadeus.Config, error)`.
   Unexported because all callers are within the same package.

2. **`doctor.go` checkConfig**: Inlines the file-reading logic
   (`os.ReadFile` + `yaml.Unmarshal`) directly, removing the redundant
   `ErrNotExist` handling that `os.Stat` already covers.

3. **Root `config.go`**: Retains `Config` type definitions, `DefaultConfig()`,
   `ValidateConfig()`, and `ValidLang()` — all pure functions with no I/O.

## Consequences

### Positive

- Root `config.go` becomes genuinely types-only, closing the ADR 0014 gap
- `loadConfig` is unexported, minimizing API surface
- `checkConfig` is simplified by removing redundant error handling

### Negative

- File-reading logic exists in two places (cmd/config.go and doctor.go) — but
  the doctor version is specialized (stat-first, no default fallback)
