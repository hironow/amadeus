package integration_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

// dmailContent returns a valid D-Mail frontmatter + body for testing.
func dmailContent(name string) []byte {
	return []byte(fmt.Sprintf(`---
dmail-schema-version: "1"
name: %s
kind: report
description: test
---
body
`, name))
}

func setupInboxArchiveDirs(t *testing.T) string {
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

func TestMonitorInbox_Integration_MultipleFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// given: root with inbox/ and archive/, 2 pre-existing D-Mail files
	root := setupInboxArchiveDirs(t)

	if err := os.WriteFile(filepath.Join(root, "inbox", "report-a.md"), dmailContent("report-a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inbox", "report-b.md"), dmailContent("report-b"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// when: start MonitorInbox
	ch, err := session.MonitorInbox(ctx, root, &domain.NopLogger{})
	if err != nil {
		t.Fatalf("MonitorInbox returned error: %v", err)
	}

	// then: read 2 initial items from channel
	received := make(map[string]bool)
	for i := 0; i < 2; i++ {
		select {
		case dmail, ok := <-ch:
			if !ok {
				t.Fatal("channel closed before delivering all initial D-Mails")
			}
			received[dmail.Name] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for initial D-Mail #%d", i+1)
		}
	}

	if !received["report-a"] {
		t.Error("expected report-a to be delivered")
	}
	if !received["report-b"] {
		t.Error("expected report-b to be delivered")
	}

	// when: wait for watcher to settle, then write a 3rd file
	time.Sleep(100 * time.Millisecond)

	if err := os.WriteFile(filepath.Join(root, "inbox", "report-c.md"), dmailContent("report-c"), 0o644); err != nil {
		t.Fatal(err)
	}

	// then: read 3rd item from channel
	select {
	case dmail, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before delivering watched D-Mail")
		}
		if dmail.Name != "report-c" {
			t.Errorf("expected name report-c, got %s", dmail.Name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for fsnotify D-Mail report-c")
	}

	// then: verify all 3 files in archive/
	archiveEntries, err := os.ReadDir(filepath.Join(root, "archive"))
	if err != nil {
		t.Fatalf("ReadDir archive: %v", err)
	}
	archived := make(map[string]bool)
	for _, e := range archiveEntries {
		archived[e.Name()] = true
	}
	for _, name := range []string{"report-a.md", "report-b.md", "report-c.md"} {
		if !archived[name] {
			t.Errorf("expected %s to be in archive/", name)
		}
	}

	// then: verify inbox/ is empty
	inboxEntries, err := os.ReadDir(filepath.Join(root, "inbox"))
	if err != nil {
		t.Fatalf("ReadDir inbox: %v", err)
	}
	if len(inboxEntries) != 0 {
		names := make([]string, 0, len(inboxEntries))
		for _, e := range inboxEntries {
			names = append(names, e.Name())
		}
		t.Errorf("expected inbox/ to be empty, got %v", names)
	}
}

func TestMonitorInbox_Integration_DedupOnRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// given: root with inbox/ and archive/, same file in both (simulating restart)
	root := setupInboxArchiveDirs(t)

	content := dmailContent("report-dup")
	if err := os.WriteFile(filepath.Join(root, "inbox", "report-dup.md"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "archive", "report-dup.md"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// when: start MonitorInbox
	ch, err := session.MonitorInbox(ctx, root, &domain.NopLogger{})
	if err != nil {
		t.Fatalf("MonitorInbox returned error: %v", err)
	}

	// then: no dmail should be delivered (dedup skips already-archived files)
	select {
	case dmail, ok := <-ch:
		if ok {
			t.Errorf("expected no D-Mail delivery (dedup), but got: %s", dmail.Name)
		}
	case <-time.After(500 * time.Millisecond):
		// expected: no delivery within timeout
	}

	// then: verify inbox file was cleaned up
	if _, err := os.Stat(filepath.Join(root, "inbox", "report-dup.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("expected report-dup.md to be removed from inbox/ after dedup cleanup")
	}

	// then: archive still has the file
	if _, err := os.Stat(filepath.Join(root, "archive", "report-dup.md")); errors.Is(err, fs.ErrNotExist) {
		t.Error("expected report-dup.md to remain in archive/")
	}
}
