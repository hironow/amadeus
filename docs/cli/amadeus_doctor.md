## amadeus doctor

Run health checks

### Synopsis

Run health checks on the amadeus environment.

Each check reports one of four statuses: OK (passed), FAIL (exit 1),
SKIP (dependency missing), WARN (advisory, exit 0).

The context-budget check estimates token consumption per category
(tools, skills, plugins, mcp, hooks) and marks the heaviest.
When the threshold (20,000 tokens) is exceeded, a category-specific
hint recommends adjusting .claude/settings.json.

```
amadeus doctor [path] [flags]
```

### Options

```
  -h, --help     help for doctor
  -j, --json     output as JSON
      --repair   Auto-fix repairable issues
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

