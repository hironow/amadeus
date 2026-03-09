package platform_test

import (
	"context"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/platform"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTruncateValue_short_string(t *testing.T) {
	result := platform.TruncateValue("hello", 512)
	if result != "hello" {
		t.Errorf("got %q, want %q", result, "hello")
	}
}

func TestTruncateValue_exact_limit(t *testing.T) {
	result := platform.TruncateValue("abc", 3)
	if result != "abc" {
		t.Errorf("got %q, want %q", result, "abc")
	}
}

func TestTruncateValue_over_limit(t *testing.T) {
	result := platform.TruncateValue("abcdef", 3)
	if result != "abc..." {
		t.Errorf("got %q, want %q", result, "abc...")
	}
}

func TestTruncateValue_zero_limit(t *testing.T) {
	result := platform.TruncateValue("hello", 0)
	if result != "..." {
		t.Errorf("got %q, want %q", result, "...")
	}
}

func TestFormatRawEvent(t *testing.T) {
	result := platform.FormatRawEvent("tool_use", `{"name":"Read","id":"toolu_01X"}`, 512)
	want := `tool_use:{"name":"Read","id":"toolu_01X"}`
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestFormatRawEvent_truncates_long_json(t *testing.T) {
	result := platform.FormatRawEvent("tool_result", "abcdefghij", 5)
	want := "tool_result:abcde..."
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestSyntheticToolID(t *testing.T) {
	if id := platform.SyntheticToolID(0); id != "synthetic-0" {
		t.Errorf("got %q, want synthetic-0", id)
	}
	if id := platform.SyntheticToolID(42); id != "synthetic-42" {
		t.Errorf("got %q, want synthetic-42", id)
	}
}

func TestSpanEmittingStreamReader_emits_tool_spans(t *testing.T) {
	// given: NDJSON stream with tool_use inside assistant message + tool_result
	input := strings.Join([]string{
		`{"type":"assistant","session_id":"s1","message":{"id":"msg_1","role":"assistant","model":"claude-opus-4-6","content":[{"type":"tool_use","name":"Read","id":"toolu_01","input":{"file_path":"/tmp/x"}}]}}`,
		`{"type":"tool_result","tool_use_id":"toolu_01"}`,
		`{"type":"result","subtype":"success","session_id":"s1","result":"done","usage":{"input_tokens":100,"output_tokens":50},"stop_reason":"end_turn"}`,
	}, "\n")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	// when
	result, messages, err := emitter.CollectAll()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(messages) != 3 {
		t.Errorf("got %d messages, want 3", len(messages))
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()
	var toolSpans []tracetest.SpanStub
	for _, s := range spans {
		if strings.HasPrefix(s.Name, "execute_tool") {
			toolSpans = append(toolSpans, s)
		}
	}
	if len(toolSpans) != 1 {
		t.Fatalf("got %d tool spans, want 1; all spans: %v", len(toolSpans), spanNames(spans))
	}
	if toolSpans[0].Name != "execute_tool Read" {
		t.Errorf("span name = %q, want 'execute_tool Read'", toolSpans[0].Name)
	}

	// Verify raw events
	rawEvents := emitter.RawEvents()
	if len(rawEvents) == 0 {
		t.Error("expected non-empty raw events")
	}
}

func TestSpanEmittingStreamReader_synthetic_id_fallback(t *testing.T) {
	// given: tool_use with no ID in content block
	input := strings.Join([]string{
		`{"type":"assistant","session_id":"s1","message":{"content":[{"type":"tool_use","name":"Bash","input":{}}]}}`,
		`{"type":"result","result":"done","usage":{"input_tokens":10,"output_tokens":5}}`,
	}, "\n")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	// when
	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()
	var toolSpans []tracetest.SpanStub
	for _, s := range spans {
		if strings.HasPrefix(s.Name, "execute_tool") {
			toolSpans = append(toolSpans, s)
		}
	}
	if len(toolSpans) != 1 {
		t.Fatalf("got %d tool spans, want 1", len(toolSpans))
	}

	found := false
	for _, attr := range toolSpans[0].Attributes {
		if attr.Key == "gen_ai.tool.call.id" && strings.HasPrefix(attr.Value.AsString(), "synthetic-") {
			found = true
		}
	}
	if !found {
		t.Error("expected synthetic tool ID attribute")
	}
}

func TestSpanEmittingStreamReader_preserves_result_and_text(t *testing.T) {
	// given
	input := strings.Join([]string{
		`{"type":"assistant","session_id":"s1","message":{"id":"msg_1","role":"assistant","model":"claude-opus-4-6","content":[{"type":"text","text":"I will read the file."}]}}`,
		`{"type":"assistant","session_id":"s1","message":{"content":[{"type":"tool_use","name":"Read","id":"toolu_01","input":{"file_path":"/tmp/x"}}]}}`,
		`{"type":"tool_result","tool_use_id":"toolu_01"}`,
		`{"type":"assistant","session_id":"s1","message":{"content":[{"type":"text","text":"Here is the result."}]}}`,
		`{"type":"result","subtype":"success","session_id":"s1","result":"Here is the result.","usage":{"input_tokens":200,"output_tokens":100},"stop_reason":"end_turn"}`,
	}, "\n")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	// when
	result, messages, err := emitter.CollectAll()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Result != "Here is the result." {
		t.Errorf("result = %v, want 'Here is the result.'", result)
	}
	if len(messages) != 5 {
		t.Errorf("got %d messages, want 5", len(messages))
	}

	text, _ := messages[0].ExtractText()
	if text != "I will read the file." {
		t.Errorf("text = %q, want 'I will read the file.'", text)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	rawEvents := emitter.RawEvents()
	if len(rawEvents) != 5 {
		t.Errorf("got %d raw events, want 5", len(rawEvents))
	}
}

// spanNames is a test helper that returns span names for debugging.
func spanNames(spans []tracetest.SpanStub) []string {
	names := make([]string, len(spans))
	for i, s := range spans {
		names[i] = s.Name
	}
	return names
}
