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

// initTracer sets up the OpenTelemetry TracerProvider and updates
// amadeus.Tracer. Called from PersistentPreRunE; shutdown is registered
// via cobra.OnFinalize.
func initTracer(serviceName, ver string) func(context.Context) error {
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
