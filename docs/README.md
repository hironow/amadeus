# amadeus docs

## Architecture

- [gate-directory.md](gate-directory.md) — `.gate/` directory structure specification
- [policies.md](policies.md) — Event → Policy mapping (WHEN event THEN command)
- [otel-backends.md](otel-backends.md) — OpenTelemetry backend configuration (Jaeger, Weave)
- [stdio-convention.md](stdio-convention.md) — stdin/stdout/stderr convention
- [testing.md](testing.md) — Test strategy and conventions

## CLI Reference

- [amadeus](cli/amadeus.md) — Root command
- [amadeus init](cli/amadeus_init.md) — Initialize a project
- [amadeus check](cli/amadeus_check.md) — Run integrity verification
- [amadeus validate](cli/amadeus_validate.md) — Validate configuration
- [amadeus log](cli/amadeus_log.md) — Show divergence log
- [amadeus sync](cli/amadeus_sync.md) — Sync state
- [amadeus mark-commented](cli/amadeus_mark-commented.md) — Mark D-Mails as commented
- [amadeus status](cli/amadeus_status.md) — Show verification status
- [amadeus doctor](cli/amadeus_doctor.md) — Diagnose configuration issues
- [amadeus clean](cli/amadeus_clean.md) — Clean state files
- [amadeus rebuild](cli/amadeus_rebuild.md) — Rebuild state from events
- [amadeus archive-prune](cli/amadeus_archive-prune.md) — Prune archived data
- [amadeus install-hook](cli/amadeus_install-hook.md) — Install git hook
- [amadeus uninstall-hook](cli/amadeus_uninstall-hook.md) — Uninstall git hook
- [amadeus version](cli/amadeus_version.md) — Show version
- [amadeus update](cli/amadeus_update.md) — Self-update

## Architecture Decision Records

See [adr/README.md](adr/README.md) for the full index.
