package platform

import (
	"encoding/json"
	"fmt"

	"github.com/hironow/amadeus/internal/domain"
)

// subagentToolNames lists tool names that indicate subagent creation.
// Provider-agnostic: Claude Code uses "Task"/"Agent", others may differ.
var subagentToolNames = map[string]bool{
	"Task":  true,
	"Agent": true,
}

// StreamNormalizer converts provider-specific StreamMessage into unified SessionStreamEvent.
type StreamNormalizer struct {
	toolName          string
	provider          domain.Provider
	sessionID         string            // captured from first non-empty provider session_id
	codingSessionID   string            // our CodingSessionRecord.ID
	subagents         map[string]string // tool_use_id -> subagent_id
	lastErr           error             // cached for SessionEnd
}

// NewStreamNormalizer creates a normalizer for the given tool and provider.
func NewStreamNormalizer(toolName string, provider domain.Provider) *StreamNormalizer {
	return &StreamNormalizer{
		toolName:  toolName,
		provider:  provider,
		subagents: make(map[string]string),
	}
}

// SetCodingSessionID sets the tracking session ID for emitted events.
func (n *StreamNormalizer) SetCodingSessionID(id string) {
	n.codingSessionID = id
}

// Normalize converts a StreamMessage into a SessionStreamEvent.
// Returns nil for messages that should not be published.
func (n *StreamNormalizer) Normalize(msg *StreamMessage, raw json.RawMessage) *domain.SessionStreamEvent {
	// Capture provider session ID from first non-empty occurrence.
	if n.sessionID == "" && msg.SessionID != "" {
		n.sessionID = msg.SessionID
	}

	var ev *domain.SessionStreamEvent

	switch {
	case msg.Type == "system" && msg.Subtype == "init":
		ev = n.normalizeInit(msg)
	case msg.Type == "system" && msg.Subtype == "hook_started":
		ev = n.normalizeHookStart(msg)
	case msg.Type == "system" && msg.Subtype == "hook_response":
		ev = n.normalizeHookResult(msg)
	case msg.Type == "assistant":
		ev = n.normalizeAssistant(msg)
	case msg.Type == "tool_result":
		ev = n.normalizeToolResult(msg)
	case msg.Type == "result":
		ev = n.normalizeResult(msg)
	case msg.Type == "rate_limit_event":
		ev = n.normalizeRateLimit(msg)
	default:
		if msg.IsError {
			ev = n.normalizeError(msg)
		}
	}

	if ev == nil {
		return nil
	}

	ev.ProviderSessionID = n.sessionID
	ev.SessionID = n.codingSessionID
	ev.WithRaw(string(raw))

	return ev
}

// SessionEnd creates a session_end event for any exit path (normal or abnormal).
func (n *StreamNormalizer) SessionEnd(providerSessionID string, runErr error) domain.SessionStreamEvent {
	data := map[string]any{}
	if runErr != nil {
		data["error"] = runErr.Error()
	}
	dataJSON, _ := json.Marshal(data)
	ev := domain.NewSessionStreamEvent(n.toolName, n.provider, domain.StreamSessionEnd, dataJSON)
	ev.ProviderSessionID = providerSessionID
	ev.SessionID = n.codingSessionID
	return ev
}

func (n *StreamNormalizer) normalizeInit(msg *StreamMessage) *domain.SessionStreamEvent {
	tools := msg.Tools
	servers := make([]map[string]string, 0, len(msg.MCPServers))
	for _, s := range msg.MCPServers {
		servers = append(servers, map[string]string{"name": s.Name, "status": s.Status})
	}
	data, _ := json.Marshal(map[string]any{
		"model":       msg.Model,
		"tools":       tools,
		"mcp_servers": servers,
	})
	ev := domain.NewSessionStreamEvent(n.toolName, n.provider, domain.StreamSessionStart, data)
	return &ev
}

func (n *StreamNormalizer) normalizeAssistant(msg *StreamMessage) *domain.SessionStreamEvent {
	// Check for tool_use (including subagent starts).
	toolBlocks, _ := msg.ExtractToolUse()
	if len(toolBlocks) > 0 {
		// Emit first tool_use as the event (multiple tools in one message are rare).
		tool := toolBlocks[0]
		if subagentToolNames[tool.Name] {
			return n.normalizeSubagentStart(tool, msg)
		}
		summary, _ := truncateInput(tool.Input, 200)
		data, _ := json.Marshal(map[string]any{
			"tool_name":      tool.Name,
			"tool_id":        tool.ID,
			"parent_tool_id": msg.ParentToolUseID,
			"summary":        summary,
		})
		ev := domain.NewSessionStreamEvent(n.toolName, n.provider, domain.StreamToolUseStart, data)
		return &ev
	}

	// Check for thinking blocks.
	am, _ := msg.ParseAssistantMessage()
	if am != nil {
		for _, block := range am.Content {
			if block.Type == "thinking" && block.Thinking != "" {
				text, _ := domain.TruncateField(block.Thinking, domain.RawFieldMaxBytes)
				data, _ := json.Marshal(map[string]string{"text": text})
				ev := domain.NewSessionStreamEvent(n.toolName, n.provider, domain.StreamThinking, data)
				return &ev
			}
		}
	}

	// Text output.
	text, _ := msg.ExtractText()
	if text != "" {
		truncated, _ := domain.TruncateField(text, domain.RawFieldMaxBytes)
		data, _ := json.Marshal(map[string]string{"text": truncated})
		ev := domain.NewSessionStreamEvent(n.toolName, n.provider, domain.StreamAssistantText, data)
		return &ev
	}

	return nil
}

func (n *StreamNormalizer) normalizeSubagentStart(tool ContentBlock, msg *StreamMessage) *domain.SessionStreamEvent {
	subID := fmt.Sprintf("sub_%s", tool.ID)
	n.subagents[tool.ID] = subID
	desc, _ := truncateInput(tool.Input, 200)
	data, _ := json.Marshal(map[string]any{
		"subagent_id":       subID,
		"parent_session_id": n.codingSessionID,
		"description":       desc,
	})
	ev := domain.NewSessionStreamEvent(n.toolName, n.provider, domain.StreamSubagentStart, data)
	ev.SubagentID = subID
	ev.ParentSessionID = n.codingSessionID
	return &ev
}

func (n *StreamNormalizer) normalizeToolResult(msg *StreamMessage) *domain.SessionStreamEvent {
	// Check if this is a subagent end.
	if subID, ok := n.subagents[msg.ToolUseID]; ok {
		delete(n.subagents, msg.ToolUseID)
		data, _ := json.Marshal(map[string]any{
			"subagent_id": subID,
			"status":      "completed",
		})
		ev := domain.NewSessionStreamEvent(n.toolName, n.provider, domain.StreamSubagentEnd, data)
		ev.SubagentID = subID
		return &ev
	}

	data, _ := json.Marshal(map[string]any{
		"tool_id": msg.ToolUseID,
		"status":  "completed",
	})
	ev := domain.NewSessionStreamEvent(n.toolName, n.provider, domain.StreamToolResult, data)
	return &ev
}

func (n *StreamNormalizer) normalizeResult(msg *StreamMessage) *domain.SessionStreamEvent {
	data := map[string]any{}
	if msg.Usage != nil {
		data["input_tokens"] = msg.Usage.InputTokens
		data["output_tokens"] = msg.Usage.OutputTokens
	}
	if msg.TotalCost > 0 {
		data["cost_usd"] = msg.TotalCost
	}
	if msg.Duration > 0 {
		data["duration_ms"] = msg.Duration
	}
	dataJSON, _ := json.Marshal(data)
	ev := domain.NewSessionStreamEvent(n.toolName, n.provider, domain.StreamSessionEnd, dataJSON)
	return &ev
}

func (n *StreamNormalizer) normalizeHookStart(msg *StreamMessage) *domain.SessionStreamEvent {
	data, _ := json.Marshal(map[string]string{
		"hook_name":  msg.HookName,
		"hook_event": msg.HookEvent,
		"command":    msg.Command,
	})
	ev := domain.NewSessionStreamEvent(n.toolName, n.provider, domain.StreamHookStart, data)
	return &ev
}

func (n *StreamNormalizer) normalizeHookResult(msg *StreamMessage) *domain.SessionStreamEvent {
	exitCode := 0
	if msg.ExitCode != nil {
		exitCode = *msg.ExitCode
	}
	data, _ := json.Marshal(map[string]any{
		"hook_name": msg.HookName,
		"exit_code": exitCode,
		"outcome":   msg.Outcome,
	})
	ev := domain.NewSessionStreamEvent(n.toolName, n.provider, domain.StreamHookResult, data)
	return &ev
}

func (n *StreamNormalizer) normalizeRateLimit(msg *StreamMessage) *domain.SessionStreamEvent {
	data := map[string]any{}
	if msg.RateLimitInfo != nil {
		data["status"] = msg.RateLimitInfo.Status
		data["resets_at"] = msg.RateLimitInfo.ResetsAt
		data["utilization"] = msg.RateLimitInfo.Utilization
	}
	dataJSON, _ := json.Marshal(data)
	ev := domain.NewSessionStreamEvent(n.toolName, n.provider, domain.StreamRateLimit, dataJSON)
	return &ev
}

func (n *StreamNormalizer) normalizeError(msg *StreamMessage) *domain.SessionStreamEvent {
	data, _ := json.Marshal(map[string]any{
		"message":     msg.Result,
		"recoverable": false,
	})
	ev := domain.NewSessionStreamEvent(n.toolName, n.provider, domain.StreamError, data)
	return &ev
}

// truncateInput extracts a summary from tool input JSON.
func truncateInput(input json.RawMessage, maxLen int) (string, bool) {
	if len(input) == 0 {
		return "", false
	}
	s := string(input)
	return domain.TruncateField(s, maxLen)
}
