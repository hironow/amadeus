package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/cmd"
)

func TestStatusCmd_SubcommandExists(t *testing.T) {
	// given
	root := cmd.NewRootCommand()

	// then
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "status" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("status subcommand not found")
	}
}

func TestStatusCmd_FailsWithoutInit(t *testing.T) {
	// given: a temp dir without .gate/
	dir := t.TempDir()
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"status", dir})

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
