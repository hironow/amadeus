package domain

import (
	"bytes"
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
	Category    string   `json:"category,omitempty"`
}

// DiffCheckParams holds the template parameters for a diff-based check.
type DiffCheckParams struct {
	PreviousScores  string
	PRDiffs         string
	RelevantADRs    string
	LinkedDoDs      string
	LinkedIssueIDs  string
	PRReviewSummary string
}

// FullCheckParams holds the template parameters for a full calibration check.
type FullCheckParams struct {
	CodebaseStructure string
	AllADRs           string
	RecentDoDs        string
	DependencyMap     string
}

// stripMarkdownCodeBlock removes markdown code block wrappers (```json ... ```)
// from data. Claude's stream-json result field is always a text string, so when
// Claude is asked for JSON output, it may wrap the response in a markdown code
// block. This function strips that wrapper to extract the raw JSON.
func stripMarkdownCodeBlock(data []byte) []byte {
	trimmed := bytes.TrimSpace(data)
	if !bytes.HasPrefix(trimmed, []byte("```")) {
		return trimmed
	}
	// Remove opening fence line (```json, ```JSON, or just ```)
	if idx := bytes.IndexByte(trimmed, '\n'); idx >= 0 {
		trimmed = trimmed[idx+1:]
	} else {
		return data // no newline after opening fence — return as-is
	}
	// Remove closing fence
	if idx := bytes.LastIndex(trimmed, []byte("```")); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	return bytes.TrimSpace(trimmed)
}

// ParseClaudeResponse parses raw JSON bytes into a ClaudeResponse.
// Handles markdown code block wrappers that Claude may add around JSON output.
func ParseClaudeResponse(data []byte) (ClaudeResponse, error) {
	cleaned := stripMarkdownCodeBlock(data)
	var resp ClaudeResponse
	if err := json.Unmarshal(cleaned, &resp); err != nil {
		return resp, fmt.Errorf("failed to parse Claude response: %w", err)
	}
	return resp, nil
}
