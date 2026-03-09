package session_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/session"
)

func TestParsePRReviewJSON_Approved(t *testing.T) {
	// given
	data := []byte(`{
		"reviewDecision": "APPROVED",
		"reviews": [
			{"author": {"login": "reviewer1"}, "body": "LGTM", "state": "APPROVED"}
		],
		"statusCheckRollup": [
			{"name": "ci/test", "status": "COMPLETED", "conclusion": "SUCCESS"},
			{"name": "ci/lint", "status": "COMPLETED", "conclusion": "SUCCESS"}
		]
	}`)

	// when
	review, err := session.ExportParsePRReviewJSON("#42", data)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if review.Number != "#42" {
		t.Errorf("Number = %q, want %q", review.Number, "#42")
	}
	if review.ReviewDecision != "APPROVED" {
		t.Errorf("ReviewDecision = %q, want APPROVED", review.ReviewDecision)
	}
	if review.CIStatus != "SUCCESS" {
		t.Errorf("CIStatus = %q, want SUCCESS", review.CIStatus)
	}
	if review.HasUnresolvedReviews() {
		t.Error("HasUnresolvedReviews should be false for APPROVED")
	}
	if review.HasCIFailure() {
		t.Error("HasCIFailure should be false for SUCCESS")
	}
	if len(review.Comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(review.Comments))
	}
	if review.Comments[0].Author != "reviewer1" {
		t.Errorf("Comment author = %q, want reviewer1", review.Comments[0].Author)
	}
}

func TestParsePRReviewJSON_ChangesRequested(t *testing.T) {
	// given
	data := []byte(`{
		"reviewDecision": "CHANGES_REQUESTED",
		"reviews": [
			{"author": {"login": "senior"}, "body": "Please fix error handling", "state": "CHANGES_REQUESTED"}
		],
		"statusCheckRollup": [
			{"name": "ci/test", "status": "COMPLETED", "conclusion": "FAILURE"}
		]
	}`)

	// when
	review, err := session.ExportParsePRReviewJSON("#99", data)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !review.HasUnresolvedReviews() {
		t.Error("HasUnresolvedReviews should be true for CHANGES_REQUESTED")
	}
	if !review.HasCIFailure() {
		t.Error("HasCIFailure should be true for FAILURE")
	}
}

func TestParsePRReviewJSON_EmptyChecks(t *testing.T) {
	// given
	data := []byte(`{"reviewDecision": "", "reviews": [], "statusCheckRollup": []}`)

	// when
	review, err := session.ExportParsePRReviewJSON("#1", data)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if review.CIStatus != "" {
		t.Errorf("CIStatus = %q, want empty for no checks", review.CIStatus)
	}
}

func TestParsePRReviewJSON_MixedCIStatus(t *testing.T) {
	// given — one success, one pending
	data := []byte(`{
		"reviewDecision": "APPROVED",
		"reviews": [],
		"statusCheckRollup": [
			{"name": "ci/test", "status": "COMPLETED", "conclusion": "SUCCESS"},
			{"name": "ci/deploy", "status": "IN_PROGRESS", "conclusion": ""}
		]
	}`)

	// when
	review, err := session.ExportParsePRReviewJSON("#5", data)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if review.CIStatus != "PENDING" {
		t.Errorf("CIStatus = %q, want PENDING for mixed status", review.CIStatus)
	}
}

func TestParsePRReviewJSON_InvalidJSON(t *testing.T) {
	// given
	data := []byte(`{invalid`)

	// when
	_, err := session.ExportParsePRReviewJSON("#1", data)

	// then
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
