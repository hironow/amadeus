## amadeus run

Run continuous divergence check and PR convergence

```
amadeus run [path] [flags]
```

### Options

```
      --approve-cmd string   external command for approval ({message} placeholder)
      --auto-approve         skip approval gate
      --base string          upstream branch for post-merge divergence check
  -n, --dry-run              generate prompt only (post-merge)
  -f, --full                 force full calibration check
  -h, --help                 help for run
  -j, --json                 output as JSON
      --notify-cmd string    external command for notifications ({title} and {message} placeholders)
  -q, --quiet                summary-only output
      --review-cmd string    code review command after check (exit 0=pass, non-zero=comments)
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

