package domain

import (
	"encoding/json"
	"fmt"
)

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

// ParseClaudeResponse parses raw JSON bytes into a ClaudeResponse.
func ParseClaudeResponse(data []byte) (ClaudeResponse, error) {
	var resp ClaudeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, fmt.Errorf("failed to parse Claude response: %w", err)
	}
	return resp, nil
}
