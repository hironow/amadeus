package cmd

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/hironow/amadeus"
)

// InitTracer sets up the OpenTelemetry TracerProvider and updates
// amadeus.Tracer. If OTEL_EXPORTER_OTLP_ENDPOINT or
// OTEL_EXPORTER_OTLP_TRACES_ENDPOINT is set, it creates an OTLP HTTP
// exporter with a BatchSpanProcessor. Otherwise, it keeps the noop
// TracerProvider. Returns a shutdown function.
func InitTracer(serviceName, ver string) func(context.Context) error {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		np := noop.NewTracerProvider()
		otel.SetTracerProvider(np)
		amadeus.Tracer = np.Tracer(serviceName)
		return func(context.Context) error { return nil }
	}

	exp, err := otlptracehttp.New(context.Background())
	if err != nil {
		np := noop.NewTracerProvider()
		otel.SetTracerProvider(np)
		amadeus.Tracer = np.Tracer(serviceName)
		return func(context.Context) error { return nil }
	}

	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(ver),
		),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	amadeus.Tracer = tp.Tracer(serviceName)

	return func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}
}
