package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestVersion_TextOutput(t *testing.T) {
	// given
	origVersion, origCommit, origDate := Version, Commit, Date
	Version, Commit, Date = "1.2.3", "abc1234", "2026-02-21T00:00:00Z"
	defer func() { Version, Commit, Date = origVersion, origCommit, origDate }()
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"version"})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "amadeus version 1.2.3 (commit: abc1234, built: 2026-02-21T00:00:00Z)\n"
	if got := buf.String(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestVersion_JSONOutput(t *testing.T) {
	// given
	origVersion, origCommit, origDate := Version, Commit, Date
	Version, Commit, Date = "1.2.3", "abc1234", "2026-02-21T00:00:00Z"
	defer func() { Version, Commit, Date = origVersion, origCommit, origDate }()
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"version", "--json"})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if result["version"] != "1.2.3" {
		t.Errorf("expected version=1.2.3, got %q", result["version"])
	}
	if result["commit"] != "abc1234" {
		t.Errorf("expected commit=abc1234, got %q", result["commit"])
	}
	if result["date"] != "2026-02-21T00:00:00Z" {
		t.Errorf("expected date=2026-02-21T00:00:00Z, got %q", result["date"])
	}
}
