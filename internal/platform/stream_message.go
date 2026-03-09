package platform

import (
	"encoding/json"
	"strings"
)

// StreamMessage represents a single NDJSON line from Claude Code --output-format stream-json.
type StreamMessage struct {
	Type       string          `json:"type"`
	Subtype    string          `json:"subtype,omitempty"`
	UUID       string          `json:"uuid,omitempty"`
	SessionID  string          `json:"session_id,omitempty"`
	Message    json.RawMessage `json:"message,omitempty"`
	Result     string          `json:"result,omitempty"`
	Usage      *Usage          `json:"usage,omitempty"`
	TotalCost  float64         `json:"total_cost_usd,omitempty"`
	NumTurns   int             `json:"num_turns,omitempty"`
	Duration   int64           `json:"duration_ms,omitempty"`
	IsError    bool            `json:"is_error,omitempty"`
	StopReason string          `json:"stop_reason,omitempty"`
	ToolUseID  string          `json:"tool_use_id,omitempty"`
}

// Usage holds token usage from Claude Code.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// AssistantMessage is the nested message inside SDKAssistantMessage.
type AssistantMessage struct {
	ID         string         `json:"id,omitempty"`
	Role       string         `json:"role,omitempty"`
	Model      string         `json:"model,omitempty"`
	Content    []ContentBlock `json:"content,omitempty"`
	StopReason string         `json:"stop_reason,omitempty"`
	Usage      *Usage         `json:"usage,omitempty"`
}

// ContentBlock represents a content block (text, tool_use, thinking).
type ContentBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Thinking string          `json:"thinking,omitempty"`
	ID       string          `json:"id,omitempty"`
	Name     string          `json:"name,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
}

// ParseStreamMessage parses a single NDJSON line.
func ParseStreamMessage(data []byte) (*StreamMessage, error) {
	var msg StreamMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ParseAssistantMessage extracts the AssistantMessage from Message field.
func (m *StreamMessage) ParseAssistantMessage() (*AssistantMessage, error) {
	if m.Message == nil {
		return nil, nil
	}
	var am AssistantMessage
	if err := json.Unmarshal(m.Message, &am); err != nil {
		return nil, err
	}
	return &am, nil
}

// ExtractText concatenates all text content blocks from an assistant message.
func (m *StreamMessage) ExtractText() (string, error) {
	am, err := m.ParseAssistantMessage()
	if err != nil || am == nil {
		return "", err
	}
	var sb strings.Builder
	for _, block := range am.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	return sb.String(), nil
}

// ExtractToolUse returns all tool_use content blocks from an assistant message.
func (m *StreamMessage) ExtractToolUse() ([]ContentBlock, error) {
	am, err := m.ParseAssistantMessage()
	if err != nil || am == nil {
		return nil, err
	}
	var tools []ContentBlock
	for _, block := range am.Content {
		if block.Type == "tool_use" {
			tools = append(tools, block)
		}
	}
	return tools, nil
}
