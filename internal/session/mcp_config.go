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
type MCPConfig struct { // nosemgrep: structure.multiple-exported-structs-go -- MCP config family (MCPConfig/MCPServerEntry) is a cohesive JSON wire-format pair for --mcp-config generation; splitting would fragment GenerateMCPConfig [permanent]
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

// legacyMCPConfigPath returns the pre-rename path (.run/mcp-config.json)
// for backward compatibility during migration.
func legacyMCPConfigPath(baseDir string) string {
	return filepath.Join(baseDir, domain.StateDir, ".run", "mcp-config.json")
}

// ResolveMCPConfigPath returns the active MCP config path, preferring the
// new .mcp.json location and falling back to the legacy .run/mcp-config.json.
func ResolveMCPConfigPath(baseDir string) string {
	newPath := MCPConfigPath(baseDir)
	if _, err := os.Stat(newPath); err == nil {
		return newPath
	}
	legacyPath := legacyMCPConfigPath(baseDir)
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath
	}
	return ""
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
// If a legacy .run/mcp-config.json exists and the new file does not, the
// legacy content is migrated forward to preserve custom MCP server entries.
func GenerateMCPConfig(baseDir string, mode domain.TrackingMode, force bool) (string, error) {
	path := MCPConfigPath(baseDir)

	if !force {
		if _, err := os.Stat(path); err == nil {
			return path, fmt.Errorf(".mcp.json already exists (use --force to overwrite)")
		}
	}

	// Migrate legacy config if it exists and new file doesn't.
	// Skip migration on --force to allow clean regeneration from defaults.
	cfg := MCPConfig{
		MCPServers: make(map[string]MCPServerEntry),
	}
	if !force {
		legacyPath := legacyMCPConfigPath(baseDir)
		if legacyData, err := os.ReadFile(legacyPath); err == nil {
			if jsonErr := json.Unmarshal(legacyData, &cfg); jsonErr != nil {
				cfg.MCPServers = make(map[string]MCPServerEntry)
			}
		}
	}

	// Enforce mode: wave mode = no MCP servers, linear mode = add linear server
	if mode.IsWave() {
		delete(cfg.MCPServers, "linear") // Remove stale linear server from legacy migration
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
