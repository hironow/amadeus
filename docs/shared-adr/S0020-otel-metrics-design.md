# S0020. OTel Metrics Design

**Date:** 2026-03-02
**Status:** Accepted

## Context

All four tools (phonewave, sightjack, paintress, amadeus) had OTel tracing
via OTLP HTTP to Jaeger, but no OTel metrics. Each tool had pure
`SuccessRate()` functions for historical aggregation (READ MODEL), but
lacked real-time counters for continuous monitoring. Self-improvement
feedback loops depended on CLI output rather than durable telemetry.

Key constraints:

- Jaeger v2 stores traces only, not metrics
- Existing `OTEL_EXPORTER_OTLP_ENDPOINT` targets Jaeger — sending metrics
  there causes silent drops
- Tools run as CLI processes (short-lived), not long-running servers
- Each tool is a separate Go module with its own `var Tracer`

## Decision

Add OTel Int64Counter metrics to all four tools following these principles:

1. **Independent metrics endpoint**: Use `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT`
   as a dedicated guard, independent from the tracer's
   `OTEL_EXPORTER_OTLP_ENDPOINT`. When unset, the Meter remains noop.
   This prevents metrics from being silently dropped by Jaeger-only
   environments.

2. **Noop default at root level**: Each tool's root package declares
   `var Meter metric.Meter` initialized to a noop meter. This matches the
   existing `var Tracer` pattern and ensures zero overhead when metrics
   collection is not configured.

3. **Lazy counter creation per call**: `Record*` functions call
   `Meter.Int64Counter()` on each invocation rather than caching via
   `sync.Once`. The OTel SDK deduplicates instruments internally, making
   this safe, simple, and test-friendly (meter can be swapped in tests
   without init-order concerns).

4. **Recording at event emission points**: Metric recording happens in the
   session layer alongside event emission, but before the event store
   nil-check. This ensures counters are incremented even when the event
   store is unavailable, maintaining observability during partial failures.

5. **SuccessRate() unchanged**: The existing pure `SuccessRate()` functions
   remain as historical aggregation (READ MODEL). OTel counters serve a
   different purpose: real-time monitoring. The two are complementary, not
   redundant.

### Per-Tool Metrics

| Tool       | Metric Name                | Attributes       |
|------------|----------------------------|------------------|
| phonewave  | `phonewave.delivery.total` | status, kind     |
| sightjack  | `sightjack.wave.total`     | status           |
| paintress  | `paintress.expedition.total` | status         |
| amadeus    | `amadeus.check.total`      | status           |

### Lifecycle

`initMeter()` in each tool's `internal/cmd/telemetry.go`:

- Called from `PersistentPreRunE` (same as `initTracer`)
- Returns a shutdown function stored in `shutdownMeter`
- Shutdown called from `cobra.OnFinalize` with 5-second timeout
- Meter shutdown runs before tracer shutdown

## Consequences

### Positive

- Metrics collection is opt-in and zero-overhead when disabled
- Independent endpoint guard prevents silent metric loss in Jaeger-only setups
- Consistent pattern across all four tools (same as Tracer)
- Test-friendly: `sdkmetric.NewManualReader` enables deterministic assertions

### Negative

- Two environment variables for OTel (`ENDPOINT` for traces,
  `METRICS_ENDPOINT` for metrics) — slightly more configuration surface
- Metrics backend (Prometheus, Grafana, etc.) must be provisioned separately
  from Jaeger

### Neutral

- `SuccessRate()` and OTel counters coexist — the former for CLI output,
  the latter for external monitoring systems
