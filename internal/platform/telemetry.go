package platform

import (
	"os"

	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// DetailLevel controls the verbosity of OTel span attributes.
type DetailLevel string

const (
	DetailBasic DetailLevel = "basic"
	DetailDebug DetailLevel = "debug"
)

// OTELDetailLevel holds the current detail level. Defaults to basic.
var OTELDetailLevel DetailLevel = DetailBasic

// InitDetailLevel reads OTEL_DETAIL_LEVEL from the environment and sets
// OTELDetailLevel accordingly. Call once during tracer initialization.
func InitDetailLevel() {
	if os.Getenv("OTEL_DETAIL_LEVEL") == "debug" {
		OTELDetailLevel = DetailDebug
	} else {
		OTELDetailLevel = DetailBasic
	}
}

// IsDetailDebug returns true when debug-level OTel attributes are enabled.
func IsDetailDebug() bool { return OTELDetailLevel == DetailDebug }

// Tracer is the package-level OTel tracer. Initialized to noop so library
// consumers can use amadeus without calling InitTracer. The cmd layer
// replaces this with a recording tracer when an OTLP endpoint is configured.
var Tracer trace.Tracer = noop.NewTracerProvider().Tracer("amadeus")

// Meter is the package-level OTel meter. Initialized to noop so library
// consumers can use amadeus without calling initMeter. The cmd layer
// replaces this with a recording meter when a metrics endpoint is configured.
var Meter metric.Meter = metricnoop.NewMeterProvider().Meter("amadeus")
