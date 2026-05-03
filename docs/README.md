# amadeus docs

## Architecture

- [conformance.md](conformance.md) — What/Why/How conformance table (single source), including harness layer architecture
- [gate-directory.md](gate-directory.md) — `.gate/` directory structure specification
- [self-improvement-loop.md](self-improvement-loop.md) — How amadeus participates in the observable self-improvement loop
- [policies.md](policies.md) — Event → Policy mapping (WHEN event THEN command)
- [otel-backends.md](otel-backends.md) — OpenTelemetry backend configuration (Jaeger, Weave)
- Claude subprocess isolation: `mcp-config generate` creates `.mcp.json` (MCP allowlist) and `.claude/settings.json` (plugin isolation); `--setting-sources ""` + `--settings` + `--strict-mcp-config` enforces it
- Claude log persistence: raw NDJSON saved to `.run/claude-logs/` after each invocation

- [dmail-protocol-conventions.md](dmail-protocol-conventions.md) — D-Mail filename uniqueness and archive retention conventions
- [rival-contract-v1.md](rival-contract-v1.md) — Rival Contract v1 (amadeus as drift controller; archive projection + corrective D-Mails)
- [stdio-convention.md](stdio-convention.md) — stdin/stdout/stderr convention
- [testing.md](testing.md) — Test strategy, conventions, and scenario test observer pattern

## CLI Reference

- [amadeus](cli/amadeus.md) — Root command
- [amadeus init](cli/amadeus_init.md) — Initialize .gate directory
- [amadeus run](cli/amadeus_run.md) — Run continuous divergence check and PR convergence
- [amadeus config](cli/amadeus_config.md) — View or update configuration
- [amadeus config show](cli/amadeus_config_show.md) — Show current configuration
- [amadeus config set](cli/amadeus_config_set.md) — Update configuration values
- [amadeus validate](cli/amadeus_validate.md) — Validate config file
- [amadeus log](cli/amadeus_log.md) — Show divergence log
- [amadeus sync](cli/amadeus_sync.md) — Show D-Mail sync status (JSON)
- [amadeus mark-commented](cli/amadeus_mark-commented.md) — Record that a D-Mail has been posted as a comment
- [amadeus status](cli/amadeus_status.md) — Show operational status
- [amadeus doctor](cli/amadeus_doctor.md) — Run health checks
- [amadeus clean](cli/amadeus_clean.md) — Remove state directory (.gate/)
- [amadeus rebuild](cli/amadeus_rebuild.md) — Rebuild projections from event store
- [amadeus archive-prune](cli/amadeus_archive-prune.md) — Prune old archived files
- [amadeus install-hook](cli/amadeus_install-hook.md) — Install post-merge git hook
- [amadeus uninstall-hook](cli/amadeus_uninstall-hook.md) — Remove post-merge git hook
- [amadeus version](cli/amadeus_version.md) — Print version, commit, and build information
- [amadeus update](cli/amadeus_update.md) — Self-update amadeus to the latest release

## Architecture Decision Records

- [adr/](adr/README.md) — Tool-specific ADRs
- [shared-adr/](shared-adr/README.md) — Cross-tool shared ADRs (S0001–S0035)
