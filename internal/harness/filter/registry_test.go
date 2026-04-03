package filter

import (
	"fmt"
	"strings"
	"testing"
)

func TestNewRegistry_LoadsAllPrompts(t *testing.T) {
	// when
	reg, err := NewRegistry()

	// then
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}
	names := reg.Names()
	if len(names) < 6 {
		t.Fatalf("expected at least 6 prompts, got %d: %v", len(names), names)
	}

	// Verify expected prompts exist
	for _, expected := range []string{"pr_review", "review_fix", "fileref_diff_check_en", "fileref_diff_check_ja", "fileref_full_check_en", "fileref_full_check_ja"} {
		if _, err := reg.Get(expected); err != nil {
			t.Errorf("expected prompt %q to exist: %v", expected, err)
		}
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	// given
	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}

	// when
	_, err = reg.Get("nonexistent")

	// then
	if err == nil {
		t.Fatal("expected error for nonexistent prompt")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestRegistry_Get_HasFields(t *testing.T) {
	// given
	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}

	// when
	cfg, err := reg.Get("pr_review")

	// then
	if err != nil {
		t.Fatalf("Get('pr_review') error: %v", err)
	}
	if cfg.Name != "pr_review" {
		t.Errorf("Name = %q, want %q", cfg.Name, "pr_review")
	}
	if cfg.Version < 1 {
		t.Errorf("Version = %d, want >= 1", cfg.Version)
	}
	if cfg.Description == "" {
		t.Error("Description is empty")
	}
	if len(cfg.Variables) == 0 {
		t.Error("Variables is empty")
	}
	if cfg.Template == "" {
		t.Error("Template is empty")
	}
}

func TestRegistry_Expand_PRReview(t *testing.T) {
	// given
	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}

	vars := map[string]string{
		"pr_number":   "#42",
		"pr_title":    "Add feature X",
		"base_branch": "main",
		"head_branch": "feature/x",
		"diff":        "+++ some diff content",
		"lang":        "en",
	}

	// when
	result, err := reg.Expand("pr_review", vars)

	// then
	if err != nil {
		t.Fatalf("Expand error: %v", err)
	}
	for key, val := range vars {
		if !strings.Contains(result, val) {
			t.Errorf("expanded result does not contain %s value %q", key, val)
		}
		placeholder := "{" + key + "}"
		if strings.Contains(result, placeholder) {
			t.Errorf("expanded result still contains placeholder %s", placeholder)
		}
	}
}

func TestRegistry_Expand_ReviewFix(t *testing.T) {
	// given
	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}

	vars := map[string]string{
		"branch":   "feature/fix-review",
		"comments": "Line 42: missing error check\nLine 88: unused import",
	}

	// when
	result, err := reg.Expand("review_fix", vars)

	// then
	if err != nil {
		t.Fatalf("Expand error: %v", err)
	}
	if !strings.Contains(result, "feature/fix-review") {
		t.Error("result does not contain branch name")
	}
	if !strings.Contains(result, "missing error check") {
		t.Error("result does not contain review comments")
	}
}

func TestExpandTemplate(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		vars     map[string]string
		expected string
	}{
		{
			name:     "simple substitution",
			tmpl:     "Hello {name}!",
			vars:     map[string]string{"name": "World"},
			expected: "Hello World!",
		},
		{
			name:     "multiple variables",
			tmpl:     "{a} + {b} = {c}",
			vars:     map[string]string{"a": "1", "b": "2", "c": "3"},
			expected: "1 + 2 = 3",
		},
		{
			name:     "unknown placeholder preserved",
			tmpl:     "Hello {name}, your {role} is ready",
			vars:     map[string]string{"name": "Alice"},
			expected: "Hello Alice, your {role} is ready",
		},
		{
			name:     "empty vars",
			tmpl:     "No {change}",
			vars:     map[string]string{},
			expected: "No {change}",
		},
		{
			name:     "nil vars",
			tmpl:     "No {change}",
			vars:     nil,
			expected: "No {change}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			result := ExpandTemplate(tt.tmpl, tt.vars)

			// then
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestPRReviewPrompt_MatchesLegacy verifies that the YAML template produces
// the exact same output as the old hardcoded buildPRReviewPrompt function.
func TestPRReviewPrompt_MatchesLegacy(t *testing.T) {
	// given: the old hardcoded prompt builder (reproduced here for regression)
	legacyBuild := func(prNumber, prTitle, baseBranch, headBranch, diff, lang string) string {
		return fmt.Sprintf(`You are amadeus, a post-merge integrity harness. You are evaluating a pull request diff against the project's Architecture Decision Records (ADRs) and Definitions of Done (DoDs).

## PR Information
- Number: %s
- Title: %s
- Base Branch: %s
- Head Branch: %s

## PR Diff
%s

## Instructions
1. Read all ADR files in docs/adr/ and DoD files if they exist
2. Evaluate whether this PR's changes comply with established ADRs and DoDs
3. Identify any violations, deviations, or areas of concern
4. Score the overall divergence (0 = fully compliant, 100 = completely divergent)

## Response Format (JSON)
{
  "files_read": ["docs/adr/...", ...],
  "axes": {
    "structural": {"score": 0, "details": "..."},
    "behavioral": {"score": 0, "details": "..."},
    "convention": {"score": 0, "details": "..."},
    "dependency": {"score": 0, "details": "..."}
  },
  "dmails": [
    {
      "description": "Brief description of the issue",
      "detail": "Detailed explanation",
      "targets": ["file.go"],
      "action": "retry|escalate|resolve",
      "category": "design|implementation"
    }
  ],
  "reasoning": "Overall assessment in %s"
}

Only report genuine ADR/DoD violations. Do not flag stylistic preferences or minor formatting issues.`, prNumber, prTitle, baseBranch, headBranch, diff, lang)
	}

	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}

	// when
	prNumber := "#99"
	prTitle := "Refactor domain layer"
	baseBranch := "main"
	headBranch := "feature/refactor"
	diff := "diff --git a/internal/domain/foo.go b/internal/domain/foo.go\n+++ added line"
	lang := "ja"

	legacy := legacyBuild(prNumber, prTitle, baseBranch, headBranch, diff, lang)
	expanded, err := reg.Expand("pr_review", map[string]string{
		"pr_number":   prNumber,
		"pr_title":    prTitle,
		"base_branch": baseBranch,
		"head_branch": headBranch,
		"diff":        diff,
		"lang":        lang,
	})

	// then
	if err != nil {
		t.Fatalf("Expand error: %v", err)
	}

	// The YAML template has a trailing newline from the `|` block scalar.
	// Normalize both for comparison.
	legacyNorm := strings.TrimSpace(legacy)
	expandedNorm := strings.TrimSpace(expanded)

	if legacyNorm != expandedNorm {
		// Find first difference for debugging
		legacyLines := strings.Split(legacyNorm, "\n")
		expandedLines := strings.Split(expandedNorm, "\n")
		for i := 0; i < len(legacyLines) && i < len(expandedLines); i++ {
			if legacyLines[i] != expandedLines[i] {
				t.Errorf("first difference at line %d:\n  legacy:   %q\n  expanded: %q", i+1, legacyLines[i], expandedLines[i])
				break
			}
		}
		if len(legacyLines) != len(expandedLines) {
			t.Errorf("line count differs: legacy=%d, expanded=%d", len(legacyLines), len(expandedLines))
		}
		t.Fatalf("expanded template does not match legacy output")
	}
}

// TestReviewFixPrompt_MatchesLegacy verifies that the YAML template produces
// the exact same output as the old hardcoded BuildReviewFixPrompt function.
func TestReviewFixPrompt_MatchesLegacy(t *testing.T) {
	// given: the old hardcoded prompt builder (reproduced here for regression)
	legacyBuild := func(branch, comments string) string {
		return fmt.Sprintf(`You are on branch %s. A code review found the following issues:

%s

Fix all review comments above. Commit and push your changes.
Keep fixes focused — only address the review comments, do not refactor unrelated code.`, branch, comments)
	}

	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}

	// when
	branch := "feature/fix-things"
	comments := "- Line 10: unused variable\n- Line 20: missing return"

	legacy := legacyBuild(branch, comments)
	expanded, err := reg.Expand("review_fix", map[string]string{
		"branch":   branch,
		"comments": comments,
	})

	// then
	if err != nil {
		t.Fatalf("Expand error: %v", err)
	}

	legacyNorm := strings.TrimSpace(legacy)
	expandedNorm := strings.TrimSpace(expanded)

	if legacyNorm != expandedNorm {
		legacyLines := strings.Split(legacyNorm, "\n")
		expandedLines := strings.Split(expandedNorm, "\n")
		for i := 0; i < len(legacyLines) && i < len(expandedLines); i++ {
			if legacyLines[i] != expandedLines[i] {
				t.Errorf("first difference at line %d:\n  legacy:   %q\n  expanded: %q", i+1, legacyLines[i], expandedLines[i])
				break
			}
		}
		if len(legacyLines) != len(expandedLines) {
			t.Errorf("line count differs: legacy=%d, expanded=%d", len(legacyLines), len(expandedLines))
		}
		t.Fatalf("expanded template does not match legacy output")
	}
}

func TestDefaultRegistry(t *testing.T) {
	// when
	reg, err := DefaultRegistry()

	// then
	if err != nil {
		t.Fatalf("DefaultRegistry() error: %v", err)
	}
	if reg == nil {
		t.Fatal("DefaultRegistry() returned nil")
	}

	// Verify it returns the same instance
	reg2, err := DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry() second call error: %v", err)
	}
	if reg != reg2 {
		t.Error("DefaultRegistry() returned different instances")
	}
}
