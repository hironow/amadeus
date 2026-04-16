package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newConfigCommand() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "View or update amadeus configuration",
		Long:  "View or update the .gate/config.yaml configuration file.",
		Example: `  amadeus config show /path/to/repo
  amadeus config set lang en /path/to/repo
  amadeus config set full_check.interval 20`,
	}

	configCmd.AddCommand(newConfigShowCommand())
	configCmd.AddCommand(newConfigSetCommand())

	return configCmd
}

func newConfigShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show [path]",
		Short: "Display effective configuration",
		Long:  "Display the effective configuration after applying defaults.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := resolveConfigPath(cmd, args)
			if err != nil {
				return err
			}
			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}
			out, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
}

func newConfigSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value> [path]",
		Short: "Update a configuration value",
		Long: `Update a configuration value in .gate/config.yaml.

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
  timeout_sec                           Claude CLI timeout in seconds (default: 1980)`,
		Example: `  amadeus config set lang en
  amadeus config set full_check.interval 20
  amadeus config set weights.adr_integrity 0.5`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			var pathArgs []string
			if len(args) == 3 {
				pathArgs = args[2:]
			}

			cfgPath, err := resolveConfigPath(cmd, pathArgs)
			if err != nil {
				return err
			}

			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}

			if err := setAmadeusConfigField(&cfg, key, value); err != nil {
				return err
			}

			// Validate before writing
			if errs := domain.ValidateConfig(cfg); len(errs) > 0 {
				return fmt.Errorf("invalid config after update: %s", errs[0])
			}

			out, yamlErr := yaml.Marshal(cfg)
			if yamlErr != nil {
				return fmt.Errorf("marshal config: %w", yamlErr)
			}

			if writeErr := os.MkdirAll(filepath.Dir(cfgPath), 0755); writeErr != nil {
				return fmt.Errorf("create config dir: %w", writeErr)
			}
			if writeErr := os.WriteFile(cfgPath, out, 0644); writeErr != nil {
				return fmt.Errorf("write config: %w", writeErr)
			}

			logger := loggerFrom(cmd)
			logger.Info("Updated %s = %s", key, value)
			return nil
		},
	}
}

func resolveConfigPath(cmd *cobra.Command, args []string) (string, error) {
	configPath := mustString(cmd, "config")
	if configPath != "" {
		return configPath, nil
	}
	repoRoot, err := resolveTargetDir(args)
	if err != nil {
		return "", err
	}
	return filepath.Join(repoRoot, domain.StateDir, "config.yaml"), nil
}

func setAmadeusConfigField(cfg *domain.Config, key string, value string) error {
	switch key {
	case "lang":
		if !domain.ValidLang(value) {
			return fmt.Errorf("invalid lang %q: must be ja or en", value)
		}
		cfg.Lang = value

	// Weights
	case "weights.adr_integrity":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", key, err)
		}
		cfg.Weights.ADRIntegrity = f
	case "weights.dod_fulfillment":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", key, err)
		}
		cfg.Weights.DoDFulfillment = f
	case "weights.dependency_integrity":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", key, err)
		}
		cfg.Weights.DependencyIntegrity = f
	case "weights.implicit_constraints":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", key, err)
		}
		cfg.Weights.ImplicitConstraints = f

	// Thresholds
	case "thresholds.low_max":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", key, err)
		}
		cfg.Thresholds.LowMax = f
	case "thresholds.medium_max":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", key, err)
		}
		cfg.Thresholds.MediumMax = f

	// FullCheck
	case "full_check.interval":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", key, err)
		}
		cfg.FullCheck.Interval = n
	case "full_check.on_divergence_jump":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", key, err)
		}
		cfg.FullCheck.OnDivergenceJump = f

	// Convergence
	case "convergence.window_days":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", key, err)
		}
		cfg.Convergence.WindowDays = n
	case "convergence.threshold":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", key, err)
		}
		cfg.Convergence.Threshold = n
	case "convergence.escalation_multiplier":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", key, err)
		}
		cfg.Convergence.EscalationMultiplier = n

	// Per-axis overrides
	case "per_axis_override.adr_integrity_force_high":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", key, err)
		}
		cfg.PerAxisOverride.ADRForceHigh = n
	case "per_axis_override.dod_fulfillment_force_high":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", key, err)
		}
		cfg.PerAxisOverride.DoDForceHigh = n
	case "per_axis_override.dependency_integrity_force_medium":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", key, err)
		}
		cfg.PerAxisOverride.DepForceMedium = n

	case "claude_cmd":
		cfg.ClaudeCmd = value

	case "model":
		cfg.Model = value

	case "timeout_sec":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return fmt.Errorf("invalid timeout_sec %q: must be non-negative integer", value)
		}
		cfg.TimeoutSec = n

	case "idle_timeout":
		d, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid idle_timeout %q: %w", value, err)
		}
		cfg.IdleTimeout = d

	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	return nil
}
