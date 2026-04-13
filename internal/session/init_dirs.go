package session

import (
	"os"
	"path/filepath"
)

// InitDirOption configures EnsureStateDir behavior.
type InitDirOption func(*initDirConfig)

type initDirConfig struct {
	mailDirs   bool
	extraDirs  []string
}

// WithMailDirs enables creation of inbox/, outbox/, archive/ directories.
func WithMailDirs() InitDirOption {
	return func(c *initDirConfig) { c.mailDirs = true }
}

// WithExtraDirs creates additional tool-specific directories under stateDir.
func WithExtraDirs(dirs ...string) InitDirOption {
	return func(c *initDirConfig) { c.extraDirs = append(c.extraDirs, dirs...) }
}

// EnsureStateDir creates the standard state directory structure.
// stateDir is the resolved path (e.g. /path/to/repo/.siren/).
// Always creates: stateDir, .run/, events/, insights/.
// Optional: inbox/, outbox/, archive/ (WithMailDirs), extra dirs (WithExtraDirs).
// Idempotent: safe to call multiple times.
func EnsureStateDir(stateDir string, opts ...InitDirOption) error {
	var cfg initDirConfig
	for _, o := range opts {
		o(&cfg)
	}

	// Core directories (always created)
	dirs := []string{
		stateDir,
		filepath.Join(stateDir, ".run"),
		filepath.Join(stateDir, "events"),
		filepath.Join(stateDir, "insights"),
	}

	// Mail directories (D-Mail tools)
	if cfg.mailDirs {
		for _, sub := range []string{"inbox", "outbox", "archive"} {
			dirs = append(dirs, filepath.Join(stateDir, sub))
		}
	}

	// Extra tool-specific dirs
	for _, d := range cfg.extraDirs {
		dirs = append(dirs, filepath.Join(stateDir, d))
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	return nil
}
