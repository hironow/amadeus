package amadeus

import (
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
				"target": "sightjack",
				"type": "Type-S",
				"summary": "ADR-003 needs update",
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
	if resp.DMails[0].Target != TargetSightjack {
		t.Errorf("expected target sightjack, got %s", resp.DMails[0].Target)
	}
}

func TestParseClaudeResponse_InvalidJSON(t *testing.T) {
	_, err := ParseClaudeResponse([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestBuildDiffCheckPrompt(t *testing.T) {
	params := DiffCheckParams{
		PreviousScores: `{"divergence": 0.133}`,
		PRDiffs:        "diff --git a/auth.go ...",
		RelevantADRs:   "ADR-003: Use JWT for auth",
		LinkedDoDs:     "Issue #42: Session timeout must be configurable",
	}
	prompt, err := BuildDiffCheckPrompt(params)
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

func TestBuildFullCheckPrompt(t *testing.T) {
	params := FullCheckParams{
		CodebaseStructure: "src/\n  auth/\n  cart/",
		AllADRs:           "ADR-001: Use Go\nADR-003: JWT auth",
		RecentDoDs:        "Issue #42: Session timeout",
		DependencyMap:     "auth -> cart: forbidden",
	}
	prompt, err := BuildFullCheckPrompt(params)
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
