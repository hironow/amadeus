package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// resolveTargetDir returns an absolute directory path from args[0],
// or the current working directory if no args are given.
func resolveTargetDir(args []string) (string, error) {
	if len(args) > 0 {
		abs, err := filepath.Abs(args[0])
		if err != nil {
			return "", fmt.Errorf("resolve path: %w", err)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("path not found: %w", err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("not a directory: %s", abs)
		}
		return abs, nil
	}
	return os.Getwd()
}
