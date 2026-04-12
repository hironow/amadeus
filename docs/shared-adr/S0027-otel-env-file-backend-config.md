# S0027. OTel .otel.env File Backend Configuration

**Date:** 2026-03-04
**Status:** Accepted

## Context

All 4 CLI tools (phonewave, sightjack, paintress, amadeus) share identical OTel
initialization that reads environment variables to configure OTLP HTTP export.
`initTracer()` runs in `PersistentPreRunE` before config file loading, so the
OTel SDK's env-var-driven configuration is the natural integration point.

We needed a mechanism for `init --otel-backend` to persist backend configuration
(Jaeger local, Weights & Biases Weave) so subsequent commands automatically
export traces without requiring the user to set environment variables manually.

Alternatives considered:

1. **YAML config section** — requires changes to `initTracer()` and config
   loading order; breaks the current clean env-var-driven init.
2. **Direct env var injection in init** — ephemeral; lost after shell exit.
3. **Shell profile modification** — invasive; affects all processes.

## Decision

Use a `.otel.env` file in each tool's state directory (`.phonewave/`,
`.siren/`, `.expedition/`, `.gate/`). The file uses `KEY=VALUE` format with:

- `#` comment lines and blank lines are skipped
- `${VAR}` references expanded at runtime via `os.ExpandEnv()` (no raw secrets)
- Existing environment variables always take precedence (explicit > config)
- Missing file is silently ignored (OTel stays noop)

`applyOtelEnv(stateDir)` reads this file in `PersistentPreRunE` before
`initTracer()`. The Go OTel SDK's `resource.Default()` (v1.40.0+) includes
a `fromEnv{}` detector that reads `OTEL_RESOURCE_ATTRIBUTES` automatically.

## Consequences

### Positive

- Zero changes to `initTracer()` — env vars are the native SDK interface
- `resource.Default()` picks up `OTEL_RESOURCE_ATTRIBUTES` (Weave entity/project) automatically
- Environment variables always override file values (predictable precedence)
- Secret references (`${WANDB_API_KEY}`) are expanded at runtime (no raw secrets in file)
- State-dir-local scope — multiple projects can have different backends

### Negative

- `os.Setenv` in `PersistentPreRunE` — safe for single-threaded CLI init but
  would be a concern in concurrent server scenarios
- All 3 AI coding tools resolve state dir via `--config` / `--path` / cwd.
  If cwd differs from project root and no flag is provided, `.otel.env` won't be found

### Neutral

- `.otel.env` is gitignored by default (state directories are gitignored)
- Users can always bypass the file by setting env vars directly
