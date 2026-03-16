## amadeus log

Show divergence log

### Synopsis

Display the divergence log from the event store.

Reads events from .gate/events/ and presents a chronological log of
divergence checks, D-Mail generation, and sync activity. If [path] is
omitted, the current working directory is used. Use --json to output
structured JSON for piping into downstream commands.

```
amadeus log [path] [flags]
```

### Examples

```
  # Show divergence log for current directory
  amadeus log

  # Output as JSON for scripting
  amadeus log --json

  # Show log for a specific project
  amadeus log /path/to/project
```

### Options

```
  -h, --help   help for log
  -j, --json   output as JSON
```

### Options inherited from parent commands

```
  -c, --config string   config file path
  -l, --lang string     output language (ja, en)
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -q, --quiet           Suppress all stderr output
  -v, --verbose         verbose output
```

### SEE ALSO

* [amadeus](amadeus.md)	 - Divergence meter for your codebase

