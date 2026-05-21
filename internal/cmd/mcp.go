package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

// newMCPCommand exposes `amadeus mcp` as a stdio MCP server entry
// point for the refs/issues/0027 jun15 MCP pivot Phase 2b. A claude
// code interactive session loads this binary via --mcp-config and
// calls amadeus tools from inside the human-initiated subscription
// quota.
//
// Phase 2b MVP exposes amadeus.ping + 3 stubs (next_review,
// post_comment, get_pr_status). Real tool wiring lands in subsequent
// commits on feat/jun15-mcp-pivot.
//
// Distinct from `amadeus mcp-config` which manages the .mcp.json
// configuration consumed by the legacy claude_adapter. This server
// is consumed by claude code itself.
func newMCPCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run amadeus as an MCP server over stdio (refs/issues/0027 Phase 2b MVP)",
		Long: `Start a Model Context Protocol server reading JSON-RPC 2.0
messages on stdin and writing responses on stdout.

Designed for embedding in a claude code interactive session via
--mcp-config so inference stays on the session's subscription quota
rather than crossing into the Agent SDK credit pool that gates
'claude -p' from 2026-06-15.

Phase 2b MVP scope: amadeus.ping + 3 stubs (amadeus.next_review,
amadeus.post_comment, amadeus.get_pr_status). Real wiring against
the review queue, GitHub Comments API, and convergence projection
ships in subsequent commits on the feat/jun15-mcp-pivot branch.

Not to be confused with 'amadeus mcp-config' (subcommand managing
the legacy .mcp.json file consumed by the embedded claude_adapter).`,
		Example: `  # Launch claude code with the amadeus MCP server attached
  claude --mcp-config '{"amadeus":{"command":"amadeus","args":["mcp"]}}'

  # Pipe a tools/list request manually (for debugging)
  echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | amadeus mcp`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			gateDir := filepath.Join(cwd, domain.StateDir)
			srv := session.NewMCPServer(cmd.InOrStdin(), cmd.OutOrStdout(), nil).WithGateDir(gateDir)
			return srv.Serve(cmd.Context())
		},
	}
}
