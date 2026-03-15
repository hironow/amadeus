package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

func TestNewRootCommand_NoColorFlag(t *testing.T) {
	// given
	rootCmd := cmd.NewRootCommand()

	// when
	f := rootCmd.PersistentFlags().Lookup("no-color")

	// then
	if f == nil {
		t.Fatal("--no-color PersistentFlag not found")
	}
	if f.DefValue != "false" {
		t.Errorf("--no-color default = %q, want %q", f.DefValue, "false")
	}
}

func TestRootCmd_OutputFlagExists(t *testing.T) {
	// given
	rootCmd := cmd.NewRootCommand()

	// when
	f := rootCmd.PersistentFlags().Lookup("output")

	// then
	if f == nil {
		t.Fatal("--output flag not found")
	}
	if f.DefValue != "text" {
		t.Errorf("default = %q, want text", f.DefValue)
	}
	if f.Shorthand != "o" {
		t.Errorf("shorthand = %q, want o", f.Shorthand)
	}
}

func TestRootCmd_VerboseIncreasesStderrOutput(t *testing.T) {
	// given: initialized .gate/ with config
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	os.MkdirAll(filepath.Join(gateDir, "events"), 0o755)
	os.MkdirAll(filepath.Join(gateDir, ".run"), 0o755)
	os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte("lang: en\ntracker:\n  team: TEST\n"), 0o644)

	// when: run status without verbose
	root1 := cmd.NewRootCommand()
	var stdout1, stderr1 bytes.Buffer
	root1.SetOut(&stdout1)
	root1.SetErr(&stderr1)
	root1.SetArgs([]string{"status", dir})
	root1.Execute()

	// when: run status WITH verbose
	root2 := cmd.NewRootCommand()
	var stdout2, stderr2 bytes.Buffer
	root2.SetOut(&stdout2)
	root2.SetErr(&stderr2)
	root2.SetArgs([]string{"-v", "status", dir})
	root2.Execute()

	// then: verbose should produce at least as much stderr
	if stderr2.Len() < stderr1.Len() {
		t.Errorf("verbose stderr (%d bytes) should be >= non-verbose stderr (%d bytes)",
			stderr2.Len(), stderr1.Len())
	}
}

func TestRootCmd_NoColorSetsEnv(t *testing.T) {
	// given
	origVal := os.Getenv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	t.Cleanup(func() {
		if origVal != "" {
			os.Setenv("NO_COLOR", origVal)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	})

	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	os.MkdirAll(filepath.Join(gateDir, "events"), 0o755)
	os.MkdirAll(filepath.Join(gateDir, ".run"), 0o755)
	os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte("lang: en\ntracker:\n  team: TEST\n"), 0o644)

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--no-color", "status", dir})

	// when
	root.Execute()

	// then
	if got := os.Getenv("NO_COLOR"); got == "" {
		t.Error("expected NO_COLOR env to be set after --no-color flag")
	}
	output := buf.String()
	if strings.Contains(output, "\033[") {
		t.Errorf("--no-color output should not contain ANSI codes, got: %q", output)
	}
}
