package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/cmd"
	"github.com/spf13/cobra"
)

func TestRunCmd_FlagsExist(t *testing.T) {
	// given
	root := cmd.NewRootCommand()
	var runCmd *cobra.Command
	for _, sub := range root.Commands() {
		if sub.Name() == "run" {
			runCmd = sub
			break
		}
	}
	if runCmd == nil {
		t.Fatal("run subcommand not found")
	}

	// then: verify all flags and their defaults
	flags := []struct {
		name     string
		defValue string
	}{
		{"idle-timeout", "30m0s"},
		{"auto-approve", "false"},
		{"approve-cmd", ""},
		{"notify-cmd", ""},
		{"review-cmd", ""},
		{"collector-enable", "false"},
		{"collector-disable", "false"},
		{"collector-project-id", ""},
		{"collector-api-url", ""},
		{"collector-query-limit", "0"},
		{"dry-run", "false"},
		{"full", "false"},
		{"quiet", "false"},
		{"json", "false"},
		{"base", ""},
	}
	for _, tc := range flags {
		f := runCmd.Flags().Lookup(tc.name)
		if f == nil {
			t.Errorf("--%s flag not found", tc.name)
			continue
		}
		if f.DefValue != tc.defValue {
			t.Errorf("--%s default = %q, want %q", tc.name, f.DefValue, tc.defValue)
		}
	}
}

func TestRunCmd_ShortAliases(t *testing.T) {
	// given
	root := cmd.NewRootCommand()
	var runCmd *cobra.Command
	for _, sub := range root.Commands() {
		if sub.Name() == "run" {
			runCmd = sub
			break
		}
	}
	if runCmd == nil {
		t.Fatal("run subcommand not found")
	}

	// then: verify short aliases
	aliases := []struct {
		flagName  string
		shorthand string
	}{
		{"dry-run", "n"},
		{"full", "f"},
		{"quiet", "q"},
		{"json", "j"},
	}
	for _, tc := range aliases {
		f := runCmd.Flags().Lookup(tc.flagName)
		if f == nil {
			t.Errorf("--%s flag not found", tc.flagName)
			continue
		}
		if f.Shorthand != tc.shorthand {
			t.Errorf("--%s shorthand = %q, want %q", tc.flagName, f.Shorthand, tc.shorthand)
		}
	}
}

func TestRunCmd_FailsWithoutInit(t *testing.T) {
	// given: a temp dir without .gate/
	dir := t.TempDir()
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"run", dir})

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
