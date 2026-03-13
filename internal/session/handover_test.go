package session_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestFileHandoverWriter_WritesMarkdown(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".gate")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}

	w := &session.FileHandoverWriter{}
	state := domain.HandoverState{
		Tool:       "amadeus",
		Operation:  "divergence",
		Timestamp:  time.Date(2026, 3, 14, 15, 30, 45, 0, time.UTC),
		InProgress: "Evaluating D-Mail divergence",
		Completed:  []string{"Divergence #1: Score 0.42"},
		Remaining:  []string{"Divergence #3: Not started"},
		PartialState: map[string]string{
			"Score": "0.42",
		},
	}

	err := w.WriteHandover(context.Background(), stateDir, state)
	if err != nil {
		t.Fatalf("WriteHandover: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(stateDir, "handover.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	for _, want := range []string{
		"# Handover",
		"INTERRUPTED",
		"Evaluating D-Mail divergence",
		"Divergence #1: Score 0.42",
		"Divergence #3: Not started",
		"Score",
		"0.42",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("handover.md missing %q", want)
		}
	}
}

func TestFileHandoverWriter_OverwritesPrevious(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".gate")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}

	w := &session.FileHandoverWriter{}
	first := domain.HandoverState{
		Tool: "amadeus", Operation: "divergence",
		Timestamp: time.Now(), InProgress: "first",
	}
	second := domain.HandoverState{
		Tool: "amadeus", Operation: "divergence",
		Timestamp: time.Now(), InProgress: "second",
	}

	if err := w.WriteHandover(context.Background(), stateDir, first); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteHandover(context.Background(), stateDir, second); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(stateDir, "handover.md"))
	content := string(data)
	if strings.Contains(content, "first") {
		t.Error("expected previous handover to be overwritten")
	}
	if !strings.Contains(content, "second") {
		t.Error("expected new handover content")
	}
}

func TestFileHandoverWriter_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dir := t.TempDir()
	w := &session.FileHandoverWriter{}
	state := domain.HandoverState{
		Tool: "amadeus", Operation: "divergence", Timestamp: time.Now(),
	}

	err := w.WriteHandover(ctx, dir, state)
	if err == nil {
		t.Error("expected error when context is cancelled")
	}
}
