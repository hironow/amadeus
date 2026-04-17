package domain

import (
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// StreamSchemaVersion is the current schema version for session stream events.
const StreamSchemaVersion uint8 = 1

// RawFieldMaxBytes is the maximum size of the raw field and text data fields.
const RawFieldMaxBytes = 4096

// StreamEventType identifies the kind of session stream event.
type StreamEventType string

const (
	StreamSessionStart   StreamEventType = "session_start"
	StreamSessionEnd     StreamEventType = "session_end"
	StreamToolUseStart   StreamEventType = "tool_use_start"
	StreamToolResult     StreamEventType = "tool_result"
	StreamAssistantText  StreamEventType = "assistant_text"
	StreamThinking       StreamEventType = "thinking"
	StreamHookStart      StreamEventType = "hook_start"
	StreamHookResult     StreamEventType = "hook_result"
	StreamRateLimit      StreamEventType = "rate_limit"
	StreamError          StreamEventType = "error"
	StreamSubagentStart  StreamEventType = "subagent_start"
	StreamSubagentEnd    StreamEventType = "subagent_end"
)

// SessionStreamEvent is the unified JSONL envelope for live session streaming.
// Provider-agnostic: all AI coding tools emit events in this format.
type SessionStreamEvent struct {
	SchemaVersion     uint8           `json:"v"`
	ID                string          `json:"id"`
	Timestamp         time.Time       `json:"ts"`
	Tool              string          `json:"tool"`
	SessionID         string          `json:"session_id"`
	Provider          Provider        `json:"provider"`
	ProviderSessionID string          `json:"provider_session_id,omitempty"`
	Type              StreamEventType `json:"type"`
	ParentSessionID   string          `json:"parent_session_id,omitempty"`
	SubagentID        string          `json:"subagent_id,omitempty"`
	Data              json.RawMessage `json:"data"`
	Raw               string          `json:"raw,omitempty"`
	RawTruncated      bool            `json:"raw_truncated,omitempty"`
}

// NewSessionStreamEvent creates a new event with schema version, UUID, and timestamp.
func NewSessionStreamEvent(tool string, provider Provider, eventType StreamEventType, data json.RawMessage) SessionStreamEvent {
	return SessionStreamEvent{
		SchemaVersion: StreamSchemaVersion,
		ID:            uuid.New().String(),
		Timestamp:     time.Now().UTC(),
		Tool:          tool,
		Provider:      provider,
		Type:          eventType,
		Data:          data,
	}
}

// WithRaw attaches the raw provider JSONL line, truncating if necessary.
func (e *SessionStreamEvent) WithRaw(raw string) {
	truncated, wasTruncated := TruncateField(raw, RawFieldMaxBytes)
	e.Raw = truncated
	e.RawTruncated = wasTruncated
}

// TruncateField truncates s to maxBytes at a UTF-8 boundary, appending "..."
// if truncation occurred. Returns the (possibly truncated) string and whether
// truncation happened.
func TruncateField(s string, maxBytes int) (string, bool) {
	if len(s) <= maxBytes {
		return s, false
	}
	// Reserve 3 bytes for "..." suffix.
	limit := maxBytes - 3
	if limit < 0 {
		return "...", true
	}
	// Walk back to a valid UTF-8 boundary.
	for limit > 0 && !utf8.RuneStart(s[limit]) {
		limit--
	}
	return s[:limit] + "...", true
}

// ParseSessionStreamEvent validates required fields and returns the event for pipeline chaining.
func ParseSessionStreamEvent(e SessionStreamEvent) (SessionStreamEvent, error) {
	if e.Tool == "" {
		return SessionStreamEvent{}, fmt.Errorf("session stream event: tool is required")
	}
	if e.Type == "" {
		return SessionStreamEvent{}, fmt.Errorf("session stream event: type is required")
	}
	if e.ID == "" {
		return SessionStreamEvent{}, fmt.Errorf("session stream event: id is required")
	}
	return e, nil
}
