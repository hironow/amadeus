## amadeus validate

Validate config file

### Synopsis

Validate the amadeus configuration file.

Reads .gate/config.yaml (or the path specified by --config) and checks
all fields against the configuration schema. Reports individual
validation errors with [FAIL] markers. If [path] is omitted, the
current working directory is used.

```
amadeus validate [path] [flags]
```

### Examples

```
  # Validate config in current directory
  amadeus validate

  # Validate a specific project
  amadeus validate /path/to/project

  # Validate a specific config file
  amadeus validate --config /path/to/config.yaml
```

### Options

```
  -h, --help   help for validate
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
