package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	ToolName   string                    // CLI tool name for stream events (e.g. "amadeus")
	StreamBus  port.SessionStreamPublisher // optional: live session event streaming
}

// Run executes the Claude CLI, returning only the result text.
func (a *ClaudeAdapter) Run(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (string, error) {
	result, err := a.RunDetailed(ctx, prompt, w, opts...)
	return result.Text, err
}

// RunDetailed executes the Claude CLI with the given prompt via stdin and returns
// result text plus provider session ID.
func (a *ClaudeAdapter) RunDetailed(ctx context.Context, prompt string, _ io.Writer, opts ...port.RunOption) (port.RunResult, error) {
	cfg := port.ApplyOptions(opts...)
	claudeCmd := a.ClaudeCmd
	model := a.Model

	args := []string{
		"--model", model,
		"--verbose",
		"--output-format", "stream-json",
		// NOTE: --setting-sources "" skips settings loading but does NOT suppress CLAUDE.md auto-discovery.
		// --bare would suppress it but also disables OAuth. No individual flag exists to disable CLAUDE.md
		// discovery without disabling OAuth. Acceptable tradeoff: CLAUDE.md adds context but doesn't
		// cause context budget issues in practice.
		"--setting-sources", "", // Skip user/project settings (hooks, plugins, auto-memory) while preserving OAuth auth
		"--disable-slash-commands",
		"--dangerously-skip-permissions",
		"--print",
	}

	// Settings and MCP config live under the tool's stateDir (e.g. .gate/).
	// ConfigBase is the repo root where stateDir was initialized.
	// When ConfigBase is unset, fall back to WorkDir, then CWD.
	configBase := cfg.ConfigBase
	if configBase == "" {
		configBase = effectiveDir(cfg.WorkDir)
	}

	// Load tool-specific settings when available; warn if missing
	if settingsPath := ClaudeSettingsPath(configBase); ClaudeSettingsExists(configBase) {
		args = append(args, "--settings", settingsPath)
	} else if a.Logger != nil {
		a.Logger.Warn("Claude subprocess settings not found at %s", settingsPath)
		a.Logger.Warn("Run 'amadeus mcp-config generate' to create settings.")
	}

	// --allowedTools: use caller-specified tools, or default to DivergenceMeterAllowedTools.
	allowedTools := DivergenceMeterAllowedTools
	if len(cfg.AllowedTools) > 0 {
		allowedTools = cfg.AllowedTools
	}
	args = append(args, "--allowedTools", strings.Join(allowedTools, ","))

	if cfg.ResumeSessionID != "" {
		args = append(args, "--resume", cfg.ResumeSessionID)
	} else if cfg.Continue {
		args = append(args, "--continue")
	}

	// Enforce MCP allowlist when .mcp.json (or legacy .run/mcp-config.json) exists
	if mcpPath := ResolveMCPConfigPath(configBase); mcpPath != "" {
		args = append(args, "--strict-mcp-config", "--mcp-config", mcpPath)
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
		return port.RunResult{}, fmt.Errorf("claude: %w\n%s", err, diagnostic)
	}

	// Parse stream-json with span-emitting reader for OTel + Weave integration
	sr := platform.NewStreamReader(bytes.NewReader(stdout.Bytes()))
	emitter := platform.NewSpanEmittingStreamReader(sr, ctx, platform.Tracer)
	emitter.SetInput(prompt)

	var normalizer *platform.StreamNormalizer
	if a.StreamBus != nil && a.ToolName != "" {
		normalizer = platform.NewStreamNormalizer(a.ToolName, domain.ProviderClaudeCode)
		emitter.SetStreamMessageHandler(func(msg *platform.StreamMessage, raw json.RawMessage) {
			if ev := normalizer.Normalize(msg, raw); ev != nil {
				a.StreamBus.Publish(ctx, *ev)
			}
		})
	}

	// Emit session_end on all exit paths.
	var runResultErr error
	if normalizer != nil {
		defer func() {
			endEvent := normalizer.SessionEnd("", runResultErr)
			a.StreamBus.Publish(ctx, endEvent)
		}()
	}

	result, messages, err := emitter.CollectAll()
	if err != nil {
		runResultErr = fmt.Errorf("stream-json parse: %w", err)
		return port.RunResult{}, runResultErr
	}
	if result == nil {
		runResultErr = fmt.Errorf("no result message in stream-json output")
		return port.RunResult{}, runResultErr
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

	return port.RunResult{Text: result.Result, ProviderSessionID: result.SessionID}, nil
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
