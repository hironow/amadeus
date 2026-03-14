## amadeus update

Self-update amadeus to the latest release

### Synopsis

Self-update amadeus to the latest GitHub release.

Downloads the latest release, verifies the checksum, and replaces
the current binary. Use --check to only check for updates without
installing.

```
amadeus update [flags]
```

### Examples

```
  # Check for updates
  amadeus update --check

  # Update to the latest version
  amadeus update
```

### Options

```
  -C, --check   Check for updates without installing
  -h, --help    help for update
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
