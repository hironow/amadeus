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

// CapabilityViolation represents a detected boundary violation where code exceeds
// its intended capability scope (e.g., direct external API calls bypassing adapters).
type CapabilityViolation struct {
	Boundary    string `json:"boundary"`
	Description string `json:"description"`
	File        string `json:"file,omitempty"`
}

// ClaudeResponse represents the structured JSON output from Claude.
type ClaudeResponse struct {
	FilesRead            []string               `json:"files_read,omitempty"`
	Axes                 map[Axis]AxisScore     `json:"axes"`
	DMails               []ClaudeDMailCandidate `json:"dmails"`
	Reasoning            string                 `json:"reasoning"`
	ImpactRadius         []ImpactEntry          `json:"impact_radius,omitempty"`
	CapabilityViolations []CapabilityViolation  `json:"capability_violations,omitempty"`
	ADRAlignment         ADRAlignmentMap        `json:"adr_alignment,omitempty"` // E19: per-ADR compliance scores
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

// RepeatedViolation represents an integrity axis that has scored above the
// violation threshold across multiple consecutive check results.
type RepeatedViolation struct {
	Axis        string `json:"axis"`
	Description string `json:"description"`
	Count       int    `json:"count"`
}

// DiffCheckParams holds the template parameters for a file-reference diff check prompt.
type DiffCheckParams struct {
	EvalDir            string
	HasPRReviews       bool
	LinkedIssueIDs     string
	RepeatedViolations []RepeatedViolation
	DivergenceTrend    *DivergenceTrend
}

// FullCheckParams holds the template parameters for a file-reference full check prompt.
type FullCheckParams struct {
	EvalDir string
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
		return trimmed // no newline after opening fence — return trimmed as-is
	}
	// Remove closing fence
	if idx := bytes.LastIndex(trimmed, []byte("```")); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	return bytes.TrimSpace(trimmed)
}

// extractJSON finds the first top-level JSON object ({...}) in data by
// scanning for the opening brace and matching it with the closing brace.
// This handles cases where Claude wraps JSON in natural language text.
func extractJSON(data []byte) []byte {
	start := bytes.IndexByte(data, '{')
	if start < 0 {
		return data
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(data); i++ {
		if escaped {
			escaped = false
			continue
		}
		c := data[i]
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return data[start : i+1]
			}
		}
	}
	// Unbalanced braces — return from start to end as best-effort
	return data[start:]
}

// ParseClaudeResponse parses raw JSON bytes into a ClaudeResponse.
// Handles markdown code block wrappers and text prefixes/suffixes that
// Claude may add around JSON output.
func ParseClaudeResponse(data []byte) (ClaudeResponse, error) {
	cleaned := stripMarkdownCodeBlock(data)
	var resp ClaudeResponse
	if err := json.Unmarshal(cleaned, &resp); err != nil {
		// Fallback: try to extract JSON object from mixed text
		extracted := extractJSON(cleaned)
		if err2 := json.Unmarshal(extracted, &resp); err2 != nil {
			return resp, fmt.Errorf("failed to parse Claude response: %w", err)
		}
	}
	return resp, nil
}
