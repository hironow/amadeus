## amadeus mark-commented

Record that a D-Mail has been posted as a comment

### Synopsis

Mark a D-Mail × Issue pair as commented in the sync state.

```
amadeus mark-commented <dmail-name> <issue-id> [path] [flags]
```

### Examples

```
  # Mark a D-Mail as commented on an issue
  amadeus mark-commented calibration-2024-01-15 PROJ-42

  # Mark in a specific project directory
  amadeus mark-commented calibration-2024-01-15 PROJ-42 /path/to/project

  # Output confirmation as JSON
  amadeus mark-commented calibration-2024-01-15 PROJ-42 --json
```

### Options

```
  -h, --help   help for mark-commented
  -j, --json   output as JSON
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

