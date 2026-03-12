## amadeus doctor

Run health checks against the current environment and project configuration.

Each check reports one of four statuses:

- **OK** — check passed
- **FAIL** — check failed (exit code 1)
- **SKIP** — check was not applicable
- **WARN** — advisory warning (exit code 0, not a failure)

The `context-budget` check estimates total token usage across categories
(tools, skills, plugins, mcp, hooks). When the threshold is exceeded
(default 20,000 tokens), it reports WARN with a per-category token
breakdown, marks the heaviest category, and provides a category-specific
hint (e.g., trimming plugins via `.claude/settings.json`).

```
amadeus doctor [path] [flags]
```

### Options

```
  -h, --help   help for doctor
  -j, --json   output as JSON
```

### Options inherited from parent commands

```
  -c, --config string   config file path
  -l, --lang string     output language (ja, en)
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         verbose output
```

### SEE ALSO

* [amadeus](amadeus.md)	 - Divergence meter for your codebase

