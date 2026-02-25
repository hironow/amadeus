package cmd

import (
	"errors"
	"io/fs"
	"os"

	"github.com/hironow/amadeus"
	"gopkg.in/yaml.v3"
)

// loadConfig reads a YAML configuration file from path.
// If the file does not exist, it returns DefaultConfig with no error.
func loadConfig(path string) (amadeus.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return amadeus.DefaultConfig(), nil
		}
		return amadeus.Config{}, err
	}
	cfg := amadeus.DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return amadeus.Config{}, err
	}
	return cfg, nil
}
