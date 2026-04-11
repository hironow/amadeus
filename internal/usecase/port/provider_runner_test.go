package port

// white-box-reason: port layer semgrep constraint: tests must use package port (not port_test) to avoid self-import triggering layer-port-no-import-upper rule

import (
	"context"
	"errors"
	"io"
	"testing"
)

type stubRunner struct {
	calls   int
	failN   int
	output  string
	lastOpt RunConfig
}

func (s *stubRunner) Run(_ context.Context, _ string, _ io.Writer, opts ...RunOption) (string, error) {
	s.calls++
	s.lastOpt = ApplyOptions(opts...)
	if s.calls <= s.failN {
		return "", errors.New("claude exit: non-zero")
	}
	return s.output, nil
}

func TestProviderRunner_InterfaceSatisfied(t *testing.T) {
	// given
	var runner ProviderRunner = &stubRunner{output: "ok"}

	// when
	result, err := runner.Run(context.Background(), "test", io.Discard)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
}

func TestWithAllowedTools_SetsConfig(t *testing.T) {
	// given/when
	cfg := ApplyOptions(WithAllowedTools("Read", "Write"))

	// then
	if len(cfg.AllowedTools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(cfg.AllowedTools))
	}
}

func TestWithWorkDir_SetsConfig(t *testing.T) {
	// given/when
	cfg := ApplyOptions(WithWorkDir("/tmp/work"))

	// then
	if cfg.WorkDir != "/tmp/work" {
		t.Errorf("expected '/tmp/work', got %q", cfg.WorkDir)
	}
}

func TestWithContinue_SetsConfig(t *testing.T) {
	// given/when
	cfg := ApplyOptions(WithContinue())

	// then
	if !cfg.Continue {
		t.Error("expected Continue=true")
	}
}

func TestWithModel_SetsConfig(t *testing.T) {
	// given/when
	cfg := ApplyOptions(WithModel("sonnet"))

	// then
	if cfg.Model != "sonnet" {
		t.Errorf("expected 'sonnet', got %q", cfg.Model)
	}
}

func TestApplyOptions_Empty(t *testing.T) {
	// when
	cfg := ApplyOptions()

	// then
	if cfg.AllowedTools != nil {
		t.Errorf("expected nil AllowedTools, got %v", cfg.AllowedTools)
	}
	if cfg.WorkDir != "" {
		t.Errorf("expected empty WorkDir, got %q", cfg.WorkDir)
	}
	if cfg.Continue {
		t.Error("expected Continue=false")
	}
	if cfg.Model != "" {
		t.Errorf("expected empty Model, got %q", cfg.Model)
	}
}

func TestApplyOptions_Combined(t *testing.T) {
	// given/when
	cfg := ApplyOptions(
		WithAllowedTools("Read"),
		WithWorkDir("/repo"),
		WithContinue(),
		WithModel("opus"),
	)

	// then
	if len(cfg.AllowedTools) != 1 || cfg.AllowedTools[0] != "Read" {
		t.Errorf("unexpected AllowedTools: %v", cfg.AllowedTools)
	}
	if cfg.WorkDir != "/repo" {
		t.Errorf("expected '/repo', got %q", cfg.WorkDir)
	}
	if !cfg.Continue {
		t.Error("expected Continue=true")
	}
	if cfg.Model != "opus" {
		t.Errorf("expected 'opus', got %q", cfg.Model)
	}
}
