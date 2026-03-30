## amadeus sessions

Manage AI coding sessions

### Synopsis

Manage AI coding session records. Sessions are tracked in SQLite
and can be listed, filtered, and re-entered interactively.

### Examples

```
  amadeus sessions list
  amadeus sessions list --status completed --limit 5
  amadeus sessions enter <session-record-id>
  amadeus sessions enter --provider-id <claude-session-id>
```

### Options

```
  -h, --help   help for sessions
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
* [amadeus sessions enter](amadeus_sessions_enter.md)	 - Re-enter an AI coding session interactively
* [amadeus sessions list](amadeus_sessions_list.md)	 - List recorded coding sessions

