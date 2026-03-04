package cmd

import (
	"os"
	"path/filepath"
	"strings"
)

// applyOtelEnv reads a .otel.env file from stateDir and sets environment
// variables for the OTel SDK. Existing env vars take precedence (explicit > config).
// Variable references like ${WANDB_API_KEY} are expanded at runtime via os.ExpandEnv.
// Missing file or read errors are silently ignored (OTel stays noop).
func applyOtelEnv(stateDir string) {
	path := filepath.Join(stateDir, ".otel.env")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = os.ExpandEnv(strings.TrimSpace(val))
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
