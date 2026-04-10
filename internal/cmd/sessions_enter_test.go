package cmd

// white-box-reason: cobra command routing: tests sessions enter subcommand end-to-end

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func setupSessionsEnterEnv(t *testing.T, providerSessionID, workDir string) (repoRoot string, recordID string) {
	t.Helper()
	repoRoot = t.TempDir()
	stateDir := filepath.Join(repoRoot, domain.StateDir)
	runDir := filepath.Join(stateDir, ".run")
	os.MkdirAll(runDir, 0755)

	cfgContent := "team: TestTeam\nproject: TestProject\nclaude_cmd: echo\n"
	os.WriteFile(filepath.Join(stateDir, "config.yaml"), []byte(cfgContent), 0644)

	store, err := session.NewSQLiteCodingSessionStore(filepath.Join(runDir, "sessions.db"))
	if err != nil {
		t.Fatalf("create session store: %v", err)
	}
	defer store.Close()

	rec := domain.NewCodingSessionRecord(domain.ProviderClaudeCode, "test-model", workDir)
	rec.ProviderSessionID = providerSessionID
	if err := store.Save(context.Background(), rec); err != nil {
		t.Fatalf("save session record: %v", err)
	}
	return repoRoot, rec.ID
}

func TestSessionsEnter_ByRecordID(t *testing.T) {
	workDir := t.TempDir()
	repoRoot, recordID := setupSessionsEnterEnv(t, "provider-sess-001", workDir)
	configPath := filepath.Join(repoRoot, domain.StateDir, "config.yaml")

	var stdout bytes.Buffer
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--config", configPath, "sessions", "enter", recordID})
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&bytes.Buffer{})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("sessions enter failed: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "--resume provider-sess-001") {
		t.Errorf("expected --resume provider-sess-001, got: %q", output)
	}
}

func TestSessionsEnter_ByProviderID(t *testing.T) {
	workDir := t.TempDir()
	repoRoot, _ := setupSessionsEnterEnv(t, "provider-sess-002", workDir)
	configPath := filepath.Join(repoRoot, domain.StateDir, "config.yaml")

	var stdout bytes.Buffer
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--config", configPath, "sessions", "enter", "--provider-id", "provider-sess-002"})
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&bytes.Buffer{})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("sessions enter --provider-id failed: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "--resume provider-sess-002") {
		t.Errorf("expected --resume provider-sess-002, got: %q", output)
	}
}

func TestSessionsEnter_ByPathFlag(t *testing.T) {
	workDir := t.TempDir()
	repoRoot, recordID := setupSessionsEnterEnv(t, "provider-sess-003", workDir)

	var stdout bytes.Buffer
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"sessions", "enter", "--path", repoRoot, recordID})
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&bytes.Buffer{})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("sessions enter --path failed: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "--resume provider-sess-003") {
		t.Errorf("expected --resume provider-sess-003, got: %q", output)
	}
}

func TestSessionsEnter_NoWorkDir(t *testing.T) {
	repoRoot, recordID := setupSessionsEnterEnv(t, "provider-sess-004", "")

	var stdout bytes.Buffer
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"sessions", "enter", "--path", repoRoot, recordID})
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&bytes.Buffer{})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for session with no work directory")
	}
	if !strings.Contains(err.Error(), "has no work directory recorded") {
		t.Errorf("error = %q, want 'has no work directory recorded'", err)
	}
}
