## amadeus mcp

Run amadeus as an MCP server over stdio (refs/issues/0027 Phase 2b MVP)

### Synopsis

Start a Model Context Protocol server reading JSON-RPC 2.0
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
the legacy .mcp.json file consumed by the embedded claude_adapter).

```
amadeus mcp [flags]
```

### Examples

```
  # Launch claude code with the amadeus MCP server attached
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

