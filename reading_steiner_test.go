package amadeus

import (
	"os/exec"
	"testing"
)

func TestDetectShift_NoChanges(t *testing.T) {
	repo := setupTestRepo(t)
	git := NewGitClient(repo.dir)
	hash, _ := git.CurrentCommit()
	rs := &ReadingSteiner{Git: git}
	report, err := rs.DetectShift(hash)
	if err != nil {
		t.Fatal(err)
	}
	if report.Significant {
		t.Error("expected no significant shift when no changes")
	}
}

func TestDetectShift_WithMergedPRs(t *testing.T) {
	repo := setupTestRepo(t)
	git := NewGitClient(repo.dir)
	cmd := exec.Command("git", "rev-list", "--max-parents=0", "HEAD")
	cmd.Dir = repo.dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	initialCommit := string(out[:len(out)-1])
	rs := &ReadingSteiner{Git: git}
	report, err := rs.DetectShift(initialCommit)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Significant {
		t.Error("expected significant shift with merged PRs")
	}
	if len(report.MergedPRs) != 2 {
		t.Errorf("expected 2 merged PRs, got %d", len(report.MergedPRs))
	}
}

func TestDetectShift_FullMode(t *testing.T) {
	repo := setupTestRepo(t)
	// Create directory and file inside the container
	repo.shell(t, "mkdir -p /repo/src && echo 'package main' > /repo/src/main.go")
	repo.git(t, "add", ".")
	repo.git(t, "commit", "-m", "add src")

	git := NewGitClient(repo.dir)
	rs := &ReadingSteiner{Git: git}
	report, err := rs.DetectShiftFull(repo.dir)
	if err != nil {
		t.Fatal(err)
	}
	if report.CodebaseStructure == "" {
		t.Error("expected non-empty codebase structure")
	}
	if !report.Significant {
		t.Error("expected full mode to always be significant")
	}
}
