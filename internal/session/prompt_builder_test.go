package session

import (
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestBuildDiffCheckPrompt_ReferencesFilePaths(t *testing.T) {
	// given
	params := domain.DiffCheckParams{
		EvalDir:        "/repo/.gate/.run/eval",
		HasPRReviews:   true,
		LinkedIssueIDs: "MY-123, MY-456",
	}

	// when
	prompt, err := buildDiffCheckPrompt("en", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Prompt must reference file paths, not embed content
	for _, path := range []string{
		"/repo/.gate/.run/eval/previous_scores.json",
		"/repo/.gate/.run/eval/diff.patch",
		"/repo/.gate/.run/eval/adrs.md",
		"/repo/.gate/.run/eval/dods.md",
		"/repo/.gate/.run/eval/pr_reviews.md",
	} {
		if !strings.Contains(prompt, path) {
			t.Errorf("expected eval dir path %q in prompt", path)
		}
	}
	if !strings.Contains(prompt, "files_read") {
		t.Error("expected files_read instruction in prompt")
	}
	if !strings.Contains(prompt, "impact_radius") {
		t.Error("expected impact_radius in JSON schema")
	}
	// Prompt must be small (<5K chars)
	if len(prompt) > 5000 {
		t.Errorf("expected prompt < 5000 chars, got %d", len(prompt))
	}
}

func TestBuildDiffCheckPrompt_NoPRReviews(t *testing.T) {
	// given
	params := domain.DiffCheckParams{
		EvalDir:      "/repo/.gate/.run/eval",
		HasPRReviews: false,
	}

	// when
	prompt, err := buildDiffCheckPrompt("en", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(prompt, "pr_reviews.md") {
		t.Error("expected NO pr_reviews reference when HasPRReviews=false")
	}
}

func TestBuildDiffCheckPrompt_WithLinkedIssues(t *testing.T) {
	// given
	params := domain.DiffCheckParams{
		EvalDir:        "/repo/.gate/.run/eval",
		LinkedIssueIDs: "MY-100, MY-200",
	}

	// when
	prompt, err := buildDiffCheckPrompt("en", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "MY-100, MY-200") {
		t.Error("expected linked issue IDs in prompt")
	}
	if !strings.Contains(prompt, "Linear MCP tool") {
		t.Error("expected Linear MCP tool instruction for linked issues")
	}
}

func TestBuildDiffCheckPrompt_Ja(t *testing.T) {
	// given
	params := domain.DiffCheckParams{
		EvalDir: "/repo/.gate/.run/eval",
	}

	// when
	prompt, err := buildDiffCheckPrompt("ja", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "評価ファイル (READ-ONLY)") {
		t.Error("expected '評価ファイル (READ-ONLY)' in ja prompt")
	}
}

func TestBuildDiffCheckPrompt_InvalidLang_ReturnsError(t *testing.T) {
	// given
	params := domain.DiffCheckParams{
		EvalDir: "/repo/.gate/.run/eval",
	}

	// when
	_, err := buildDiffCheckPrompt("fr", params)

	// then
	if err == nil {
		t.Error("expected error for unsupported language 'fr'")
	}
}

func TestBuildFullCheckPrompt_ReferencesFilePaths(t *testing.T) {
	// given
	params := domain.FullCheckParams{
		EvalDir: "/repo/.gate/.run/eval",
	}

	// when
	prompt, err := buildFullCheckPrompt("en", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, path := range []string{
		"/repo/.gate/.run/eval/adrs.md",
		"/repo/.gate/.run/eval/codebase_structure.md",
		"/repo/.gate/.run/eval/dependency_map.md",
		"/repo/.gate/.run/eval/dods.md",
	} {
		if !strings.Contains(prompt, path) {
			t.Errorf("expected eval dir path %q in prompt", path)
		}
	}
	if !strings.Contains(prompt, "files_read") {
		t.Error("expected files_read instruction in prompt")
	}
	if !strings.Contains(prompt, "FULL calibration") {
		t.Error("expected FULL calibration in prompt")
	}
	if !strings.Contains(prompt, "impact_radius") {
		t.Error("expected impact_radius in JSON schema")
	}
	if len(prompt) > 5000 {
		t.Errorf("expected prompt < 5000 chars, got %d", len(prompt))
	}
}

func TestBuildFullCheckPrompt_Ja(t *testing.T) {
	// given
	params := domain.FullCheckParams{
		EvalDir: "/repo/.gate/.run/eval",
	}

	// when
	prompt, err := buildFullCheckPrompt("ja", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "FULL calibration") {
		t.Error("expected 'FULL calibration' in ja full check prompt")
	}
	if !strings.Contains(prompt, "評価ファイル (READ-ONLY)") {
		t.Error("expected '評価ファイル (READ-ONLY)' in ja full check prompt")
	}
}

func TestBuildDiffCheckPrompt_WithRepeatedViolations(t *testing.T) {
	// given
	params := domain.DiffCheckParams{
		EvalDir: "/repo/.gate/.run/eval",
		RepeatedViolations: []domain.RepeatedViolation{
			{Axis: "adr_integrity", Count: 3, Description: "persistent ADR drift"},
		},
	}

	// when
	prompt, err := buildDiffCheckPrompt("en", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "Repeated Violations Warning") {
		t.Error("expected repeated violations section header")
	}
	if !strings.Contains(prompt, "adr_integrity") {
		t.Error("expected axis name in repeated violations")
	}
	if !strings.Contains(prompt, "3 consecutive violations") {
		t.Error("expected violation count")
	}
}

func TestBuildDiffCheckPrompt_WithDivergenceTrend(t *testing.T) {
	// given
	trend := &domain.DivergenceTrend{
		Class:   domain.DivergenceTrendWorsening,
		Delta:   5.2,
		Message: "Divergence is increasing steadily",
	}
	params := domain.DiffCheckParams{
		EvalDir:         "/repo/.gate/.run/eval",
		DivergenceTrend: trend,
	}

	// when
	prompt, err := buildDiffCheckPrompt("en", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "Divergence Trend") {
		t.Error("expected divergence trend section header")
	}
	if !strings.Contains(prompt, "worsening") {
		t.Error("expected trend class in prompt")
	}
	if !strings.Contains(prompt, "5.2") {
		t.Error("expected delta value in prompt")
	}
}
