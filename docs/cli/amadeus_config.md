## amadeus config

View or update amadeus configuration

### Synopsis

View or update the .gate/config.yaml configuration file.

### Examples

```
  amadeus config show /path/to/repo
  amadeus config set lang en /path/to/repo
  amadeus config set full_check.interval 20
```

### Options

```
  -h, --help   help for config
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
* [amadeus config set](amadeus_config_set.md)  - Update a configuration value
* [amadeus config show](amadeus_config_show.md)  - Display effective configuration
