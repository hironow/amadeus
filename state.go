package amadeus

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed templates/skills/*/SKILL.md
var skillTemplateFS embed.FS

// CheckType represents the type of divergence check performed.
type CheckType string

const (
	// CheckTypeDiff indicates a diff-based incremental check.
	CheckTypeDiff CheckType = "diff"
	// CheckTypeFull indicates a full repository scan.
	CheckTypeFull CheckType = "full"
)

// CheckResult holds the outcome of a single divergence check.
type CheckResult struct {
	CheckedAt           time.Time          `json:"checked_at"`
	Commit              string             `json:"commit"`
	Type                CheckType          `json:"type"`
	Divergence          float64            `json:"divergence"`
	Axes                map[Axis]AxisScore `json:"axes"`
	ImpactRadius        []ImpactEntry      `json:"impact_radius,omitempty"`
	PRsEvaluated        []string           `json:"prs_evaluated"`
	DMails              []string           `json:"dmails"`
	ConvergenceAlerts   []ConvergenceAlert `json:"convergence_alerts,omitempty"`
	CheckCountSinceFull int                `json:"check_count_since_full"`
	ForceFullNext       bool               `json:"force_full_next,omitempty"`
}

// StateStore manages reading and writing state files within the .gate/ directory.
type StateStore struct {
	Root string
}

// NewStateStore creates a StateStore rooted at the given directory path.
func NewStateStore(root string) *StateStore {
	return &StateStore{Root: root}
}

// InitGateDir creates the .gate/ directory structure and writes
// a default config.yaml if one does not already exist.
func InitGateDir(root string) error {
	dirs := []string{
		filepath.Join(root, ".run"),
		filepath.Join(root, "history"),
		filepath.Join(root, "outbox"),
		filepath.Join(root, "inbox"),
		filepath.Join(root, "archive"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	// Migrate legacy state/ → .run/ (v0.0.11 → v0.0.12)
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
			content, readErr := skillTemplateFS.ReadFile(tmplPath)
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
		cfg := DefaultConfig()
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return err
		}
		if err := os.WriteFile(configPath, data, 0o644); err != nil {
			return err
		}
	}
	gitignorePath := filepath.Join(root, ".gitignore")
	requiredEntries := []string{".run/", "outbox/", "inbox/"}
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
func (s *StateStore) SaveLatest(result CheckResult) error {
	return s.writeJSON(filepath.Join(s.Root, ".run", "latest.json"), result)
}

// SaveBaseline writes the check result as the baseline state.
func (s *StateStore) SaveBaseline(result CheckResult) error {
	return s.writeJSON(filepath.Join(s.Root, ".run", "baseline.json"), result)
}

// SaveHistory writes the check result to the history directory with a timestamped filename.
// If a file for the same second already exists, a sequential suffix is appended to avoid clobbering.
func (s *StateStore) SaveHistory(result CheckResult) error {
	base := result.CheckedAt.Format("2006-01-02T150405")
	dir := filepath.Join(s.Root, "history")
	path := filepath.Join(dir, base+".json")
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s.writeJSON(path, result)
		}
		return err
	}
	for i := 1; ; i++ {
		path = filepath.Join(dir, fmt.Sprintf("%s_%d.json", base, i))
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return s.writeJSON(path, result)
			}
			return err
		}
	}
}

// LoadLatest reads the latest check result from state/latest.json.
// If the file does not exist, it returns an empty CheckResult with no error.
func (s *StateStore) LoadLatest() (CheckResult, error) {
	var result CheckResult
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

// LoadHistory reads all check results from the history/ directory,
// sorted by CheckedAt descending (newest first).
func (s *StateStore) LoadHistory() ([]CheckResult, error) {
	histDir := filepath.Join(s.Root, "history")
	entries, err := os.ReadDir(histDir)
	if err != nil {
		return nil, err
	}
	var results []CheckResult
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(histDir, e.Name()))
		if err != nil {
			return nil, err
		}
		var r CheckResult
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("parse history %s: %w", e.Name(), err)
		}
		results = append(results, r)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].CheckedAt.After(results[j].CheckedAt)
	})
	return results, nil
}

func (s *StateStore) writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
