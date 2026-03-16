## amadeus clean

Remove state directory (.gate/)

### Synopsis

Delete the .gate/ directory to reset to a clean state. Use 'amadeus init' to reinitialize.

```
amadeus clean [path] [flags]
```

### Examples

```
  # Clean current directory (interactive confirmation)
  amadeus clean

  # Clean a specific project directory
  amadeus clean /path/to/project

  # Skip confirmation prompt
  amadeus clean --yes
```

### Options

```
  -h, --help   help for clean
      --yes    Skip confirmation prompt
```

### Options inherited from parent commands

```
  -c, --config string   config file path
  -l, --lang string     output language (ja, en)
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -q, --quiet           Suppress all stderr output
  -v, --verbose         verbose output
```

### SEE ALSO

* [amadeus](amadeus.md)	 - Divergence meter for your codebase

