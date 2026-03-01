package amadeus

import (
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Tracer is the package-level OTel tracer. Initialized to noop so library
// consumers can use amadeus without calling InitTracer. The cmd layer
// replaces this with a recording tracer when an OTLP endpoint is configured.
var Tracer trace.Tracer = noop.NewTracerProvider().Tracer("amadeus")

// Meter is the package-level OTel meter. Initialized to noop so library
// consumers can use amadeus without calling initMeter. The cmd layer
// replaces this with a recording meter when a metrics endpoint is configured.
var Meter metric.Meter = metricnoop.NewMeterProvider().Meter("amadeus")
