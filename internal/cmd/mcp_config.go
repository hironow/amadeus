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
		Short: "Manage MCP configuration for Claude subprocess isolation",
		Long: `Manage the .mcp.json file that controls which MCP servers
are available to Claude subprocess invocations.

Use 'generate' to create the initial config, then edit it to add or remove
MCP servers as needed. Claude subprocess uses --strict-mcp-config to enforce
this allowlist when the file exists.`,
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
		Short: "Generate .mcp.json and .claude/settings.json for subprocess isolation",
		Long: `Generate .mcp.json and .claude/settings.json for Claude subprocess isolation.

.mcp.json controls which MCP servers are available:
  - wave mode (default): empty config (no MCP servers)
  - linear mode (--linear): includes Linear MCP server

.claude/settings.json disables all plugins for the subprocess.

Claude subprocess uses --strict-mcp-config to enforce the MCP allowlist.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := "."
			if len(args) > 0 {
				baseDir = args[0]
			}

			logger := platform.NewLogger(cmd.ErrOrStderr(), false)
			linearFlag, _ := cmd.Flags().GetBool("linear")
			mode := domain.NewTrackingMode(linearFlag)

			path, genErr := session.GenerateMCPConfig(baseDir, mode, force)
			if genErr != nil {
				return genErr
			}

			logger.OK("Generated %s (mode: %s)", path, mode)
			if mode.IsWave() {
				logger.Info("Empty config — no MCP servers. Edit to add custom servers.")
			} else {
				logger.Info("Linear MCP server included. Edit to add/remove servers.")
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
