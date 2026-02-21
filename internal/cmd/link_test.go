package cmd

import (
	"bytes"
	"testing"
)

func TestLinkCommand_RequiresExactTwoArgs(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"link"})
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestLinkCommand_TooManyArgs(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"link", "a", "b", "c"})
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}
