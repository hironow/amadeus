package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
)

// ghPRViewResponse is the JSON shape from:
// gh pr view {number} --json reviewDecision,reviews,statusCheckRollup
type ghPRViewResponse struct {
	ReviewDecision    string          `json:"reviewDecision"`
	Reviews           []ghReview      `json:"reviews"`
	StatusCheckRollup []ghStatusCheck `json:"statusCheckRollup"`
}

type ghReview struct {
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	Body  string `json:"body"`
	State string `json:"state"`
}

type ghStatusCheck struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// parsePRReviewJSON parses gh pr view JSON output into a domain.PRReview.
func parsePRReviewJSON(prNumber string, data []byte) (domain.PRReview, error) {
	var resp ghPRViewResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return domain.PRReview{}, fmt.Errorf("parse PR review JSON: %w", err)
	}

	review := domain.PRReview{
		Number:         prNumber,
		ReviewDecision: resp.ReviewDecision,
	}

	for _, r := range resp.Reviews {
		review.Comments = append(review.Comments, domain.PRComment{
			Author: r.Author.Login,
			Body:   r.Body,
			State:  r.State,
		})
	}

	review.CIStatus = aggregateCIStatus(resp.StatusCheckRollup)
	return review, nil
}

// aggregateCIStatus determines the overall CI status from individual checks.
// Any failure → FAILURE, all success → SUCCESS, otherwise PENDING.
func aggregateCIStatus(checks []ghStatusCheck) string {
	if len(checks) == 0 {
		return ""
	}
	for _, c := range checks {
		if c.Conclusion == "FAILURE" || c.Conclusion == "failure" {
			return "FAILURE"
		}
	}
	allSuccess := true
	for _, c := range checks {
		if c.Conclusion != "SUCCESS" && c.Conclusion != "success" {
			allSuccess = false
			break
		}
	}
	if allSuccess {
		return "SUCCESS"
	}
	return "PENDING"
}

// GHClient fetches PR review data using the gh CLI.
type GHClient struct {
	Dir string
}

// FetchPRReviews fetches review details for the given PRs.
// Returns available reviews; errors are logged but non-fatal (graceful degradation).
func (g *GHClient) FetchPRReviews(prs []domain.MergedPR) []domain.PRReview {
	if !ghAvailable() {
		return nil
	}
	var reviews []domain.PRReview
	for _, pr := range prs {
		num := strings.TrimPrefix(pr.Number, "#")
		data, err := g.runGH("pr", "view", num, "--json", "reviewDecision,reviews,statusCheckRollup")
		if err != nil {
			continue // graceful: skip this PR
		}
		review, err := parsePRReviewJSON(pr.Number, data)
		if err != nil {
			continue
		}
		reviews = append(reviews, review)
	}
	return reviews
}

// ghAvailable checks whether the gh CLI is installed and accessible.
func ghAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

func (g *GHClient) runGH(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = g.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.Bytes(), nil
}
