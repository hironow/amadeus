package session_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/session"
)

func TestParseGhPRListOutput_valid(t *testing.T) {
	// given
	raw := `[
		{"number":42,"title":"Fix bug","baseRefName":"main","headRefName":"feature/fix","mergeable":"MERGEABLE"},
		{"number":99,"title":"Add feature","baseRefName":"main","headRefName":"feature/add","mergeable":"MERGEABLE"}
	]`

	// when
	prs, err := session.ExportParseGhPRListOutput([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(prs))
	}

	if prs[0].Number() != "#42" {
		t.Errorf("expected #42, got %s", prs[0].Number())
	}
	if prs[0].Title() != "Fix bug" {
		t.Errorf("expected Fix bug, got %s", prs[0].Title())
	}
	if prs[0].BaseBranch() != "main" {
		t.Errorf("expected main, got %s", prs[0].BaseBranch())
	}
	if prs[0].HeadBranch() != "feature/fix" {
		t.Errorf("expected feature/fix, got %s", prs[0].HeadBranch())
	}
	if !prs[0].Mergeable() {
		t.Error("expected mergeable to be true")
	}

	if prs[1].Number() != "#99" {
		t.Errorf("expected #99, got %s", prs[1].Number())
	}
	if prs[1].Title() != "Add feature" {
		t.Errorf("expected Add feature, got %s", prs[1].Title())
	}
}

func TestParseGhPRListOutput_conflict(t *testing.T) {
	// given
	raw := `[
		{"number":10,"title":"Conflict PR","baseRefName":"main","headRefName":"feat/conflict","mergeable":"CONFLICTING"}
	]`

	// when
	prs, err := session.ExportParseGhPRListOutput([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].Mergeable() {
		t.Error("expected mergeable to be false for CONFLICTING")
	}
	if prs[0].Number() != "#10" {
		t.Errorf("expected #10, got %s", prs[0].Number())
	}
}

func TestParseGhPRListOutput_empty(t *testing.T) {
	// given
	raw := `[]`

	// when
	prs, err := session.ExportParseGhPRListOutput([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 0 {
		t.Errorf("expected 0 PRs, got %d", len(prs))
	}
}

func TestParseGhPRListOutput_malformed(t *testing.T) {
	// given
	raw := `not valid json`

	// when
	_, err := session.ExportParseGhPRListOutput([]byte(raw))

	// then
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestParseGhPRListOutput_unknownMergeable(t *testing.T) {
	// given — UNKNOWN mergeable should be treated as not mergeable
	raw := `[
		{"number":5,"title":"Unknown state","baseRefName":"main","headRefName":"feat/unknown","mergeable":"UNKNOWN"}
	]`

	// when
	prs, err := session.ExportParseGhPRListOutput([]byte(raw))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].Mergeable() {
		t.Error("expected mergeable to be false for UNKNOWN")
	}
}
