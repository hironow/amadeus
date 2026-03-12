package cmd_test

import (
	"bytes"
	"testing"

	"github.com/hironow/amadeus/internal/cmd"
)

func TestLinkCommand_RequiresExactTwoArgs(t *testing.T) {
	rootCmd := cmd.NewRootCommand()
	rootCmd.SetArgs([]string{"link"})
	var errBuf bytes.Buffer
	rootCmd.SetErr(&errBuf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestLinkCommand_TooManyArgs(t *testing.T) {
	rootCmd := cmd.NewRootCommand()
	rootCmd.SetArgs([]string{"link", "a", "b", "c"})
	var errBuf bytes.Buffer
	rootCmd.SetErr(&errBuf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}
