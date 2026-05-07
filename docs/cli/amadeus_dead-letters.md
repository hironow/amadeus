## amadeus dead-letters

Manage dead-lettered outbox items

### Synopsis

Inspect and manage outbox items that have exceeded the maximum retry
count and are permanently stuck.

### Options

```
  -h, --help   help for dead-letters
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
* [amadeus dead-letters purge](amadeus_dead-letters_purge.md)	 - Purge dead-lettered outbox items

