package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

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
// Returns an InitResult recording what was created or skipped.
func InitGateDir(root string, logger domain.Logger) (*InitResult, error) {
	if logger == nil {
		logger = &domain.NopLogger{}
	}

	// Core directories + mail dirs
	result, err := EnsureStateDir(root, WithMailDirs())
	if err != nil {
		return result, err
	}

	// Migrate legacy state/ -> .run/
	if err := migrateLegacyState(root); err != nil {
		return result, fmt.Errorf("migrate legacy state: %w", err)
	}

	// Skill templates (idempotent sync from embedded FS)
	skillNames := []string{"dmail-sendable", "dmail-readable"}
	for _, name := range skillNames {
		destDir := filepath.Join(root, "skills", name)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return result, err
		}
		skillPath := filepath.Join(destDir, "SKILL.md")
		tmplPath := path.Join("templates", "skills", name, "SKILL.md")
		content, readErr := platform.SkillsFS.ReadFile(tmplPath)
		if readErr != nil {
			return result, fmt.Errorf("read skill template %s: %w", name, readErr)
		}
		existing, existErr := os.ReadFile(skillPath)
		if existErr != nil || !bytes.Equal(existing, content) {
			if existErr == nil {
				logger.Info("updated SKILL.md: %s (template changed)", name)
			}
			if err := os.WriteFile(skillPath, content, 0o644); err != nil {
				return result, err
			}
			result.Add("skills/"+name+"/", InitUpdated, "")
		} else {
			result.Add("skills/"+name+"/", InitSkipped, "")
		}
	}

	// Config (merge with existing)
	configPath := filepath.Join(root, "config.yaml")
	if err := writeConfigWithDefaults(configPath); err != nil {
		return result, err
	}
	result.Add("config.yaml", InitUpdated, "")

	// Gitignore (append-only)
	gateGitignoreEntries := []string{".run/", "outbox/", "inbox/", ".otel.env", "events/", ".mcp.json", ".claude/"}
	if err := EnsureGitignoreEntries(filepath.Join(root, ".gitignore"), gateGitignoreEntries); err != nil {
		return result, err
	}
	result.Add(".gitignore", InitUpdated, "")

	return result, nil
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

// writeConfigWithDefaults writes config.yaml with all defaults populated.
// If an existing config.yaml exists, user values are preserved (merged over defaults).
func writeConfigWithDefaults(configPath string) error {
	cfg := domain.DefaultConfig()

	existing, readErr := os.ReadFile(configPath)
	if readErr == nil && len(existing) > 0 {
		// Merge: defaults as base, existing values override
		var defaultMap map[string]any
		defaultData, marshalErr := yaml.Marshal(cfg)
		if marshalErr != nil {
			return fmt.Errorf("marshal default config: %w", marshalErr)
		}
		if err := yaml.Unmarshal(defaultData, &defaultMap); err != nil {
			return err
		}

		var existingMap map[string]any
		if err := yaml.Unmarshal(existing, &existingMap); err != nil {
			return err
		}

		deepMerge(defaultMap, existingMap)

		merged, err := yaml.Marshal(defaultMap)
		if err != nil {
			return err
		}
		// Validate the merged config by round-tripping through the struct
		var mergedCfg domain.Config
		if err := yaml.Unmarshal(merged, &mergedCfg); err != nil {
			return err
		}
		data, err := yaml.Marshal(mergedCfg)
		if err != nil {
			return err
		}
		return os.WriteFile(configPath, data, 0o644)
	}

	// No existing config: write defaults
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0o644)
}

// deepMerge merges src into dst recursively. src values override dst values.
// Nested maps are merged recursively; all other types are replaced.
func deepMerge(dst, src map[string]any) {
	for k, sv := range src {
		dv, exists := dst[k]
		if !exists {
			dst[k] = sv
			continue
		}
		srcMap, srcOK := sv.(map[string]any)
		dstMap, dstOK := dv.(map[string]any)
		if srcOK && dstOK {
			deepMerge(dstMap, srcMap)
		} else {
			dst[k] = sv
		}
	}
}
