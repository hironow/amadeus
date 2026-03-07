# S0017. Data Persistence Boundaries

**Date:** 2026-03-01
**Status:** Accepted

## Context

parent CLAUDE.md defines persistence boundaries for external data sources:

- **Linear**: Single source of truth for confirmed issues. Full copies are unnecessary;
  references, summaries, and categorical groupings are permitted.
- **GitHub**: Code and PRs are stored/managed on GitHub with GitHub Actions for CI/CD.
  Full re-fetch of all PRs and review comments is impractical;
  references, summaries, and categorical groupings are permitted.

Additionally, CLAUDE.md requires that git-tracked data must not contain environment-dependent
information (absolute paths, local-specific configuration). The `*.local.*` file extension
pattern separates environment-specific data from portable configuration.

## Decision

### Data Source Boundaries

| Source | Role | Full Copy | References/Summaries | Local Cache |
|--------|------|-----------|---------------------|-------------|
| Linear | Issue SoT | Never | Yes | `.run/` (gitignored) |
| GitHub | Code/PR SoT | Never | Yes | `.run/` (gitignored) |
| SQLite | Tool state | N/A (local) | N/A | `.run/` (gitignored) |
| Config | Tool settings | N/A | N/A | `*.local.*` for env-specific |

### Environment-Dependent Data Separation

1. **Portable config** (git-tracked): routing rules, thresholds, weights, language settings
2. **Local config** (`*.local.*`, gitignored): paths, credentials, machine-specific settings
3. **State data** (`.run/`, gitignored): SQLite databases, event logs, delivery logs
4. **Cache data** (`.run/`, gitignored): Linear issue cache, GitHub PR summaries

### Path Storage Rules

- Config files that are git-tracked MUST use relative paths (relative to config file location)
- Runtime resolution: `filepath.Join(filepath.Dir(configPath), relativePath)`
- Event payloads in gitignored stores MAY use absolute paths for runtime convenience
- D-Mail payloads SHOULD use repository-relative paths (e.g., `auth/session.go`)

### Reference Data Management (S0012)

When tools need to reference external data (Linear issues, GitHub PRs):

- Store lightweight references (ID, title, URL, status) not full content
- References are best-effort and may become stale
- Re-fetch individual items on demand rather than bulk sync
- Cache in `.run/` directory (gitignored)

## Consequences

### Positive

- Config files are portable across developer machines and CI environments
- Clear separation of concerns between portable and local data
- `.run/` convention provides consistent gitignored state directory
- `*.local.*` pattern enables per-developer customization without conflicts

### Negative

- Developers must remember to use `*.local.*` for environment-specific overrides
- Relative path resolution adds slight complexity to config loading

### Neutral

- amadeus is the reference implementation for this pattern (`.gate/` fully gitignored, `*.local.*` adopted)
- phonewave.yaml uses relative paths since S1 resolution
