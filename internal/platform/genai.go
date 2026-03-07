package platform

import "go.opentelemetry.io/otel/attribute"

// GenAI semantic convention attribute keys.
// See: https://opentelemetry.io/docs/specs/semconv/gen-ai/
const (
	GenAIOperationName = attribute.Key("gen_ai.operation.name")
	GenAISystem        = attribute.Key("gen_ai.system")
	GenAIRequestModel  = attribute.Key("gen_ai.request.model")
	GenAIProviderName  = attribute.Key("gen_ai.provider.name")
	GenAIResponseModel = attribute.Key("gen_ai.response.model")
	GenAIResponseID    = attribute.Key("gen_ai.response.id")
	GenAIFinishReasons = attribute.Key("gen_ai.response.finish_reasons")
	GenAIInputTokens   = attribute.Key("gen_ai.usage.input_tokens")
	GenAIOutputTokens  = attribute.Key("gen_ai.usage.output_tokens")
	GenAICacheCreate   = attribute.Key("gen_ai.usage.cache_creation.input_tokens")
	GenAICacheRead     = attribute.Key("gen_ai.usage.cache_read.input_tokens")
	GenAIToolName      = attribute.Key("gen_ai.tool.name")
	GenAIToolCallID    = attribute.Key("gen_ai.tool.call.id")
	GenAIToolType      = attribute.Key("gen_ai.tool.type")
)

// GenAISpanAttrs returns the standard GenAI semantic convention attributes
// for an Anthropic Claude invocation.
func GenAISpanAttrs(model string) []attribute.KeyValue {
	return []attribute.KeyValue{
		GenAIOperationName.String("chat"),
		GenAISystem.String("anthropic"),
		GenAIProviderName.String("anthropic"),
		GenAIRequestModel.String(model),
	}
}

// GenAIResultAttrs returns span attributes from a stream-json result message.
func GenAIResultAttrs(result *StreamMessage, responseModel, responseID string) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	if responseModel != "" {
		attrs = append(attrs, GenAIResponseModel.String(responseModel))
	}
	if responseID != "" {
		attrs = append(attrs, GenAIResponseID.String(responseID))
	}
	if result.StopReason != "" {
		attrs = append(attrs, GenAIFinishReasons.String(result.StopReason))
	}
	if result.Usage != nil {
		attrs = append(attrs, GenAIInputTokens.Int(result.Usage.InputTokens))
		attrs = append(attrs, GenAIOutputTokens.Int(result.Usage.OutputTokens))
		if result.Usage.CacheCreationInputTokens > 0 {
			attrs = append(attrs, GenAICacheCreate.Int(result.Usage.CacheCreationInputTokens))
		}
		if result.Usage.CacheReadInputTokens > 0 {
			attrs = append(attrs, GenAICacheRead.Int(result.Usage.CacheReadInputTokens))
		}
	}
	return attrs
}

// GenAIToolAttrs returns span attributes for a tool_use child span.
func GenAIToolAttrs(toolName, callID string) []attribute.KeyValue {
	return []attribute.KeyValue{
		GenAIOperationName.String("execute_tool"),
		GenAIToolName.String(toolName),
		GenAIToolCallID.String(callID),
		GenAIToolType.String("function"),
	}
}
