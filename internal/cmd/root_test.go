package cmd

import (
	"bytes"
	"testing"
)

func TestNewRootCommand_HasPersistentFlags(t *testing.T) {
	// given
	cmd := NewRootCommand(BuildInfo{Version: "test"})

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
	tests := []struct {
		name string
		args []string
	}{
		{"double-dash", []string{"--version"}},
		{"single-dash-compat", []string{"-version"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			cmd := NewRootCommand(BuildInfo{Version: "1.2.3"})
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetArgs(NormalizeArgs(tt.args))

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
		})
	}
}

func TestNewRootCommand_NoArgsReturnsError(t *testing.T) {
	// given
	cmd := NewRootCommand(BuildInfo{Version: "test"})
	cmd.SetArgs([]string{})

	// when
	err := cmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error when no subcommand provided, got nil")
	}
}

func TestNormalizeArgs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"version-single-dash", []string{"-version"}, []string{"--version"}},
		{"help-single-dash", []string{"-help"}, []string{"--help"}},
		{"double-dash-unchanged", []string{"--version"}, []string{"--version"}},
		{"other-flags-unchanged", []string{"-v", "--json", "check"}, []string{"-v", "--json", "check"}},
		{"mixed", []string{"-version", "check", "-help"}, []string{"--version", "check", "--help"}},
		{"empty", []string{}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeArgs(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("length mismatch: got %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("arg[%d]: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
