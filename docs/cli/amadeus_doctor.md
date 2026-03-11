## amadeus doctor

Run health checks

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

### Checks

The doctor command runs multiple health checks including:

- **git** — Git repository availability
- **git-remote** — Remote repository connectivity
- **gh** — GitHub CLI availability
- **claude** — Claude Code CLI availability
- **config** — `.gate/config.yaml` validity
- **fsnotify** — File system notification support
- **context-budget** — Estimates token overhead from tools, skills, plugins, MCP servers, and hook stdout in the Claude CLI context window. Warns when estimated usage exceeds a threshold (default 20K tokens).

### SEE ALSO

* [amadeus](amadeus.md)	 - Divergence meter for your codebase

