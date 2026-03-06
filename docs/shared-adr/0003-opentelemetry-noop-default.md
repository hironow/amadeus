# 0003. OpenTelemetry noop-default + OTLP HTTP

**Date:** 2026-02-23
**Status:** Accepted

## Context

Observability is essential for debugging the multi-tool D-Mail delivery
pipeline, but most users run the tools locally without a telemetry backend.
Mandatory tracing infrastructure would add startup latency and deployment
complexity for the common case.

## Decision

Adopt OpenTelemetry with a noop-default strategy across all four tools:

1. **noop default**: When neither `OTEL_EXPORTER_OTLP_ENDPOINT` nor
   `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` is set, the tracer is a no-op with
   zero overhead.
2. **OTLP HTTP exporter**: When either environment variable is set, traces are
   exported via OTLP HTTP (compatible with Jaeger v2, Grafana Tempo, etc.).
   `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` takes precedence per the OpenTelemetry
   specification.
3. **Tracer lifecycle in `main.go`**: `InitTracer()` is called in `main()` with
   `defer shutdown(ctx)`. This avoids cobra's `PersistentPostRunE` which is
   skipped when `RunE` returns an error.
4. **Span propagation**: Each significant operation (scan, deliver, verify)
   creates a child span for end-to-end trace visibility.

## Consequences

### Positive

- Zero cost for users who do not need tracing
- Full distributed tracing when a backend is available
- Tracer shutdown is guaranteed via `defer` regardless of command outcome

### Negative

- Tracer lifecycle is coupled to `main.go` rather than cobra hooks
- Each tool must independently configure its service name and resource attributes

### Neutral

- Shutdown mechanism varies per tool: phonewave/paintress/amadeus use
  `defer shutdown(ctx)` in `main.go`, sightjack uses `cobra.OnFinalize` +
  `sync.Once`. Both approaches guarantee cleanup; the choice depends on each
  tool's initialization order
