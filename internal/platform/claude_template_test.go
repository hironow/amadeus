package platform_test

import (
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
)

func TestBuildDiffCheckPrompt_ReferencesFilePaths(t *testing.T) {
	// given
	params := domain.DiffCheckParams{
		EvalDir:        "/repo/.gate/.run/eval",
		HasPRReviews:   true,
		LinkedIssueIDs: "MY-123, MY-456",
	}

	// when
	prompt, err := platform.BuildDiffCheckPrompt("en", params)

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
	prompt, err := platform.BuildDiffCheckPrompt("en", params)

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
	prompt, err := platform.BuildDiffCheckPrompt("en", params)

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
	prompt, err := platform.BuildDiffCheckPrompt("ja", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "Eval Files (READ-ONLY)") {
		t.Error("expected 'Eval Files (READ-ONLY)' in ja prompt")
	}
}

func TestBuildDiffCheckPrompt_InvalidLang_ReturnsError(t *testing.T) {
	// given
	params := domain.DiffCheckParams{
		EvalDir: "/repo/.gate/.run/eval",
	}

	// when
	_, err := platform.BuildDiffCheckPrompt("fr", params)

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
	prompt, err := platform.BuildFullCheckPrompt("en", params)

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
	prompt, err := platform.BuildFullCheckPrompt("ja", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "FULL calibration") {
		t.Error("expected 'FULL calibration' in ja full check prompt")
	}
}
