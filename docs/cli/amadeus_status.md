## amadeus status

Show amadeus operational status

### Synopsis

Display operational status including check history, divergence,
success rate, and pending d-mail counts.

Output goes to stderr (human-readable) by default.
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
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         verbose output
```

### SEE ALSO

* [amadeus](amadeus.md)	 - Divergence meter for your codebase

