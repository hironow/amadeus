## amadeus mcp-config

Manage MCP wiring for Claude Code sessions

### Synopsis

Manage the .mcp.json file that controls which MCP servers
are available to Claude Code sessions.

Use 'generate' to create the initial config, then edit it to add or remove
MCP servers as needed. Claude Code uses --strict-mcp-config to enforce this
allowlist when the file exists.

### Examples

```
  amadeus mcp-config generate
  amadeus mcp-config generate --linear
  amadeus mcp-config generate --force
```

### Options

```
  -h, --help   help for mcp-config
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
* [amadeus mcp-config generate](amadeus_mcp-config_generate.md)	 - Generate .mcp.json and .claude/settings.json for Claude Code sessions

