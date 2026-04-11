package platform_test

import (
	"context"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/platform"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// ---------------------------------------------------------------------------
// Extended span emission tests: hooks, thinking, rate_limit
// These test the "ext" layer added to SpanEmittingStreamReader.
// ---------------------------------------------------------------------------

func TestExt_hook_started_and_response_create_child_span(t *testing.T) {
	// given: stream with hook_started → hook_response pair
	input := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"sess-hook"}`,
		`{"type":"system","subtype":"hook_started","session_id":"sess-hook","hook_name":"UserPromptSubmit","command":"echo hello"}`,
		`{"type":"system","subtype":"hook_response","session_id":"sess-hook","hook_name":"UserPromptSubmit","exit_code":0,"stdout":"hello\n"}`,
		`{"type":"result","subtype":"success","session_id":"sess-hook","result":"done","usage":{"input_tokens":10,"output_tokens":5},"stop_reason":"end_turn"}`,
	}, "\n")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "provider.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	// when
	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	// then: a child span "hook UserPromptSubmit" should exist
	spans := exporter.GetSpans()
	var hookSpans []tracetest.SpanStub
	for _, s := range spans {
		if strings.HasPrefix(s.Name, "hook ") {
			hookSpans = append(hookSpans, s)
		}
	}
	if len(hookSpans) != 1 {
		t.Fatalf("got %d hook spans, want 1; all spans: %v", len(hookSpans), spanNames(spans))
	}
	if hookSpans[0].Name != "hook UserPromptSubmit" {
		t.Errorf("span name = %q, want %q", hookSpans[0].Name, "hook UserPromptSubmit")
	}
}

func TestExt_thinking_adds_event_to_parent(t *testing.T) {
	// given: stream with thinking content block
	input := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"sess-think"}`,
		`{"type":"assistant","session_id":"sess-think","message":{"content":[{"type":"thinking","thinking":"Let me analyze this carefully.","signature":"sig-1"}]}}`,
		`{"type":"result","subtype":"success","session_id":"sess-think","result":"done","usage":{"input_tokens":10,"output_tokens":5},"stop_reason":"end_turn"}`,
	}, "\n")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "provider.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	// when
	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	// then: parent span should have a "gen_ai.thinking" event
	spans := exporter.GetSpans()
	var parentStub tracetest.SpanStub
	for _, s := range spans {
		if s.Name == "provider.invoke" {
			parentStub = s
			break
		}
	}

	found := false
	for _, ev := range parentStub.Events {
		if ev.Name == "gen_ai.thinking" {
			found = true
		}
	}
	if !found {
		t.Error("expected gen_ai.thinking event on parent span")
	}
}

func TestExt_rate_limit_adds_event_to_parent(t *testing.T) {
	// given: stream with rate_limit_event
	input := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"sess-rl"}`,
		`{"type":"rate_limit_event","session_id":"sess-rl"}`,
		`{"type":"result","subtype":"success","session_id":"sess-rl","result":"done","usage":{"input_tokens":10,"output_tokens":5},"stop_reason":"end_turn"}`,
	}, "\n")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "provider.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	// when
	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	// then: parent span should have a "rate_limit" event
	spans := exporter.GetSpans()
	var parentStub tracetest.SpanStub
	for _, s := range spans {
		if s.Name == "provider.invoke" {
			parentStub = s
			break
		}
	}

	found := false
	for _, ev := range parentStub.Events {
		if ev.Name == "rate_limit" {
			found = true
		}
	}
	if !found {
		t.Error("expected rate_limit event on parent span")
	}
}

func TestExt_orphan_hook_response_does_not_panic(t *testing.T) {
	// given: hook_response without matching hook_started
	input := strings.Join([]string{
		`{"type":"system","subtype":"hook_response","session_id":"sess-1","hook_name":"Orphan","exit_code":0}`,
		`{"type":"result","subtype":"success","session_id":"sess-1","result":"done","usage":{"input_tokens":10,"output_tokens":5},"stop_reason":"end_turn"}`,
	}, "\n")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "provider.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	// when: should not panic
	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())
}
