# Contributing

Thank you for your interest in this project.

## Current Status

This project is in **0.0.x** (pre-release). We are working toward **0.1.0**, focusing on:

- API stabilization
- CLI command finalization
- OpenTelemetry instrumentation refinement
- Bug fixes
- Guarding against unintended API cost increases

During this phase, the public interface (commands, flags, configuration schema, output format) may change without notice.

## How to Contribute

### Issues First

If you have a question, feature request, or bug report, please **open an issue**. We do implement features suggested through issues when they align with the project's direction.

### Pull Requests

Pull request creation is restricted to collaborators. This setting will remain unchanged during the 0.0.x series.

If you'd like to propose a code change, please start with an issue to discuss the approach.

### Commit and PR Title Convention

This project uses [Conventional Commits](https://www.conventionalcommits.org/). Since all PRs are squash-merged, **the PR title becomes the commit message** and must follow this format:

```
type: description
```

Common types: `feat`, `fix`, `tidy`, `docs`, `test`, `ci`

**Breaking changes** must include `!` after the type:

```
feat!: remove OTEL_DETAIL_LEVEL environment variable (BREAKING)
```

The `(BREAKING)` suffix is optional but recommended for visibility. GoReleaser uses this convention to group release notes automatically.

### Fork

You are welcome to fork this repository under the terms of the [Apache 2.0 License](LICENSE).

## Development Note

This project is developed with AI coding tools. The codebase follows TDD (Test-Driven Development) and the architectural patterns described in `docs/` and `docs/adr/`.
