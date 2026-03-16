## amadeus archive-prune

Prune old archived files

### Synopsis

Prune archived d-mail files and expired event files.

By default, runs in dry-run mode showing what would be deleted.
Pass --execute to actually remove the files.

```
amadeus archive-prune [path] [flags]
```

### Examples

```
  # Dry-run: list expired files (default 30 days)
  amadeus archive-prune

  # Delete expired files (with confirmation)
  amadeus archive-prune --execute

  # Delete without confirmation
  amadeus archive-prune --execute --yes

  # Custom retention period
  amadeus archive-prune --days 7 --execute

  # JSON output for scripting
  amadeus archive-prune -o json

  # Rebuild archive index from existing files
  amadeus archive-prune --rebuild-index
```

### Options

```
  -d, --days int        Retention days (default 30)
  -n, --dry-run         Dry-run mode (default behavior, explicit for scripting)
  -x, --execute         Execute pruning (default: dry-run)
  -h, --help            help for archive-prune
      --rebuild-index   Rebuild archive index from existing files without pruning
  -y, --yes             Skip confirmation prompt
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

