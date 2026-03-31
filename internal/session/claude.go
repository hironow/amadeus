package session

import (
	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// DivergenceMeterAllowedTools is the minimal tool set for divergence evaluation.
// The divergence meter only needs to read pre-collected content from the prompt;
// all filesystem I/O is done by Go before invoking Claude.
var DivergenceMeterAllowedTools = []string{
	"Read",
	"Bash(cat:*)",
}

// DefaultClaudeRunner returns a ClaudeRunner that invokes the given Claude CLI command.
// Both claudeCmd and model are expected to be set by the caller (from config).
func DefaultClaudeRunner(claudeCmd string, model string, logger domain.Logger) port.ClaudeRunner {
	return &ClaudeAdapter{ClaudeCmd: claudeCmd, Model: model, Logger: logger}
}
