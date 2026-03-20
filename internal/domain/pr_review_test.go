package domain_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestPRReview_HasUnresolvedReviews(t *testing.T) {
	tests := []struct {
		name     string
		decision string
		want     bool
	}{
		{"approved", "APPROVED", false},
		{"changes requested", "CHANGES_REQUESTED", true},
		{"empty", "", false},
		{"review required", "REVIEW_REQUIRED", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := domain.PRReview{ReviewDecision: tt.decision}
			if got := pr.HasUnresolvedReviews(); got != tt.want {
				t.Errorf("HasUnresolvedReviews() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatPRReviewSummary_Empty(t *testing.T) {
	got := domain.FormatPRReviewSummary(nil)
	if got != "" {
		t.Errorf("expected empty string for nil reviews, got %q", got)
	}
}

func TestFormatPRReviewSummary_WithReviews(t *testing.T) {
	reviews := []domain.PRReview{
		{
			Number:         "#42",
			ReviewDecision: "CHANGES_REQUESTED",
			CIStatus:       "FAILURE",
			Comments: []domain.PRComment{
				{Author: "senior", Body: "Fix error handling", State: "CHANGES_REQUESTED"},
			},
		},
		{
			Number:         "#43",
			ReviewDecision: "APPROVED",
			CIStatus:       "SUCCESS",
		},
	}

	got := domain.FormatPRReviewSummary(reviews)

	if !strings.Contains(got, "### PR #42") {
		t.Error("should contain PR #42 header")
	}
	if !strings.Contains(got, "CHANGES_REQUESTED") {
		t.Error("should contain CHANGES_REQUESTED")
	}
	if !strings.Contains(got, "@senior") {
		t.Error("should contain reviewer name")
	}
	if !strings.Contains(got, "### PR #43") {
		t.Error("should contain PR #43 header")
	}
	if !strings.Contains(got, "APPROVED") {
		t.Error("should contain APPROVED")
	}
}

func TestFormatPRReviewSummary_TruncatesLongComments(t *testing.T) {
	longBody := strings.Repeat("x", 600)
	reviews := []domain.PRReview{
		{
			Number: "#1",
			Comments: []domain.PRComment{
				{Author: "a", Body: longBody, State: "SUBMITTED"},
			},
		},
	}

	got := domain.FormatPRReviewSummary(reviews)

	if strings.Contains(got, longBody) {
		t.Error("long comment body should be truncated")
	}
	if !strings.Contains(got, "...(truncated)") {
		t.Error("truncated comment should end with ...(truncated)")
	}
}

func TestFormatPRReviewSummary_UTF8SafeTruncation(t *testing.T) {
	// given: a comment body with multi-byte Japanese characters
	// Each Japanese character is 3 bytes in UTF-8
	japaneseBody := strings.Repeat("あ", 600) // 600 runes, 1800 bytes
	reviews := []domain.PRReview{
		{
			Number: "#1",
			Comments: []domain.PRComment{
				{Author: "reviewer", Body: japaneseBody, State: "SUBMITTED"},
			},
		},
	}

	// when
	got := domain.FormatPRReviewSummary(reviews)

	// then: should not contain invalid UTF-8 sequences
	for i, r := range got {
		if r == '\uFFFD' {
			t.Errorf("found invalid UTF-8 replacement character at position %d", i)
			break
		}
	}
	// should be truncated (600 runes > 500 per-comment limit)
	if strings.Contains(got, japaneseBody) {
		t.Error("expected Japanese comment to be truncated")
	}
	if !strings.Contains(got, "...(truncated)") {
		t.Error("truncated comment should end with ...(truncated)")
	}
}

func TestFormatPRReviewSummary_ShortCommentNotTruncated(t *testing.T) {
	body := "Short comment"
	reviews := []domain.PRReview{
		{
			Number: "#1",
			Comments: []domain.PRComment{
				{Author: "a", Body: body, State: "SUBMITTED"},
			},
		},
	}
	got := domain.FormatPRReviewSummary(reviews)
	if !strings.Contains(got, body) {
		t.Error("short comment should not be truncated")
	}
}

func TestFormatPRReviewSummary_TotalBudgetEnforced(t *testing.T) {
	// given: 50 PRs each with long comments that would exceed 8000 runes total
	var reviews []domain.PRReview
	for i := 0; i < 50; i++ {
		reviews = append(reviews, domain.PRReview{
			Number:         fmt.Sprintf("#%d", i+1),
			ReviewDecision: "CHANGES_REQUESTED",
			CIStatus:       "FAILURE",
			Comments: []domain.PRComment{
				{Author: "reviewer", Body: strings.Repeat("x", 400), State: "SUBMITTED"},
				{Author: "author", Body: strings.Repeat("y", 400), State: "SUBMITTED"},
			},
		})
	}

	// when
	got := domain.FormatPRReviewSummary(reviews)

	// then: output must not exceed 8000 runes
	runeCount := len([]rune(got))
	if runeCount > 8000 {
		t.Errorf("total budget exceeded: got %d runes, want <= 8000", runeCount)
	}
}

func TestPRReview_HasCIFailure(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"success", "SUCCESS", false},
		{"failure", "FAILURE", true},
		{"pending", "PENDING", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := domain.PRReview{CIStatus: tt.status}
			if got := pr.HasCIFailure(); got != tt.want {
				t.Errorf("HasCIFailure() = %v, want %v", got, tt.want)
			}
		})
	}
}
