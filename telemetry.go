package amadeus

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

var tracer trace.Tracer = noop.NewTracerProvider().Tracer("amadeus")

func InitTracer(serviceName, ver string) func(context.Context) error {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		// No endpoint configured — keep the noop tracer to avoid
		// accidentally recording spans via a host's global provider.
		return func(context.Context) error { return nil }
	}

	exp, err := otlptracehttp.New(context.Background())
	if err != nil {
		// Exporter creation failed — keep noop so the CLI is not blocked.
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
	tracer = tp.Tracer(serviceName)

	return func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}
}
