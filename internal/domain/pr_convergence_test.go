package domain_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

// --- PRState constructor tests ---

func TestNewPRState_valid(t *testing.T) {
	ps, err := domain.NewPRState("#42", "Fix bug", "main", "feature/fix", true, 3, []string{"file.go"}, nil, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ps.Number() != "#42" {
		t.Errorf("expected #42, got %q", ps.Number())
	}
	if ps.BaseBranch() != "main" {
		t.Errorf("expected main, got %q", ps.BaseBranch())
	}
	if !ps.HasConflict() {
		t.Error("expected HasConflict true when conflictFiles non-empty")
	}
}

func TestNewPRState_emptyNumber_fails(t *testing.T) {
	_, err := domain.NewPRState("", "title", "main", "feat", true, 0, nil, nil, "")
	if err == nil {
		t.Fatal("expected error for empty number")
	}
}

func TestNewPRState_emptyBaseBranch_fails(t *testing.T) {
	_, err := domain.NewPRState("#1", "title", "", "feat", true, 0, nil, nil, "")
	if err == nil {
		t.Fatal("expected error for empty baseBranch")
	}
}

func TestNewPRState_emptyHeadBranch_fails(t *testing.T) {
	_, err := domain.NewPRState("#1", "title", "main", "", true, 0, nil, nil, "")
	if err == nil {
		t.Fatal("expected error for empty headBranch")
	}
}

func TestNewPRState_noConflict(t *testing.T) {
	ps, err := domain.NewPRState("#1", "title", "main", "feat", true, 0, nil, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps.HasConflict() {
		t.Error("expected HasConflict false when conflictFiles is nil")
	}
}
