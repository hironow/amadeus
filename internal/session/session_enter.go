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
	IsolationFlags    []string  // provider-specific subprocess isolation flags
	Stdin             io.Reader // injected by cmd layer (typically os.Stdin)
	Stdout            io.Writer // injected by cmd layer (typically os.Stdout)
	Stderr            io.Writer // injected by cmd layer (typically os.Stderr)
}

// EnterSession launches the provider CLI in interactive mode, resuming a
// specific session. The IsolationFlags are provider-specific (see
// BuildClaudeIsolationFlags for the Claude implementation).
// stdin/stdout/stderr are connected to the user's TTY.
func EnterSession(ctx context.Context, cfg EnterConfig) error {
	if cfg.WorkDir == "" {
		return fmt.Errorf("EnterSession: WorkDir is required to prevent CWD drift")
	}
	if cfg.ProviderSessionID == "" {
		return fmt.Errorf("EnterSession: ProviderSessionID is required")
	}

	args := make([]string, 0, len(cfg.IsolationFlags)+2)
	args = append(args, cfg.IsolationFlags...)
	args = append(args, "--resume", cfg.ProviderSessionID)

	cmd := platform.NewShellCmd(ctx, cfg.ProviderCmd, args...)
	cmd.Dir = cfg.WorkDir
	cmd.Stdin = cfg.Stdin
	cmd.Stdout = cfg.Stdout
	cmd.Stderr = cfg.Stderr

	return cmd.Run()
}

// BuildClaudeIsolationFlags returns the Claude-specific isolation flags for
// subprocess invocation. These match the flags used by ClaudeAdapter but omit
// --print and --output-format since this is an interactive session.
// This is a Claude-specific runtime seam (see S0037).
func BuildClaudeIsolationFlags(configBase string) []string {
	var args []string
	args = append(args, "--setting-sources", "")
	args = append(args, "--disable-slash-commands")

	if settingsPath := ClaudeSettingsPath(configBase); ClaudeSettingsExists(configBase) {
		args = append(args, "--settings", settingsPath)
	}
	if mcpPath := ResolveMCPConfigPath(configBase); mcpPath != "" {
		args = append(args, "--strict-mcp-config", "--mcp-config", mcpPath)
	}

	return args
}
