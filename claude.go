package amadeus

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os/exec"
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

// runClaude executes the Claude CLI with the given prompt via stdin and returns raw output.
// Uses --dangerously-skip-permissions because amadeus runs non-interactively with --print.
func runClaude(ctx context.Context, prompt string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "claude",
		"--model", "opus",
		"--output-format", "json",
		"--dangerously-skip-permissions",
		"--print",
	)
	cmd.Stdin = bytes.NewBufferString(prompt)
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
