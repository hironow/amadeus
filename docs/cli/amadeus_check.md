## amadeus check

Run divergence check

```
amadeus check [path] [flags]
```

### Options

```
      --approve-cmd string   external command for approval ({message} placeholder)
      --auto-approve         skip approval gate
  -n, --dry-run              generate prompt only
  -f, --full                 force full calibration check
  -h, --help                 help for check
  -j, --json                 output as JSON
      --notify-cmd string    external command for notifications ({title} and {message} placeholders)
  -q, --quiet                summary-only output
      --review-cmd string    code review command after check (exit 0=pass, non-zero=comments)
```

### Options inherited from parent commands

```
  -c, --config string   config file path
  -l, --lang string     output language (ja, en)
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         verbose output
```

### SEE ALSO

* [amadeus](amadeus.md)  - Divergence meter for your codebase
