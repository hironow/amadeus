package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

func TestInsightEntry_Format(t *testing.T) {
	entry := domain.InsightEntry{
		Title: "auth module CI flaky",
		What:  "3 consecutive CI failures on auth module changes",
		Why:   "OAuth token refresh times out in GitHub Actions network",
		How:   "Extend OAuth timeout to 30s in CI environment",
		When:  "CI environment with auth module changes",
		Who:   "paintress expedition #28, #30, #31",
		Constraints: "May self-resolve with OAuth provider changes",
		Extra: map[string]string{
			"failure-type":   "ci-red",
			"gradient-level": "0",
		},
	}

	formatted := entry.Format()

	if !strings.Contains(formatted, "## Insight: auth module CI flaky") {
		t.Errorf("missing title heading, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "- **what**: 3 consecutive") {
		t.Errorf("missing what field, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "- **failure-type**: ci-red") {
		t.Errorf("missing extra field, got:\n%s", formatted)
	}
}

func TestInsightFile_Marshal(t *testing.T) {
	now := time.Date(2026, 3, 10, 15, 30, 0, 0, time.FixedZone("JST", 9*3600))
	file := domain.InsightFile{
		SchemaVersion: "1",
		Kind:          "lumina",
		Tool:          "paintress",
		UpdatedAt:     now,
		Entries: []domain.InsightEntry{
			{
				Title:       "test insight",
				What:        "observed X",
				Why:         "because Y",
				How:         "do Z",
				When:        "always",
				Who:         "test",
				Constraints: "none",
			},
		},
	}

	data, err := file.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		t.Errorf("should start with YAML frontmatter delimiter, got:\n%s", text)
	}
	if !strings.Contains(text, `insight-schema-version: "1"`) {
		t.Errorf("missing schema version, got:\n%s", text)
	}
	if !strings.Contains(text, "entries: 1") {
		t.Errorf("missing entry count, got:\n%s", text)
	}
	if !strings.Contains(text, "## Insight: test insight") {
		t.Errorf("missing insight entry, got:\n%s", text)
	}
}

func TestInsightFile_Unmarshal(t *testing.T) {
	raw := `---
insight-schema-version: "1"
kind: lumina
tool: paintress
updated_at: "2026-03-10T15:30:00+09:00"
entries: 1
---

## Insight: test insight

- **what**: observed X
- **why**: because Y
- **how**: do Z
- **when**: always
- **who**: test
- **constraints**: none
- **failure-type**: ci-red
`

	file, err := domain.UnmarshalInsightFile([]byte(raw))
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if file.Kind != "lumina" {
		t.Errorf("expected kind lumina, got %s", file.Kind)
	}
	if len(file.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.Entries))
	}
	if file.Entries[0].Title != "test insight" {
		t.Errorf("expected title 'test insight', got %q", file.Entries[0].Title)
	}
	if file.Entries[0].Extra["failure-type"] != "ci-red" {
		t.Errorf("expected extra failure-type ci-red, got %q", file.Entries[0].Extra["failure-type"])
	}
}

func TestInsightContext_Format(t *testing.T) {
	ctx := domain.InsightContext{
		Insights: []domain.InsightSummary{
			{Source: ".expedition/insights/lumina.md", Summary: "auth CI flaky"},
		},
	}

	if ctx.Insights[0].Source != ".expedition/insights/lumina.md" {
		t.Errorf("unexpected source: %s", ctx.Insights[0].Source)
	}
}
