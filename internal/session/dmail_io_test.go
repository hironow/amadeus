package session_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/amadeus/internal/session"
)

const testDMailContent = `---
dmail-schema-version: "1"
name: report-001
kind: report
description: test
---
body
`

func setupInboxArchive(t *testing.T) string {
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

func TestReceiveDMail_ArchivesAndRemoves(t *testing.T) {
	// given
	root := setupInboxArchive(t)
	filename := "report-001.md"
	inboxPath := filepath.Join(root, "inbox", filename)
	if err := os.WriteFile(inboxPath, []byte(testDMailContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	dmail, err := session.ReceiveDMailFromInbox(root, filename)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dmail == nil {
		t.Fatal("expected non-nil DMail")
	}
	if dmail.Name != "report-001" {
		t.Errorf("expected name report-001, got %s", dmail.Name)
	}

	// file should be archived
	archivePath := filepath.Join(root, "archive", filename)
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive file should exist: %v", err)
	}

	// file should be removed from inbox
	if _, err := os.Stat(inboxPath); !errors.Is(err, fs.ErrNotExist) {
		t.Error("inbox file should have been removed")
	}
}

func TestReceiveDMail_SkipsIfAlreadyArchived(t *testing.T) {
	// given
	root := setupInboxArchive(t)
	filename := "report-001.md"
	inboxPath := filepath.Join(root, "inbox", filename)
	archivePath := filepath.Join(root, "archive", filename)
	if err := os.WriteFile(inboxPath, []byte(testDMailContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, []byte(testDMailContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	dmail, err := session.ReceiveDMailFromInbox(root, filename)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dmail != nil {
		t.Error("expected nil DMail for dedup case")
	}

	// inbox file should be cleaned up
	if _, err := os.Stat(inboxPath); !errors.Is(err, fs.ErrNotExist) {
		t.Error("inbox file should have been removed for dedup")
	}
}

func TestReceiveDMail_ParseError(t *testing.T) {
	// given
	root := setupInboxArchive(t)
	filename := "bad-mail.md"
	inboxPath := filepath.Join(root, "inbox", filename)
	if err := os.WriteFile(inboxPath, []byte("not valid dmail"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	dmail, err := session.ReceiveDMailFromInbox(root, filename)

	// then
	if err == nil {
		t.Fatal("expected error for invalid content")
	}
	if dmail != nil {
		t.Error("expected nil DMail on parse error")
	}

	// file should still be in inbox for retry
	if _, err := os.Stat(inboxPath); err != nil {
		t.Error("inbox file should remain for retry")
	}

	// file should NOT be in archive
	archivePath := filepath.Join(root, "archive", filename)
	if _, err := os.Stat(archivePath); !errors.Is(err, fs.ErrNotExist) {
		t.Error("archive file should not exist on parse error")
	}
}
