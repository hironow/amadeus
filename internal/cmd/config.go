package cmd

import (
	"errors"
	"io/fs"
	"os"

	"github.com/hironow/amadeus/internal/domain"
	"gopkg.in/yaml.v3"
)

// loadConfig reads a YAML configuration file from path.
// If the file does not exist, it returns DefaultConfig with no error.
func loadConfig(path string) (domain.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return domain.DefaultConfig(), nil
		}
		return domain.Config{}, err
	}
	cfg := domain.DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return domain.Config{}, err
	}
	return cfg, nil
}
