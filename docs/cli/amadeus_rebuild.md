## amadeus rebuild

Rebuild projections from event store

### Synopsis

Replays all events from .gate/events/ to regenerate .run/ projection files and archive/ D-Mails from scratch.
NOTE: Inbox-sourced D-Mails (consumed via ScanInbox) are NOT reconstructed because
inbox.consumed events contain only metadata, not the full D-Mail content.

```
amadeus rebuild [path] [flags]
```

### Examples

```
  # Rebuild projections in current directory
  amadeus rebuild

  # Rebuild for a specific project
  amadeus rebuild /path/to/project
```

### Options

```
  -h, --help   help for rebuild
```

### Options inherited from parent commands

```
  -c, --config string   config file path
  -l, --lang string     output language (ja, en)
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         verbose output
```

### SEE ALSO

* [amadeus](amadeus.md)  - Divergence meter for your codebase
