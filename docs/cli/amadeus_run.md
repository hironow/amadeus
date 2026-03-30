## amadeus run

Run continuous divergence check and PR convergence

### Synopsis

Run continuous divergence checking with D-Mail generation and optional
PR convergence analysis.

Without --base: performs a one-shot divergence check (phases 0-4),
generates D-Mails from divergence scoring, then enters a waiting loop
that monitors the inbox for incoming D-Mails and re-checks on arrival.

With --base: runs a daemon loop that monitors the inbox and performs
post-merge divergence checks against the specified upstream branch,
adding PR convergence analysis via the gh CLI on top of divergence
scoring.

If [path] is omitted, the current working directory is used. Requires
'amadeus init' to have been run first.

```
amadeus run [path] [flags]
```

### Examples

```
  # One-shot divergence check with D-Mail waiting loop
  amadeus run

  # Continuous post-merge check against main branch
  amadeus run --base main

  # Dry-run mode (generate prompts without executing)
  amadeus run --dry-run

  # Full calibration check with JSON output
  amadeus run --full --json
```

### Options

```
      --approve-cmd string      external command for approval ({message} placeholder)
      --auto-approve            skip approval gate
      --base string             upstream branch for post-merge divergence check
  -n, --dry-run                 generate prompt only (post-merge)
  -f, --full                    force full calibration check
  -h, --help                    help for run
      --idle-timeout duration   idle timeout — exit after no D-Mail activity (0 = 24h safety cap, negative = disable) (default 30m0s)
  -j, --json                    output as JSON
      --no-merge                disable automatic PR merging (only effective with --base)
      --notify-cmd string       external command for notifications ({title} and {message} placeholders)
  -q, --quiet                   summary-only output
      --review-cmd string       code review command after check (exit 0=pass, non-zero=comments)
```

### Options inherited from parent commands

```
  -c, --config string   config file path
  -l, --lang string     output language (ja, en)
      --linear          Use Linear MCP for issue tracking (default: wave-centric mode)
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         verbose output
```

### SEE ALSO

* [amadeus](amadeus.md)	 - Divergence meter for your codebase

