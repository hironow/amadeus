package filter

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed prompts/*.yaml
var promptsFS embed.FS

// PromptConfig represents a single prompt template loaded from YAML.
type PromptConfig struct {
	Name        string     `yaml:"name"`
	Version     int        `yaml:"version"`
	Description string     `yaml:"description"`
	Variables   []Variable `yaml:"variables"`
	Template    string     `yaml:"template"`
}

// Variable documents a placeholder variable used in a prompt template.
type Variable struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// Registry holds all loaded prompt configurations and provides
// lookup and expansion. It is safe for concurrent use after construction.
type Registry struct {
	configs map[string]PromptConfig
}

// singleton registry (loaded once from embedded YAML).
var (
	defaultRegistry     *Registry
	defaultRegistryOnce sync.Once
	defaultRegistryErr  error
)

// DefaultRegistry returns the process-wide PromptRegistry.
// The registry is created lazily on first call and cached.
func DefaultRegistry() (*Registry, error) {
	defaultRegistryOnce.Do(func() {
		defaultRegistry, defaultRegistryErr = NewRegistry()
	})
	return defaultRegistry, defaultRegistryErr
}

// MustDefaultRegistry returns the process-wide PromptRegistry or panics.
// Safe to call because prompts are embedded at compile time.
func MustDefaultRegistry() *Registry {
	r, err := DefaultRegistry()
	if err != nil {
		panic("prompt registry: " + err.Error())
	}
	return r
}

// NewRegistry loads all YAML prompt files from the embedded filesystem
// and returns a populated Registry.
func NewRegistry() (*Registry, error) {
	r := &Registry{configs: make(map[string]PromptConfig)}

	entries, err := fs.ReadDir(promptsFS, "prompts")
	if err != nil {
		return nil, fmt.Errorf("read prompts dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		data, readErr := promptsFS.ReadFile("prompts/" + entry.Name())
		if readErr != nil {
			return nil, fmt.Errorf("read prompt file %s: %w", entry.Name(), readErr)
		}

		var cfg PromptConfig
		if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr != nil {
			return nil, fmt.Errorf("parse prompt file %s: %w", entry.Name(), unmarshalErr)
		}

		if cfg.Name == "" {
			return nil, fmt.Errorf("prompt file %s: missing 'name' field", entry.Name())
		}
		if _, exists := r.configs[cfg.Name]; exists {
			return nil, fmt.Errorf("duplicate prompt name %q in %s", cfg.Name, entry.Name())
		}

		r.configs[cfg.Name] = cfg
	}

	return r, nil
}

// Get returns the PromptConfig for the given name.
func (r *Registry) Get(name string) (PromptConfig, error) {
	cfg, ok := r.configs[name]
	if !ok {
		return PromptConfig{}, fmt.Errorf("prompt %q not found", name)
	}
	return cfg, nil
}

// Expand looks up the named prompt and replaces all {key} placeholders
// with corresponding values from vars. Unknown placeholders are left as-is.
func (r *Registry) Expand(name string, vars map[string]string) (string, error) {
	cfg, err := r.Get(name)
	if err != nil {
		return "", err
	}
	return ExpandTemplate(cfg.Template, vars), nil
}

// MustExpand is like Expand but panics on error.
func (r *Registry) MustExpand(name string, vars map[string]string) string {
	result, err := r.Expand(name, vars)
	if err != nil {
		panic("prompt expand " + name + ": " + err.Error())
	}
	return result
}

// Names returns a sorted list of all registered prompt names.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.configs))
	for name := range r.configs {
		names = append(names, name)
	}
	return names
}

// ExpandTemplate performs simple {key} substitution on a template string.
// Placeholders not present in vars are left unchanged.
func ExpandTemplate(tmpl string, vars map[string]string) string {
	result := tmpl
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{"+k+"}", v)
	}
	return result
}
