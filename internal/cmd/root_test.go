package cmd_test

import (
	"bytes"
	"testing"

	"github.com/hironow/amadeus/internal/cmd"
	"github.com/spf13/cobra"
)

func TestNewRootCommand_HasPersistentFlags(t *testing.T) {
	// given
	rootCmd := cmd.NewRootCommand()

	// then
	for _, name := range []string{"config", "verbose", "lang"} {
		if rootCmd.PersistentFlags().Lookup(name) == nil {
			t.Errorf("expected PersistentFlag %q to exist", name)
		}
	}
	// shorthand checks
	if sh := rootCmd.PersistentFlags().ShorthandLookup("c"); sh == nil || sh.Name != "config" {
		t.Error("expected -c shorthand for --config")
	}
	if sh := rootCmd.PersistentFlags().ShorthandLookup("v"); sh == nil || sh.Name != "verbose" {
		t.Error("expected -v shorthand for --verbose")
	}
	if sh := rootCmd.PersistentFlags().ShorthandLookup("l"); sh == nil || sh.Name != "lang" {
		t.Error("expected -l shorthand for --lang")
	}
}

func TestNewRootCommand_VersionOutput(t *testing.T) {
	// given
	origVersion := cmd.Version
	cmd.Version = "1.2.3"
	defer func() { cmd.Version = origVersion }()
	rootCmd := cmd.NewRootCommand()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"--version"})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if got != "amadeus version 1.2.3\n" {
		t.Errorf("expected 'amadeus version 1.2.3\\n', got %q", got)
	}
}

func TestNewRootCommand_NoArgsReturnsError(t *testing.T) {
	// given
	rootCmd := cmd.NewRootCommand()
	rootCmd.SetArgs([]string{})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error when no subcommand provided, got nil")
	}
}

func TestSubcommand_ShortAliases(t *testing.T) {
	root := cmd.NewRootCommand()

	// Build a map of subcommand name → *cobra.Command for easy lookup.
	subs := map[string]*cobra.Command{}
	for _, sub := range root.Commands() {
		subs[sub.Name()] = sub
	}

	tests := []struct {
		subcommand string
		flag       string
		shorthand  string
	}{
		// archive-prune
		{"archive-prune", "days", "d"},
		{"archive-prune", "dry-run", "n"},
		{"archive-prune", "yes", "y"},
		// version
		{"version", "json", "j"},
		// update
		{"update", "check", "C"},
		// doctor
		{"doctor", "json", "j"},
		// log
		{"log", "json", "j"},
	}

	for _, tt := range tests {
		t.Run(tt.subcommand+"/"+tt.flag, func(t *testing.T) {
			// given
			sub, ok := subs[tt.subcommand]
			if !ok {
				t.Fatalf("subcommand %q not found", tt.subcommand)
			}

			// then — flag exists
			f := sub.Flags().Lookup(tt.flag)
			if f == nil {
				t.Fatalf("flag --%s not found on %s", tt.flag, tt.subcommand)
			}

			// then — shorthand matches
			if f.Shorthand != tt.shorthand {
				t.Errorf("expected shorthand %q for --%s on %s, got %q",
					tt.shorthand, tt.flag, tt.subcommand, f.Shorthand)
			}
		})
	}
}
