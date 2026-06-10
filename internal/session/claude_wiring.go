package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
)

// Project wiring for the jun15 MCP pivot entry skill (refs issue 0032,
// decision D5(a), conformance constraints C4/C5). This file is
// intentionally separate from mcp_config.go / state.go: mcp_config.go
// is canonical-locked across the sibling tools.

// InstallClaudeSkills materializes the embedded Claude Code entry
// skills into the target project's .claude/skills/: a bare `claude`
// session auto-discovers project skills there, so /review-gate works
// without plugin machinery or launch flags. Idempotent: files are
// rewritten only when the embedded template changed.
func InstallClaudeSkills(baseDir string, skillsFS fs.FS, logger domain.Logger) error {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	const srcPrefix = "templates/claude-skills"
	destRoot := filepath.Join(baseDir, ".claude", "skills")

	return fs.WalkDir(skillsFS, srcPrefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(srcPrefix, path)
		if relErr != nil {
			return fmt.Errorf("relative path for %s: %w", path, relErr)
		}
		dest := filepath.Join(destRoot, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		data, err := fs.ReadFile(skillsFS, path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		existing, readErr := os.ReadFile(dest)
		if readErr != nil || !bytes.Equal(existing, data) {
			if readErr == nil {
				logger.Info("updated SKILL.md: %s (template changed)", rel)
			}
			return os.WriteFile(dest, data, 0o644)
		}
		return nil
	})
}

// RootMCPConfigPath returns the project-root .mcp.json path — the file
// Claude Code auto-discovers (project scope, pending-approval flow).
func RootMCPConfigPath(baseDir string) string {
	return filepath.Join(baseDir, ".mcp.json")
}

// UpsertRootMCPConfig merge-writes this tool's MCP server entry into
// the project-root .mcp.json: existing entries from sibling tools (and
// any foreign top-level keys) are preserved so all five tap tools can
// share one root config for omni-sessions. Idempotent; the state-dir
// .mcp.json written by GenerateMCPConfig stays as the isolated
// `sessions enter` wiring.
func UpsertRootMCPConfig(baseDir string) (string, error) {
	path := RootMCPConfigPath(baseDir)

	root := map[string]json.RawMessage{}
	if data, err := os.ReadFile(path); err == nil {
		if jsonErr := json.Unmarshal(data, &root); jsonErr != nil {
			return "", fmt.Errorf("root .mcp.json invalid JSON (fix or remove it): %w", jsonErr)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("read root .mcp.json: %w", err)
	}

	servers := map[string]json.RawMessage{}
	if raw, ok := root["mcpServers"]; ok {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return "", fmt.Errorf("root .mcp.json mcpServers invalid: %w", err)
		}
	}
	entry, err := json.Marshal(MCPServerEntry{Command: "amadeus", Args: []string{"mcp"}})
	if err != nil {
		return "", fmt.Errorf("marshal server entry: %w", err)
	}
	servers["amadeus"] = entry
	serversRaw, err := json.Marshal(servers)
	if err != nil {
		return "", fmt.Errorf("marshal mcpServers: %w", err)
	}
	root["mcpServers"] = serversRaw

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal root .mcp.json: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("write root .mcp.json: %w", err)
	}
	return path, nil
}
