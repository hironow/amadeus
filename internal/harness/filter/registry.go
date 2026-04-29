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
	"slices"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed prompts/*.yaml
var promptsFS embed.FS

// promptFile is the on-disk YAML schema (unmarshaling target).
type promptFile struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description"`
	Variables   map[string]string `yaml:"variables"`
	Template    string            `yaml:"template"`
}

// PromptConfig is the read-only API type.
type PromptConfig struct { // nosemgrep: structure.multiple-exported-structs-go -- filter registry family (PromptConfig/PromptRegistry) is a cohesive prompt lookup contract; PromptConfig co-locates with PromptRegistry as the API type it holds [permanent]
	Name        string
	Version     string
	Description string
	Variables   map[string]string
	Template    string
}

// PromptRegistry holds all loaded prompt configurations and provides
// lookup and expansion. It is safe for concurrent use after construction.
type PromptRegistry struct {
	entries map[string]PromptConfig
}

// singleton registry (loaded once from embedded YAML).
var (
	defaultRegistry     *PromptRegistry
	defaultRegistryOnce sync.Once
	defaultRegistryErr  error
)

// Default returns the process-wide PromptRegistry.
// The registry is created lazily on first call and cached.
func Default() (*PromptRegistry, error) {
	defaultRegistryOnce.Do(func() {
		defaultRegistry, defaultRegistryErr = NewRegistry()
	})
	return defaultRegistry, defaultRegistryErr
}

// MustDefault returns the process-wide PromptRegistry or panics.
// Safe to call because prompts are embedded at compile time.
func MustDefault() *PromptRegistry {
	r, err := Default()
	if err != nil {
		panic("prompt registry: " + err.Error())
	}
	return r
}

// NewRegistry loads all YAML prompt files from the embedded filesystem
// and returns a populated PromptRegistry.
func NewRegistry() (*PromptRegistry, error) {
	return NewRegistryFromFS(promptsFS)
}

// NewRegistryFromFS loads all YAML prompt files from the given filesystem
// and returns a populated PromptRegistry. The filesystem must contain a
// "prompts/" directory with .yaml files.
func NewRegistryFromFS(fsys fs.FS) (*PromptRegistry, error) {
	r := &PromptRegistry{entries: make(map[string]PromptConfig)}

	entries, err := fs.ReadDir(fsys, "prompts")
	if err != nil {
		return nil, fmt.Errorf("read prompts dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		data, readErr := fs.ReadFile(fsys, "prompts/"+entry.Name())
		if readErr != nil {
			return nil, fmt.Errorf("read prompt file %s: %w", entry.Name(), readErr)
		}

		var pf promptFile
		if unmarshalErr := yaml.Unmarshal(data, &pf); unmarshalErr != nil {
			return nil, fmt.Errorf("parse prompt file %s: %w", entry.Name(), unmarshalErr)
		}

		if pf.Name == "" {
			return nil, fmt.Errorf("prompt file %s: missing 'name' field", entry.Name())
		}
		if pf.Template == "" {
			return nil, fmt.Errorf("prompt file %s: missing 'template' field", entry.Name())
		}
		if _, exists := r.entries[pf.Name]; exists {
			return nil, fmt.Errorf("duplicate prompt name %q in %s", pf.Name, entry.Name())
		}

		r.entries[pf.Name] = PromptConfig{
			Name:        pf.Name,
			Version:     pf.Version,
			Description: pf.Description,
			Variables:   pf.Variables,
			Template:    pf.Template,
		}
	}

	if len(r.entries) == 0 {
		return nil, fmt.Errorf("no prompt files found in prompts/ directory")
	}

	return r, nil
}

// Get returns the PromptConfig for the given name.
func (r *PromptRegistry) Get(name string) (PromptConfig, error) {
	cfg, ok := r.entries[name]
	if !ok {
		return PromptConfig{}, fmt.Errorf("prompt %q not found", name)
	}
	return cfg, nil
}

// Expand looks up the named prompt and replaces all {key} placeholders
// with corresponding values from vars. Unknown placeholders are left as-is.
func (r *PromptRegistry) Expand(name string, vars map[string]string) (string, error) {
	cfg, err := r.Get(name)
	if err != nil {
		return "", err
	}
	return ExpandTemplate(cfg.Template, vars), nil
}

// MustExpand is like Expand but panics on error.
func (r *PromptRegistry) MustExpand(name string, vars map[string]string) string {
	result, err := r.Expand(name, vars)
	if err != nil {
		panic("prompt expand " + name + ": " + err.Error())
	}
	return result
}

// Names returns a sorted list of all registered prompt names.
func (r *PromptRegistry) Names() []string {
	names := make([]string, 0, len(r.entries))
	for name := range r.entries {
		names = append(names, name)
	}
	slices.Sort(names)
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
