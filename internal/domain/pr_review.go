package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// PRReview holds review metadata for a merged pull request.
type PRReview struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go — domain model for PR review; Comments is a parsed list from gh API response [permanent]
	Number         string // e.g. "#42"
	ReviewDecision string // APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED, or ""
	CIStatus       string // SUCCESS, FAILURE, PENDING, or ""
	Comments       []PRComment
}

// PRComment represents a single review comment on a PR.
type PRComment struct {
	Author string
	Body   string
	State  string // PENDING, SUBMITTED, DISMISSED
}

// HasUnresolvedReviews reports whether the PR has non-approved review decisions.
func (pr PRReview) HasUnresolvedReviews() bool {
	return pr.ReviewDecision == "CHANGES_REQUESTED"
}

// HasCIFailure reports whether CI checks failed.
func (pr PRReview) HasCIFailure() bool {
	return pr.CIStatus == "FAILURE"
}

// FormatPRReviewSummary formats a slice of PRReviews into a human-readable
// summary string suitable for inclusion in a Claude prompt.
// Returns empty string if no reviews are provided.
// truncationMarker is appended when a string is truncated.
const truncationMarker = "...(truncated)"

// truncateRuneSafe truncates a string to at most maxRunes runes (including
// the truncation marker), appending "...(truncated)" if truncation occurs.
// This is safe for multi-byte UTF-8 characters (Japanese, Chinese, etc.)
// unlike byte-level truncation.
func truncateRuneSafe(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	markerLen := utf8.RuneCountInString(truncationMarker)
	cutAt := maxRunes - markerLen
	if cutAt < 0 {
		cutAt = 0
	}
	runes := []rune(s)
	return string(runes[:cutAt]) + truncationMarker
}

// totalBudgetRunes is the maximum rune count for the entire PR review summary.
const totalBudgetRunes = 8000

// perCommentMaxRunes is the maximum rune count for a single PR comment body.
const perCommentMaxRunes = 500

// FormatPRReviewSummary formats a slice of PRReviews into a human-readable
// summary string suitable for inclusion in a Claude prompt.
// Returns empty string if no reviews are provided.
func FormatPRReviewSummary(reviews []PRReview) string {
	if len(reviews) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, r := range reviews {
		fmt.Fprintf(&sb, "### PR %s\n", r.Number)
		fmt.Fprintf(&sb, "- Review decision: %s\n", valueOrNone(r.ReviewDecision))
		fmt.Fprintf(&sb, "- CI status: %s\n", valueOrNone(r.CIStatus))
		if len(r.Comments) > 0 {
			sb.WriteString("- Review comments:\n")
			for _, c := range r.Comments {
				body := truncateRuneSafe(c.Body, perCommentMaxRunes)
				fmt.Fprintf(&sb, "  - @%s (%s): %s\n", c.Author, c.State, body)
			}
		}
		sb.WriteString("\n")
	}
	result := sb.String()
	return truncateRuneSafe(result, totalBudgetRunes)
}

func valueOrNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
