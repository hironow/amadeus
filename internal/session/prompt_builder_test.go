// white-box-reason: tests unexported prompt section renderers (renderPRReviewsSection etc.)
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

func TestBuildDiffCheckPrompt_IncludesRivalContractContext(t *testing.T) {
	// given a diff-check params carrying a current Rival Contract v1
	// projection with non-empty Intent/Decisions/Boundaries/Evidence.
	current := &domain.RivalContractContext{
		ContractID: "auth-session-expiry",
		Revision:   2,
		Title:      "Add session expiry enforcement",
		Intent:     "Prevent expired sessions from authorizing API calls.",
		Decisions:  "Enforce expiry in middleware before handler execution.",
		Boundaries: "Do not add OAuth, refresh tokens, or background cleanup.",
		Evidence:   "test: just test\nnfr.p95_latency_ms: <= 200",
	}
	params := domain.DiffCheckParams{
		EvalDir:         "/repo/.gate/.run/eval",
		CurrentContract: current,
	}

	// when
	prompt, err := buildDiffCheckPrompt("en", params)

	// then the prompt embeds a dedicated Rival Contract section with the
	// four contract-aware fields (Intent/Decisions/Boundaries/Evidence).
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "Rival Contract") {
		t.Error("expected 'Rival Contract' header in diff-check prompt")
	}
	for _, want := range []string{
		"auth-session-expiry",
		"Add session expiry enforcement",
		"Prevent expired sessions",
		"Enforce expiry in middleware",
		"Do not add OAuth",
		"nfr.p95_latency_ms",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("expected diff-check prompt to contain %q", want)
		}
	}
}

func TestBuildDiffCheckPrompt_NoRivalContract_OmitsSection(t *testing.T) {
	// given diff-check params without any Rival Contract context.
	params := domain.DiffCheckParams{EvalDir: "/repo/.gate/.run/eval"}

	// when
	prompt, err := buildDiffCheckPrompt("en", params)

	// then the prompt MUST NOT include the Rival Contract section header
	// (graceful degradation: existing flows keep working unchanged).
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(prompt, "Rival Contract") {
		t.Error("expected NO Rival Contract section when CurrentContract is nil")
	}
}

func TestBuildFullCheckPrompt_IncludesRivalContractContext(t *testing.T) {
	// given a full-check params carrying a current Rival Contract v1
	// projection. Full-check also receives contract context so calibration
	// scoring is contract-aware.
	current := &domain.RivalContractContext{
		ContractID: "auth-session-expiry",
		Revision:   1,
		Title:      "Add session expiry enforcement",
		Intent:     "Prevent expired sessions from authorizing API calls.",
		Decisions:  "Enforce expiry in middleware before handler execution.",
		Boundaries: "Do not add OAuth, refresh tokens, or background cleanup.",
		Evidence:   "test: just test",
	}
	params := domain.FullCheckParams{
		EvalDir:         "/repo/.gate/.run/eval",
		CurrentContract: current,
	}

	// when
	prompt, err := buildFullCheckPrompt("en", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "Rival Contract") {
		t.Error("expected 'Rival Contract' header in full-check prompt")
	}
	for _, want := range []string{
		"auth-session-expiry",
		"Prevent expired sessions",
		"Do not add OAuth",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("expected full-check prompt to contain %q", want)
		}
	}
}

func TestBuildFullCheckPrompt_NoRivalContract_OmitsSection(t *testing.T) {
	// given full-check params with no Rival Contract context.
	params := domain.FullCheckParams{EvalDir: "/repo/.gate/.run/eval"}

	// when
	prompt, err := buildFullCheckPrompt("en", params)

	// then graceful degradation: no Rival Contract section.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(prompt, "Rival Contract") {
		t.Error("expected NO Rival Contract section when CurrentContract is nil")
	}
}

func TestBuildDiffCheckPrompt_RivalContractContext_Ja(t *testing.T) {
	// given the same contract context on the Japanese diff-check prompt.
	current := &domain.RivalContractContext{
		ContractID: "auth-session-expiry",
		Revision:   1,
		Title:      "Add session expiry enforcement",
		Intent:     "Prevent expired sessions.",
		Decisions:  "Enforce expiry in middleware.",
		Boundaries: "Do not add OAuth.",
		Evidence:   "test: just test",
	}
	params := domain.DiffCheckParams{EvalDir: "/repo/.gate/.run/eval", CurrentContract: current}

	// when
	prompt, err := buildDiffCheckPrompt("ja", params)

	// then the section is present in the Japanese template too.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "Rival Contract") {
		t.Error("expected 'Rival Contract' header in ja diff-check prompt")
	}
	if !strings.Contains(prompt, "auth-session-expiry") {
		t.Error("expected contract ID in ja diff-check prompt")
	}
}
