## amadeus config set

Update a configuration value

### Synopsis

Update a configuration value in .gate/config.yaml.

Supported keys:
  lang                              Language (ja or en)
  weights.adr_integrity             ADR weight (0.0-1.0)
  weights.dod_fulfillment           DoD weight (0.0-1.0)
  weights.dependency_integrity      Dependency weight (0.0-1.0)
  weights.implicit_constraints      Implicit constraints weight (0.0-1.0)
  thresholds.low_max                Low severity max threshold
  thresholds.medium_max             Medium severity max threshold
  full_check.interval               Full check interval (runs)
  full_check.on_divergence_jump     Divergence jump threshold
  convergence.window_days           Convergence detection window (days)
  convergence.threshold             Convergence threshold count
  convergence.escalation_multiplier                       Escalation multiplier
  per_axis_override.adr_integrity_force_high               ADR force-high threshold (0-100)
  per_axis_override.dod_fulfillment_force_high             DoD force-high threshold (0-100)
  per_axis_override.dependency_integrity_force_medium       Dep force-medium threshold (0-100)
  claude_cmd                            Claude CLI command name (default: claude)
  model                                 Claude model name (default: opus)
  timeout_sec                           Claude CLI timeout in seconds (default: 1980)

```
amadeus config set <key> <value> [path] [flags]
```

### Examples

```
  amadeus config set lang en
  amadeus config set full_check.interval 20
  amadeus config set weights.adr_integrity 0.5
```

### Options

```
  -h, --help   help for set
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

* [amadeus config](amadeus_config.md)	 - View or update amadeus configuration

