## amadeus install-hook

Install post-merge git hook

### Synopsis

Install a post-merge git hook that triggers amadeus after git pull/merge.

The hook is installed into the current repository's .git/hooks/ directory.
It runs 'amadeus run' automatically after each merge, enabling continuous
divergence monitoring without manual intervention. The command must be
run from within a git repository.

```
amadeus install-hook [flags]
```

### Examples

```
  # Install hook in current git repository
  amadeus install-hook

  # Verify the hook was installed
  cat .git/hooks/post-merge
```

### Options

```
  -h, --help   help for install-hook
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

