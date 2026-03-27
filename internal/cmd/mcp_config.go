package cmd

import (
	"fmt"

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
		Short: "Generate .mcp.json for --strict-mcp-config isolation",
		Long: `Generate a .mcp.json file that controls which MCP servers
are available to Claude subprocess invocations.

In wave mode (default): generates empty config (no MCP servers).
In linear mode (--linear): includes Linear MCP server.

The generated file can be freely edited to add custom MCP servers.
Claude subprocess uses --strict-mcp-config to enforce this allowlist.`,
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
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing .mcp.json")
	return cmd
}
