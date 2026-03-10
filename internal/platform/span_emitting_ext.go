package platform

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ---------------------------------------------------------------------------
// Extended span emission: hooks, thinking, rate_limit
// This file extends SpanEmittingStreamReader with additional OTel mappings
// beyond core tool_use handling.
// ---------------------------------------------------------------------------

// handleExtMessage dispatches extended message types that are not covered by
// the core tool_use / tool_result handlers. Returns true if the message was
// handled by an ext handler.
func (s *SpanEmittingStreamReader) handleExtMessage(msg *StreamMessage) bool {
	switch {
	case msg.Type == "system" && msg.Subtype == "hook_started":
		s.handleHookStarted(msg)
		return true
	case msg.Type == "system" && msg.Subtype == "hook_response":
		s.handleHookResponse(msg)
		return true
	case msg.Type == "assistant":
		return s.handleThinkingBlocks(msg)
	case msg.Type == "rate_limit_event":
		s.handleRateLimit(msg)
		return true
	default:
		return false
	}
}

// handleHookStarted creates a child span for a hook execution.
func (s *SpanEmittingStreamReader) handleHookStarted(msg *StreamMessage) {
	hookName := msg.HookName
	if hookName == "" {
		hookName = "unknown"
	}
	spanName := "hook " + hookName
	attrs := []attribute.KeyValue{
		attribute.String("hook.name", hookName),
	}
	if msg.Command != "" {
		attrs = append(attrs, attribute.String("hook.command", msg.Command))
	}
	if s.sessionID != "" {
		attrs = append(attrs, WeaveThreadNestedAttrs(s.sessionID)...)
	}

	_, hookSpan := s.tracer.Start(s.parentCtx, spanName, // nosemgrep: adr0003-otel-span-without-defer-end -- map-managed spans, End() in handleHookResponse/endAllOpenSpans [permanent]
		trace.WithAttributes(attrs...),
	)
	s.openSpans["hook:"+hookName] = hookSpan
}

// handleHookResponse ends the child span for a hook execution.
func (s *SpanEmittingStreamReader) handleHookResponse(msg *StreamMessage) {
	hookName := msg.HookName
	if hookName == "" {
		hookName = "unknown"
	}
	key := "hook:" + hookName
	span, ok := s.openSpans[key]
	if !ok {
		return // orphan hook_response — no matching hook_started
	}
	if msg.ExitCode != nil {
		span.SetAttributes(attribute.Int("hook.exit_code", *msg.ExitCode))
	}
	span.End()
	delete(s.openSpans, key)
}

// handleThinkingBlocks adds gen_ai.thinking events to the parent span
// for each thinking content block found in an assistant message.
// Returns true if at least one thinking block was processed.
func (s *SpanEmittingStreamReader) handleThinkingBlocks(msg *StreamMessage) bool {
	am, err := msg.ParseAssistantMessage()
	if err != nil || am == nil {
		return false
	}
	found := false
	parentSpan := trace.SpanFromContext(s.parentCtx)
	for _, block := range am.Content {
		if block.Type == "thinking" {
			parentSpan.AddEvent("gen_ai.thinking",
				trace.WithAttributes(
					attribute.String("gen_ai.thinking.text", TruncateValue(block.Thinking, s.maxValueLen)),
				),
			)
			found = true
		}
	}
	return found
}

// handleRateLimit adds a rate_limit event to the parent span.
func (s *SpanEmittingStreamReader) handleRateLimit(msg *StreamMessage) {
	parentSpan := trace.SpanFromContext(s.parentCtx)
	parentSpan.AddEvent("rate_limit")
}
