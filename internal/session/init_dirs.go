package session

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// InitAction describes what happened to a path during init.
type InitAction int

const (
	// InitCreated means the path was newly created.
	InitCreated InitAction = iota
	// InitSkipped means the path already existed.
	InitSkipped
	// InitUpdated means the path was updated (e.g. config merge).
	InitUpdated
	// InitWarning means a non-fatal issue occurred.
	InitWarning
)

// InitEntry represents a single init action.
type InitEntry struct {
	Path   string     // relative path (e.g. ".siren/config.yaml")
	Action InitAction // what happened
	Detail string     // optional detail (e.g. warning message)
}

// InitResult collects init actions for unified display.
type InitResult struct {
	StateDir string      // display name (e.g. ".siren")
	Entries  []InitEntry
}

// Add records an init action.
func (r *InitResult) Add(path string, action InitAction, detail string) {
	r.Entries = append(r.Entries, InitEntry{Path: path, Action: action, Detail: detail})
}

// Warnings returns detail strings for all warning entries.
func (r *InitResult) Warnings() []string {
	var ws []string
	for _, e := range r.Entries {
		if e.Action == InitWarning {
			ws = append(ws, e.Detail)
		}
	}
	return ws
}

// PrintInitResult writes init results to w in a unified format.
// Output is human-readable (stderr), not machine data (stdout).
func PrintInitResult(w io.Writer, result *InitResult) {
	fmt.Fprintf(w, "\nInitialized %s/\n", result.StateDir)
	for _, e := range result.Entries {
		switch e.Action {
		case InitCreated:
			fmt.Fprintf(w, "  + %s\n", e.Path)
		case InitSkipped:
			fmt.Fprintf(w, "  - %s (already exists)\n", e.Path)
		case InitUpdated:
			fmt.Fprintf(w, "  ~ %s\n", e.Path)
		case InitWarning:
			fmt.Fprintf(w, "  ! %s\n", e.Detail)
		}
	}
}

// InitDirOption configures EnsureStateDir behavior.
type InitDirOption func(*initDirConfig)

type initDirConfig struct {
	mailDirs  bool
	extraDirs []string
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
// Returns an InitResult recording what was created or skipped. Idempotent.
func EnsureStateDir(stateDir string, opts ...InitDirOption) (*InitResult, error) {
	var cfg initDirConfig
	for _, o := range opts {
		o(&cfg)
	}

	base := filepath.Base(stateDir)
	result := &InitResult{StateDir: base}

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
		existed := dirExists(d)
		if err := os.MkdirAll(d, 0o755); err != nil {
			return result, err
		}
		rel := relOrBase(stateDir, d, base)
		if existed {
			result.Add(rel, InitSkipped, "")
		} else {
			result.Add(rel, InitCreated, "")
		}
	}

	return result, nil
}

// dirExists returns true if path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// relOrBase returns a display-friendly relative path.
func relOrBase(stateDir, path, base string) string {
	if path == stateDir {
		return base + "/"
	}
	rel, err := filepath.Rel(filepath.Dir(stateDir), path)
	if err != nil {
		return filepath.Base(path)
	}
	return rel + "/"
}
