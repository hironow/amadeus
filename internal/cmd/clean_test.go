package cmd_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/cmd"
)

func TestCleanCmd_NothingToClean(t *testing.T) {
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
	rootCmd.SetArgs([]string{"clean", "--yes"})

	// when
	execErr := rootCmd.Execute()

	// then: should succeed with "nothing to clean" message
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}
	if got := buf.String(); !strings.Contains(got, "Nothing to clean") {
		t.Errorf("expected 'Nothing to clean' in output, got: %s", got)
	}
}

func TestCleanCmd_DeletesGateDir(t *testing.T) {
	// given: .gate/ directory exists
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	if err := os.MkdirAll(gateDir, 0755); err != nil {
		t.Fatalf("create gate dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte("{}"), 0644); err != nil {
		t.Fatalf("create config: %v", err)
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
	rootCmd.SetArgs([]string{"clean", "--yes"})

	// when
	execErr := rootCmd.Execute()

	// then: should succeed and delete .gate/
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}
	if _, err := os.Stat(gateDir); !errors.Is(err, fs.ErrNotExist) {
		t.Error("expected .gate/ dir to be deleted")
	}
}
