package platform

import "go.opentelemetry.io/otel/attribute"

// GenAI semantic convention attribute keys.
// See: https://opentelemetry.io/docs/specs/semconv/gen-ai/
const (
	GenAIOperationName = attribute.Key("gen_ai.operation.name")
	GenAISystem        = attribute.Key("gen_ai.system")
	GenAIRequestModel  = attribute.Key("gen_ai.request.model")
)

// GenAISpanAttrs returns the standard GenAI semantic convention attributes
// for an Anthropic Claude invocation.
func GenAISpanAttrs(model string) []attribute.KeyValue {
	return []attribute.KeyValue{
		GenAIOperationName.String("chat"),
		GenAISystem.String("anthropic"),
		GenAIRequestModel.String(model),
	}
}
