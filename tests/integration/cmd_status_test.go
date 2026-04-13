package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/cmd"
	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestStatusCmd_Registered(t *testing.T) {
	// given
	root := cmd.NewRootCommand()

	// when
	statusCmd, _, err := root.Find([]string{"status"})

	// then
	if err != nil {
		t.Fatalf("expected status command to be registered, got: %v", err)
	}
	if statusCmd.Name() != "status" {
		t.Errorf("expected command name 'status', got %q", statusCmd.Name())
	}
}

func TestStatusCmd_FailsWithoutGateDir(t *testing.T) {
	// given: empty directory with no .gate/
	dir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"status"})

	// when
	execErr := rootCmd.Execute()

	// then: should fail with init guidance
	if execErr == nil {
		t.Fatal("expected error for uninitialized state, got nil")
	}
	got := execErr.Error()
	if !strings.Contains(got, "init") {
		t.Errorf("expected error to mention 'init', got: %s", got)
	}
}

func TestStatusCmd_TextOutput(t *testing.T) {
	// given: initialized .gate/ directory
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	if _, err := session.InitGateDir(gateDir, &domain.NopLogger{}, ""); err != nil {
		t.Fatal(err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	rootCmd := cmd.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{"status"})

	// when
	execErr := rootCmd.Execute()

	// then: should succeed
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}

	// Text output goes to stdout (per S0027)
	text := stdout.String()
	if !strings.Contains(text, "amadeus status") {
		t.Errorf("expected stdout to contain 'amadeus status:', got:\n%s", text)
	}
}

func TestStatusCmd_JSONOutput(t *testing.T) {
	// given: initialized .gate/ directory with an event
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	if _, err := session.InitGateDir(gateDir, &domain.NopLogger{}, ""); err != nil {
		t.Fatal(err)
	}

	// Add a check event
	store := session.NewEventStore(gateDir, &domain.NopLogger{})
	now := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	ev, err := domain.NewEvent(domain.EventCheckCompleted, domain.CheckCompletedData{
		Result: domain.CheckResult{
			CheckedAt:  now,
			Commit:     "abc123",
			Type:       domain.CheckTypeDiff,
			Divergence: 0.12,
		},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), ev); err != nil {
		t.Fatal(err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	rootCmd := cmd.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{"status", "-o", "json"})

	// when
	execErr := rootCmd.Execute()

	// then: should succeed with valid JSON on stdout
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}

	var parsed map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, stdout.String())
	}
	if parsed["check_count"] != float64(1) {
		t.Errorf("expected check_count=1, got %v", parsed["check_count"])
	}
	if parsed["divergence"] != 0.12 {
		t.Errorf("expected divergence=0.12, got %v", parsed["divergence"])
	}
}

func TestStatusCmd_WithPath(t *testing.T) {
	// given: initialized .gate/ directory at a specific path
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	if _, err := session.InitGateDir(gateDir, &domain.NopLogger{}, ""); err != nil {
		t.Fatal(err)
	}

	rootCmd := cmd.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{"status", dir})

	// when
	execErr := rootCmd.Execute()

	// then: should succeed
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}
	text := stdout.String()
	if !strings.Contains(text, "amadeus status") {
		t.Errorf("expected stdout to contain 'amadeus status:', got:\n%s", text)
	}
}
