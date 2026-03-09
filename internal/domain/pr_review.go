package domain

import (
	"fmt"
	"strings"
)

// PRReview holds review metadata for a merged pull request.
type PRReview struct {
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
				body := c.Body
				if len(body) > 200 {
					body = body[:200] + "..."
				}
				fmt.Fprintf(&sb, "  - @%s (%s): %s\n", c.Author, c.State, body)
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func valueOrNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
