package port

import (
	"context"
	"io"
)

// RunOption configures optional behavior for a provider runner invocation.
type RunOption func(*RunConfig)

// RunConfig holds per-invocation configuration for a provider runner.
type RunConfig struct {
	AllowedTools    []string
	WorkDir         string // sets cmd.Dir for the subprocess
	ConfigBase      string // base directory for resolving stateDir settings (defaults to WorkDir)
	Continue        bool   // passes --continue to resume a prior session
	Model           string // overrides the default model for this invocation
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

// WithWorkDir sets the working directory for the provider subprocess.
func WithWorkDir(dir string) RunOption {
	return func(c *RunConfig) {
		c.WorkDir = dir
	}
}

// WithContinue enables --continue mode to resume a prior provider session.
func WithContinue() RunOption {
	return func(c *RunConfig) {
		c.Continue = true
	}
}

// WithConfigBase sets the base directory for resolving tool stateDir settings
// (e.g. .claude/settings.json under the stateDir). When unset, WorkDir is used.
// Use this when WorkDir is a worktree that doesn't contain the stateDir.
func WithConfigBase(dir string) RunOption {
	return func(c *RunConfig) {
		c.ConfigBase = dir
	}
}

// WithModel overrides the default model for this invocation.
func WithModel(model string) RunOption {
	return func(c *RunConfig) {
		c.Model = model
	}
}

// WithResume targets a specific provider session for continuation.
// Mutually exclusive with WithContinue.
func WithResume(providerSessionID string) RunOption {
	return func(c *RunConfig) {
		c.ResumeSessionID = providerSessionID
	}
}

// ClaudeRunner executes an AI coding tool and returns the result text.
// Provider-agnostic: implementations wrap any CLI (Claude, Codex, Copilot, etc.).
// Implementations may stream intermediate output to w.
//
// TODO(rename): ClaudeRunner → ProviderRunner — legacy name from Claude-only era.
// The interface is fully provider-agnostic; rename blocked only by 40+ call sites.
type ClaudeRunner interface {
	Run(ctx context.Context, prompt string, w io.Writer, opts ...RunOption) (string, error)
}
