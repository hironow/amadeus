## amadeus uninstall-hook

Remove post-merge git hook

### Synopsis

Remove the amadeus post-merge git hook from the current repository.

Only removes the hook if it was installed by amadeus. The command must
be run from within a git repository.

```
amadeus uninstall-hook [flags]
```

### Examples

```
  # Remove the post-merge hook
  amadeus uninstall-hook
```

### Options

```
  -h, --help   help for uninstall-hook
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

* [amadeus](amadeus.md)  - Divergence meter for your codebase
