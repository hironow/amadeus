## amadeus sync

Show D-Mail sync status (JSON)

### Synopsis

Output unsynced D-Mails and pending Linear comments as JSON.

```
amadeus sync [path] [flags]
```

### Examples

```
  # Show sync status for current directory
  amadeus sync

  # Show sync status for a specific project
  amadeus sync /path/to/project
```

### Options

```
  -h, --help   help for sync
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

* [amadeus](amadeus.md)	 - Divergence meter for your codebase

