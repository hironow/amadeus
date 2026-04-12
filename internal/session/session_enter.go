package session

import (
	"context"
	"fmt"
	"io"

	"github.com/hironow/amadeus/internal/platform"
)

// EnterConfig holds configuration for interactive session re-entry.
type EnterConfig struct {
	ProviderCmd       string    // CLI command (from config.claude_cmd — legacy seam)
	ProviderSessionID string    // provider native session ID
	WorkDir           string    // working directory (from session record)
	ConfigBase        string    // base directory for resolving stateDir settings
	Stdin             io.Reader // injected by cmd layer (typically os.Stdin)
	Stdout            io.Writer // injected by cmd layer (typically os.Stdout)
	Stderr            io.Writer // injected by cmd layer (typically os.Stderr)
}

// EnterSession launches the provider CLI in interactive mode with --resume.
// stdin/stdout/stderr are connected to the user's TTY.
// This function uses Claude-specific isolation flags (see S0037 runtime seams).
// It hands control to the user's terminal.
func EnterSession(ctx context.Context, cfg EnterConfig) error {
	if cfg.WorkDir == "" {
		return fmt.Errorf("EnterSession: WorkDir is required to prevent CWD drift")
	}
	if cfg.ProviderSessionID == "" {
		return fmt.Errorf("EnterSession: ProviderSessionID is required")
	}

	args := buildIsolationFlags(cfg)
	args = append(args, "--resume", cfg.ProviderSessionID)

	cmd := platform.NewShellCmd(ctx, cfg.ProviderCmd, args...)
	cmd.Dir = cfg.WorkDir
	cmd.Stdin = cfg.Stdin
	cmd.Stdout = cfg.Stdout
	cmd.Stderr = cfg.Stderr

	return cmd.Run()
}

// buildIsolationFlags returns the isolation flags for subprocess invocation.
// These match the flags used by ClaudeAdapter but omit --print and --output-format
// since this is an interactive session.
func buildIsolationFlags(cfg EnterConfig) []string {
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
