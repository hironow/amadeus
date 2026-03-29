package session

import (
	"context"
	"fmt"
	"os"

	"github.com/hironow/amadeus/internal/platform"
)

// EnterConfig holds configuration for interactive session re-entry.
type EnterConfig struct {
	ProviderCmd       string // CLI command (from config.claude_cmd)
	ProviderSessionID string // provider native session ID
	WorkDir           string // working directory (from session record)
	ConfigBase        string // base directory for resolving stateDir settings
}

// EnterSession launches the provider CLI in interactive mode with --resume.
// stdin/stdout/stderr are connected to the user's TTY. This function does NOT
// use ClaudeAdapter — it hands control to the user's terminal.
func EnterSession(ctx context.Context, cfg EnterConfig) error {
	if cfg.WorkDir == "" {
		return fmt.Errorf("EnterSession: WorkDir is required to prevent CWD drift")
	}
	if cfg.ProviderSessionID == "" {
		return fmt.Errorf("EnterSession: ProviderSessionID is required")
	}

	args := buildEnterIsolationFlags(cfg)
	args = append(args, "--resume", cfg.ProviderSessionID)

	cmd := platform.NewShellCmd(ctx, cfg.ProviderCmd, args...)
	cmd.Dir = cfg.WorkDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func buildEnterIsolationFlags(cfg EnterConfig) []string {
	var args []string
	args = append(args, "--setting-sources", "")
	args = append(args, "--disable-slash-commands")

	if settingsPath := ClaudeSettingsPath(cfg.ConfigBase); ClaudeSettingsExists(cfg.ConfigBase) {
		args = append(args, "--settings", settingsPath)
	}
	if mcpPath := ResolveMCPConfigPath(cfg.ConfigBase); mcpPath != "" {
		args = append(args, "--strict-mcp-config", "--mcp-config", mcpPath)
	}

	return args
}
