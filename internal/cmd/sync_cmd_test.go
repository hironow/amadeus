package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/cmd"
)

func TestSyncCmd_SubcommandExists(t *testing.T) {
	// given
	root := cmd.NewRootCommand()

	// then
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "sync" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("sync subcommand not found")
	}
}

func TestSyncCmd_FailsWithoutInit(t *testing.T) {
	// given: a temp dir without .gate/
	dir := t.TempDir()
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"sync", dir})

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
