package session_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const maxSessionFileLines = 700

func TestSessionFileSize_NoGodModule(t *testing.T) {
	// given — all non-test .go files in internal/session/
	sessionDir := filepath.Join(".")
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		t.Fatalf("failed to read session directory: %v", err)
	}

	// when/then — each file must be under the threshold
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(sessionDir, name))
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		lines := bytes.Count(data, []byte("\n"))
		if lines > maxSessionFileLines {
			t.Errorf("session/%s has %d lines (max %d) — consider splitting responsibilities", name, lines, maxSessionFileLines)
		}
	}
}
