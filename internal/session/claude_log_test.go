package session_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/session"
)

func TestWriteClaudeLog_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	events := []string{
		`{"type":"assistant","content":"hello"}`,
		`{"type":"result","result":"done"}`,
	}

	err := session.WriteClaudeLog(dir, events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logDir := filepath.Join(dir, ".gate", ".run", "claude-logs")
	entries, readErr := os.ReadDir(logDir)
	if readErr != nil {
		t.Fatalf("read log dir: %v", readErr)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(entries))
	}
	if !strings.HasSuffix(entries[0].Name(), ".jsonl") {
		t.Errorf("expected .jsonl extension, got %s", entries[0].Name())
	}

	data, _ := os.ReadFile(filepath.Join(logDir, entries[0].Name()))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestWriteClaudeLog_EmptyEvents_NoOp(t *testing.T) {
	dir := t.TempDir()
	err := session.WriteClaudeLog(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logDir := filepath.Join(dir, ".gate", ".run", "claude-logs")
	if _, statErr := os.Stat(logDir); !errors.Is(statErr, fs.ErrNotExist) {
		t.Error("expected no directory for empty events")
	}
}
