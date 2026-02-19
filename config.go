package amadeus

import (
	"errors"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the complete Amadeus configuration.
type Config struct {
	Weights         Weights         `yaml:"weights"`
	Thresholds      Thresholds      `yaml:"thresholds"`
	PerAxisOverride PerAxisOverride `yaml:"per_axis_override"`
	FullCheck       FullCheckConfig `yaml:"full_check"`
}

// FullCheckConfig controls the full scan strategy.
type FullCheckConfig struct {
	Interval         int     `yaml:"interval"`
	OnDivergenceJump float64 `yaml:"on_divergence_jump"`
}

// DefaultConfig returns a Config populated with architecture-document defaults.
func DefaultConfig() Config {
	sc := DefaultThresholds()
	return Config{
		Weights:         DefaultWeights(),
		Thresholds:      sc.Thresholds,
		PerAxisOverride: sc.PerAxisOverride,
		FullCheck: FullCheckConfig{
			Interval:         10,
			OnDivergenceJump: 0.15,
		},
	}
}

// LoadConfig reads a YAML configuration file from path.
// If the file does not exist, it returns DefaultConfig with no error.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return Config{}, err
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
