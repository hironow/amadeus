package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/cmd"
	"github.com/hironow/amadeus/internal/domain"
	"github.com/spf13/cobra"
)

func TestInitCommand_AlreadyInitialized(t *testing.T) {
	// given: .gate/ directory already exists
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	if err := os.MkdirAll(gateDir, 0755); err != nil {
		t.Fatalf("create gate dir: %v", err)
	}

	// amadeus init uses os.Getwd(), so chdir to temp dir
	t.Chdir(dir)

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

	t.Chdir(dir)

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

	t.Chdir(dir)

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

func TestInitCmd_OtelBackend_CreatesOtelEnv(t *testing.T) {
	// given
	dir := t.TempDir()
	t.Chdir(dir)

	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"init", "--otel-backend", "jaeger"})

	// when
	execErr := rootCmd.Execute()

	// then
	if execErr != nil {
		t.Fatalf("init --otel-backend jaeger failed: %v", execErr)
	}
	otelPath := filepath.Join(dir, ".gate", ".otel.env")
	data, readErr := os.ReadFile(otelPath)
	if readErr != nil {
		t.Fatalf(".otel.env not created: %v", readErr)
	}
	if !strings.Contains(string(data), "OTEL_EXPORTER_OTLP_ENDPOINT") {
		t.Errorf("expected OTEL_EXPORTER_OTLP_ENDPOINT in .otel.env, got:\n%s", data)
	}
}

func TestInitCmd_Snapshot(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"init"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	stateDir := filepath.Join(dir, domain.StateDir)
	got := walkStateDir(t, stateDir)

	want := []string{
		".gitignore",
		".run/",
		"archive/",
		"config.yaml",
		"events/",
		"inbox/",
		"insights/",
		"outbox/",
		"skills/",
		"skills/dmail-readable/",
		"skills/dmail-readable/SKILL.md",
		"skills/dmail-sendable/",
		"skills/dmail-sendable/SKILL.md",
	}

	if !slices.Equal(want, got) {
		t.Errorf("init snapshot mismatch\nwant: %v\ngot:  %v", want, got)
	}
}

func TestInitCmd_ConfigHeader(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"init"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, domain.StateDir, "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.HasPrefix(string(data), "# amadeus configuration") {
		t.Errorf("expected config header comment, got:\n%s", string(data)[:min(len(data), 80)])
	}
}

func TestInitCmd_GitignoreComplete(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"init"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, domain.StateDir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	content := string(data)
	for _, entry := range []string{".run/", "outbox/", "inbox/", "archive/", "insights/", ".otel.env", "events/", ".mcp.json", ".claude/"} {
		if !strings.Contains(content, entry) {
			t.Errorf("expected %q in .gitignore, got:\n%s", entry, content)
		}
	}
}

func walkStateDir(t *testing.T, stateDir string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(stateDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(stateDir, path)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			rel += "/"
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk state dir: %v", err)
	}
	sort.Strings(paths)
	return paths
}

func TestInitCommand_OtelFlags_Exist(t *testing.T) {
	// given
	rootCmd := cmd.NewRootCommand()

	// when — find init subcommand
	var initCmd *cobra.Command
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "init" {
			initCmd = sub
			break
		}
	}
	if initCmd == nil {
		t.Fatal("init subcommand not found")
	}

	// then — otel flags exist
	for _, flag := range []string{"otel-backend", "otel-entity", "otel-project"} {
		if initCmd.Flags().Lookup(flag) == nil {
			t.Errorf("init flag --%s not found", flag)
		}
	}
}
