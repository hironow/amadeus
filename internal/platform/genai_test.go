package platform_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/platform"
	"go.opentelemetry.io/otel/attribute"
)

func TestGenAISpanAttrs_ReturnsCorrectAttributes(t *testing.T) {
	// given
	model := "claude-opus-4-6"

	// when
	attrs := platform.GenAISpanAttrs(model)

	// then
	if len(attrs) != 4 {
		t.Fatalf("attribute count: got %d, want 4", len(attrs))
	}

	expected := []attribute.KeyValue{
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.system", "anthropic"),
		attribute.String("gen_ai.provider.name", "anthropic"),
		attribute.String("gen_ai.request.model", "claude-opus-4-6"),
	}
	for i, want := range expected {
		got := attrs[i]
		if got.Key != want.Key {
			t.Errorf("attrs[%d].Key: got %q, want %q", i, got.Key, want.Key)
		}
		if got.Value.AsString() != want.Value.AsString() {
			t.Errorf("attrs[%d].Value: got %q, want %q", i, got.Value.AsString(), want.Value.AsString())
		}
	}
}

func TestGenAIResultAttrs_includes_usage(t *testing.T) {
	// given
	usage := &platform.Usage{InputTokens: 5000, OutputTokens: 2000, CacheCreationInputTokens: 100, CacheReadInputTokens: 800}
	result := &platform.StreamMessage{
		Type:       "result",
		StopReason: "end_turn",
		TotalCost:  0.15,
		Usage:      usage,
	}

	// when
	attrs := platform.GenAIResultAttrs(result, "claude-opus-4-6", "msg_123")
	attrMap := make(map[string]any)
	for _, a := range attrs {
		attrMap[string(a.Key)] = a.Value.AsInterface()
	}

	// then
	if v, ok := attrMap["gen_ai.usage.input_tokens"]; !ok || v.(int64) != 5000 {
		t.Errorf("input_tokens = %v, want 5000", v)
	}
	if v, ok := attrMap["gen_ai.usage.output_tokens"]; !ok || v.(int64) != 2000 {
		t.Errorf("output_tokens = %v, want 2000", v)
	}
	if v, ok := attrMap["gen_ai.response.model"]; !ok || v.(string) != "claude-opus-4-6" {
		t.Errorf("response.model = %v, want claude-opus-4-6", v)
	}
	if v, ok := attrMap["gen_ai.response.id"]; !ok || v.(string) != "msg_123" {
		t.Errorf("response.id = %v, want msg_123", v)
	}
}

func TestGenAIToolAttrs(t *testing.T) {
	// given / when
	attrs := platform.GenAIToolAttrs("Read", "toolu_123")
	attrMap := make(map[string]any)
	for _, a := range attrs {
		attrMap[string(a.Key)] = a.Value.AsInterface()
	}

	// then
	if v := attrMap["gen_ai.operation.name"]; v != "execute_tool" {
		t.Errorf("operation.name = %v, want execute_tool", v)
	}
	if v := attrMap["gen_ai.tool.name"]; v != "Read" {
		t.Errorf("tool.name = %v, want Read", v)
	}
}
