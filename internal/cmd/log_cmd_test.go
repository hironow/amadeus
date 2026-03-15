package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/cmd"
)

func TestLogCmd_SubcommandExists(t *testing.T) {
	// given
	root := cmd.NewRootCommand()

	// when
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "log" {
			found = true
			break
		}
	}

	// then
	if !found {
		t.Fatal("log subcommand not found")
	}
}

func TestLogCmd_JSONFlagExists(t *testing.T) {
	// given
	root := cmd.NewRootCommand()
	logCmd, _, err := root.Find([]string{"log"})
	if err != nil {
		t.Fatalf("find log command: %v", err)
	}

	// when
	f := logCmd.Flags().Lookup("json")

	// then
	if f == nil {
		t.Fatal("--json flag not found")
	}
	if f.Shorthand != "j" {
		t.Errorf("--json shorthand = %q, want %q", f.Shorthand, "j")
	}
	if f.DefValue != "false" {
		t.Errorf("--json default = %q, want %q", f.DefValue, "false")
	}
}

func TestLogCmd_FailsWithoutInit(t *testing.T) {
	// given: a temp dir without .gate/
	dir := t.TempDir()
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"log", dir})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for uninitialized state")
	}
	if !strings.Contains(err.Error(), "init") {
		t.Errorf("expected error to mention 'init', got: %s", err.Error())
	}
}

func TestLogCmd_RejectsTooManyArgs(t *testing.T) {
	// given
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"log", "arg1", "arg2"})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestLogCmd_AcceptsInitializedDir(t *testing.T) {
	// given: a temp dir with .gate/ and minimal config
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	if err := os.MkdirAll(filepath.Join(gateDir, "events"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gateDir, ".run"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte("lang: en\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"log", dir})

	// when
	err := root.Execute()

	// then: should succeed (empty log is OK)
	if err != nil {
		t.Fatalf("log on initialized dir failed: %v", err)
	}
}
