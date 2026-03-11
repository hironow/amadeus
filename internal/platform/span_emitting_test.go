package platform_test

import (
	"context"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/platform"
	"go.opentelemetry.io/otel/attribute"
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

// findAttr returns the attribute value for the given key, or nil if not found.
func findAttr(span tracetest.SpanStub, key attribute.Key) *attribute.Value {
	for _, a := range span.Attributes {
		if a.Key == key {
			v := a.Value
			return &v
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Weave thread attribute tests (TDD Red)
// ref: Weave thread organization design doc
// ---------------------------------------------------------------------------

func TestSpanEmittingStreamReader_weave_thread_id_from_session(t *testing.T) {
	// given: NDJSON stream with session_id="sess-abc-123"
	input := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"sess-abc-123"}`,
		`{"type":"assistant","session_id":"sess-abc-123","message":{"id":"msg_1","role":"assistant","model":"claude-opus-4-6","content":[{"type":"text","text":"hello"}]}}`,
		`{"type":"result","subtype":"success","session_id":"sess-abc-123","result":"hello","usage":{"input_tokens":10,"output_tokens":5},"stop_reason":"end_turn","num_turns":1}`,
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

	// then: emitter must expose WeaveThreadAttrs so the caller can set them
	// on the parent span. The thread_id must come from the session_id.
	threadAttrs := emitter.WeaveThreadAttrs()

	gotThreadID := ""
	gotIsTurn := false
	for _, kv := range threadAttrs {
		if kv.Key == "wandb.thread_id" {
			gotThreadID = kv.Value.AsString()
		}
		if kv.Key == "wandb.is_turn" {
			gotIsTurn = kv.Value.AsBool()
		}
	}

	if gotThreadID != "sess-abc-123" {
		t.Errorf("wandb.thread_id = %q, want %q", gotThreadID, "sess-abc-123")
	}
	if !gotIsTurn {
		t.Error("wandb.is_turn should be true for the parent (turn) span")
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())
}

func TestSpanEmittingStreamReader_tool_spans_have_thread_id_no_turn(t *testing.T) {
	// given: stream with tool_use in a session
	input := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"sess-tool-test"}`,
		`{"type":"assistant","session_id":"sess-tool-test","message":{"content":[{"type":"tool_use","name":"Read","id":"toolu_01","input":{"file_path":"/tmp/x"}}]}}`,
		`{"type":"tool_result","tool_use_id":"toolu_01"}`,
		`{"type":"result","subtype":"success","session_id":"sess-tool-test","result":"done","usage":{"input_tokens":100,"output_tokens":50},"stop_reason":"end_turn","num_turns":1}`,
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

	// then: tool child spans must have wandb.thread_id but NOT wandb.is_turn=true
	spans := exporter.GetSpans()
	for _, s := range spans {
		if !strings.HasPrefix(s.Name, "execute_tool") {
			continue
		}

		threadID := findAttr(s, "wandb.thread_id")
		if threadID == nil {
			t.Errorf("tool span %q missing wandb.thread_id", s.Name)
		} else if threadID.AsString() != "sess-tool-test" {
			t.Errorf("tool span %q wandb.thread_id = %q, want %q", s.Name, threadID.AsString(), "sess-tool-test")
		}

		isTurn := findAttr(s, "wandb.is_turn")
		if isTurn != nil && isTurn.AsBool() {
			t.Errorf("tool span %q must NOT have wandb.is_turn=true (nested spans are not turns)", s.Name)
		}
	}
}

func TestSpanEmittingStreamReader_weave_io_attrs(t *testing.T) {
	// given: stream with text content and result
	input := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"sess-io"}`,
		`{"type":"assistant","session_id":"sess-io","message":{"id":"msg_1","role":"assistant","model":"claude-opus-4-6","content":[{"type":"text","text":"The answer is 42."}]}}`,
		`{"type":"result","subtype":"success","session_id":"sess-io","result":"The answer is 42.","usage":{"input_tokens":10,"output_tokens":5},"stop_reason":"end_turn","num_turns":1}`,
	}, "\n")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)
	emitter.SetInput("What is the meaning of life?")

	// when
	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// then: WeaveIOAttrs should contain both input.value and output.value
	ioAttrs := emitter.WeaveIOAttrs()

	gotInput := ""
	gotOutput := ""
	for _, kv := range ioAttrs {
		if kv.Key == "input.value" {
			gotInput = kv.Value.AsString()
		}
		if kv.Key == "output.value" {
			gotOutput = kv.Value.AsString()
		}
	}

	if gotInput != "What is the meaning of life?" {
		t.Errorf("input.value = %q, want %q", gotInput, "What is the meaning of life?")
	}
	if gotOutput != "The answer is 42." {
		t.Errorf("output.value = %q, want %q", gotOutput, "The answer is 42.")
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())
}

func TestGenAIWeaveThreadAttrs(t *testing.T) {
	// given
	threadID := "thread-test-456"

	// when: create Weave thread attributes for a turn span
	attrs := platform.WeaveThreadTurnAttrs(threadID)

	// then
	gotThreadID := ""
	gotIsTurn := false
	for _, kv := range attrs {
		if kv.Key == "wandb.thread_id" {
			gotThreadID = kv.Value.AsString()
		}
		if kv.Key == "wandb.is_turn" {
			gotIsTurn = kv.Value.AsBool()
		}
	}

	if gotThreadID != threadID {
		t.Errorf("wandb.thread_id = %q, want %q", gotThreadID, threadID)
	}
	if !gotIsTurn {
		t.Error("wandb.is_turn should be true for turn attrs")
	}
}

func TestGenAIWeaveThreadNestedAttrs(t *testing.T) {
	// given
	threadID := "thread-test-456"

	// when: create Weave thread attributes for a nested (non-turn) span
	attrs := platform.WeaveThreadNestedAttrs(threadID)

	// then
	gotThreadID := ""
	for _, kv := range attrs {
		if kv.Key == "wandb.thread_id" {
			gotThreadID = kv.Value.AsString()
		}
		if kv.Key == "wandb.is_turn" {
			t.Error("nested attrs should NOT include wandb.is_turn")
		}
	}

	if gotThreadID != threadID {
		t.Errorf("wandb.thread_id = %q, want %q", gotThreadID, threadID)
	}
}
