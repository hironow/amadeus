package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	hookMarkerBegin = "# >>> amadeus hook — do not edit this section"
	hookMarkerEnd   = "# <<< amadeus hook"
	hookScript      = `amadeus check --quiet 2>/dev/null || true`
)

// InstallHook writes the amadeus post-merge hook into the given git directory.
// If a post-merge hook already exists, the amadeus section is appended.
// If the amadeus section already exists, an error is returned.
func InstallHook(gitDir string) error {
	hookPath := filepath.Join(gitDir, "hooks", "post-merge")

	existing, err := os.ReadFile(hookPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read hook: %w", err)
	}

	content := string(existing)
	if strings.Contains(content, hookMarkerBegin) {
		return fmt.Errorf("amadeus hook already installed in %s", hookPath)
	}

	section := fmt.Sprintf("\n%s\n%s\n%s\n", hookMarkerBegin, hookScript, hookMarkerEnd)

	if len(existing) == 0 {
		// New hook file: add shebang
		section = "#!/bin/sh\n" + section
	} else {
		// Append to existing hook
		if !strings.HasSuffix(content, "\n") {
			section = "\n" + section
		}
	}

	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	if len(existing) == 0 {
		return os.WriteFile(hookPath, []byte(section), 0o755)
	}
	return os.WriteFile(hookPath, []byte(content+section), 0o755)
}

// UninstallHook removes the amadeus section from the post-merge hook.
// If the hook contains only the amadeus section, the file is removed entirely.
// If no amadeus section is found, an error is returned.
func UninstallHook(gitDir string) error {
	hookPath := filepath.Join(gitDir, "hooks", "post-merge")

	data, err := os.ReadFile(hookPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("no post-merge hook found")
		}
		return fmt.Errorf("read hook: %w", err)
	}

	content := string(data)
	beginIdx := strings.Index(content, hookMarkerBegin)
	if beginIdx < 0 {
		return fmt.Errorf("amadeus hook not found in %s", hookPath)
	}

	endRelIdx := strings.Index(content[beginIdx:], hookMarkerEnd)
	if endRelIdx < 0 {
		return fmt.Errorf("malformed amadeus hook section (missing end marker)")
	}
	endIdx := beginIdx + endRelIdx + len(hookMarkerEnd)
	// Consume trailing newline
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	// Remove leading newline before the section if present
	if beginIdx > 0 && content[beginIdx-1] == '\n' {
		beginIdx--
	}

	remaining := content[:beginIdx] + content[endIdx:]
	remaining = strings.TrimRight(remaining, "\n")

	// If only shebang (or nothing) remains, remove the file
	trimmed := strings.TrimSpace(remaining)
	if trimmed == "" || trimmed == "#!/bin/sh" || trimmed == "#!/usr/bin/env sh" || trimmed == "#!/usr/bin/env bash" || trimmed == "#!/bin/bash" {
		return os.Remove(hookPath)
	}

	return os.WriteFile(hookPath, []byte(remaining+"\n"), 0o755)
}
