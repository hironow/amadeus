package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
)

// MCPConfig is the JSON structure for --mcp-config.
type MCPConfig struct {
	MCPServers map[string]MCPServerEntry `json:"mcpServers"`
}

// MCPServerEntry defines a single MCP server.
type MCPServerEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// ClaudeSettingsPath returns the path to the .claude/settings.json file
// under the tool state directory.
func ClaudeSettingsPath(baseDir string) string {
	return filepath.Join(baseDir, domain.StateDir, ".claude", "settings.json")
}

// ClaudeSettingsExists reports whether .claude/settings.json exists.
func ClaudeSettingsExists(baseDir string) bool {
	_, err := os.Stat(ClaudeSettingsPath(baseDir))
	return err == nil
}

// GenerateClaudeSettings creates a minimal .claude/settings.json that
// disables all plugins for Claude subprocess isolation.
func GenerateClaudeSettings(baseDir string, force bool) (string, error) {
	path := ClaudeSettingsPath(baseDir)

	if !force {
		if _, err := os.Stat(path); err == nil {
			return path, fmt.Errorf(".claude/settings.json already exists (use --force to overwrite)")
		}
	}

	settings := map[string]any{
		"enabledPlugins": map[string]any{},
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal settings: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("write settings: %w", err)
	}

	return path, nil
}

// MCPConfigPath returns the path to the .mcp.json file.
func MCPConfigPath(baseDir string) string {
	return filepath.Join(baseDir, domain.StateDir, ".mcp.json")
}

// MCPConfigExists reports whether the .mcp.json file exists and is valid JSON.
func MCPConfigExists(baseDir string) (bool, error) {
	path := MCPConfigPath(baseDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false, fmt.Errorf(".mcp.json invalid JSON: %w", err)
	}
	return true, nil
}

// GenerateMCPConfig creates a .mcp.json file.
// Linear mode includes Linear MCP server. Wave mode produces empty config.
func GenerateMCPConfig(baseDir string, mode domain.TrackingMode, force bool) (string, error) {
	path := MCPConfigPath(baseDir)

	if !force {
		if _, err := os.Stat(path); err == nil {
			return path, fmt.Errorf(".mcp.json already exists (use --force to overwrite)")
		}
	}

	cfg := MCPConfig{
		MCPServers: make(map[string]MCPServerEntry),
	}

	if mode.IsLinear() {
		cfg.MCPServers["linear"] = MCPServerEntry{
			Command: "npx",
			Args:    []string{"-y", "@anthropic-ai/claude-code-mcp-adapter", "--transport", "http", "https://mcp.linear.app/mcp"},
		}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal mcp-config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write mcp-config: %w", err)
	}

	return path, nil
}
