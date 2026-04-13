package domain_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestNewExecuteCheckCommand(t *testing.T) {
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewExecuteCheckCommand(rp)
	if cmd.RepoPath().String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %s", cmd.RepoPath().String())
	}
}

func TestNewRunSyncCommand(t *testing.T) {
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewRunSyncCommand(rp)
	if cmd.RepoPath().String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %s", cmd.RepoPath().String())
	}
}

func TestNewRebuildCommand(t *testing.T) {
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewRebuildCommand(rp)
	if cmd.RepoPath().String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %s", cmd.RepoPath().String())
	}
}

func TestNewInitCommand(t *testing.T) {
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewInitCommand(rp, "")
	if cmd.RepoRoot().String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %s", cmd.RepoRoot().String())
	}
}

func TestNewArchivePruneCommand(t *testing.T) {
	rp, _ := domain.NewRepoPath("/tmp/repo")
	d, _ := domain.NewDays(30)
	cmd := domain.NewArchivePruneCommand(rp, d, true, false)
	if cmd.RepoPath().String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %s", cmd.RepoPath().String())
	}
	if cmd.Days().Int() != 30 {
		t.Errorf("expected 30, got %d", cmd.Days().Int())
	}
	if !cmd.DryRun() {
		t.Error("expected DryRun true")
	}
	if cmd.Yes() {
		t.Error("expected Yes false")
	}
}

func TestNewExecuteRunCommand(t *testing.T) {
	rp, err := domain.NewRepoPath("/tmp/repo")
	if err != nil {
		t.Fatal(err)
	}
	cmd := domain.NewExecuteRunCommand(rp, "main")
	if cmd.BaseBranch() != "main" {
		t.Errorf("got %q", cmd.BaseBranch())
	}
	if cmd.RepoPath().String() != "/tmp/repo" {
		t.Errorf("got %q", cmd.RepoPath().String())
	}
}

func TestNewExecuteRunCommand_emptyBase(t *testing.T) {
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewExecuteRunCommand(rp, "")
	if cmd.BaseBranch() != "" {
		t.Error("expected empty")
	}
}
