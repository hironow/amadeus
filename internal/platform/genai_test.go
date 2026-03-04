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
	if len(attrs) != 3 {
		t.Fatalf("attribute count: got %d, want 3", len(attrs))
	}

	expected := []attribute.KeyValue{
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.system", "anthropic"),
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
