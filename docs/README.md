# amadeus docs

## Architecture

- [conformance.md](conformance.md) — What/Why/How conformance table (single source)
- [gate-directory.md](gate-directory.md) — `.gate/` directory structure specification
- [policies.md](policies.md) — Event → Policy mapping (WHEN event THEN command)
- [otel-backends.md](otel-backends.md) — OpenTelemetry backend configuration (Jaeger, Weave)
- [dmail-protocol-conventions.md](dmail-protocol-conventions.md) — D-Mail filename uniqueness and archive retention conventions
- [stdio-convention.md](stdio-convention.md) — stdin/stdout/stderr convention
- [testing.md](testing.md) — Test strategy and conventions

## CLI Reference

- [amadeus](cli/amadeus.md) — Root command
- [amadeus init](cli/amadeus_init.md) — Initialize a project (`--force` to regenerate)
- [amadeus run](cli/amadeus_run.md) — Daemon mode: divergence check + PR convergence + fsnotify inbox watcher
- [amadeus config show](cli/amadeus_config_show.md) — Show current configuration
- [amadeus config set](cli/amadeus_config_set.md) — Update configuration values
- [amadeus validate](cli/amadeus_validate.md) — Validate configuration
- [amadeus log](cli/amadeus_log.md) — Show divergence log
- [amadeus sync](cli/amadeus_sync.md) — Sync state
- [amadeus mark-commented](cli/amadeus_mark-commented.md) — Mark D-Mails as commented
- [amadeus status](cli/amadeus_status.md) — Show verification status
- [amadeus doctor](cli/amadeus_doctor.md) — Diagnose configuration issues (context-budget per-item diagnostics, WARN status)
- [amadeus clean](cli/amadeus_clean.md) — Clean state files
- [amadeus rebuild](cli/amadeus_rebuild.md) — Rebuild state from events
- [amadeus archive-prune](cli/amadeus_archive-prune.md) — Prune archived data
- [amadeus install-hook](cli/amadeus_install-hook.md) — Install git hook
- [amadeus uninstall-hook](cli/amadeus_uninstall-hook.md) — Uninstall git hook
- [amadeus version](cli/amadeus_version.md) — Show version
- [amadeus update](cli/amadeus_update.md) — Self-update

## Architecture Decision Records

- [adr/](adr/README.md) — Tool-specific ADRs
- [shared-adr/](shared-adr/README.md) — Cross-tool shared ADRs (S0001–S0031)
