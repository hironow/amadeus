package domain_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestParseClaudeResponse_Valid(t *testing.T) {
	raw := `{
		"axes": {
			"adr_integrity": {"score": 15, "details": "ADR-003 minor tension"},
			"dod_fulfillment": {"score": 20, "details": "Issue #42 edge case"},
			"dependency_integrity": {"score": 10, "details": "clean"},
			"implicit_constraints": {"score": 5, "details": "naming drift"}
		},
		"dmails": [
			{
				"description": "ADR-003 needs update",
				"detail": "Auth module violates ADR-003"
			}
		],
		"reasoning": "Minor tensions detected"
	}`
	resp, err := domain.ParseClaudeResponse([]byte(raw))
	if err != nil {
		t.Fatalf("ParseClaudeResponse failed: %v", err)
	}
	if resp.Axes[domain.AxisADR].Score != 15 {
		t.Errorf("expected ADR score 15, got %d", resp.Axes[domain.AxisADR].Score)
	}
	if len(resp.DMails) != 1 {
		t.Fatalf("expected 1 D-Mail, got %d", len(resp.DMails))
	}
	if resp.DMails[0].Description != "ADR-003 needs update" {
		t.Errorf("expected description 'ADR-003 needs update', got %s", resp.DMails[0].Description)
	}
}

func TestParseClaudeResponse_InvalidJSON(t *testing.T) {
	_, err := domain.ParseClaudeResponse([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseClaudeResponse_WithImpactRadius(t *testing.T) {
	// given
	raw := `{
		"axes": {
			"adr_integrity": {"score": 10, "details": "ok"},
			"dod_fulfillment": {"score": 0, "details": "ok"},
			"dependency_integrity": {"score": 0, "details": "ok"},
			"implicit_constraints": {"score": 0, "details": "ok"}
		},
		"dmails": [],
		"reasoning": "minor",
		"impact_radius": [
			{"area": "auth/session.go", "impact": "direct", "detail": "Session validation changed"},
			{"area": "api/middleware.go", "impact": "indirect", "detail": "Uses auth session"}
		]
	}`

	// when
	resp, err := domain.ParseClaudeResponse([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ImpactRadius) != 2 {
		t.Fatalf("expected 2 impact entries, got %d", len(resp.ImpactRadius))
	}
	if resp.ImpactRadius[0].Area != "auth/session.go" {
		t.Errorf("expected area 'auth/session.go', got %q", resp.ImpactRadius[0].Area)
	}
	if resp.ImpactRadius[0].Impact != "direct" {
		t.Errorf("expected impact 'direct', got %q", resp.ImpactRadius[0].Impact)
	}
	if resp.ImpactRadius[1].Impact != "indirect" {
		t.Errorf("expected impact 'indirect', got %q", resp.ImpactRadius[1].Impact)
	}
}

func TestParseClaudeResponse_WithoutImpactRadius_BackwardCompatible(t *testing.T) {
	// given: existing JSON format without impact_radius
	raw := `{
		"axes": {
			"adr_integrity": {"score": 5, "details": "ok"}
		},
		"dmails": [],
		"reasoning": "clean"
	}`

	// when
	resp, err := domain.ParseClaudeResponse([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ImpactRadius != nil {
		t.Errorf("expected nil impact_radius for old format, got %v", resp.ImpactRadius)
	}
}

func TestParseClaudeResponse_MarkdownCodeBlock(t *testing.T) {
	// given: Claude wraps JSON output in markdown code block (```json ... ```)
	// This is the root cause of the "invalid character '`'" parse failure.
	raw := "```json\n" + `{
		"axes": {
			"adr_integrity": {"score": 15, "details": "ADR-003 minor tension"},
			"dod_fulfillment": {"score": 20, "details": "ok"},
			"dependency_integrity": {"score": 0, "details": "ok"},
			"implicit_constraints": {"score": 0, "details": "ok"}
		},
		"dmails": [],
		"reasoning": "Minor tensions detected"
	}` + "\n```"

	// when
	resp, err := domain.ParseClaudeResponse([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("expected ParseClaudeResponse to strip markdown wrapper, got: %v", err)
	}
	if resp.Axes[domain.AxisADR].Score != 15 {
		t.Errorf("expected ADR score 15, got %d", resp.Axes[domain.AxisADR].Score)
	}
}

func TestParseClaudeResponse_MarkdownCodeBlockBare(t *testing.T) {
	// given: ``` without language specifier
	raw := "```\n" + `{"axes":{},"dmails":[],"reasoning":"ok"}` + "\n```"

	// when
	resp, err := domain.ParseClaudeResponse([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("expected bare code block to parse, got: %v", err)
	}
	if resp.Reasoning != "ok" {
		t.Errorf("expected reasoning 'ok', got %q", resp.Reasoning)
	}
}

func TestParseClaudeResponse_MarkdownCodeBlockWithWhitespace(t *testing.T) {
	// given: code block with leading/trailing whitespace
	raw := "\n  ```json\n" + `{"axes":{},"dmails":[],"reasoning":"padded"}` + "\n```  \n"

	// when
	resp, err := domain.ParseClaudeResponse([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("expected whitespace-padded code block to parse, got: %v", err)
	}
	if resp.Reasoning != "padded" {
		t.Errorf("expected reasoning 'padded', got %q", resp.Reasoning)
	}
}

func TestParseClaudeResponse_TextPrefixBeforeJSON(t *testing.T) {
	// given: Claude sometimes returns natural language text before the JSON response.
	// e.g., "Certainly, here's the analysis:\n\n{...}"
	// This causes: "invalid character 'C' looking for beginning of value"
	raw := `Certainly, here's the analysis of the codebase divergence:

{
	"axes": {
		"adr_integrity": {"score": 10, "details": "minor"},
		"dod_fulfillment": {"score": 0, "details": "ok"},
		"dependency_integrity": {"score": 0, "details": "ok"},
		"implicit_constraints": {"score": 5, "details": "naming"}
	},
	"dmails": [],
	"reasoning": "Low divergence"
}`

	// when
	resp, err := domain.ParseClaudeResponse([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("expected text-prefixed JSON to parse, got: %v", err)
	}
	if resp.Axes[domain.AxisADR].Score != 10 {
		t.Errorf("expected ADR score 10, got %d", resp.Axes[domain.AxisADR].Score)
	}
	if resp.Reasoning != "Low divergence" {
		t.Errorf("expected reasoning 'Low divergence', got %q", resp.Reasoning)
	}
}

func TestParseClaudeResponse_TextSuffixAfterJSON(t *testing.T) {
	// given: Claude sometimes adds text after the JSON too
	raw := `{
	"axes": {
		"adr_integrity": {"score": 0, "details": "ok"}
	},
	"dmails": [],
	"reasoning": "clean"
}

Let me know if you need more details about the analysis.`

	// when
	resp, err := domain.ParseClaudeResponse([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("expected JSON-with-suffix to parse, got: %v", err)
	}
	if resp.Reasoning != "clean" {
		t.Errorf("expected reasoning 'clean', got %q", resp.Reasoning)
	}
}

func TestParseClaudeResponse_TextPrefixWithMarkdownBlock(t *testing.T) {
	// given: Text prefix + markdown code block wrapper
	raw := "Here's the divergence analysis:\n\n```json\n" + `{
	"axes": {},
	"dmails": [],
	"reasoning": "mixed"
}` + "\n```\n\nLet me know if you have questions."

	// when
	resp, err := domain.ParseClaudeResponse([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("expected text+markdown to parse, got: %v", err)
	}
	if resp.Reasoning != "mixed" {
		t.Errorf("expected reasoning 'mixed', got %q", resp.Reasoning)
	}
}

func TestParseClaudeResponse_WithFilesRead(t *testing.T) {
	// given: response includes files_read field from file-reference prompt
	raw := `{
		"files_read": ["adrs", "dods", "diff", "previous_scores"],
		"axes": {
			"adr_integrity": {"score": 10, "details": "ok"}
		},
		"dmails": [],
		"reasoning": "evaluated"
	}`

	// when
	resp, err := domain.ParseClaudeResponse([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.FilesRead) != 4 {
		t.Fatalf("expected 4 files_read entries, got %d", len(resp.FilesRead))
	}
	if resp.FilesRead[0] != "adrs" {
		t.Errorf("expected first files_read to be 'adrs', got %q", resp.FilesRead[0])
	}
}

func TestParseClaudeResponse_WithoutFilesRead_BackwardCompatible(t *testing.T) {
	// given: old-style response without files_read
	raw := `{
		"axes": {"adr_integrity": {"score": 0, "details": "ok"}},
		"dmails": [],
		"reasoning": "ok"
	}`

	// when
	resp, err := domain.ParseClaudeResponse([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.FilesRead != nil {
		t.Errorf("expected nil FilesRead for old format, got %v", resp.FilesRead)
	}
}

func TestParseClaudeResponse_WithCapabilityViolations(t *testing.T) {
	// given: response includes capability_violations field
	raw := `{
		"axes": {
			"adr_integrity": {"score": 10, "details": "ok"},
			"dod_fulfillment": {"score": 0, "details": "ok"},
			"dependency_integrity": {"score": 0, "details": "ok"},
			"implicit_constraints": {"score": 0, "details": "ok"}
		},
		"dmails": [],
		"reasoning": "found capability violation",
		"capability_violations": [
			{
				"boundary": "external_api",
				"description": "Direct call to external service bypasses approved adapter",
				"file": "internal/service/payments.go"
			}
		]
	}`

	// when
	resp, err := domain.ParseClaudeResponse([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.CapabilityViolations) != 1 {
		t.Fatalf("expected 1 capability violation, got %d", len(resp.CapabilityViolations))
	}
	if resp.CapabilityViolations[0].Boundary != "external_api" {
		t.Errorf("expected boundary 'external_api', got %q", resp.CapabilityViolations[0].Boundary)
	}
	if resp.CapabilityViolations[0].File != "internal/service/payments.go" {
		t.Errorf("expected file path, got %q", resp.CapabilityViolations[0].File)
	}
}

func TestParseClaudeResponse_WithoutCapabilityViolations_BackwardCompatible(t *testing.T) {
	// given: old-style response without capability_violations
	raw := `{
		"axes": {"adr_integrity": {"score": 0, "details": "ok"}},
		"dmails": [],
		"reasoning": "ok"
	}`

	// when
	resp, err := domain.ParseClaudeResponse([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CapabilityViolations != nil {
		t.Errorf("expected nil CapabilityViolations for old format, got %v", resp.CapabilityViolations)
	}
}
