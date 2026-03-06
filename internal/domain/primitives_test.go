package domain_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestNewRepoPath_Valid(t *testing.T) {
	rp, err := domain.NewRepoPath("/tmp/repo")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rp.String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %q", rp.String())
	}
}

func TestNewRepoPath_RejectsEmpty(t *testing.T) {
	_, err := domain.NewRepoPath("")
	if err == nil {
		t.Fatal("expected error for empty RepoPath")
	}
}

func TestNewDays_Valid(t *testing.T) {
	d, err := domain.NewDays(30)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if d.Int() != 30 {
		t.Errorf("expected 30, got %d", d.Int())
	}
}

func TestNewDays_RejectsZero(t *testing.T) {
	_, err := domain.NewDays(0)
	if err == nil {
		t.Fatal("expected error for zero Days")
	}
}

func TestNewDays_RejectsNegative(t *testing.T) {
	_, err := domain.NewDays(-1)
	if err == nil {
		t.Fatal("expected error for negative Days")
	}
}
