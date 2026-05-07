## amadeus dashboard

Show TAP ecosystem divergence dashboard

### Synopsis

Display cross-repository divergence status for the TAP ecosystem.

Shows divergence scores for all tools (phonewave, sightjack, paintress, amadeus)
with an aggregated ecosystem score.

Default tool directory resolution uses sibling directory convention:
  ../phonewave/.phonewave/  ../sightjack/.siren/
  ../paintress/.expedition/  ./.gate/

Output goes to stdout (human-readable text by default).
Use -o json for machine-readable JSON output.

```
amadeus dashboard [path] [flags]
```

### Examples

```
  # Show ecosystem dashboard
  amadeus dashboard

  # Show dashboard for a specific project root
  amadeus dashboard /path/to/project

  # JSON output for scripting
  amadeus dashboard -o json

  # Custom tool directories (comma-separated tool=dir pairs)
  amadeus dashboard --tool-dirs "phonewave=/other/.phonewave,sightjack=/other/.siren"
```

### Options

```
  -h, --help               help for dashboard
      --tool-dirs string   Override tool state dirs (comma-separated tool=dir pairs)
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

