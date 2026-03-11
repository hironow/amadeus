package session

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

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

// defaultClaudeRunner executes the real Claude CLI as a subprocess.
type defaultClaudeRunner struct {
	cmd   string // Claude CLI command name (from config)
	model string // Claude model name (from config)
}

// Run executes the Claude CLI with the given prompt via stdin and returns raw output.
// Uses --dangerously-skip-permissions because amadeus runs non-interactively with --print.
// The w and opts parameters are accepted for interface compatibility but unused by amadeus.
func (d *defaultClaudeRunner) Run(ctx context.Context, prompt string, _ io.Writer, _ ...port.RunOption) (string, error) {
	claudeCmd := d.cmd
	model := d.model
	cmd := platform.NewShellCmd(ctx, claudeCmd,
		"--model", model,
		"--verbose",
		"--output-format", "stream-json",
		"--allowedTools", strings.Join(DivergenceMeterAllowedTools, ","),
		"--dangerously-skip-permissions",
		"--print",
	)
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

	return result.Result, nil
}

// DefaultClaudeRunner returns a ClaudeRunner that invokes the given Claude CLI command.
// Both claudeCmd and model are expected to be set by the caller (from config).
func DefaultClaudeRunner(claudeCmd string, model string) port.ClaudeRunner {
	return &defaultClaudeRunner{cmd: claudeCmd, model: model}
}
