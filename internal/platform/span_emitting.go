package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// SanitizeUTF8 replaces invalid UTF-8 bytes with the Unicode replacement character.
// Use this before passing strings from external sources (Claude output, file system,
// error messages) to OTel span attributes, which require valid UTF-8.
func SanitizeUTF8(s string) string {
	return strings.ToValidUTF8(s, "\uFFFD")
}

// SanitizeUTF8Slice sanitizes each element of a string slice for OTel safety.
func SanitizeUTF8Slice(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = SanitizeUTF8(s)
	}
	return out
}

// DefaultMaxValueLen is the default truncation limit for raw event values.
const DefaultMaxValueLen = 512

// TruncateValue truncates s to maxLen characters, appending "..." if truncated.
func TruncateValue(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// FormatRawEvent formats a stream event as "type:truncated_json" for storage
// in the stream.raw_events span attribute.
func FormatRawEvent(eventType, jsonData string, maxValueLen int) string {
	return eventType + ":" + TruncateValue(jsonData, maxValueLen)
}

// SyntheticToolID generates a fallback tool ID when the stream does not provide one.
func SyntheticToolID(seq int) string {
	return fmt.Sprintf("synthetic-%d", seq)
}

// SpanEmittingStreamReader wraps StreamReader to emit OTel child spans
// for tool_use events in real-time.
type SpanEmittingStreamReader struct {
	reader       *StreamReader
	parentCtx    context.Context
	tracer       trace.Tracer
	openSpans    map[string]trace.Span
	rawEvents    []string
	maxValueLen  int
	syntheticSeq int
	sessionID    string         // captured from stream session_id for Weave thread_id
	resultText   string         // captured from result message for Weave output.value
	inputText    string         // caller-provided prompt for Weave input.value
	initMsg      *StreamMessage // captured from system:init for InitAttrs()
}

// NewSpanEmittingStreamReader creates a SpanEmittingStreamReader.
func NewSpanEmittingStreamReader(reader *StreamReader, parentCtx context.Context, tracer trace.Tracer) *SpanEmittingStreamReader {
	return &SpanEmittingStreamReader{
		reader:      reader,
		parentCtx:   parentCtx,
		tracer:      tracer,
		openSpans:   make(map[string]trace.Span),
		maxValueLen: DefaultMaxValueLen,
	}
}

// RawEvents returns the collected raw event strings.
func (s *SpanEmittingStreamReader) RawEvents() []string {
	return s.rawEvents
}

// WeaveThreadAttrs returns Weave thread attributes for the parent (turn) span.
// The thread_id is derived from the Claude session_id seen in the stream.
func (s *SpanEmittingStreamReader) WeaveThreadAttrs() []attribute.KeyValue {
	if s.sessionID == "" {
		return nil
	}
	return WeaveThreadTurnAttrs(s.sessionID)
}

// SetInput records the prompt text for Weave input.value mapping.
func (s *SpanEmittingStreamReader) SetInput(prompt string) {
	s.inputText = prompt
}

// WeaveIOAttrs returns Weave I/O attributes (input.value and output.value).
func (s *SpanEmittingStreamReader) WeaveIOAttrs() []attribute.KeyValue {
	var attrs []attribute.KeyValue
	if s.inputText != "" {
		attrs = append(attrs, WeaveInputVal.String(SanitizeUTF8(s.inputText)))
	}
	if s.resultText != "" {
		attrs = append(attrs, WeaveOutputVal.String(SanitizeUTF8(s.resultText)))
	}
	return attrs
}

// CollectAll reads all messages, emitting tool spans, and returns the result.
func (s *SpanEmittingStreamReader) CollectAll() (*StreamMessage, []*StreamMessage, error) {
	var messages []*StreamMessage
	var result *StreamMessage
	for {
		msg, err := s.reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			s.endAllOpenSpans()
			return result, messages, err
		}
		messages = append(messages, msg)
		s.processMessage(msg)
		if msg.Type == "result" {
			result = msg
			s.endAllOpenSpans()
		}
	}
	s.endAllOpenSpans()
	return result, messages, nil
}

func (s *SpanEmittingStreamReader) processMessage(msg *StreamMessage) {
	raw, _ := json.Marshal(msg)
	s.rawEvents = append(s.rawEvents, FormatRawEvent(msg.Type, string(raw), s.maxValueLen))

	// Capture session_id for Weave thread_id (first non-empty wins).
	if s.sessionID == "" && msg.SessionID != "" {
		s.sessionID = msg.SessionID
	}

	// Capture result text for Weave output.value.
	if msg.Type == "result" && msg.Result != "" {
		s.resultText = msg.Result
	}

	// Extended handlers (hooks, thinking, rate_limit) — see span_emitting_ext.go
	s.handleExtMessage(msg)

	switch msg.Type {
	case "assistant":
		s.handleAssistant(msg)
	case "tool_result":
		s.handleToolResult(msg)
	}
}

func (s *SpanEmittingStreamReader) handleAssistant(msg *StreamMessage) {
	tools, err := msg.ExtractToolUse()
	if err != nil {
		return
	}
	for _, tool := range tools {
		toolID := tool.ID
		if toolID == "" {
			toolID = SyntheticToolID(s.syntheticSeq)
			s.syntheticSeq++
		}

		spanName := "execute_tool " + tool.Name
		toolAttrs := GenAIToolAttrs(tool.Name, toolID)
		if s.sessionID != "" {
			toolAttrs = append(toolAttrs, WeaveThreadNestedAttrs(s.sessionID)...)
		}
		_, toolSpan := s.tracer.Start(s.parentCtx, spanName, // nosemgrep: adr0003-otel-span-without-defer-end -- map-managed spans, End() in handleToolResult/endAllOpenSpans [permanent]
			trace.WithAttributes(toolAttrs...),
		)

		if len(tool.Input) > 0 {
			toolSpan.SetAttributes(GenAIToolInput.String(SanitizeUTF8(TruncateValue(string(tool.Input), s.maxValueLen))))
		}

		s.openSpans[toolID] = toolSpan
	}
}

func (s *SpanEmittingStreamReader) handleToolResult(msg *StreamMessage) {
	toolUseID := msg.ToolUseID
	if toolUseID == "" {
		return
	}
	span, ok := s.openSpans[toolUseID]
	if !ok {
		return
	}
	span.End()
	delete(s.openSpans, toolUseID)
}

func (s *SpanEmittingStreamReader) endAllOpenSpans() {
	for id, span := range s.openSpans {
		span.End()
		delete(s.openSpans, id)
	}
}
