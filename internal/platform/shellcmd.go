package platform

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

// ParseShellCommand splits a command string that may contain leading
// KEY=VALUE environment variables and tilde (~) paths into components.
//
// Example: "CLAUDE_CONFIG_DIR=~/.claude-work-b ~/.local/bin/claude"
// → env=["CLAUDE_CONFIG_DIR=/Users/x/.claude-work-b"], bin="/Users/x/.local/bin/claude"
func ParseShellCommand(cmdLine string) (env []string, bin string, extraArgs []string) {
	fields := strings.Fields(cmdLine)
	if len(fields) == 0 {
		return nil, cmdLine, nil
	}

	i := 0
	for i < len(fields) {
		if idx := strings.Index(fields[i], "="); idx > 0 && isEnvKey(fields[i][:idx]) {
			env = append(env, ExpandTilde(fields[i]))
			i++
			continue
		}
		break
	}

	if i >= len(fields) {
		return env, "", nil
	}

	bin = ExpandTilde(fields[i])
	if i+1 < len(fields) {
		extraArgs = fields[i+1:]
	}
	return
}

// ExpandTilde expands a leading ~ to the user's home directory.
// Also handles KEY=~/value patterns in environment variable assignments.
func ExpandTilde(s string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return s
	}
	// Handle KEY=~/value
	if idx := strings.Index(s, "=~/"); idx > 0 {
		return s[:idx+1] + filepath.Join(home, s[idx+3:])
	}
	// Handle ~/path
	if strings.HasPrefix(s, "~/") {
		return filepath.Join(home, s[2:])
	}
	return s
}

// NewShellCmd creates an exec.Cmd from a command string that may contain
// leading KEY=VALUE environment variables and tilde paths.
// Additional args are appended after any args parsed from cmdLine.
func NewShellCmd(ctx context.Context, cmdLine string, args ...string) *exec.Cmd {
	env, bin, cmdArgs := ParseShellCommand(cmdLine)
	allArgs := make([]string, 0, len(cmdArgs)+len(args))
	allArgs = append(allArgs, cmdArgs...)
	allArgs = append(allArgs, args...)
	cmd := exec.CommandContext(ctx, bin, allArgs...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd
}

// LookPathShell extracts the binary from a shell-like command string
// (ignoring leading KEY=VALUE env vars) and runs exec.LookPath on it.
func LookPathShell(cmdLine string) (string, error) {
	_, bin, _ := ParseShellCommand(cmdLine)
	return exec.LookPath(bin)
}

// isEnvKey checks if s is a valid environment variable name ([A-Za-z_][A-Za-z0-9_]*).
func isEnvKey(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if i == 0 {
			if !unicode.IsLetter(c) && c != '_' {
				return false
			}
		} else {
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' {
				return false
			}
		}
	}
	return true
}
