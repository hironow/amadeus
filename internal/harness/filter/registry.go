// Package filter defines LLM action spaces: prompt templates,
// response schemas, and variable specifications.
//
// The PromptRegistry loads YAML prompt files from the embedded prompts/
// directory and provides Get/Expand methods for simple {key} substitution.
// This externalizes prompt strings from Go source, enabling versioning
// and auditing without code changes.
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

// ExpandTemplate performs {key} substitution and {#if key}...{#else}...{/if}
// conditionals on a template string. Placeholders not present in vars are
// left unchanged. A key is truthy if present, non-empty, and not "false".
func ExpandTemplate(tmpl string, vars map[string]string) string {
	result := processConditionals(tmpl, vars)

	// Two-pass expansion prevents variable values containing {key} patterns
	// from being re-expanded by subsequent iterations. Pass 1 replaces
	// placeholders with unique tokens; pass 2 replaces tokens with values.
	const sentinel = "\x00PROMPT_VAR_"
	for k := range vars {
		result = strings.ReplaceAll(result, "{"+k+"}", sentinel+k+"\x00")
	}
	for k, v := range vars {
		result = strings.ReplaceAll(result, sentinel+k+"\x00", v)
	}
	return result
}

// processConditionals handles {#if key}...{#else}...{/if} blocks.
func processConditionals(tmpl string, vars map[string]string) string {
	for {
		start := strings.Index(tmpl, "{#if ")
		if start == -1 {
			return tmpl
		}
		closeTag := strings.Index(tmpl[start:], "}")
		if closeTag == -1 {
			return tmpl
		}
		key := tmpl[start+len("{#if ") : start+closeTag]
		endTag := "{/if}"
		endIdx := strings.Index(tmpl[start:], endTag)
		if endIdx == -1 {
			return tmpl
		}
		endIdx += start
		body := tmpl[start+closeTag+1 : endIdx]
		var ifBlock, elseBlock string
		if elseIdx := strings.Index(body, "{#else}"); elseIdx != -1 {
			ifBlock = body[:elseIdx]
			elseBlock = body[elseIdx+len("{#else}"):]
		} else {
			ifBlock = body
		}
		val, exists := vars[key]
		truthy := exists && val != "" && val != "false"
		var replacement string
		if truthy {
			replacement = ifBlock
		} else {
			replacement = elseBlock
		}
		tmpl = tmpl[:start] + replacement + tmpl[endIdx+len(endTag):]
	}
}
