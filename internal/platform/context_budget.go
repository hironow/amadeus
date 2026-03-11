package platform

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"
)

// charsPerToken is the approximate ratio of characters to tokens.
// Claude tokenizer averages ~4 characters per token for English text.
const charsPerToken = 4

// tokensPerTool is the estimated token overhead per tool definition in context.
const tokensPerTool = 150

// tokensPerSkill is the estimated token overhead per skill loaded in context.
const tokensPerSkill = 500

// tokensPerPlugin is the estimated token overhead per plugin loaded in context.
const tokensPerPlugin = 200

// tokensPerMCPServer is the estimated token overhead per MCP server in context.
const tokensPerMCPServer = 300

// ContextBudgetReport summarises the estimated context consumption
// from Claude Code hooks, plugins, skills, and MCP servers.
type ContextBudgetReport struct {
	ToolCount        int
	SkillCount       int
	PluginCount      int
	MCPServerCount   int
	HookContextBytes int
	EstimatedTokens  int
}

// Exceeds returns true if EstimatedTokens exceeds the given threshold.
func (r ContextBudgetReport) Exceeds(threshold int) bool {
	return r.EstimatedTokens > threshold
}

// WarningMessage returns a human-readable warning if the budget exceeds
// the threshold, or an empty string if within budget.
func (r ContextBudgetReport) WarningMessage(threshold int) string {
	if !r.Exceeds(threshold) {
		return ""
	}
	return fmt.Sprintf(
		"context budget exceeded: estimated %d tokens (threshold %d). "+
			"tools=%d, skills=%d, plugins=%d, mcp_servers=%d, hook_bytes=%d. "+
			"Consider reducing installed plugins/skills or using an allowlist.",
		r.EstimatedTokens, threshold,
		r.ToolCount, r.SkillCount, r.PluginCount, r.MCPServerCount,
		r.HookContextBytes,
	)
}

// CalculateContextBudget analyses stream messages to estimate context
// consumption from init metadata and hook responses.
func CalculateContextBudget(messages []*StreamMessage) ContextBudgetReport {
	var report ContextBudgetReport

	for _, msg := range messages {
		if msg.Type != "system" {
			continue
		}

		switch msg.Subtype {
		case "init":
			report.ToolCount = len(msg.Tools)
			report.SkillCount = len(msg.Skills)
			report.PluginCount = len(msg.Plugins)
			report.MCPServerCount = len(msg.MCPServers)

		case "hook_response":
			report.HookContextBytes += len(msg.Stdout)
		}
	}

	report.EstimatedTokens = report.ToolCount*tokensPerTool +
		report.SkillCount*tokensPerSkill +
		report.PluginCount*tokensPerPlugin +
		report.MCPServerCount*tokensPerMCPServer +
		report.HookContextBytes/charsPerToken

	return report
}

// DefaultContextBudgetThreshold is the default warning threshold in estimated tokens.
// 20K tokens leaves reasonable headroom in a 200K context window.
const DefaultContextBudgetThreshold = 20000

// Attrs returns OTel span attributes for the context budget report.
func (r ContextBudgetReport) Attrs() []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Int("context_budget.tools", r.ToolCount),
		attribute.Int("context_budget.skills", r.SkillCount),
		attribute.Int("context_budget.plugins", r.PluginCount),
		attribute.Int("context_budget.mcp_servers", r.MCPServerCount),
		attribute.Int("context_budget.hook_bytes", r.HookContextBytes),
		attribute.Int("context_budget.estimated_tokens", r.EstimatedTokens),
	}
}
