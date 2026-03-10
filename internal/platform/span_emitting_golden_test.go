package platform_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/platform"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// ---------------------------------------------------------------------------
// Golden file scenario tests: real Claude CLI stream-json replayed through
// SpanEmittingStreamReader. These validate that the OTel mapping matches the
// actual wire format produced by `claude --output-format stream-json`.
// ---------------------------------------------------------------------------

// loadGolden reads a golden JSONL file from testdata/.
func loadGolden(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("failed to read golden file %s: %v", name, err)
	}
	return string(data)
}

func TestGolden_hook_id_based_keying(t *testing.T) {
	// The golden file has 4 hook_started with the SAME hook_name but different
	// hook_ids. hook_responses arrive in a different order. Each hook must
	// produce its own child span, keyed by hook_id (not hook_name).
	input := loadGolden(t, "golden_simple.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()
	var hookSpans []tracetest.SpanStub
	for _, s := range spans {
		if strings.HasPrefix(s.Name, "hook ") {
			hookSpans = append(hookSpans, s)
		}
	}

	// Must have exactly 4 hook spans (one per hook_id), not 2 (one per unique hook_name).
	if len(hookSpans) != 4 {
		t.Fatalf("got %d hook spans, want 4 (one per hook_id); names: %v", len(hookSpans), spanNames(spans))
	}

	// Verify both hook names are represented.
	nameCount := make(map[string]int)
	for _, s := range hookSpans {
		nameCount[s.Name]++
	}
	if nameCount["hook SessionStart:startup"] != 3 {
		t.Errorf("want 3 spans named 'hook SessionStart:startup', got %d", nameCount["hook SessionStart:startup"])
	}
	if nameCount["hook SessionStart:compact"] != 1 {
		t.Errorf("want 1 span named 'hook SessionStart:compact', got %d", nameCount["hook SessionStart:compact"])
	}
}

func TestGolden_hook_exit_code_on_span(t *testing.T) {
	// hook-ccc has exit_code=1. The span attribute hook.exit_code must be 1.
	input := loadGolden(t, "golden_simple.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()

	// Find a hook span with exit_code=1
	foundFailure := false
	for _, s := range spans {
		if !strings.HasPrefix(s.Name, "hook ") {
			continue
		}
		for _, attr := range s.Attributes {
			if string(attr.Key) == "hook.exit_code" && attr.Value.AsInt64() == 1 {
				foundFailure = true
			}
		}
	}
	if !foundFailure {
		t.Error("expected at least one hook span with hook.exit_code=1")
	}
}

func TestGolden_hook_event_attribute(t *testing.T) {
	// Each hook span should have a hook.event attribute from the hook_event field.
	input := loadGolden(t, "golden_simple.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()
	for _, s := range spans {
		if !strings.HasPrefix(s.Name, "hook ") {
			continue
		}
		found := false
		for _, attr := range s.Attributes {
			if string(attr.Key) == "hook.event" && attr.Value.AsString() == "SessionStart" {
				found = true
			}
		}
		if !found {
			t.Errorf("hook span %q missing hook.event=SessionStart attribute", s.Name)
		}
	}
}

func TestGolden_thinking_event_from_real_stream(t *testing.T) {
	input := loadGolden(t, "golden_simple.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()
	var parentStub tracetest.SpanStub
	for _, s := range spans {
		if s.Name == "claude.invoke" {
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
		t.Error("expected gen_ai.thinking event on parent span from golden file")
	}
}

func TestGolden_rate_limit_event_with_info(t *testing.T) {
	// rate_limit event should exist, and carry rate_limit_info attributes.
	input := loadGolden(t, "golden_simple.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()
	var parentStub tracetest.SpanStub
	for _, s := range spans {
		if s.Name == "claude.invoke" {
			parentStub = s
			break
		}
	}

	// Find rate_limit event with status attribute
	foundEvent := false
	foundStatus := false
	for _, ev := range parentStub.Events {
		if ev.Name == "rate_limit" {
			foundEvent = true
			for _, attr := range ev.Attributes {
				if string(attr.Key) == "rate_limit.status" {
					foundStatus = true
					if attr.Value.AsString() != "allowed_warning" {
						t.Errorf("rate_limit.status = %q, want %q", attr.Value.AsString(), "allowed_warning")
					}
				}
			}
		}
	}
	if !foundEvent {
		t.Error("expected rate_limit event on parent span")
	}
	if !foundStatus {
		t.Error("expected rate_limit.status attribute on rate_limit event")
	}
}

func TestGolden_session_id_captured(t *testing.T) {
	input := loadGolden(t, "golden_simple.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	attrs := emitter.WeaveThreadAttrs()
	if len(attrs) == 0 {
		t.Fatal("expected Weave thread attrs from golden session_id")
	}
	foundThreadID := false
	for _, attr := range attrs {
		if string(attr.Key) == "wandb.thread_id" && attr.Value.AsString() == "sess-golden-01" {
			foundThreadID = true
		}
	}
	if !foundThreadID {
		t.Error("expected wandb.thread_id = sess-golden-01")
	}
}

// ---------------------------------------------------------------------------
// MCP golden file tests: golden_mcp.jsonl
// Exercises mixed hook_event types and rate_limit without warning.
// ---------------------------------------------------------------------------

func TestGoldenMCP_mixed_hook_events(t *testing.T) {
	// golden_mcp.jsonl has 2 SessionStart hooks + 1 UserPromptSubmit hook.
	// Each should produce its own span with correct hook.event attribute.
	input := loadGolden(t, "golden_mcp.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()
	var hookSpans []tracetest.SpanStub
	for _, s := range spans {
		if strings.HasPrefix(s.Name, "hook ") {
			hookSpans = append(hookSpans, s)
		}
	}

	if len(hookSpans) != 3 {
		t.Fatalf("got %d hook spans, want 3; names: %v", len(hookSpans), spanNames(spans))
	}

	// Count hook events
	eventCount := make(map[string]int)
	for _, s := range hookSpans {
		for _, attr := range s.Attributes {
			if string(attr.Key) == "hook.event" {
				eventCount[attr.Value.AsString()]++
			}
		}
	}
	if eventCount["SessionStart"] != 2 {
		t.Errorf("want 2 SessionStart hook events, got %d", eventCount["SessionStart"])
	}
	if eventCount["UserPromptSubmit"] != 1 {
		t.Errorf("want 1 UserPromptSubmit hook event, got %d", eventCount["UserPromptSubmit"])
	}
}

func TestGoldenMCP_rate_limit_allowed_no_warning(t *testing.T) {
	// golden_mcp.jsonl has rate_limit status="allowed" (not a warning).
	input := loadGolden(t, "golden_mcp.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()
	var parentStub tracetest.SpanStub
	for _, s := range spans {
		if s.Name == "claude.invoke" {
			parentStub = s
			break
		}
	}

	for _, ev := range parentStub.Events {
		if ev.Name == "rate_limit" {
			for _, attr := range ev.Attributes {
				if string(attr.Key) == "rate_limit.status" && attr.Value.AsString() != "allowed" {
					t.Errorf("rate_limit.status = %q, want %q", attr.Value.AsString(), "allowed")
				}
			}
			return
		}
	}
	t.Error("expected rate_limit event on parent span")
}

func TestGoldenMCP_thinking_and_text_both_present(t *testing.T) {
	// golden_mcp.jsonl has a thinking block followed by a text block.
	// Thinking should become a parent span event; text should be extractable.
	input := loadGolden(t, "golden_mcp.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, messages, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	// Verify thinking event on parent span
	spans := exporter.GetSpans()
	var parentStub tracetest.SpanStub
	for _, s := range spans {
		if s.Name == "claude.invoke" {
			parentStub = s
			break
		}
	}
	foundThinking := false
	for _, ev := range parentStub.Events {
		if ev.Name == "gen_ai.thinking" {
			foundThinking = true
		}
	}
	if !foundThinking {
		t.Error("expected gen_ai.thinking event")
	}

	// Verify text is extractable from assistant messages
	foundText := false
	for _, msg := range messages {
		if msg.Type == "assistant" {
			text, _ := msg.ExtractText()
			if strings.Contains(text, "3 MCP servers") {
				foundText = true
			}
		}
	}
	if !foundText {
		t.Error("expected assistant text containing '3 MCP servers'")
	}
}

func TestGoldenMCP_weave_session_id(t *testing.T) {
	input := loadGolden(t, "golden_mcp.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	attrs := emitter.WeaveThreadAttrs()
	foundThreadID := false
	for _, attr := range attrs {
		if string(attr.Key) == "wandb.thread_id" && attr.Value.AsString() == "sess-golden-mcp" {
			foundThreadID = true
		}
	}
	if !foundThreadID {
		t.Error("expected wandb.thread_id = sess-golden-mcp")
	}
}

// ---------------------------------------------------------------------------
// Skills golden file tests: golden_skills.jsonl
// All 4 hooks share the same name, responses arrive in reverse order.
// High utilization rate_limit_event.
// ---------------------------------------------------------------------------

func TestGoldenSkills_four_same_name_hooks_reverse_order(t *testing.T) {
	// All 4 hook_started share "SessionStart:startup", responses arrive reversed.
	// Each must produce its own span via hook_id keying.
	input := loadGolden(t, "golden_skills.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()
	var hookSpans []tracetest.SpanStub
	for _, s := range spans {
		if strings.HasPrefix(s.Name, "hook ") {
			hookSpans = append(hookSpans, s)
		}
	}

	if len(hookSpans) != 4 {
		t.Fatalf("got %d hook spans, want 4; names: %v", len(hookSpans), spanNames(spans))
	}

	// All should have the same span name
	for _, s := range hookSpans {
		if s.Name != "hook SessionStart:startup" {
			t.Errorf("span name = %q, want %q", s.Name, "hook SessionStart:startup")
		}
	}

	// Each should have a unique hook.id attribute
	hookIDs := make(map[string]bool)
	for _, s := range hookSpans {
		for _, attr := range s.Attributes {
			if string(attr.Key) == "hook.id" {
				hookIDs[attr.Value.AsString()] = true
			}
		}
	}
	if len(hookIDs) != 4 {
		t.Errorf("got %d unique hook.id values, want 4", len(hookIDs))
	}
}

func TestGoldenSkills_high_utilization_rate_limit(t *testing.T) {
	input := loadGolden(t, "golden_skills.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()
	var parentStub tracetest.SpanStub
	for _, s := range spans {
		if s.Name == "claude.invoke" {
			parentStub = s
			break
		}
	}

	for _, ev := range parentStub.Events {
		if ev.Name == "rate_limit" {
			for _, attr := range ev.Attributes {
				if string(attr.Key) == "rate_limit.utilization" {
					if attr.Value.AsString() != "0.92" {
						t.Errorf("rate_limit.utilization = %q, want %q", attr.Value.AsString(), "0.92")
					}
					return
				}
			}
			t.Error("rate_limit event missing utilization attribute")
			return
		}
	}
	t.Error("expected rate_limit event on parent span")
}

func TestGoldenSkills_result_text(t *testing.T) {
	input := loadGolden(t, "golden_skills.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	result, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	if result == nil {
		t.Fatal("expected result message")
	}
	if !strings.Contains(result.Result, "10 skills") {
		t.Errorf("result = %q, want to contain '10 skills'", result.Result)
	}
}

// ---------------------------------------------------------------------------
// Init metadata tests: system:init attributes on parent span
// ---------------------------------------------------------------------------

func TestGolden_init_model_on_parent_span(t *testing.T) {
	input := loadGolden(t, "golden_simple.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	initAttrs := emitter.InitAttrs()
	foundModel := false
	for _, attr := range initAttrs {
		if string(attr.Key) == "claude.init.model" && attr.Value.AsString() == "claude-opus-4-6" {
			foundModel = true
		}
	}
	if !foundModel {
		t.Error("expected claude.init.model = claude-opus-4-6")
	}
}

func TestGoldenMCP_init_mcp_servers(t *testing.T) {
	input := loadGolden(t, "golden_mcp.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	initAttrs := emitter.InitAttrs()

	// Should have mcp_server names
	foundMCP := false
	for _, attr := range initAttrs {
		if string(attr.Key) == "claude.init.mcp_servers" {
			foundMCP = true
			servers := attr.Value.AsStringSlice()
			if len(servers) != 3 {
				t.Errorf("mcp_servers count = %d, want 3", len(servers))
			}
		}
	}
	if !foundMCP {
		t.Error("expected claude.init.mcp_servers attribute")
	}
}

func TestGoldenSkills_init_counts(t *testing.T) {
	input := loadGolden(t, "golden_skills.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	initAttrs := emitter.InitAttrs()

	checks := map[string]int64{
		"claude.init.tools_count":   8,
		"claude.init.skills_count":  10,
		"claude.init.plugins_count": 4,
	}
	for key, want := range checks {
		found := false
		for _, attr := range initAttrs {
			if string(attr.Key) == key {
				found = true
				if attr.Value.AsInt64() != want {
					t.Errorf("%s = %d, want %d", key, attr.Value.AsInt64(), want)
				}
			}
		}
		if !found {
			t.Errorf("expected %s attribute", key)
		}
	}
}

// ---------------------------------------------------------------------------
// hook_response outcome/stderr tests
// ---------------------------------------------------------------------------

func TestGolden_hook_outcome_attribute(t *testing.T) {
	input := loadGolden(t, "golden_simple.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()
	foundFailureOutcome := false
	for _, s := range spans {
		if !strings.HasPrefix(s.Name, "hook ") {
			continue
		}
		for _, attr := range s.Attributes {
			if string(attr.Key) == "hook.outcome" && attr.Value.AsString() == "failure" {
				foundFailureOutcome = true
			}
		}
	}
	if !foundFailureOutcome {
		t.Error("expected at least one hook span with hook.outcome=failure")
	}
}

func TestGolden_result_captures_output(t *testing.T) {
	input := loadGolden(t, "golden_simple.jsonl")

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, parentSpan := tracer.Start(context.Background(), "claude.invoke") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, End() called explicitly [permanent]
	sr := platform.NewStreamReader(strings.NewReader(input))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, tracer)
	emitter.SetInput("1+1=")

	result, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parentSpan.End()
	tp.ForceFlush(context.Background())

	if result == nil {
		t.Fatal("expected result message")
	}
	if result.Result != "2" {
		t.Errorf("result = %q, want %q", result.Result, "2")
	}

	ioAttrs := emitter.WeaveIOAttrs()
	foundInput := false
	foundOutput := false
	for _, attr := range ioAttrs {
		if string(attr.Key) == "input.value" && attr.Value.AsString() == "1+1=" {
			foundInput = true
		}
		if string(attr.Key) == "output.value" && attr.Value.AsString() == "2" {
			foundOutput = true
		}
	}
	if !foundInput {
		t.Error("expected input.value = '1+1='")
	}
	if !foundOutput {
		t.Error("expected output.value = '2'")
	}
}
