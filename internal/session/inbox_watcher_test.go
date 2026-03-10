package session_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

const validDMailContent = `---
dmail-schema-version: "1"
name: report-001
kind: report
description: test report
---
body
`

func setupInboxDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestMonitorInbox_InitialDrain(t *testing.T) {
	// given
	root := setupInboxDir(t)
	if err := os.WriteFile(filepath.Join(root, "inbox", "report-001.md"), []byte(validDMailContent), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// when
	ch, err := session.MonitorInbox(ctx, root, &domain.NopLogger{})
	if err != nil {
		t.Fatalf("MonitorInbox returned error: %v", err)
	}

	// then — initial file should be delivered on channel
	select {
	case dmail, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before delivering initial D-Mail")
		}
		if dmail.Name != "report-001" {
			t.Errorf("expected name report-001, got %s", dmail.Name)
		}
		if dmail.Kind != domain.KindReport {
			t.Errorf("expected kind report, got %s", dmail.Kind)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial D-Mail")
	}

	// Verify file was archived and removed from inbox
	if _, err := os.Stat(filepath.Join(root, "archive", "report-001.md")); errors.Is(err, fs.ErrNotExist) {
		t.Error("expected file to be archived")
	}
	if _, err := os.Stat(filepath.Join(root, "inbox", "report-001.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("expected file to be removed from inbox")
	}
}

func TestMonitorInbox_WatchesNewFiles(t *testing.T) {
	// given
	root := setupInboxDir(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := session.MonitorInbox(ctx, root, &domain.NopLogger{})
	if err != nil {
		t.Fatalf("MonitorInbox returned error: %v", err)
	}

	// Let watcher settle before writing
	time.Sleep(100 * time.Millisecond)

	// when — write a new file to inbox
	if err := os.WriteFile(filepath.Join(root, "inbox", "report-001.md"), []byte(validDMailContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// then — new file should be delivered via fsnotify
	select {
	case dmail, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before delivering watched D-Mail")
		}
		if dmail.Name != "report-001" {
			t.Errorf("expected name report-001, got %s", dmail.Name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for fsnotify D-Mail")
	}

	// Verify archived
	if _, err := os.Stat(filepath.Join(root, "archive", "report-001.md")); errors.Is(err, fs.ErrNotExist) {
		t.Error("expected file to be archived")
	}
}

func TestMonitorInbox_ContextCancellation(t *testing.T) {
	// given
	root := setupInboxDir(t)

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := session.MonitorInbox(ctx, root, &domain.NopLogger{})
	if err != nil {
		t.Fatalf("MonitorInbox returned error: %v", err)
	}

	// when
	cancel()

	// then — channel should eventually close
	select {
	case _, ok := <-ch:
		if ok {
			// Received a value, that's fine — keep draining until closed
			for range ch {
			}
		}
		// Channel closed, test passes
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for channel to close after context cancellation")
	}
}
