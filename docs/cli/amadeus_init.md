## amadeus init

Initialize .gate directory

### Synopsis

Initialize the .gate/ state directory for amadeus divergence tracking.

Creates the directory structure required by amadeus: config.yaml,
events/, .run/, archive/, and insights/. If [path] is omitted, the
current working directory is used. Use --force to reinitialize an
existing .gate/ directory.

Optionally configure an OpenTelemetry backend (Jaeger or Weave) with
the --otel-backend flag. The generated .otel.env file is written into
.gate/ and loaded automatically on subsequent runs.

```
amadeus init [path] [flags]
```

### Examples

```
  # Initialize in current directory
  amadeus init

  # Initialize a specific project directory
  amadeus init /path/to/project

  # Reinitialize (overwrite existing .gate/)
  amadeus init --force

  # Initialize with Jaeger OTel backend
  amadeus init --otel-backend jaeger
```

### Options

```
      --force                 Overwrite existing state directory (re-initialize)
  -h, --help                  help for init
      --otel-backend string   OTel backend: jaeger, weave
      --otel-entity string    Weave entity/team (required for weave)
      --otel-project string   Weave project (required for weave)
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

