package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"go.opentelemetry.io/otel/trace"
)

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
		_, toolSpan := s.tracer.Start(s.parentCtx, spanName,
			trace.WithAttributes(GenAIToolAttrs(tool.Name, toolID)...),
		)

		if len(tool.Input) > 0 {
			toolSpan.SetAttributes(GenAIToolInput.String(TruncateValue(string(tool.Input), s.maxValueLen)))
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
