package usecase_test

import (
	"fmt"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase"
	"github.com/hironow/amadeus/internal/usecase/port"
)

type stubInitRunner struct {
	called  bool
	baseDir string
	err     error
}

func (s *stubInitRunner) InitProject(baseDir string, _ ...port.InitOption) ([]string, error) {
	s.called = true
	s.baseDir = baseDir
	return nil, s.err
}

func TestRunInit_ValidCommand(t *testing.T) {
	runner := &stubInitRunner{}
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewInitCommand(rp, "")

	_, err := usecase.RunInit(cmd, runner)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !runner.called {
		t.Fatal("expected InitProject to be called")
	}
	if runner.baseDir != "/tmp/repo" {
		t.Errorf("expected baseDir /tmp/repo, got %q", runner.baseDir)
	}
}

func TestRunInit_RunnerError(t *testing.T) {
	runner := &stubInitRunner{err: fmt.Errorf("disk full")}
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewInitCommand(rp, "")

	_, err := usecase.RunInit(cmd, runner)

	if err == nil {
		t.Fatal("expected error from runner")
	}
}
