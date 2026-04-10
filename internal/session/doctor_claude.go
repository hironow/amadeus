package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
)

func CheckClaudeAuth(mcpOutput string, mcpErr error, claudeCmd string) domain.DoctorCheck {
	if mcpErr != nil {
		hint := buildLoginHint(claudeCmd)
		return domain.DoctorCheck{
			Name:    "claude-auth",
			Status:  domain.CheckWarn,
			Message: "not authenticated: " + mcpErr.Error(),
			Hint:    hint,
		}
	}
	return domain.DoctorCheck{
		Name:    "claude-auth",
		Status:  domain.CheckOK,
		Message: "authenticated",
	}
}

// buildLoginHint constructs a login hint that preserves any env prefix
// from the configured claude command (e.g. "CLAUDE_CONFIG_DIR=/path claude").
func buildLoginHint(claudeCmd string) string {
	envPrefix := extractEnvPrefix(claudeCmd)
	if envPrefix == "" {
		return `run "claude login" to authenticate`
	}
	return fmt.Sprintf(`run "%s claude login" to authenticate`, envPrefix)
}

// extractEnvPrefix extracts leading KEY=VALUE pairs from a command string.
// Returns the env prefix portion or empty string if none.
func extractEnvPrefix(cmd string) string {
	parts := strings.Fields(cmd)
	var envParts []string
	for _, p := range parts {
		if strings.Contains(p, "=") {
			envParts = append(envParts, p)
		} else {
			break
		}
	}
	return strings.Join(envParts, " ")
}

// CheckLinearMCP verifies Linear MCP is connected by parsing `claude mcp list` output.
// Looks for a line containing "linear", "✓", and "connected" (case-insensitive).
// Requires "✓" to avoid false positives from "disconnected" or "not connected".
func CheckLinearMCP(mcpOutput string, mcpErr error) domain.DoctorCheck {
	if mcpErr != nil {
		return domain.DoctorCheck{
			Name:    "linear-mcp",
			Status:  domain.CheckWarn,
			Message: fmt.Sprintf("claude mcp list failed: %v", mcpErr),
			Hint:    `run "claude login" to authenticate`,
		}
	}

	output := strings.ToLower(mcpOutput)
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "linear") && strings.Contains(line, "✓") && strings.Contains(line, "connected") {
			return domain.DoctorCheck{
				Name:    "linear-mcp",
				Status:  domain.CheckOK,
				Message: "Linear MCP connected",
			}
		}
	}

	return domain.DoctorCheck{
		Name:    "linear-mcp",
		Status:  domain.CheckWarn,
		Message: "Linear MCP not found or not connected",
		Hint: "run \"claude mcp add --transport http --scope project linear https://mcp.linear.app/mcp\" in your project root\n" +
			"  (a fully compatible local-only Linear MCP alternative is planned — check the project README for updates)",
	}
}

// CheckClaudeInference determines if the Claude CLI can perform inference
// by interpreting the result of a minimal "1+1=" prompt.
func CheckClaudeInference(output string, err error) domain.DoctorCheck {
	if err != nil {
		return domain.DoctorCheck{
			Name:    "claude-inference",
			Status:  domain.CheckWarn,
			Message: "inference failed: " + err.Error(),
			Hint: `"signal: killed" = CLI startup too slow (timeout 3m); ` +
				`"nested session" = CLAUDECODE env var leaked (doctor should filter it); ` +
				`otherwise check API key, quota, and model access`,
		}
	}
	if strings.TrimSpace(output) != "2" {
		return domain.DoctorCheck{
			Name:    "claude-inference",
			Status:  domain.CheckWarn,
			Message: "unexpected response: " + strings.TrimSpace(output),
			Hint:    "model returned unexpected output; check model access and API quota",
		}
	}
	return domain.DoctorCheck{
		Name:    "claude-inference",
		Status:  domain.CheckOK,
		Message: "inference OK",
	}
}

// CheckSkillMD verifies that both dmail-sendable and dmail-readable SKILL.md files exist.
func CheckSkillMD(repoRoot string) domain.DoctorCheck {
	skillsDir := filepath.Join(repoRoot, domain.StateDir, "skills")
	required := []string{"dmail-sendable", "dmail-readable"}
	var missing []string
	for _, name := range required {
		path := filepath.Join(skillsDir, name, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return domain.DoctorCheck{
			Name:    "SKILL.md",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("missing: %s — run 'amadeus init'", strings.Join(missing, ", ")),
			Hint:    `run "amadeus init" to regenerate skill files`,
		}
	}
	// Check for deprecated "kind: feedback" (split into design-feedback / implementation-feedback)
	for _, name := range required {
		path := filepath.Join(skillsDir, name, "SKILL.md")
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "kind: feedback") &&
			!strings.Contains(content, "kind: design-feedback") &&
			!strings.Contains(content, "kind: implementation-feedback") {
			return domain.DoctorCheck{
				Name:    "SKILL.md",
				Status:  domain.CheckFail,
				Message: fmt.Sprintf("%s/SKILL.md uses deprecated kind 'feedback'", name),
				Hint:    "deprecated kind 'feedback'; migrate to 'design-feedback' or 'implementation-feedback' (run 'amadeus init --force' to regenerate SKILL.md)",
			}
		}
	}

	return domain.DoctorCheck{
		Name:    "SKILL.md",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s (dmail-sendable, dmail-readable)", skillsDir),
	}
}

// RunDoctorWithClaudeCmd executes all health checks with a configurable Claude command.

func ExtractStreamResult(streamJSON string) string {
	for _, line := range strings.Split(streamJSON, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg struct {
			Type   string `json:"type"`
			Result string `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &msg); err == nil && msg.Type == "result" {
			return msg.Result
		}
	}
	return ""
}

// CheckContextBudget parses stream-json output from a Claude CLI invocation
// and reports context budget health based on hooks, plugins, skills, and MCP servers.
func CheckContextBudget(streamJSON string, baseDir string) domain.DoctorCheck {
	var messages []*platform.StreamMessage
	for _, line := range strings.Split(streamJSON, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		msg, err := platform.ParseStreamMessage([]byte(line))
		if err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	report := platform.CalculateContextBudget(messages)
	breakdown := report.DetailedBreakdown()

	// Build detailed message lines
	var lines []string
	for _, item := range breakdown {
		marker := ""
		if item.Heaviest {
			marker = " <- heaviest"
		}
		if item.Category == "hooks" {
			if item.Bytes > 0 {
				lines = append(lines, fmt.Sprintf("  hooks: %d bytes (%d tok)%s", item.Bytes, item.Tokens, marker))
			}
		} else {
			if item.Count > 0 {
				lines = append(lines, fmt.Sprintf("  %s: %d (%d tok)%s", item.Category, item.Count, item.Tokens, marker))
			}
		}
	}

	status := domain.CheckOK
	msg := fmt.Sprintf("estimated %d tokens", report.EstimatedTokens)
	if report.Exceeds(platform.DefaultContextBudgetThreshold) {
		status = domain.CheckWarn
		msg = fmt.Sprintf("estimated %d tokens (threshold: %d)", report.EstimatedTokens, platform.DefaultContextBudgetThreshold)
	}
	if len(lines) > 0 {
		msg += "\n" + strings.Join(lines, "\n")
	}

	result := domain.DoctorCheck{
		Name:    "context-budget",
		Status:  status,
		Message: msg,
	}

	// Hint logic: only when threshold exceeded
	if report.Exceeds(platform.DefaultContextBudgetThreshold) {
		projectSettings := filepath.Join(baseDir, ".claude", "settings.json")
		if _, err := os.Stat(projectSettings); err == nil {
			result.Hint = ".claude/settings.json の設定を見直してください"
		} else {
			var heaviest string
			for _, item := range breakdown {
				if item.Heaviest {
					heaviest = item.Category
					break
				}
			}
			switch heaviest {
			case "mcp_servers":
				result.Hint = ".claude/settings.json をプロジェクトに作成し、必要な MCP server のみ定義を推奨"
			case "tools":
				result.Hint = "tools は plugins/MCP 由来 → .claude/settings.json で plugins/MCP を絞ることを推奨"
			default:
				result.Hint = ".claude/settings.json をプロジェクトに作成し、必要なプラグインのみ有効化を推奨"
			}
		}
	}

	return result
}

// CheckGHAuth verifies that the GitHub CLI is authenticated by running
// `gh auth status`. Returns OK if authenticated, WARN if not.
func CheckGHAuth(ctx context.Context) domain.DoctorCheck {
	cmd := exec.CommandContext(ctx, "gh", "auth", "status", "--active")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return domain.DoctorCheck{
			Name:    "gh-auth",
			Status:  domain.CheckWarn,
			Message: "gh not authenticated",
			Hint:    "run 'gh auth login' to authenticate",
		}
	}
	output := string(out)
	if strings.Contains(output, "Logged in") || strings.Contains(output, "✓") {
		return domain.DoctorCheck{
			Name:    "gh-auth",
			Status:  domain.CheckOK,
			Message: "gh authenticated",
		}
	}
	return domain.DoctorCheck{
		Name:    "gh-auth",
		Status:  domain.CheckWarn,
		Message: "gh auth status unclear: " + output,
		Hint:    "run 'gh auth login' to authenticate",
	}
}
