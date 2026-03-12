package main

import (
	"bytes"
	"strings"
	"testing"

	cmd "github.com/hironow/amadeus/internal/cmd"
	"github.com/spf13/cobra"
)

func TestRootCommand_Help(t *testing.T) {
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("expected help output, got empty string")
	}
}

func TestRootCommand_UnknownSubcommand(t *testing.T) {
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"nonexistent"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

func TestSubcommands_Exist(t *testing.T) {
	rootCmd := cmd.NewRootCommand()

	expected := []string{"init", "doctor", "run", "status", "clean", "archive-prune", "version", "update"}
	for _, name := range expected {
		found := false
		for _, c := range rootCmd.Commands() {
			if c.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q not found", name)
		}
	}
}

func TestRootCommand_PersistentFlags(t *testing.T) {
	rootCmd := cmd.NewRootCommand()

	flags := []struct {
		long  string
		short string
	}{
		{"verbose", "v"},
		{"config", "c"},
	}
	for _, f := range flags {
		flag := rootCmd.PersistentFlags().Lookup(f.long)
		if flag == nil {
			t.Errorf("root command missing persistent flag %q", f.long)
			continue
		}
		if flag.Shorthand != f.short {
			t.Errorf("flag %q: shorthand = %q, want %q", f.long, flag.Shorthand, f.short)
		}
	}
}

func TestRootCommand_VersionFlag(t *testing.T) {
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--version"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "amadeus version") {
		t.Errorf("--version output should contain 'amadeus version', got %q", output)
	}
}

func TestVersionCommand_Output(t *testing.T) {
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "amadeus") {
		t.Error("version output should contain 'amadeus'")
	}
	if !strings.Contains(output, "commit:") {
		t.Error("version output should contain 'commit:'")
	}
}

func TestVersionCommand_JSONFlag(t *testing.T) {
	for _, flag := range []string{"--json", "-j"} {
		t.Run(flag, func(t *testing.T) {
			rootCmd := cmd.NewRootCommand()
			buf := new(bytes.Buffer)
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)
			rootCmd.SetArgs([]string{"version", flag})

			err := rootCmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			if !strings.Contains(output, `"version"`) {
				t.Error("JSON output should contain 'version' key")
			}
			if !strings.Contains(output, `"commit"`) {
				t.Error("JSON output should contain 'commit' key")
			}
		})
	}
}

func TestUpdateCommand_HasCheckFlag(t *testing.T) {
	rootCmd := cmd.NewRootCommand()
	var updateCmd *cobra.Command
	for _, c := range rootCmd.Commands() {
		if c.Name() == "update" {
			updateCmd = c
			break
		}
	}
	if updateCmd == nil {
		t.Fatal("update subcommand not found")
	}

	flag := updateCmd.Flags().Lookup("check")
	if flag == nil {
		t.Fatal("update subcommand missing flag 'check'")
	}
	if flag.Shorthand != "C" {
		t.Errorf("flag 'check': shorthand = %q, want %q", flag.Shorthand, "C")
	}
}
