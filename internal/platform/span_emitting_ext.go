package platform

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ---------------------------------------------------------------------------
// Extended span emission: hooks, thinking, rate_limit, init
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
	case msg.Type == "system" && msg.Subtype == "init":
		s.handleInit(msg)
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

// hookSpanKey returns the openSpans map key for a hook. Prefers hook_id
// (unique per hook invocation) over hook_name (which can collide when
// multiple hooks share the same name).
func hookSpanKey(msg *StreamMessage) string {
	if msg.HookID != "" {
		return "hook:" + msg.HookID
	}
	name := msg.HookName
	if name == "" {
		name = "unknown"
	}
	return "hook:" + name
}

// handleHookStarted creates a child span for a hook execution.
func (s *SpanEmittingStreamReader) handleHookStarted(msg *StreamMessage) {
	hookName := msg.HookName
	if hookName == "" {
		hookName = "unknown"
	}
	spanName := "hook " + hookName
	attrs := []attribute.KeyValue{
		attribute.String("hook.name", SanitizeUTF8(hookName)),
	}
	if msg.HookID != "" {
		attrs = append(attrs, attribute.String("hook.id", SanitizeUTF8(msg.HookID)))
	}
	if msg.HookEvent != "" {
		attrs = append(attrs, attribute.String("hook.event", SanitizeUTF8(msg.HookEvent)))
	}
	if msg.Command != "" {
		attrs = append(attrs, attribute.String("hook.command", SanitizeUTF8(msg.Command)))
	}
	if s.sessionID != "" {
		attrs = append(attrs, WeaveThreadNestedAttrs(s.sessionID)...)
	}

	_, hookSpan := s.tracer.Start(s.parentCtx, spanName, // nosemgrep: adr0003-otel-span-without-defer-end -- map-managed spans, End() in handleHookResponse/endAllOpenSpans [permanent]
		trace.WithAttributes(attrs...),
	)
	s.openSpans[hookSpanKey(msg)] = hookSpan
}

// handleHookResponse ends the child span for a hook execution.
func (s *SpanEmittingStreamReader) handleHookResponse(msg *StreamMessage) {
	key := hookSpanKey(msg)
	span, ok := s.openSpans[key]
	if !ok {
		return // orphan hook_response — no matching hook_started
	}
	if msg.ExitCode != nil {
		span.SetAttributes(attribute.Int("hook.exit_code", *msg.ExitCode))
	}
	if msg.Outcome != "" {
		span.SetAttributes(attribute.String("hook.outcome", SanitizeUTF8(msg.Outcome)))
	}
	span.End()
	delete(s.openSpans, key)
}

// handleInit captures init metadata for later retrieval via InitAttrs().
func (s *SpanEmittingStreamReader) handleInit(msg *StreamMessage) {
	s.initMsg = msg
}

// InitAttrs returns OTel attributes extracted from the system:init message.
// Returns nil if no init message was seen.
func (s *SpanEmittingStreamReader) InitAttrs() []attribute.KeyValue {
	if s.initMsg == nil {
		return nil
	}
	msg := s.initMsg
	var attrs []attribute.KeyValue

	if msg.Model != "" {
		attrs = append(attrs, attribute.String("claude.init.model", SanitizeUTF8(msg.Model)))
	}
	if len(msg.MCPServers) > 0 {
		names := make([]string, len(msg.MCPServers))
		for i, srv := range msg.MCPServers {
			names[i] = SanitizeUTF8(srv.Name)
		}
		attrs = append(attrs, attribute.StringSlice("claude.init.mcp_servers", names))
	}
	if len(msg.Tools) > 0 {
		attrs = append(attrs, attribute.Int("claude.init.tools_count", len(msg.Tools)))
	}
	if len(msg.Skills) > 0 {
		attrs = append(attrs, attribute.Int("claude.init.skills_count", len(msg.Skills)))
	}
	if len(msg.Plugins) > 0 {
		attrs = append(attrs, attribute.Int("claude.init.plugins_count", len(msg.Plugins)))
	}

	return attrs
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
					attribute.String("gen_ai.thinking.text", SanitizeUTF8(TruncateValue(block.Thinking, s.maxValueLen))),
				),
			)
			found = true
		}
	}
	return found
}

// handleRateLimit adds a rate_limit event to the parent span.
// If rate_limit_info is present, its fields are attached as event attributes.
func (s *SpanEmittingStreamReader) handleRateLimit(msg *StreamMessage) {
	parentSpan := trace.SpanFromContext(s.parentCtx)
	var eventOpts []trace.EventOption
	if info := msg.RateLimitInfo; info != nil {
		attrs := []attribute.KeyValue{}
		if info.Status != "" {
			attrs = append(attrs, attribute.String("rate_limit.status", SanitizeUTF8(info.Status)))
		}
		if info.RateLimitType != "" {
			attrs = append(attrs, attribute.String("rate_limit.type", SanitizeUTF8(info.RateLimitType)))
		}
		if info.Utilization > 0 {
			attrs = append(attrs, attribute.Float64("rate_limit.utilization", info.Utilization))
		}
		if len(attrs) > 0 {
			eventOpts = append(eventOpts, trace.WithAttributes(attrs...))
		}
	}
	parentSpan.AddEvent("rate_limit", eventOpts...)
}
