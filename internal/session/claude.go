package session

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/usecase/port"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// DivergenceMeterAllowedTools is the minimal tool set for divergence evaluation.
// The divergence meter only needs to read pre-collected content from the prompt;
// all filesystem I/O is done by Go before invoking Claude.
var DivergenceMeterAllowedTools = []string{
	"Read",
	"Bash(cat:*)",
}

// ClaudeAdapter executes the real Claude CLI as a subprocess.
// Exported for composition (e.g. RetryRunner wrapping).
type ClaudeAdapter struct {
	ClaudeCmd  string        // Claude CLI command name (from config)
	Model      string        // Claude model name (from config)
	TimeoutSec int           // per-invocation timeout in seconds (reserved for future use)
	Logger     domain.Logger // nil-safe; warnings are sent here instead of raw stderr
}

// Run executes the Claude CLI with the given prompt via stdin and returns raw output.
// Uses --dangerously-skip-permissions because amadeus runs non-interactively with --print.
// The writer parameter is accepted for interface compatibility and ignored; opts are applied via port.ApplyOptions and control allowed tools, working directory, and continuation behavior.
func (a *ClaudeAdapter) Run(ctx context.Context, prompt string, _ io.Writer, opts ...port.RunOption) (string, error) {
	cfg := port.ApplyOptions(opts...)
	claudeCmd := a.ClaudeCmd
	model := a.Model

	args := []string{
		"--model", model,
		"--verbose",
		"--output-format", "stream-json",
		"--disable-slash-commands",
		"--dangerously-skip-permissions",
		"--print",
	}

	// --allowedTools: use caller-specified tools, or default to DivergenceMeterAllowedTools.
	allowedTools := DivergenceMeterAllowedTools
	if len(cfg.AllowedTools) > 0 {
		allowedTools = cfg.AllowedTools
	}
	args = append(args, "--allowedTools", strings.Join(allowedTools, ","))

	if cfg.Continue {
		args = append(args, "--continue")
	}

	// Enforce MCP allowlist when mcp-config.json exists
	if mcpPath := MCPConfigPath(effectiveDir(cfg.WorkDir)); mcpPath != "" {
		if _, statErr := os.Stat(mcpPath); statErr == nil {
			args = append(args, "--strict-mcp-config", "--mcp-config", mcpPath)
		}
	}

	cmd := platform.NewShellCmd(ctx, claudeCmd, args...)
	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}
	cmd.Stdin = bytes.NewBufferString(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Include both stderr and stdout in the error: Claude CLI in
		// stream-json mode may report errors via stdout NDJSON while
		// stderr remains empty.
		diagnostic := stderr.String()
		if diagnostic == "" {
			diagnostic = stdout.String()
		}
		// Suppress raw NDJSON from user-facing errors; log full content
		// at debug level so it remains available with --verbose.
		if platform.IsNDJSON(diagnostic) {
			if a.Logger != nil {
				a.Logger.Debug("claude raw output:\n%s", diagnostic)
			}
			diagnostic = platform.SummarizeNDJSON(diagnostic)
		}
		return "", fmt.Errorf("claude: %w\n%s", err, diagnostic)
	}

	// Parse stream-json with span-emitting reader for OTel + Weave integration
	sr := platform.NewStreamReader(bytes.NewReader(stdout.Bytes()))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, platform.Tracer)
	emitter.SetInput(prompt)

	result, messages, err := emitter.CollectAll()
	if err != nil {
		return "", fmt.Errorf("stream-json parse: %w", err)
	}
	if result == nil {
		return "", fmt.Errorf("no result message in stream-json output")
	}

	// Set GenAI and Weave attributes on the parent invoke span
	span := trace.SpanFromContext(ctx)
	var responseModel, responseID string
	for _, msg := range messages {
		if msg.Type == "assistant" {
			if am, _ := msg.ParseAssistantMessage(); am != nil {
				if am.Model != "" {
					responseModel = am.Model
				}
				if am.ID != "" {
					responseID = am.ID
				}
			}
		}
		if msg.Type == "result" {
			span.SetAttributes(platform.GenAIResultAttrs(msg, responseModel, responseID)...)
		}
	}
	if rawEvents := emitter.RawEvents(); len(rawEvents) > 0 {
		sanitized := make([]string, len(rawEvents))
		for i, ev := range rawEvents {
			sanitized[i] = platform.SanitizeUTF8(ev)
		}
		span.SetAttributes(attribute.StringSlice("stream.raw_events", platform.SanitizeUTF8Slice(sanitized)))
	}
	if result.SessionID != "" {
		span.SetAttributes(platform.GenAISessionAttrs(result.SessionID)...)
	}
	if weaveAttrs := emitter.WeaveThreadAttrs(); len(weaveAttrs) > 0 {
		span.SetAttributes(weaveAttrs...)
	}
	if ioAttrs := emitter.WeaveIOAttrs(); len(ioAttrs) > 0 {
		span.SetAttributes(ioAttrs...)
	}
	if initAttrs := emitter.InitAttrs(); len(initAttrs) > 0 {
		span.SetAttributes(initAttrs...)
	}

	// Context budget measurement: estimate and record hook/plugin context consumption.
	budget := platform.CalculateContextBudget(messages)
	span.SetAttributes(budget.Attrs()...)
	if warning := budget.WarningMessage(platform.DefaultContextBudgetThreshold); warning != "" {
		if a.Logger != nil {
			a.Logger.Warn("%s", warning)
		}
	}

	// Phase 5: persist raw events to .run/claude-logs/
	if raw := emitter.RawEvents(); len(raw) > 0 {
		if logErr := WriteClaudeLog(effectiveDir(cfg.WorkDir), raw); logErr != nil && a.Logger != nil {
			a.Logger.Warn("claude-log write: %v", logErr)
		}
	}

	return result.Result, nil
}

// DefaultClaudeRunner returns a ClaudeRunner that invokes the given Claude CLI command.
// Both claudeCmd and model are expected to be set by the caller (from config).
func DefaultClaudeRunner(claudeCmd string, model string, logger domain.Logger) port.ClaudeRunner {
	return &ClaudeAdapter{ClaudeCmd: claudeCmd, Model: model, Logger: logger}
}

// effectiveDir returns dir if non-empty, otherwise ".".
func effectiveDir(dir string) string {
	if dir != "" {
		return dir
	}
	return "."
}
