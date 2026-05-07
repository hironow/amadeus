## amadeus mcp-config generate

Generate .mcp.json and .claude/settings.json for subprocess isolation

### Synopsis

Generate .mcp.json and .claude/settings.json for Claude subprocess isolation.

.mcp.json controls which MCP servers are available:
  - wave mode (default): empty config (no MCP servers)
  - linear mode (--linear): includes Linear MCP server

.claude/settings.json disables all plugins for the subprocess.

Claude subprocess uses --strict-mcp-config to enforce the MCP allowlist.

```
amadeus mcp-config generate [path] [flags]
```

### Options

```
      --force   Overwrite existing .mcp.json
  -h, --help    help for generate
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

* [amadeus mcp-config](amadeus_mcp-config.md)	 - Manage MCP configuration for Claude subprocess isolation

