package platform_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
)

func TestStreamNormalizer_Init(t *testing.T) {
	t.Parallel()
	n := platform.NewStreamNormalizer("sightjack", domain.ProviderClaudeCode)
	msg := &platform.StreamMessage{
		Type:      "system",
		Subtype:   "init",
		SessionID: "claude-sess-1",
		Model:     "opus",
		Tools:     []string{"Read", "Write"},
	}
	raw, _ := json.Marshal(msg)
	ev := n.Normalize(msg, raw)
	if ev == nil {
		t.Fatal("expected event for system:init")
	}
	if ev.Type != domain.StreamSessionStart {
		t.Errorf("Type = %q, want %q", ev.Type, domain.StreamSessionStart)
	}
	if ev.ProviderSessionID != "claude-sess-1" {
		t.Errorf("ProviderSessionID = %q, want %q", ev.ProviderSessionID, "claude-sess-1")
	}
}

func TestStreamNormalizer_ToolUse(t *testing.T) {
	t.Parallel()
	n := platform.NewStreamNormalizer("paintress", domain.ProviderClaudeCode)
	// Simulate an assistant message with tool_use content block.
	content := []map[string]any{{
		"type":  "tool_use",
		"id":    "toolu_01",
		"name":  "Read",
		"input": json.RawMessage(`{"file_path":"/src/main.go"}`),
	}}
	msgJSON := map[string]any{
		"content": content,
	}
	messageBytes, _ := json.Marshal(msgJSON)
	msg := &platform.StreamMessage{
		Type:    "assistant",
		Message: messageBytes,
	}
	raw, _ := json.Marshal(msg)
	ev := n.Normalize(msg, raw)
	if ev == nil {
		t.Fatal("expected event for tool_use")
	}
	if ev.Type != domain.StreamToolUseStart {
		t.Errorf("Type = %q, want %q", ev.Type, domain.StreamToolUseStart)
	}
	if ev.Tool != "paintress" {
		t.Errorf("Tool = %q, want %q", ev.Tool, "paintress")
	}
}

func TestStreamNormalizer_SubagentLifecycle(t *testing.T) {
	t.Parallel()
	n := platform.NewStreamNormalizer("sightjack", domain.ProviderClaudeCode)
	n.SetCodingSessionID("session-123")

	// Subagent start: tool_use with name "Task".
	content := []map[string]any{{
		"type":  "tool_use",
		"id":    "toolu_sub_01",
		"name":  "Task",
		"input": json.RawMessage(`{"description":"explore codebase"}`),
	}}
	messageBytes, _ := json.Marshal(map[string]any{"content": content})
	startMsg := &platform.StreamMessage{
		Type:    "assistant",
		Message: messageBytes,
	}
	raw, _ := json.Marshal(startMsg)
	startEv := n.Normalize(startMsg, raw)
	if startEv == nil {
		t.Fatal("expected subagent_start event")
	}
	if startEv.Type != domain.StreamSubagentStart {
		t.Errorf("Type = %q, want %q", startEv.Type, domain.StreamSubagentStart)
	}
	if startEv.SubagentID == "" {
		t.Error("SubagentID should be non-empty")
	}
	if startEv.ParentSessionID != "session-123" {
		t.Errorf("ParentSessionID = %q, want %q", startEv.ParentSessionID, "session-123")
	}

	// Subagent end: tool_result matching the tool_use_id.
	endMsg := &platform.StreamMessage{
		Type:      "tool_result",
		ToolUseID: "toolu_sub_01",
	}
	endRaw, _ := json.Marshal(endMsg)
	endEv := n.Normalize(endMsg, endRaw)
	if endEv == nil {
		t.Fatal("expected subagent_end event")
	}
	if endEv.Type != domain.StreamSubagentEnd {
		t.Errorf("Type = %q, want %q", endEv.Type, domain.StreamSubagentEnd)
	}
}

func TestStreamNormalizer_RawTruncation(t *testing.T) {
	t.Parallel()
	n := platform.NewStreamNormalizer("amadeus", domain.ProviderClaudeCode)
	// Create a large system:init message.
	msg := &platform.StreamMessage{
		Type:    "system",
		Subtype: "init",
		Model:   "opus",
	}
	// Build raw > 4KB.
	bigRaw := make([]byte, 5000)
	for i := range bigRaw {
		bigRaw[i] = 'x'
	}
	ev := n.Normalize(msg, bigRaw)
	if ev == nil {
		t.Fatal("expected event")
	}
	if !ev.RawTruncated {
		t.Error("raw should be truncated")
	}
	if len(ev.Raw) > domain.RawFieldMaxBytes {
		t.Errorf("raw length %d exceeds max %d", len(ev.Raw), domain.RawFieldMaxBytes)
	}
}

func TestStreamNormalizer_Result_SavesUsageForSessionEnd(t *testing.T) {
	t.Parallel()
	n := platform.NewStreamNormalizer("amadeus", domain.ProviderClaudeCode)
	msg := &platform.StreamMessage{
		Type:      "result",
		SessionID: "sess-1",
		Result:    "done",
		Usage:     &platform.Usage{InputTokens: 1000, OutputTokens: 500},
		TotalCost: 0.05,
		Duration:  12000,
	}
	raw, _ := json.Marshal(msg)

	ev := n.Normalize(msg, raw)
	if ev != nil {
		t.Fatalf("normalizeResult should return nil, got type=%s", ev.Type)
	}

	endEv := n.SessionEnd("provider-1", nil)
	if endEv.Type != domain.StreamSessionEnd {
		t.Errorf("Type = %q, want %q", endEv.Type, domain.StreamSessionEnd)
	}
	data := string(endEv.Data)
	if !strings.Contains(data, "1000") {
		t.Errorf("SessionEnd should contain saved input_tokens, got: %s", data)
	}
}

func TestNormalize_OutputPassesDomainValidator(t *testing.T) {
	t.Parallel()
	n := platform.NewStreamNormalizer("amadeus", domain.ProviderClaudeCode)
	n.SetCodingSessionID("test-session")

	// session_start should pass validation
	startMsg := &platform.StreamMessage{
		Type:      "system",
		Subtype:   "init",
		SessionID: "claude-sess-1",
		Model:     "opus",
		Tools:     []string{"Read", "Write"},
	}
	raw, _ := json.Marshal(startMsg)
	if ev := n.Normalize(startMsg, raw); ev != nil {
		if _, err := domain.ParseSessionStreamEvent(*ev); err != nil {
			t.Errorf("session_start event failed validation: %v", err)
		}
	}

	// tool_use should pass validation
	content := []map[string]any{{
		"type":  "tool_use",
		"id":    "toolu_01",
		"name":  "Read",
		"input": json.RawMessage(`{"file_path":"/src/main.go"}`),
	}}
	messageBytes, _ := json.Marshal(map[string]any{"content": content})
	toolMsg := &platform.StreamMessage{
		Type:    "assistant",
		Message: messageBytes,
	}
	raw, _ = json.Marshal(toolMsg)
	if ev := n.Normalize(toolMsg, raw); ev != nil {
		if _, err := domain.ParseSessionStreamEvent(*ev); err != nil {
			t.Errorf("tool_use event failed validation: %v", err)
		}
	}

	// SessionEnd should pass validation
	endEvent := n.SessionEnd("provider-123", nil)
	if _, err := domain.ParseSessionStreamEvent(endEvent); err != nil {
		t.Errorf("session_end event failed validation: %v", err)
	}
}

func TestStreamNormalizer_SessionEnd(t *testing.T) {
	t.Parallel()
	n := platform.NewStreamNormalizer("sightjack", domain.ProviderClaudeCode)
	n.SetCodingSessionID("sess-abc")

	// Normal end.
	ev := n.SessionEnd("provider-1", nil)
	if ev.Type != domain.StreamSessionEnd {
		t.Errorf("Type = %q, want %q", ev.Type, domain.StreamSessionEnd)
	}
	if ev.SessionID != "sess-abc" {
		t.Errorf("SessionID = %q, want %q", ev.SessionID, "sess-abc")
	}

	// Error end.
	ev2 := n.SessionEnd("provider-1", context.DeadlineExceeded)
	var data map[string]any
	json.Unmarshal(ev2.Data, &data)
	if data["error"] == nil {
		t.Error("error end should contain error field in data")
	}
}
