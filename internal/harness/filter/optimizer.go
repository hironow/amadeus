package filter

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// EvalCase is a universal evaluation case for prompt optimization.
// Prompt-agnostic: works with any instruction type.
type EvalCase struct {
	UID         string            // unique identifier
	Input       string            // input data (diff, scan result, etc.)
	GroundTruth string            // expected output
	Metadata    map[string]string // optional metadata
}

// OptimizedResult is the result of a prompt optimization run.
type OptimizedResult struct {
	Template   string              // optimized template text
	Score      float64             // final score [0.0, 1.0]
	Iterations int                 // number of iterations run
	History    []map[string]string // optimization history (optional)
}

// PromptOptimizer is the port interface for prompt optimization backends.
// Implementations: GEPABackend, DSPyAdapter, ManualA/BAdapter, etc.
type PromptOptimizer interface {
	// Optimize runs optimization on a prompt using train/val cases.
	Optimize(promptName string, trainCases, valCases []EvalCase, maxIterations int) (*OptimizedResult, error)

	// Evaluate scores a prompt's current template on a dataset.
	// Returns a score in [0.0, 1.0].
	Evaluate(promptName string, cases []EvalCase) (float64, error)
}

// Save writes an updated PromptConfig back to the prompts directory on disk.
// Used by optimization backends to persist improved templates.
// The version field should be incremented by the caller before saving.
//
// filterDir is the on-disk path returned by PromptsDir() (the filter package root).
// After saving, call NewRegistryFromFS(os.DirFS(filterDir)) to reload.
func Save(filterDir string, cfg PromptConfig) error {
	data := map[string]any{
		"name":        cfg.Name,
		"version":     cfg.Version,
		"description": cfg.Description,
		"variables":   cfg.Variables,
		"template":    cfg.Template,
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal prompt %s: %w", cfg.Name, err)
	}

	path := filepath.Join(filterDir, "prompts", cfg.Name+".yaml")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("write prompt %s: %w", cfg.Name, err)
	}

	return nil
}

// PromptsDir returns the on-disk path to the filter package root relative to
// the project root. This is used by optimization tools (not runtime).
// The returned path is the parent of the prompts/ subdirectory, suitable for
// os.DirFS() + NewRegistryFromFS() which expects to find "prompts/" inside.
func PromptsDir(projectRoot string) string {
	return filepath.Join(projectRoot, "internal", "harness", "filter")
}
