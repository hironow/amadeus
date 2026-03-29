package port

import (
	"context"
	"io"
)

// RunOption configures optional behavior for a ClaudeRunner invocation.
type RunOption func(*RunConfig)

// RunConfig holds per-invocation configuration for ClaudeRunner.
type RunConfig struct {
	AllowedTools    []string
	WorkDir         string // sets cmd.Dir for the subprocess
	ConfigBase      string // base directory for resolving stateDir settings (defaults to WorkDir)
	Continue        bool   // passes --continue to resume a prior session
	ResumeSessionID string // passes --resume <id> to target a specific session (mutually exclusive with Continue)
}

// ApplyOptions applies RunOption functions to a RunConfig and returns it.
func ApplyOptions(opts ...RunOption) RunConfig {
	var c RunConfig
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// WithAllowedTools restricts the tools available to the Claude model.
func WithAllowedTools(tools ...string) RunOption {
	return func(c *RunConfig) {
		c.AllowedTools = tools
	}
}

// WithWorkDir sets the working directory for the Claude subprocess.
func WithWorkDir(dir string) RunOption {
	return func(c *RunConfig) {
		c.WorkDir = dir
	}
}

// WithConfigBase sets the base directory for resolving tool stateDir settings
// (e.g. .gate/.claude/settings.json). When unset, WorkDir is used.
func WithConfigBase(dir string) RunOption {
	return func(c *RunConfig) {
		c.ConfigBase = dir
	}
}

// WithContinue enables --continue mode to resume a prior Claude session.
func WithContinue() RunOption {
	return func(c *RunConfig) {
		c.Continue = true
	}
}

// WithResume targets a specific provider session for continuation.
// Mutually exclusive with WithContinue.
func WithResume(providerSessionID string) RunOption {
	return func(c *RunConfig) {
		c.ResumeSessionID = providerSessionID
	}
}

// ClaudeRunner executes the Claude CLI and returns the result text.
// Implementations may stream intermediate output to w.
type ClaudeRunner interface {
	Run(ctx context.Context, prompt string, w io.Writer, opts ...RunOption) (string, error)
}
