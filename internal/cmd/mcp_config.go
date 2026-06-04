package cmd

import (
	"fmt"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
	"github.com/spf13/cobra"
)

func newMCPConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp-config",
		Short: "Manage MCP wiring for Claude Code sessions",
		Long: `Manage the .mcp.json file that controls which MCP servers
are available to Claude Code sessions.

Use 'generate' to create the initial config, then edit it to add or remove
MCP servers as needed. Claude Code uses --strict-mcp-config to enforce this
allowlist when the file exists.`,
		Example: `  amadeus mcp-config generate
  amadeus mcp-config generate --linear
  amadeus mcp-config generate --force`,
	}

	cmd.AddCommand(newMCPConfigGenerateCommand())
	return cmd
}

func newMCPConfigGenerateCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "generate [path]",
		Short: "Generate .mcp.json and .claude/settings.json for Claude Code sessions",
		Long: `Generate .mcp.json and .claude/settings.json for Claude Code MCP sessions.

.mcp.json controls which MCP servers are available:
  - wave mode (default): empty config (no MCP servers)
  - linear mode (--linear): includes Linear MCP server

.claude/settings.json disables plugins so the session uses only the configured MCP surface.

Claude Code uses --strict-mcp-config to enforce the MCP allowlist.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := "."
			if len(args) > 0 {
				baseDir = args[0]
			}

			logger := platform.NewLogger(cmd.ErrOrStderr(), false)
			linearFlag := mustBool(cmd, "linear")
			mode := domain.NewTrackingMode(linearFlag)

			path, genErr := session.GenerateMCPConfig(baseDir, mode, force)
			if genErr != nil {
				if strings.Contains(genErr.Error(), "already exists") {
					logger.Warn("mcp: %v", genErr)
					path = session.MCPConfigPath(baseDir)
				} else {
					return genErr
				}
			} else {
				logger.OK("Generated %s (mode: %s)", path, mode)
				if mode.IsWave() {
					logger.Info("Empty config — no MCP servers. Edit to add custom servers.")
				} else {
					logger.Info("Linear MCP server included. Edit to add/remove servers.")
				}
			}

			settingsPath, settingsErr := session.GenerateClaudeSettings(baseDir, force)
			if settingsErr != nil {
				if strings.Contains(settingsErr.Error(), "already exists") {
					logger.Warn("settings: %v", settingsErr)
				} else {
					return fmt.Errorf("settings: %w", settingsErr)
				}
			} else {
				logger.OK("Generated %s", settingsPath)
			}

			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing .mcp.json")
	return cmd
}
