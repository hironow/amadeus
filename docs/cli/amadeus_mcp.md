## amadeus mcp

Run amadeus as an MCP server over stdio (review queue + PR comment data plane)

### Synopsis

Start a Model Context Protocol server reading JSON-RPC 2.0
messages on stdin and writing responses on stdout.

Designed for embedding in a Claude Code interactive session via
--mcp-config so inference stays on the session's subscription quota
rather than crossing into the Agent SDK credit pool that gates
'claude -p' from 2026-06-15.

Exposes amadeus.ping, amadeus.next_review + amadeus.get_pr_status
(read the gate event store + convergence projection), and
amadeus.post_comment (posts a review comment to GitHub via
'gh pr comment' when a CommentPoster is wired; cmd wires one by
default).

Not to be confused with 'amadeus mcp-config' (subcommand writing
the Claude Code MCP allowlist that points back to this stdio server).

```
amadeus mcp [flags]
```

### Examples

```
  # Launch Claude Code with the amadeus MCP server attached
  claude --mcp-config '{"amadeus":{"command":"amadeus","args":["mcp"]}}'

  # Pipe a tools/list request manually (for debugging)
  echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | amadeus mcp
```

### Options

```
  -h, --help   help for mcp
```

### Options inherited from parent commands

```
  -c, --config string   config file path
  -l, --lang string     output language (ja, en)
      --linear          Use Linear MCP for issue tracking (default: wave-centric mode)
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -q, --quiet           Suppress all stderr output
  -v, --verbose         verbose output
```

### SEE ALSO

* [amadeus](amadeus.md)	 - Divergence meter for your codebase

