package platform_test

import (
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
)

func TestBuildDiffCheckPrompt(t *testing.T) {
	params := domain.DiffCheckParams{
		PreviousScores: `{"divergence": 0.133}`,
		PRDiffs:        "diff --git a/auth.go ...",
		RelevantADRs:   "ADR-003: Use JWT for auth",
		LinkedDoDs:     "Issue #42: Session timeout must be configurable",
	}
	prompt, err := platform.BuildDiffCheckPrompt("ja", params)
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
	params := domain.DiffCheckParams{
		PreviousScores: `{"divergence": 0.1}`,
		PRDiffs:        "diff --git a/auth.go ...",
	}

	// when
	prompt, err := platform.BuildDiffCheckPrompt("ja", params)

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
	params := domain.FullCheckParams{
		CodebaseStructure: "src/",
	}

	// when
	prompt, err := platform.BuildFullCheckPrompt("ja", params)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "impact_radius") {
		t.Error("expected 'impact_radius' in full check JSON schema")
	}
}

func TestBuildFullCheckPrompt(t *testing.T) {
	params := domain.FullCheckParams{
		CodebaseStructure: "src/\n  auth/\n  cart/",
		AllADRs:           "ADR-001: Use Go\nADR-003: JWT auth",
		RecentDoDs:        "Issue #42: Session timeout",
		DependencyMap:     "auth -> cart: forbidden",
	}
	prompt, err := platform.BuildFullCheckPrompt("ja", params)
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
	params := domain.DiffCheckParams{
		PreviousScores: `{"divergence": 0.1}`,
		PRDiffs:        "diff --git a/auth.go ...",
	}

	// when
	prompt, err := platform.BuildDiffCheckPrompt("en", params)

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
	params := domain.FullCheckParams{
		CodebaseStructure: "src/",
	}

	// when
	prompt, err := platform.BuildFullCheckPrompt("en", params)

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
	params := domain.DiffCheckParams{
		PreviousScores: `{"divergence": 0.1}`,
		PRDiffs:        "diff --git a/auth.go ...",
	}

	// when
	_, err := platform.BuildDiffCheckPrompt("fr", params)

	// then
	if err == nil {
		t.Error("expected error for unsupported language 'fr'")
	}
}
