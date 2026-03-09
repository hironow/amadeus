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
  convergence.escalation_multiplier Escalation multiplier

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
  -v, --verbose         verbose output
```

### SEE ALSO

* [amadeus config](amadeus_config.md)  - View or update amadeus configuration
