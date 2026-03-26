## amadeus status

Show amadeus operational status

### Synopsis

Display operational status including check history, divergence,
success rate, and pending d-mail counts.

Output goes to stdout by default (human-readable text).
Use -o json for machine-readable JSON output to stdout.

```
amadeus status [path] [flags]
```

### Examples

```
  # Show status for current directory
  amadeus status

  # Show status for a specific project
  amadeus status /path/to/project

  # JSON output for scripting
  amadeus status -o json
```

### Options

```
  -h, --help   help for status
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

* [amadeus](amadeus.md)  - Divergence meter for your codebase
