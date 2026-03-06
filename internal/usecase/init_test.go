package usecase_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase"
)

type stubInitRunner struct {
	called   bool
	stateDir string
	err      error
}

func (s *stubInitRunner) InitGateDir(stateDir string) error {
	s.called = true
	s.stateDir = stateDir
	return s.err
}

func TestRunInit_ValidCommand(t *testing.T) {
	runner := &stubInitRunner{}
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewInitCommand(rp)

	err := usecase.RunInit(cmd, runner)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !runner.called {
		t.Fatal("expected InitGateDir to be called")
	}
	expected := filepath.Join("/tmp/repo", domain.StateDir)
	if runner.stateDir != expected {
		t.Errorf("expected stateDir %q, got %q", expected, runner.stateDir)
	}
}

func TestRunInit_RunnerError(t *testing.T) {
	runner := &stubInitRunner{err: fmt.Errorf("disk full")}
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewInitCommand(rp)

	err := usecase.RunInit(cmd, runner)

	if err == nil {
		t.Fatal("expected error from runner")
	}
}
