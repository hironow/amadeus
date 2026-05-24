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
// Exposes amadeus.ping + amadeus.next_review + amadeus.get_pr_status
// (read the gate event store / convergence projection) +
// amadeus.post_comment (posts to GitHub via `gh pr comment`).
//
// Distinct from `amadeus mcp-config` which manages the .mcp.json
// configuration consumed by the legacy claude_adapter. This server
// is consumed by claude code itself.
func newMCPCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run amadeus as an MCP server over stdio (review queue + PR comment data plane)",
		Long: `Start a Model Context Protocol server reading JSON-RPC 2.0
messages on stdin and writing responses on stdout.

Designed for embedding in a claude code interactive session via
--mcp-config so inference stays on the session's subscription quota
rather than crossing into the Agent SDK credit pool that gates
'claude -p' from 2026-06-15.

Exposes amadeus.ping, amadeus.next_review + amadeus.get_pr_status
(read the gate event store + convergence projection), and
amadeus.post_comment (posts a review comment to GitHub via
'gh pr comment' when a CommentPoster is wired; cmd wires one by
default).

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
			// CommentPoster is wired unconditionally: the MCP tool only
			// fires when the human-initiated claude-code session calls
			// post_comment, so the adapter being present does not produce
			// side effects on its own. Errors from `gh pr comment` surface
			// to the session via the response's reason field.
			poster := session.NewGhPRWriter(cwd)
			srv := session.NewMCPServer(cmd.InOrStdin(), cmd.OutOrStdout(), nil).
				WithGateDir(gateDir).
				WithCommentPoster(poster)
			return srv.Serve(cmd.Context())
		},
	}
}
