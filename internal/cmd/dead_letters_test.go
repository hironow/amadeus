package cmd_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/cmd"

	_ "modernc.org/sqlite"
)

// setupDeadLetterDB creates a .gate/.run/outbox.db with the given number of dead-lettered rows.
func setupDeadLetterDB(t *testing.T, dir string, deadCount int) {
	t.Helper()
	runDir := filepath.Join(dir, ".gate", ".run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Also create archive and outbox dirs (NewOutboxStoreForDir expects them)
	for _, sub := range []string{"archive", "outbox"} {
		if err := os.MkdirAll(filepath.Join(dir, ".gate", sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	dbPath := filepath.Join(runDir, "outbox.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA auto_vacuum=INCREMENTAL",
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
	} {
		if _, pErr := db.Exec(pragma); pErr != nil {
			t.Fatal(pErr)
		}
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS staged (
		name        TEXT PRIMARY KEY,
		data        BLOB    NOT NULL,
		flushed     INTEGER NOT NULL DEFAULT 0,
		retry_count INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		t.Fatal(err)
	}

	stmt, err := db.Prepare(`INSERT INTO staged (name, data, flushed, retry_count) VALUES (?, ?, 0, 5)`)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	for i := range deadCount {
		name := fmt.Sprintf("dead-%d", i)
		if _, execErr := stmt.Exec(name, []byte("payload")); execErr != nil {
			t.Fatal(execErr)
		}
	}
}

func TestDeadLettersPurge_DryRun_ShowsCount(t *testing.T) {
	// given
	dir := t.TempDir()
	setupDeadLetterDB(t, dir, 2)
	t.Chdir(dir)

	root := cmd.NewRootCommand()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"dead-letters", "purge"})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := stderr.String()
	if !strings.Contains(output, "2 dead-lettered") {
		t.Errorf("expected count in output, got: %s", output)
	}
	if !strings.Contains(output, "dry-run") {
		t.Errorf("expected dry-run hint, got: %s", output)
	}
}

func TestDeadLettersPurge_Execute_DeletesItems(t *testing.T) {
	// given
	dir := t.TempDir()
	setupDeadLetterDB(t, dir, 3)
	t.Chdir(dir)

	root := cmd.NewRootCommand()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"dead-letters", "purge", "--execute", "--yes"})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := stderr.String()
	if !strings.Contains(output, "Purged 3") {
		t.Errorf("expected 'Purged 3' in output, got: %s", output)
	}
}

func TestDeadLettersPurge_NoDeadLetters(t *testing.T) {
	// given: DB exists but has no dead letters
	dir := t.TempDir()
	setupDeadLetterDB(t, dir, 0)
	t.Chdir(dir)

	root := cmd.NewRootCommand()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"dead-letters", "purge"})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "No dead-lettered items") {
		t.Errorf("expected 'No dead-lettered items', got: %s", stderr.String())
	}
}

func TestDeadLettersPurge_NoDB(t *testing.T) {
	// given: no .gate directory at all
	dir := t.TempDir()
	t.Chdir(dir)

	root := cmd.NewRootCommand()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"dead-letters", "purge"})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "No outbox database found") {
		t.Errorf("expected 'No outbox database found', got: %s", stderr.String())
	}
}

func TestDeadLettersPurge_JSONOutput(t *testing.T) {
	// given
	dir := t.TempDir()
	setupDeadLetterDB(t, dir, 2)
	t.Chdir(dir)

	root := cmd.NewRootCommand()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"dead-letters", "purge", "--execute", "--yes", "-o", "json"})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result struct {
		Count  int `json:"count"`
		Purged int `json:"purged"`
	}
	if jsonErr := json.Unmarshal(stdout.Bytes(), &result); jsonErr != nil {
		t.Fatalf("invalid JSON: %v (output: %s)", jsonErr, stdout.String())
	}
	if result.Count != 2 {
		t.Errorf("count = %d, want 2", result.Count)
	}
	if result.Purged != 2 {
		t.Errorf("purged = %d, want 2", result.Purged)
	}
}

func TestDeadLettersPurge_Execute_NoConfirm_Cancels(t *testing.T) {
	// given
	dir := t.TempDir()
	setupDeadLetterDB(t, dir, 1)
	t.Chdir(dir)

	root := cmd.NewRootCommand()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	// Simulate user typing "n" to the confirmation prompt
	root.SetIn(strings.NewReader("n\n"))
	root.SetArgs([]string{"dead-letters", "purge", "--execute"})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "Cancelled") {
		t.Errorf("expected 'Cancelled', got: %s", stderr.String())
	}
}
