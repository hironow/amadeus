package session

// white-box-reason: OTel instrumentation: tests unexported tracer setup and span attribute verification

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/usecase/port"
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

// testInternalCheckEventEmitter implements port.CheckEventEmitter for internal session tests.
type testInternalCheckEventEmitter struct {
	agg *domain.CheckAggregate
}

func (e *testInternalCheckEventEmitter) EmitInboxConsumed(data domain.InboxConsumedData, now time.Time) error {
	_, err := e.agg.RecordInboxConsumed(data, now)
	return err
}
func (e *testInternalCheckEventEmitter) EmitForceFullNextSet(prevDiv, currDiv float64, now time.Time) error {
	_, err := e.agg.RecordForceFullNextSet(prevDiv, currDiv, now)
	return err
}
func (e *testInternalCheckEventEmitter) EmitDMailGenerated(dmail domain.DMail, now time.Time) error {
	_, err := e.agg.RecordDMailGenerated(dmail, now)
	return err
}
func (e *testInternalCheckEventEmitter) EmitConvergenceDetected(alert domain.ConvergenceAlert, now time.Time) error {
	_, err := e.agg.RecordConvergenceDetected(alert, now)
	return err
}
func (e *testInternalCheckEventEmitter) EmitDMailCommented(dmailName, issueID string, now time.Time) error {
	_, err := e.agg.RecordDMailCommented(dmailName, issueID, now)
	return err
}
func (e *testInternalCheckEventEmitter) EmitCheck(result domain.CheckResult, now time.Time) error {
	_, err := e.agg.RecordCheck(result, now)
	return err
}
func (e *testInternalCheckEventEmitter) EmitRunStarted(data domain.RunStartedData, now time.Time) error {
	_, err := e.agg.RecordRunStarted(data, now)
	return err
}
func (e *testInternalCheckEventEmitter) EmitRunStopped(data domain.RunStoppedData, now time.Time) error {
	_, err := e.agg.RecordRunStopped(data, now)
	return err
}
func (e *testInternalCheckEventEmitter) EmitPRConvergenceChecked(data domain.PRConvergenceCheckedData, now time.Time) error {
	_, err := e.agg.RecordPRConvergenceChecked(data, now)
	return err
}
func (e *testInternalCheckEventEmitter) EmitPRMerged(_ domain.PRMergedData, _ time.Time) error {
	return nil
}
func (e *testInternalCheckEventEmitter) EmitPRMergeSkipped(_ domain.PRMergeSkippedData, _ time.Time) error {
	return nil
}

// testInternalCheckStateProvider implements port.CheckStateManager for internal session tests.
type testInternalCheckStateProvider struct {
	agg *domain.CheckAggregate
}

func (m *testInternalCheckStateProvider) ShouldFullCheck(forceFlag bool) bool {
	return m.agg.ShouldFullCheck(forceFlag)
}
func (m *testInternalCheckStateProvider) ForceFullNext() bool     { return m.agg.ForceFullNext() }
func (m *testInternalCheckStateProvider) SetForceFullNext(v bool) { m.agg.SetForceFullNext(v) }
func (m *testInternalCheckStateProvider) ShouldPromoteToFull(prev, curr float64) bool {
	return m.agg.ShouldPromoteToFull(prev, curr)
}
func (m *testInternalCheckStateProvider) AdvanceCheckCount(fullCheck bool, wasForced bool) {
	m.agg.AdvanceCheckCount(fullCheck, wasForced)
}
func (m *testInternalCheckStateProvider) Restore(result domain.CheckResult) { m.agg.Restore(result) }

// fakeProviderRunner returns a fixed JSON response for testing.
type fakeProviderRunner struct {
	response string
}

func (f *fakeProviderRunner) Run(_ context.Context, _ string, _ io.Writer, _ ...port.RunOption) (string, error) {
	return f.response, nil
}

var _ port.ProviderRunner = (*fakeProviderRunner)(nil)

func TestRunDivergenceMeter_EmitsClaudeInvokeSpan(t *testing.T) {
	// given
	exporter := setupTestTracer(t)

	// Minimal valid Claude response that ParseClaudeResponse can handle
	fakeResp := `{
		"axes": {},
		"dmails": [],
		"reasoning": "test",
		"impact_radius": []
	}`

	cfg := domain.DefaultConfig()
	agg := domain.NewCheckAggregate(cfg)
	a := &Amadeus{
		Config:  cfg,
		Claude:  &fakeProviderRunner{response: fakeResp},
		Logger:  &domain.NopLogger{},
		Emitter: &testInternalCheckEventEmitter{agg: agg},
		State:   &testInternalCheckStateProvider{agg: agg},
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
		if s.Name == "provider.invoke" {
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
					t.Errorf("missing gen_ai attribute %q on provider.invoke span", key)
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
				t.Error("missing gen_ai.request.model attribute on provider.invoke span")
			}

			// Cross-tool conformance: provider.model and provider.timeout_sec must be present
			conformanceAttrs := []string{"provider.model", "provider.timeout_sec"}
			for _, key := range conformanceAttrs {
				var attrFound bool
				for _, attr := range s.Attributes {
					if string(attr.Key) == key {
						attrFound = true
					}
				}
				if !attrFound {
					t.Errorf("missing cross-tool conformance attribute %q on provider.invoke span", key)
				}
			}
		}
	}
	if !invokeFound {
		t.Error("expected 'provider.invoke' span to be emitted by runDivergenceMeter")
	}
}
