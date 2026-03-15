package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/cmd"
)

func TestMarkCommentedCmd_SubcommandExists(t *testing.T) {
	// given
	root := cmd.NewRootCommand()

	// when
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "mark-commented" {
			found = true
			break
		}
	}

	// then
	if !found {
		t.Fatal("mark-commented subcommand not found")
	}
}

func TestMarkCommentedCmd_RequiresArgs_ZeroArgs(t *testing.T) {
	// given
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"mark-commented"})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for zero args")
	}
}

func TestMarkCommentedCmd_RequiresArgs_OneArg(t *testing.T) {
	// given
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"mark-commented", "dmail-name"})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for one arg (needs at least 2)")
	}
}

func TestMarkCommentedCmd_RejectsTooManyArgs(t *testing.T) {
	// given
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"mark-commented", "a", "b", "c", "d"})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for too many args (max 3)")
	}
}

func TestMarkCommentedCmd_FailsWithoutInit(t *testing.T) {
	// given: a temp dir without .gate/
	dir := t.TempDir()
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"mark-commented", "dmail-name", "PROJ-42", dir})

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

func TestMarkCommentedCmd_JSONFlagExists(t *testing.T) {
	// given
	root := cmd.NewRootCommand()
	markCmd, _, err := root.Find([]string{"mark-commented"})
	if err != nil {
		t.Fatalf("find mark-commented command: %v", err)
	}

	// when
	f := markCmd.Flags().Lookup("json")

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
