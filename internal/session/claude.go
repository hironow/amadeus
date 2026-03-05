package session

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/hironow/amadeus/internal/usecase/port"
)

// DivergenceMeterAllowedTools is the minimal tool set for divergence evaluation.
// The divergence meter only needs to read pre-collected content from the prompt;
// all filesystem I/O is done by Go before invoking Claude.
var DivergenceMeterAllowedTools = []string{
	"Read",
	"Bash(cat:*)",
}

// defaultClaudeRunner executes the real Claude CLI as a subprocess.
type defaultClaudeRunner struct{}

// Run executes the Claude CLI with the given prompt via stdin and returns raw output.
// Uses --dangerously-skip-permissions because amadeus runs non-interactively with --print.
func (d *defaultClaudeRunner) Run(ctx context.Context, prompt string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "claude",
		"--model", "opus",
		"--output-format", "json",
		"--allowedTools", strings.Join(DivergenceMeterAllowedTools, ","),
		"--dangerously-skip-permissions",
		"--print",
	)
	cmd.Stdin = bytes.NewBufferString(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude: %w\n%s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// DefaultClaudeRunner returns the default ClaudeRunner that invokes the real Claude CLI.
func DefaultClaudeRunner() port.ClaudeRunner {
	return &defaultClaudeRunner{}
}
