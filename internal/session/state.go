package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/usecase/port"
	"gopkg.in/yaml.v3"
)

// Compile-time check that ProjectionStore implements port.StateReader.
var _ port.StateReader = (*ProjectionStore)(nil)

// ProjectionStore manages reading and writing materialized projection files within the .gate/ directory.
type ProjectionStore struct {
	Root string
}

// NewProjectionStore creates a ProjectionStore rooted at the given directory path.
func NewProjectionStore(root string) *ProjectionStore {
	return &ProjectionStore{Root: root}
}

func (s *ProjectionStore) runDir() string {
	return filepath.Join(s.Root, ".run")
}

// InitGateDir creates the .gate/ directory structure and writes
// a default config.yaml if one does not already exist.
func InitGateDir(root string) error {
	dirs := []string{
		filepath.Join(root, ".run"),
		filepath.Join(root, "events"),
		filepath.Join(root, "outbox"),
		filepath.Join(root, "inbox"),
		filepath.Join(root, "archive"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	// Migrate legacy state/ -> .run/ (v0.0.11 -> v0.0.12)
	if err := migrateLegacyState(root); err != nil {
		return fmt.Errorf("migrate legacy state: %w", err)
	}
	// Create skills directories and default SKILL.md files from embedded templates
	skillNames := []string{"dmail-sendable", "dmail-readable"}
	for _, name := range skillNames {
		destDir := filepath.Join(root, "skills", name)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}
		skillPath := filepath.Join(destDir, "SKILL.md")
		if _, err := os.Stat(skillPath); errors.Is(err, fs.ErrNotExist) {
			tmplPath := path.Join("templates", "skills", name, "SKILL.md")
			content, readErr := platform.SkillTemplateFS.ReadFile(tmplPath)
			if readErr != nil {
				return fmt.Errorf("read skill template %s: %w", name, readErr)
			}
			if err := os.WriteFile(skillPath, content, 0o644); err != nil {
				return err
			}
		}
	}

	configPath := filepath.Join(root, "config.yaml")
	if _, err := os.Stat(configPath); errors.Is(err, fs.ErrNotExist) {
		cfg := domain.DefaultConfig()
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return err
		}
		if err := os.WriteFile(configPath, data, 0o644); err != nil {
			return err
		}
	}
	gitignorePath := filepath.Join(root, ".gitignore")
	requiredEntries := []string{".run/", "outbox/", "inbox/", ".otel.env"}
	if _, err := os.Stat(gitignorePath); errors.Is(err, fs.ErrNotExist) {
		content := strings.Join(requiredEntries, "\n") + "\n"
		if err := os.WriteFile(gitignorePath, []byte(content), 0o644); err != nil {
			return err
		}
	} else if err == nil {
		existing, readErr := os.ReadFile(gitignorePath)
		if readErr == nil {
			var toAdd []string
			for _, entry := range requiredEntries {
				if !strings.Contains(string(existing), entry) {
					toAdd = append(toAdd, entry)
				}
			}
			if len(toAdd) > 0 {
				f, openErr := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0o644)
				if openErr == nil {
					defer f.Close()
					if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
						f.Write([]byte("\n"))
					}
					for _, entry := range toAdd {
						f.Write([]byte(entry + "\n"))
					}
				}
			}
		}
	}
	return nil
}

// migrateLegacyState moves files from the legacy state/ directory to .run/.
// Files are only moved if the destination does not already exist, preventing
// accidental overwrites. After migration, the empty state/ directory is removed.
func migrateLegacyState(root string) error {
	legacyDir := filepath.Join(root, "state")
	if _, err := os.Stat(legacyDir); errors.Is(err, fs.ErrNotExist) {
		return nil // no legacy directory, nothing to do
	}

	runDir := filepath.Join(root, ".run")
	for _, name := range []string{"latest.json", "baseline.json"} {
		src := filepath.Join(legacyDir, name)
		dst := filepath.Join(runDir, name)
		if _, err := os.Stat(src); errors.Is(err, fs.ErrNotExist) {
			continue // file doesn't exist in legacy dir
		}
		if _, err := os.Stat(dst); err == nil {
			continue // destination already exists, don't overwrite
		}
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("migrate %s: %w", name, err)
		}
	}

	// Remove legacy directory if empty
	entries, err := os.ReadDir(legacyDir)
	if err != nil {
		return nil // non-critical, ignore
	}
	if len(entries) == 0 {
		os.Remove(legacyDir)
	}
	return nil
}

// SaveLatest writes the check result as the latest state.
func (s *ProjectionStore) SaveLatest(result domain.CheckResult) error {
	return s.writeJSON(filepath.Join(s.Root, ".run", "latest.json"), result)
}

// SaveBaseline writes the check result as the baseline state.
func (s *ProjectionStore) SaveBaseline(result domain.CheckResult) error {
	return s.writeJSON(filepath.Join(s.Root, ".run", "baseline.json"), result)
}

// LoadLatest reads the latest check result from .run/latest.json.
// If the file does not exist, it returns an empty CheckResult with no error.
func (s *ProjectionStore) LoadLatest() (domain.CheckResult, error) {
	var result domain.CheckResult
	data, err := os.ReadFile(filepath.Join(s.Root, ".run", "latest.json"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return result, nil
		}
		return result, err
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	return result, nil
}

func (s *ProjectionStore) writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
