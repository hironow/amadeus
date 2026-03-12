package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/cmd"
)

func TestInitCommand_AlreadyInitialized(t *testing.T) {
	// given: .gate/ directory already exists
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	if err := os.MkdirAll(gateDir, 0755); err != nil {
		t.Fatalf("create gate dir: %v", err)
	}

	// amadeus init uses os.Getwd(), so chdir to temp dir
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
	rootCmd.SetArgs([]string{"init"})

	// when
	execErr := rootCmd.Execute()

	// then: should fail with "already exists" or "already initialized"
	if execErr == nil {
		t.Fatal("expected error for already initialized, got nil")
	}
	if got := execErr.Error(); !strings.Contains(got, "already exists") && !strings.Contains(got, "already initialized") {
		t.Errorf("expected 'already exists' or 'already initialized' in error, got: %s", got)
	}
}

func TestInitCommand_AlreadyExists_SuggestsForce(t *testing.T) {
	// given: .gate/ directory already exists
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	if err := os.MkdirAll(gateDir, 0755); err != nil {
		t.Fatalf("create gate dir: %v", err)
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
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"init"})

	// when
	execErr := rootCmd.Execute()

	// then
	if execErr == nil {
		t.Fatal("expected error when .gate already exists")
	}
	if !strings.Contains(execErr.Error(), "--force") {
		t.Errorf("expected '--force' hint in error, got: %v", execErr)
	}
}

func TestInitCommand_Force_OverwritesExisting(t *testing.T) {
	// given: .gate/ directory already exists
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	if err := os.MkdirAll(gateDir, 0755); err != nil {
		t.Fatalf("create gate dir: %v", err)
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
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"init", "--force"})

	// when
	execErr := rootCmd.Execute()

	// then
	if execErr != nil {
		t.Fatalf("init --force failed: %v", execErr)
	}
}
