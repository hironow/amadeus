## amadeus improvement-stats

Show improvement outcome statistics

### Synopsis

Display outcome statistics from the self-improving loop.
Shows resolved/failed_again/escalated/pending counts grouped by failure type.

Output goes to stdout by default (human-readable text).
Use -o json for machine-readable JSON output.

```
amadeus improvement-stats [path] [flags]
```

### Examples

```
  # Show improvement stats for current directory
  amadeus improvement-stats

  # JSON output for scripting
  amadeus improvement-stats -o json
```

### Options

```
  -h, --help            help for improvement-stats
  -o, --output string   Output format: json
```

### Options inherited from parent commands

```
  -c, --config string   config file path
  -l, --lang string     output language (ja, en)
      --linear          Use Linear MCP for issue tracking (default: wave-centric mode)
      --no-color        Disable colored output (respects NO_COLOR env)
  -q, --quiet           Suppress all stderr output
  -v, --verbose         verbose output
```

### SEE ALSO

* [amadeus](amadeus.md)	 - Divergence meter for your codebase

