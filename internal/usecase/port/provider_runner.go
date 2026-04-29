package port

import (
	"context"
	"io"
)

// RunOption configures optional behavior for a provider runner invocation.
type RunOption func(*RunConfig)

// RunConfig holds per-invocation configuration for a provider runner.
type RunConfig struct { // nosemgrep: structure.exported-struct-and-interface-go -- RunConfig co-locates with ProviderRunner interface as its configuration type; splitting would fragment the provider runner API [permanent]
	AllowedTools    []string
	WorkDir         string // sets cmd.Dir for the subprocess
	ConfigBase      string // base directory for resolving provider-specific settings (defaults to WorkDir)
	Continue        bool   // resume a prior session (implementation maps to provider-specific flag)
	Model           string // overrides the default model for this invocation
	ResumeSessionID string // target a specific provider session for continuation (mutually exclusive with Continue)
	CodingSessionID string // our CodingSessionRecord.ID for stream event correlation
}

// ApplyOptions applies RunOption functions to a RunConfig and returns it.
func ApplyOptions(opts ...RunOption) RunConfig {
	var c RunConfig
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// WithAllowedTools restricts the tools available to the provider model.
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

// WithContinue enables continuation of a prior provider session.
func WithContinue() RunOption {
	return func(c *RunConfig) {
		c.Continue = true
	}
}

// WithConfigBase sets the base directory for resolving tool stateDir settings
// (e.g. provider-specific settings under the stateDir). When unset, WorkDir is used.
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

// WithCodingSessionID sets the CodingSessionRecord.ID for stream event correlation.
// SessionTrackingAdapter injects this so live stream events can be correlated with
// the persisted coding session record.
func WithCodingSessionID(id string) RunOption {
	return func(c *RunConfig) {
		c.CodingSessionID = id
	}
}

// WithResume targets a specific provider session for continuation.
// Mutually exclusive with WithContinue.
func WithResume(providerSessionID string) RunOption {
	return func(c *RunConfig) {
		c.ResumeSessionID = providerSessionID
	}
}

// ProviderRunner executes an AI coding tool and returns the result text.
// Provider-agnostic: implementations wrap any CLI (Claude, Codex, Copilot, etc.).
// Implementations may stream intermediate output to w.
type ProviderRunner interface { // nosemgrep: structure.multiple-exported-interfaces-go -- provider runner port family (RunConfig/ProviderRunner) is a cohesive API; see RunConfig [permanent]
	Run(ctx context.Context, prompt string, w io.Writer, opts ...RunOption) (string, error)
}
