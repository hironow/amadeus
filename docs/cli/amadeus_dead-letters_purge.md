## amadeus dead-letters purge

Purge dead-lettered outbox items

### Synopsis

Remove outbox items that have exceeded the maximum retry count.

By default, runs in dry-run mode showing the count of dead-lettered items.
Pass --execute to actually delete them.

```
amadeus dead-letters purge [path] [flags]
```

### Examples

```
  # Dry-run: show count of dead-lettered items
  amadeus dead-letters purge

  # Delete dead-lettered items (with confirmation)
  amadeus dead-letters purge --execute

  # Delete without confirmation
  amadeus dead-letters purge --execute --yes

  # JSON output for scripting
  amadeus dead-letters purge -o json
```

### Options

```
  -x, --execute   Execute purge (default: dry-run)
  -h, --help      help for purge
  -y, --yes       Skip confirmation prompt
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

* [amadeus dead-letters](amadeus_dead-letters.md)	 - Manage dead-lettered outbox items

