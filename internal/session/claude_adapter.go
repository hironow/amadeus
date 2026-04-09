package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/usecase/port"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ClaudeAdapter implements port.ClaudeRunner by executing the Claude CLI
// as a subprocess with streaming (--output-format stream-json).
// It does NOT retry; wrap with RetryRunner for that.
type ClaudeAdapter struct {
	ClaudeCmd      string
	Model          string
	TimeoutSec     int
	Logger         domain.Logger
	ToolName       string                       // CLI tool name for stream events (e.g. "sightjack")
	StreamBus      port.SessionStreamPublisher   // optional: live session event streaming
	// NewCmd overrides command creation. If nil, platform.NewShellCmd is used.
	NewCmd     func(ctx context.Context, name string, args ...string) *exec.Cmd
	// CancelFunc sets cmd.Cancel for graceful shutdown. If nil, default (process kill) is used.
	CancelFunc func(cmd *exec.Cmd) func() error
}

// newCmd returns the command constructor, defaulting to platform.NewShellCmd.
func (a *ClaudeAdapter) newCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	if a.NewCmd != nil {
		return a.NewCmd(ctx, name, args...)
	}
	return platform.NewShellCmd(ctx, name, args...)
}

// Run executes the Claude CLI once with streaming, returning only the result text.
func (a *ClaudeAdapter) Run(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (string, error) {
	result, err := a.RunDetailed(ctx, prompt, w, opts...)
	return result.Text, err
}

// RunDetailed executes the Claude CLI once with streaming, returning the result
// text and provider session ID.
func (a *ClaudeAdapter) RunDetailed(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (port.RunResult, error) {
	rc := port.ApplyOptions(opts...)

	model := a.Model
	if rc.Model != "" {
		model = rc.Model
	}

	_, span := platform.Tracer.Start(ctx, "claude.invoke",
		trace.WithAttributes(
			append([]attribute.KeyValue{
				attribute.String("claude.model", platform.SanitizeUTF8(model)),
				attribute.Int("claude.timeout_sec", a.TimeoutSec),
			}, platform.GenAISpanAttrs(model)...)...,
		),
	)
	defer span.End()

	// Apply per-call timeout only when the caller has not already set a deadline.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && a.TimeoutSec > 0 {
		timeout := time.Duration(a.TimeoutSec) * time.Second
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var args []string
	if model != "" {
		args = append(args, "--model", model)
	}
	if rc.ResumeSessionID != "" {
		args = append(args, "--resume", rc.ResumeSessionID)
	} else if rc.Continue {
		args = append(args, "--continue")
	}
	args = append(args, "--verbose", "--output-format", "stream-json")
	// NOTE: --setting-sources "" skips settings loading but does NOT suppress CLAUDE.md auto-discovery.
	// --bare would suppress it but also disables OAuth. No individual flag exists to disable CLAUDE.md
	// discovery without disabling OAuth. Acceptable tradeoff: CLAUDE.md adds context but doesn't
	// cause context budget issues in practice.
	args = append(args, "--setting-sources", "") // Skip user/project settings (hooks, plugins, auto-memory) while preserving OAuth auth
	args = append(args, "--disable-slash-commands")
	if len(rc.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(rc.AllowedTools, ","))
	}

	// Settings and MCP config live under the tool's stateDir.
	// ConfigBase is the repo root (continent) where stateDir was initialized.
	// When ConfigBase is unset, fall back to WorkDir, then CWD.
	configBase := rc.ConfigBase
	if configBase == "" {
		configBase = effectiveDir(rc.WorkDir)
	}

	// Load tool-specific settings when available; warn if missing
	if settingsPath := ClaudeSettingsPath(configBase); ClaudeSettingsExists(configBase) {
		args = append(args, "--settings", settingsPath)
	} else if a.Logger != nil {
		a.Logger.Warn("Claude subprocess settings not found at %s", settingsPath)
		a.Logger.Warn("Run 'mcp-config generate' to create settings.")
	}

	// Enforce MCP allowlist when .mcp.json (or legacy .run/mcp-config.json) exists
	if mcpPath := ResolveMCPConfigPath(configBase); mcpPath != "" {
		args = append(args, "--strict-mcp-config", "--mcp-config", mcpPath)
	}
	args = append(args, "--dangerously-skip-permissions", "--print")

	cmd := a.newCmd(ctx, a.ClaudeCmd, args...)
	if a.CancelFunc != nil {
		cmd.Cancel = a.CancelFunc(cmd)
	}
	cmd.WaitDelay = 3 * time.Second
	if rc.WorkDir != "" {
		cmd.Dir = rc.WorkDir
	}

	// Pass prompt via stdin to avoid E2BIG for large prompts.
	cmd.Stdin = strings.NewReader(prompt)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return port.RunResult{}, fmt.Errorf("stdout pipe: %w", err)
	}
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return port.RunResult{}, fmt.Errorf("claude start: %w", err)
	}

	var output strings.Builder
	var responseModel, responseID string
	var providerSessionID string
	var runResultErr error
	streamErr := make(chan error, 1)
	done := make(chan struct{})

	// Create normalizer at RunDetailed scope so defer can emit session_end.
	var normalizer *platform.StreamNormalizer
	if a.StreamBus != nil && a.ToolName != "" {
		normalizer = platform.NewStreamNormalizer(a.ToolName, domain.ProviderClaudeCode)
		normalizer.SetCodingSessionID(rc.CodingSessionID)
		defer func() {
			endEvent := normalizer.SessionEnd(providerSessionID, runResultErr)
			// Use Background: ctx may be cancelled, but session_end must still publish.
			if vErr := domain.ValidateSessionStreamEvent(endEvent); vErr != nil {
				if a.Logger != nil {
					a.Logger.Warn("session_end dropped (invalid): %v", vErr)
				}
			} else {
				// Use Background: caller ctx may be cancelled, but session_end must still publish.
				a.StreamBus.Publish(context.Background(), endEvent)
			}
		}()
	}

	go func() {
		defer close(done)
		sr := platform.NewStreamReader(stdout)
		if a.Logger != nil {
			sr.SetLogger(a.Logger)
		}
		emitter := platform.NewSpanEmittingStreamReader(sr, ctx, platform.Tracer)
		emitter.SetInput(prompt)

		// Wire live stream event bus when available.
		if normalizer != nil {
			emitter.SetStreamMessageHandler(func(msg *platform.StreamMessage, raw json.RawMessage) {
				if ev := normalizer.Normalize(msg, raw); ev != nil {
					if vErr := domain.ValidateSessionStreamEvent(*ev); vErr != nil {
						if a.Logger != nil {
							a.Logger.Warn("stream event dropped (invalid): %v", vErr)
						}
						return
					}
					a.StreamBus.Publish(ctx, *ev)
				}
			})
		}

		result, messages, readErr := emitter.CollectAll()
		if readErr != nil {
			streamErr <- readErr
			return
		}
		if result == nil {
			streamErr <- fmt.Errorf("no result message in stream-json output")
			return
		}

		for _, msg := range messages {
			switch msg.Type {
			case "assistant":
				text, _ := msg.ExtractText()
				if text != "" {
					if w != nil {
						_, _ = w.Write([]byte(text))
					}
					output.WriteString(text)
				}
				tools, _ := msg.ExtractToolUse()
				for _, t := range tools {
					if a.Logger != nil {
						a.Logger.Info("  tool: %s", t.Name)
					}
				}
				if am, _ := msg.ParseAssistantMessage(); am != nil {
					if am.Model != "" {
						responseModel = am.Model
					}
					if am.ID != "" {
						responseID = am.ID
					}
				}
			case "result":
				output.Reset()
				output.WriteString(msg.Result)
				span.SetAttributes(platform.GenAIResultAttrs(msg, responseModel, responseID)...)
			}
		}

		if rawEvents := emitter.RawEvents(); len(rawEvents) > 0 {
			span.SetAttributes(attribute.StringSlice("stream.raw_events", platform.SanitizeUTF8Slice(rawEvents)))
		}
		if result != nil && result.SessionID != "" {
			providerSessionID = result.SessionID
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

		// Context budget measurement
		budget := platform.CalculateContextBudget(messages)
		span.SetAttributes(budget.Attrs()...)
		if warning := budget.WarningMessage(platform.DefaultContextBudgetThreshold); warning != "" {
			if a.Logger != nil {
				a.Logger.Warn("%s", warning)
			}
		}

		// Persist raw events to .run/claude-logs/
		if raw := emitter.RawEvents(); len(raw) > 0 {
			if logErr := WriteClaudeLog(effectiveDir(rc.WorkDir), raw); logErr != nil && a.Logger != nil {
				a.Logger.Warn("claude-log write: %v", logErr)
			}
		}
	}()

	<-done

	var readError error
	select {
	case sErr := <-streamErr:
		readError = sErr
	default:
	}

	// Log captured stderr at debug level; suppress raw NDJSON from errors.
	stderr := stderrBuf.String()
	if stderr != "" && a.Logger != nil {
		a.Logger.Debug("claude stderr:\n%s", stderr)
	}

	makeResult := func() port.RunResult {
		return port.RunResult{Text: output.String(), ProviderSessionID: providerSessionID, Stderr: stderr}
	}

	if waitErr := cmd.Wait(); waitErr != nil {
		span.RecordError(waitErr)
		diagnostic := stderr
		if diagnostic != "" {
			if platform.IsNDJSON(diagnostic) {
				diagnostic = platform.SummarizeNDJSON(diagnostic)
			}
			runResultErr = fmt.Errorf("claude exit: %w\n%s", waitErr, diagnostic)
			return makeResult(), runResultErr
		}
		runResultErr = fmt.Errorf("claude exit: %w", waitErr)
		return makeResult(), runResultErr
	}

	if readError != nil {
		runResultErr = fmt.Errorf("stream read: %w", readError)
		return makeResult(), runResultErr
	}

	return makeResult(), nil
}

// effectiveDir returns dir if non-empty, otherwise ".".
func effectiveDir(dir string) string {
	if dir != "" {
		return dir
	}
	return "."
}
