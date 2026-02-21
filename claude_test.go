package amadeus

import (
	"context"
	"strings"
	"testing"
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
	resp, err := ParseClaudeResponse([]byte(raw))
	if err != nil {
		t.Fatalf("ParseClaudeResponse failed: %v", err)
	}
	if resp.Axes[AxisADR].Score != 15 {
		t.Errorf("expected ADR score 15, got %d", resp.Axes[AxisADR].Score)
	}
	if len(resp.DMails) != 1 {
		t.Fatalf("expected 1 D-Mail, got %d", len(resp.DMails))
	}
	if resp.DMails[0].Description != "ADR-003 needs update" {
		t.Errorf("expected description 'ADR-003 needs update', got %s", resp.DMails[0].Description)
	}
}

func TestParseClaudeResponse_InvalidJSON(t *testing.T) {
	_, err := ParseClaudeResponse([]byte("not json"))
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
	resp, err := ParseClaudeResponse([]byte(raw))

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
	resp, err := ParseClaudeResponse([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ImpactRadius != nil {
		t.Errorf("expected nil impact_radius for old format, got %v", resp.ImpactRadius)
	}
}

func TestBuildDiffCheckPrompt(t *testing.T) {
	params := DiffCheckParams{
		PreviousScores: `{"divergence": 0.133}`,
		PRDiffs:        "diff --git a/auth.go ...",
		RelevantADRs:   "ADR-003: Use JWT for auth",
		LinkedDoDs:     "Issue #42: Session timeout must be configurable",
	}
	prompt, err := BuildDiffCheckPrompt("ja", params)
	if err != nil {
		t.Fatalf("BuildDiffCheckPrompt failed: %v", err)
	}
	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
	for _, want := range []string{"Previous State", "Changes Since Last Check", "ADRs"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing expected section %q", want)
		}
	}
}

func TestBuildDiffCheckPrompt_IncludesImpactRadiusSchema(t *testing.T) {
	// given
	params := DiffCheckParams{
		PreviousScores: `{"divergence": 0.1}`,
		PRDiffs:        "diff --git a/auth.go ...",
	}

	// when
	prompt, err := BuildDiffCheckPrompt("ja", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "impact_radius") {
		t.Error("expected 'impact_radius' in diff check JSON schema")
	}
}

func TestBuildFullCheckPrompt_IncludesImpactRadiusSchema(t *testing.T) {
	// given
	params := FullCheckParams{
		CodebaseStructure: "src/",
	}

	// when
	prompt, err := BuildFullCheckPrompt("ja", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "impact_radius") {
		t.Error("expected 'impact_radius' in full check JSON schema")
	}
}

func TestBuildFullCheckPrompt(t *testing.T) {
	params := FullCheckParams{
		CodebaseStructure: "src/\n  auth/\n  cart/",
		AllADRs:           "ADR-001: Use Go\nADR-003: JWT auth",
		RecentDoDs:        "Issue #42: Session timeout",
		DependencyMap:     "auth -> cart: forbidden",
	}
	prompt, err := BuildFullCheckPrompt("ja", params)
	if err != nil {
		t.Fatalf("BuildFullCheckPrompt failed: %v", err)
	}
	if !strings.Contains(prompt, "FULL calibration") {
		t.Error("prompt missing 'FULL calibration' section")
	}
	if !strings.Contains(prompt, "Codebase Structure") {
		t.Error("prompt missing 'Codebase Structure' section")
	}
}

func TestBuildDiffCheckPrompt_En(t *testing.T) {
	// given
	params := DiffCheckParams{
		PreviousScores: `{"divergence": 0.1}`,
		PRDiffs:        "diff --git a/auth.go ...",
	}

	// when
	prompt, err := BuildDiffCheckPrompt("en", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "Previous State") {
		t.Error("expected 'Previous State' in en diff check prompt")
	}
	if !strings.Contains(prompt, "impact_radius") {
		t.Error("expected 'impact_radius' in en diff check JSON schema")
	}
}

func TestBuildFullCheckPrompt_En(t *testing.T) {
	// given
	params := FullCheckParams{
		CodebaseStructure: "src/",
	}

	// when
	prompt, err := BuildFullCheckPrompt("en", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "FULL calibration") {
		t.Error("expected 'FULL calibration' in en full check prompt")
	}
}

func TestBuildDiffCheckPrompt_InvalidLang_ReturnsError(t *testing.T) {
	// given
	params := DiffCheckParams{
		PreviousScores: `{"divergence": 0.1}`,
		PRDiffs:        "diff --git a/auth.go ...",
	}

	// when
	_, err := BuildDiffCheckPrompt("fr", params)

	// then
	if err == nil {
		t.Error("expected error for unsupported language 'fr'")
	}
}

// installFakeClaude replaces runClaude with a fake that returns canned JSON.
// Returns a cleanup function that restores the original.
func installFakeClaude(response string) func() {
	orig := runClaude
	runClaude = func(_ context.Context, _ string) ([]byte, error) {
		return []byte(response), nil
	}
	return func() { runClaude = orig }
}

func TestRunClaude_FakeInstallation(t *testing.T) {
	// given
	canned := `{
		"axes": {
			"adr_integrity": {"score": 10, "details": "test"},
			"dod_fulfillment": {"score": 0, "details": "ok"},
			"dependency_integrity": {"score": 0, "details": "ok"},
			"implicit_constraints": {"score": 0, "details": "ok"}
		},
		"dmails": [],
		"reasoning": "fake response"
	}`
	cleanup := installFakeClaude(canned)
	defer cleanup()

	// when
	raw, err := runClaude(context.Background(), "test prompt")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, err := ParseClaudeResponse(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if resp.Axes[AxisADR].Score != 10 {
		t.Errorf("expected ADR score 10, got %d", resp.Axes[AxisADR].Score)
	}
	if resp.Reasoning != "fake response" {
		t.Errorf("expected reasoning 'fake response', got %q", resp.Reasoning)
	}
}
