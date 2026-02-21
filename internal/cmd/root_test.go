package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestNewRootCommand_HasPersistentFlags(t *testing.T) {
	// given
	cmd := NewRootCommand()

	// then
	for _, name := range []string{"config", "verbose", "lang"} {
		if cmd.PersistentFlags().Lookup(name) == nil {
			t.Errorf("expected PersistentFlag %q to exist", name)
		}
	}
	// shorthand checks
	if sh := cmd.PersistentFlags().ShorthandLookup("c"); sh == nil || sh.Name != "config" {
		t.Error("expected -c shorthand for --config")
	}
	if sh := cmd.PersistentFlags().ShorthandLookup("v"); sh == nil || sh.Name != "verbose" {
		t.Error("expected -v shorthand for --verbose")
	}
	if sh := cmd.PersistentFlags().ShorthandLookup("l"); sh == nil || sh.Name != "lang" {
		t.Error("expected -l shorthand for --lang")
	}
}

func TestNewRootCommand_VersionOutput(t *testing.T) {
	// given
	origVersion := Version
	Version = "1.2.3"
	defer func() { Version = origVersion }()
	cmd := NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--version"})

	// when
	err := cmd.Execute()

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
	cmd := NewRootCommand()
	cmd.SetArgs([]string{})

	// when
	err := cmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error when no subcommand provided, got nil")
	}
}

func TestSubcommand_ShortAliases(t *testing.T) {
	root := NewRootCommand()

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
		// check
		{"check", "json", "j"},
		{"check", "dry-run", "n"},
		{"check", "full", "f"},
		{"check", "quiet", "q"},
		// resolve
		{"resolve", "approve", "a"},
		{"resolve", "reject", "r"},
		{"resolve", "json", "j"},
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

func TestResolve_ReasonIsLongOnly(t *testing.T) {
	// given
	root := NewRootCommand()
	var resolve *cobra.Command
	for _, sub := range root.Commands() {
		if sub.Name() == "resolve" {
			resolve = sub
			break
		}
	}
	if resolve == nil {
		t.Fatal("resolve subcommand not found")
	}

	// then — --reason has no shorthand
	f := resolve.Flags().Lookup("reason")
	if f == nil {
		t.Fatal("--reason flag not found")
	}
	if f.Shorthand != "" {
		t.Errorf("expected --reason to be long-only, got shorthand %q", f.Shorthand)
	}
}
