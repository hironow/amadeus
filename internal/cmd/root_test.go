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
	// given
	cmd := NewRootCommand(BuildInfo{Version: "1.2.3"})
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
