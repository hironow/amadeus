package domain

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"text/template"
)

//go:embed templates/*.md.tmpl
var templateFS embed.FS

// ImpactEntry represents a single entry in the impact radius map.
type ImpactEntry struct {
	Area   string `json:"area"`
	Impact string `json:"impact"` // direct, indirect, transitive
	Detail string `json:"detail"`
}

// ClaudeResponse represents the structured JSON output from Claude.
type ClaudeResponse struct {
	Axes         map[Axis]AxisScore     `json:"axes"`
	DMails       []ClaudeDMailCandidate `json:"dmails"`
	Reasoning    string                 `json:"reasoning"`
	ImpactRadius []ImpactEntry          `json:"impact_radius,omitempty"`
}

// ClaudeDMailCandidate is a D-Mail candidate produced by Claude's evaluation.
type ClaudeDMailCandidate struct {
	Description string   `json:"description"`
	Detail      string   `json:"detail"`
	Issues      []string `json:"issues,omitempty"`
	Targets     []string `json:"targets,omitempty"`
	Action      string   `json:"action,omitempty"`
}

// DiffCheckParams holds the template parameters for a diff-based check.
type DiffCheckParams struct {
	PreviousScores string
	PRDiffs        string
	RelevantADRs   string
	LinkedDoDs     string
	LinkedIssueIDs string
}

// FullCheckParams holds the template parameters for a full calibration check.
type FullCheckParams struct {
	CodebaseStructure string
	AllADRs           string
	RecentDoDs        string
	DependencyMap     string
}

// BuildDiffCheckPrompt renders the diff_check template for the given language.
func BuildDiffCheckPrompt(lang string, params DiffCheckParams) (string, error) {
	name := fmt.Sprintf("templates/diff_check_%s.md.tmpl", lang)
	return renderTemplate(name, params)
}

// BuildFullCheckPrompt renders the full_check template for the given language.
func BuildFullCheckPrompt(lang string, params FullCheckParams) (string, error) {
	name := fmt.Sprintf("templates/full_check_%s.md.tmpl", lang)
	return renderTemplate(name, params)
}

// ParseClaudeResponse parses raw JSON bytes into a ClaudeResponse.
func ParseClaudeResponse(data []byte) (ClaudeResponse, error) {
	var resp ClaudeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, fmt.Errorf("failed to parse Claude response: %w", err)
	}
	return resp, nil
}

// ClaudeRunner executes the Claude CLI and returns raw JSON output.
type ClaudeRunner interface {
	Run(ctx context.Context, prompt string) ([]byte, error)
}

// renderTemplate parses and executes a template from the embedded filesystem.
func renderTemplate(name string, data any) (string, error) {
	tmpl, err := template.ParseFS(templateFS, name)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}
