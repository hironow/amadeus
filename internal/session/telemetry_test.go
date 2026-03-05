package session

import (
	"context"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/port"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// setupTestTracer installs an InMemoryExporter so spans are immediately
// available for inspection. Restores the global TracerProvider after the test.
func setupTestTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	platform.Tracer = tp.Tracer("amadeus-test")
	t.Cleanup(func() {
		tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
		platform.Tracer = prev.Tracer("amadeus")
	})
	return exp
}

// fakeClaudeRunner returns a fixed JSON response for testing.
type fakeClaudeRunner struct {
	response []byte
}

func (f *fakeClaudeRunner) Run(_ context.Context, _ string) ([]byte, error) {
	return f.response, nil
}

var _ port.ClaudeRunner = (*fakeClaudeRunner)(nil)

func TestRunDivergenceMeter_EmitsClaudeInvokeSpan(t *testing.T) {
	// given
	exporter := setupTestTracer(t)

	// Minimal valid Claude response that ParseClaudeResponse can handle
	fakeResp := []byte(`{
		"axes": {},
		"dmails": [],
		"reasoning": "test",
		"impact_radius": []
	}`)

	a := &Amadeus{
		Config:    domain.DefaultConfig(),
		Claude:    &fakeClaudeRunner{response: fakeResp},
		Logger:    &domain.NopLogger{},
		Aggregate: domain.NewCheckAggregate(domain.DefaultConfig()),
	}

	ctx, span := platform.Tracer.Start(context.Background(), "test.parent")
	defer span.End()

	// when
	_, err := a.runDivergenceMeter(ctx, "test prompt", true, domain.CheckResult{}, true)

	// then
	if err != nil {
		t.Fatalf("runDivergenceMeter: %v", err)
	}

	spans := exporter.GetSpans()
	var invokeFound bool
	for _, s := range spans {
		if s.Name == "claude.invoke" {
			invokeFound = true
			// Verify gen_ai.* semantic convention attributes
			requiredAttrs := map[string]string{
				"gen_ai.operation.name": "chat",
				"gen_ai.system":         "anthropic",
			}
			for key, want := range requiredAttrs {
				var attrFound bool
				for _, attr := range s.Attributes {
					if string(attr.Key) == key {
						attrFound = true
						if got := attr.Value.AsString(); got != want {
							t.Errorf("attr %s = %q, want %q", key, got, want)
						}
					}
				}
				if !attrFound {
					t.Errorf("missing gen_ai attribute %q on claude.invoke span", key)
				}
			}
			// gen_ai.request.model should be present
			var modelFound bool
			for _, attr := range s.Attributes {
				if string(attr.Key) == "gen_ai.request.model" {
					modelFound = true
				}
			}
			if !modelFound {
				t.Error("missing gen_ai.request.model attribute on claude.invoke span")
			}

			// Cross-tool conformance: claude.model and claude.timeout_sec must be present
			conformanceAttrs := []string{"claude.model", "claude.timeout_sec"}
			for _, key := range conformanceAttrs {
				var attrFound bool
				for _, attr := range s.Attributes {
					if string(attr.Key) == key {
						attrFound = true
					}
				}
				if !attrFound {
					t.Errorf("missing cross-tool conformance attribute %q on claude.invoke span", key)
				}
			}
		}
	}
	if !invokeFound {
		t.Error("expected 'claude.invoke' span to be emitted by runDivergenceMeter")
	}
}
