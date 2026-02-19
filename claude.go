package amadeus

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"text/template"
)

//go:embed templates/*.md.tmpl
var templateFS embed.FS

// ClaudeClient wraps invocation of the Claude CLI.
type ClaudeClient struct {
	Command string
	Model   string
	Timeout int
	DryRun  bool
}

// NewClaudeClient returns a ClaudeClient with sensible defaults.
func NewClaudeClient() *ClaudeClient {
	return &ClaudeClient{
		Command: "claude",
		Model:   "opus",
		Timeout: 300,
	}
}

// ClaudeResponse represents the structured JSON output from Claude.
type ClaudeResponse struct {
	Axes      map[Axis]AxisScore     `json:"axes"`
	DMails    []ClaudeDMailCandidate `json:"dmails"`
	Reasoning string                 `json:"reasoning"`
}

// ClaudeDMailCandidate is a D-Mail candidate produced by Claude's evaluation.
type ClaudeDMailCandidate struct {
	Target  DMailTarget `json:"target"`
	Type    string      `json:"type"`
	Summary string      `json:"summary"`
	Detail  string      `json:"detail"`
}

// DiffCheckParams holds the template parameters for a diff-based check.
type DiffCheckParams struct {
	PreviousScores string
	PRDiffs        string
	RelevantADRs   string
	LinkedDoDs     string
}

// FullCheckParams holds the template parameters for a full calibration check.
type FullCheckParams struct {
	CodebaseStructure string
	AllADRs           string
	RecentDoDs        string
	DependencyMap     string
}

// BuildDiffCheckPrompt renders the diff_check template with the given parameters.
func BuildDiffCheckPrompt(params DiffCheckParams) (string, error) {
	return renderTemplate("templates/diff_check.md.tmpl", params)
}

// BuildFullCheckPrompt renders the full_check template with the given parameters.
func BuildFullCheckPrompt(params FullCheckParams) (string, error) {
	return renderTemplate("templates/full_check.md.tmpl", params)
}

// ParseClaudeResponse parses raw JSON bytes into a ClaudeResponse.
func ParseClaudeResponse(data []byte) (ClaudeResponse, error) {
	var resp ClaudeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, fmt.Errorf("failed to parse Claude response: %w", err)
	}
	return resp, nil
}

// Run executes the Claude CLI with the given prompt and returns raw output.
func (c *ClaudeClient) Run(ctx context.Context, prompt string) ([]byte, error) {
	if c.DryRun {
		return nil, nil
	}
	args := []string{"-p", "--output-format", "json", "--model", c.Model}
	cmd := exec.CommandContext(ctx, c.Command, args...)
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude: %w\n%s", err, stderr.String())
	}
	return stdout.Bytes(), nil
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
