package session

import (
	"context"
	"errors"
	"io"
	"os/exec"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// ClaudeAdapter implements port.ProviderRunner. Post jun15 MCP pivot
// (refs/issues/0027), the Run / RunDetailed bodies return
// ErrMCPPivotDeprecated rather than executing `claude --print` via
// exec.Command. LLM inference now happens inside a human-initiated
// claude code interactive session driven by the amadeus MCP server
// (`amadeus mcp` subcommand) plus the /review-gate slash command
// defined in plugins/amadeus/skills/review-gate/SKILL.md.
//
// The struct retains its config fields so call sites (run / sync /
// pr-review / merge / etc.) that construct it via ClaudeAdapter{...}
// still compile. Composite roots can wire it in but every Run / RunDetailed
// call short-circuits to ErrMCPPivotDeprecated until the MCP-driven
// pipeline replaces this adapter entirely.
type ClaudeAdapter struct { // nosemgrep: domain-primitives.public-string-field-go — internal CLI subprocess adapter; fields exported for struct literal initialization in cmd (composition root), not domain primitives [permanent]
	ClaudeCmd  string
	Model      string
	TimeoutSec int
	Logger     domain.Logger
	ToolName   string                      // CLI tool name for stream events (e.g. "amadeus")
	StreamBus  port.SessionStreamPublisher // optional: live session event streaming
	// NewCmd overrides command creation. Kept on the struct so existing
	// composition roots and tests retain their wiring; the field is no
	// longer read by Run / RunDetailed post pivot.
	NewCmd func(ctx context.Context, name string, args ...string) *exec.Cmd
	// CancelFunc retained for the same reason as NewCmd above.
	CancelFunc func(cmd *exec.Cmd) func() error
}

// ErrMCPPivotDeprecated is returned by ClaudeAdapter.Run /
// ClaudeAdapter.RunDetailed now that the LLM invocation layer has
// moved to a human-initiated claude code interactive session per
// refs/issues/0027 (jun15 MCP pivot). Callers should surface this
// error and direct the operator to launch claude code with:
//
//	claude --plugin-dir ./plugins/amadeus \
//	       --mcp-config '{"amadeus":{"command":"amadeus","args":["mcp"]}}'
//
// then invoke the /review-gate slash command.
var ErrMCPPivotDeprecated = errors.New(
	"amadeus Go CLI claude adapter deprecated post jun15 MCP pivot: " +
		"use claude code /review-gate skill (refs/issues/0027)",
)

// Run returns ErrMCPPivotDeprecated. The previous implementation
// invoked `claude --print` via exec.Command and streamed stream-json
// from stdout (~290 lines), which is forbidden post the jun15 MCP
// pivot (refs/issues/0027 §5 billing boundary).
func (a *ClaudeAdapter) Run(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (string, error) {
	result, err := a.RunDetailed(ctx, prompt, w, opts...)
	return result.Text, err
}

// RunDetailed returns ErrMCPPivotDeprecated. See Run for context.
func (a *ClaudeAdapter) RunDetailed(_ context.Context, _ string, _ io.Writer, _ ...port.RunOption) (port.RunResult, error) {
	if a.Logger != nil {
		a.Logger.Warn("amadeus: ClaudeAdapter.RunDetailed() is deprecated (refs/issues/0027 jun15 MCP pivot); use the claude code /review-gate skill instead.")
	}
	return port.RunResult{}, ErrMCPPivotDeprecated
}
