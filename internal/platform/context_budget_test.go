package platform

// white-box-reason: tests internal ContextBudget parsing from stream output

import "testing"

func TestContextBudget_EmptyMessages(t *testing.T) {
	t.Parallel()

	// given
	var messages []*StreamMessage

	// when
	report := CalculateContextBudget(messages)

	// then
	if report.ToolCount != 0 {
		t.Errorf("expected ToolCount=0, got %d", report.ToolCount)
	}
	if report.SkillCount != 0 {
		t.Errorf("expected SkillCount=0, got %d", report.SkillCount)
	}
	if report.PluginCount != 0 {
		t.Errorf("expected PluginCount=0, got %d", report.PluginCount)
	}
	if report.MCPServerCount != 0 {
		t.Errorf("expected MCPServerCount=0, got %d", report.MCPServerCount)
	}
	if report.HookContextBytes != 0 {
		t.Errorf("expected HookContextBytes=0, got %d", report.HookContextBytes)
	}
	if report.EstimatedTokens != 0 {
		t.Errorf("expected EstimatedTokens=0, got %d", report.EstimatedTokens)
	}
}

func TestContextBudget_InitOnly(t *testing.T) {
	t.Parallel()

	// given: init message with tools, skills, plugins, MCP servers
	messages := []*StreamMessage{
		{
			Type:    "system",
			Subtype: "init",
			Tools:   []string{"Read", "Write", "Bash", "mcp__deepwiki__ask"},
			Skills:  []string{"commit", "review-pr", "debug"},
			Plugins: []PluginInfo{{Name: "superpowers"}, {Name: "linear"}},
			MCPServers: []MCPServerInfo{
				{Name: "deepwiki", Status: "connected"},
				{Name: "linear", Status: "connected"},
			},
		},
	}

	// when
	report := CalculateContextBudget(messages)

	// then
	if report.ToolCount != 4 {
		t.Errorf("expected ToolCount=4, got %d", report.ToolCount)
	}
	if report.SkillCount != 3 {
		t.Errorf("expected SkillCount=3, got %d", report.SkillCount)
	}
	if report.PluginCount != 2 {
		t.Errorf("expected PluginCount=2, got %d", report.PluginCount)
	}
	if report.MCPServerCount != 2 {
		t.Errorf("expected MCPServerCount=2, got %d", report.MCPServerCount)
	}
	if report.HookContextBytes != 0 {
		t.Errorf("expected HookContextBytes=0 (no hooks), got %d", report.HookContextBytes)
	}
	if report.EstimatedTokens <= 0 {
		t.Error("expected EstimatedTokens > 0 with tools/skills/plugins")
	}
}

func TestContextBudget_HookResponses(t *testing.T) {
	t.Parallel()

	// given: hook_response messages with stdout content
	messages := []*StreamMessage{
		{
			Type:    "system",
			Subtype: "hook_response",
			Stdout:  "hook output with 30 characters",
		},
		{
			Type:    "system",
			Subtype: "hook_response",
			Stdout:  "another 20 chars!!!",
		},
	}

	// when
	report := CalculateContextBudget(messages)

	// then: should sum stdout bytes
	expectedBytes := len("hook output with 30 characters") + len("another 20 chars!!!")
	if report.HookContextBytes != expectedBytes {
		t.Errorf("expected HookContextBytes=%d, got %d", expectedBytes, report.HookContextBytes)
	}
	if report.EstimatedTokens <= 0 {
		t.Error("expected EstimatedTokens > 0 from hook context")
	}
}

func TestContextBudget_InitPlusHooks(t *testing.T) {
	t.Parallel()

	// given: init + hook responses
	messages := []*StreamMessage{
		{
			Type:    "system",
			Subtype: "init",
			Tools:   []string{"Read", "Write"},
			Skills:  []string{"commit"},
			Plugins: []PluginInfo{{Name: "superpowers"}},
			MCPServers: []MCPServerInfo{
				{Name: "deepwiki", Status: "connected"},
			},
		},
		{
			Type:    "system",
			Subtype: "hook_response",
			Stdout:  "hook context data here",
		},
	}

	// when
	report := CalculateContextBudget(messages)

	// then: both init and hook contribute to estimated tokens
	if report.ToolCount != 2 {
		t.Errorf("expected ToolCount=2, got %d", report.ToolCount)
	}
	if report.HookContextBytes != len("hook context data here") {
		t.Errorf("expected HookContextBytes=%d, got %d", len("hook context data here"), report.HookContextBytes)
	}

	// tokens from init components + hook bytes should be positive
	if report.EstimatedTokens <= 0 {
		t.Error("expected EstimatedTokens > 0")
	}
}

func TestContextBudget_IgnoresNonSystemMessages(t *testing.T) {
	t.Parallel()

	// given: assistant message (not system)
	messages := []*StreamMessage{
		{
			Type: "assistant",
		},
		{
			Type:   "result",
			Result: "some result",
		},
	}

	// when
	report := CalculateContextBudget(messages)

	// then: non-system messages should not affect budget
	if report.ToolCount != 0 {
		t.Errorf("expected ToolCount=0, got %d", report.ToolCount)
	}
	if report.EstimatedTokens != 0 {
		t.Errorf("expected EstimatedTokens=0, got %d", report.EstimatedTokens)
	}
}

func TestContextBudgetExceeded_BelowThreshold(t *testing.T) {
	t.Parallel()

	// given
	report := ContextBudgetReport{EstimatedTokens: 5000}

	// when
	exceeded := report.Exceeds(10000)

	// then
	if exceeded {
		t.Error("expected not exceeded when below threshold")
	}
}

func TestContextBudgetExceeded_AboveThreshold(t *testing.T) {
	t.Parallel()

	// given
	report := ContextBudgetReport{EstimatedTokens: 15000}

	// when
	exceeded := report.Exceeds(10000)

	// then
	if !exceeded {
		t.Error("expected exceeded when above threshold")
	}
}

func TestContextBudgetWarning_NotExceeded(t *testing.T) {
	t.Parallel()

	// given
	report := ContextBudgetReport{EstimatedTokens: 5000}

	// when
	msg := report.WarningMessage(10000)

	// then
	if msg != "" {
		t.Errorf("expected empty warning, got %q", msg)
	}
}

func TestContextBudgetWarning_Exceeded(t *testing.T) {
	t.Parallel()

	// given
	report := ContextBudgetReport{
		EstimatedTokens:  15000,
		ToolCount:        8,
		SkillCount:       10,
		PluginCount:      4,
		MCPServerCount:   4,
		HookContextBytes: 50000,
	}

	// when
	msg := report.WarningMessage(10000)

	// then
	if msg == "" {
		t.Error("expected non-empty warning message")
	}
}

func TestContextBudgetAttrs_ReturnsAllFields(t *testing.T) {
	t.Parallel()

	// given
	report := ContextBudgetReport{
		ToolCount:        8,
		SkillCount:       10,
		PluginCount:      4,
		MCPServerCount:   4,
		HookContextBytes: 5000,
		EstimatedTokens:  12000,
	}

	// when
	attrs := report.Attrs()

	// then: should return 6 attributes
	if len(attrs) != 6 {
		t.Fatalf("expected 6 attributes, got %d", len(attrs))
	}

	// verify key names exist
	keys := make(map[string]bool)
	for _, attr := range attrs {
		keys[string(attr.Key)] = true
	}
	expected := []string{
		"context_budget.tools",
		"context_budget.skills",
		"context_budget.plugins",
		"context_budget.mcp_servers",
		"context_budget.hook_bytes",
		"context_budget.estimated_tokens",
	}
	for _, key := range expected {
		if !keys[key] {
			t.Errorf("missing attribute key %q", key)
		}
	}
}

func TestContextBudgetReport_DetailedBreakdown(t *testing.T) {
	t.Parallel()

	// given
	report := ContextBudgetReport{
		ToolCount:        45,
		SkillCount:       12,
		PluginCount:      40,
		MCPServerCount:   5,
		HookContextBytes: 800,
		EstimatedTokens:  22450,
	}

	// when
	breakdown := report.DetailedBreakdown()

	// then
	if len(breakdown) != 5 {
		t.Fatalf("got %d items, want 5", len(breakdown))
	}

	// Find heaviest
	var heaviest string
	for _, item := range breakdown {
		if item.Heaviest {
			heaviest = item.Category
		}
	}
	if heaviest != "plugins" {
		t.Errorf("heaviest = %q, want plugins", heaviest)
	}

	// Verify token calculations
	for _, item := range breakdown {
		switch item.Category {
		case "tools":
			if item.Count != 45 || item.Tokens != 6750 {
				t.Errorf("tools: count=%d tokens=%d, want 45/6750", item.Count, item.Tokens)
			}
		case "skills":
			if item.Count != 12 || item.Tokens != 6000 {
				t.Errorf("skills: count=%d tokens=%d, want 12/6000", item.Count, item.Tokens)
			}
		case "plugins":
			if item.Count != 40 || item.Tokens != 8000 {
				t.Errorf("plugins: count=%d tokens=%d, want 40/8000", item.Count, item.Tokens)
			}
		case "mcp_servers":
			if item.Count != 5 || item.Tokens != 1500 {
				t.Errorf("mcp_servers: count=%d tokens=%d, want 5/1500", item.Count, item.Tokens)
			}
		case "hooks":
			if item.Bytes != 800 || item.Tokens != 200 {
				t.Errorf("hooks: bytes=%d tokens=%d, want 800/200", item.Bytes, item.Tokens)
			}
		}
	}
}

func TestContextBudgetReport_DetailedBreakdown_SkillsHeaviest(t *testing.T) {
	t.Parallel()

	// given
	report := ContextBudgetReport{
		ToolCount:       2,
		SkillCount:      100,
		PluginCount:     1,
		EstimatedTokens: 50500,
	}

	// when
	breakdown := report.DetailedBreakdown()

	// then
	var heaviest string
	for _, item := range breakdown {
		if item.Heaviest {
			heaviest = item.Category
		}
	}
	if heaviest != "skills" {
		t.Errorf("heaviest = %q, want skills", heaviest)
	}
}

func TestContextBudgetReport_DetailedBreakdown_AllZero(t *testing.T) {
	t.Parallel()

	// given
	report := ContextBudgetReport{}

	// when
	breakdown := report.DetailedBreakdown()

	// then
	if len(breakdown) != 5 {
		t.Fatalf("got %d items, want 5", len(breakdown))
	}
	for _, item := range breakdown {
		if item.Heaviest {
			t.Errorf("no item should be heaviest when all are zero")
		}
	}
}

func TestEstimateTokens_TokensPerChar(t *testing.T) {
	t.Parallel()

	// given: 8 bytes of hook output (divisible by charsPerToken=4)
	messages := []*StreamMessage{
		{
			Type:    "system",
			Subtype: "hook_response",
			Stdout:  "12345678", // 8 bytes -> 2 tokens
		},
	}

	// when
	report := CalculateContextBudget(messages)

	// then: 8 bytes / 4 chars-per-token = 2 tokens
	if report.EstimatedTokens != 2 {
		t.Errorf("expected EstimatedTokens=2, got %d", report.EstimatedTokens)
	}
}
