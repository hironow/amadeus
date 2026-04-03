package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionCmd_TextOutput(t *testing.T) {
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
	out := buf.String()
	if !strings.Contains(out, "amadeus v1.2.3") {
		t.Errorf("expected 'amadeus v1.2.3' in output, got: %s", out)
	}
	if !strings.Contains(out, "commit: abc1234") {
		t.Errorf("expected 'commit: abc1234' in output, got: %s", out)
	}
	if !strings.Contains(out, "go:") {
		t.Errorf("expected 'go:' in output, got: %s", out)
	}
}

func TestVersionCmd_JSONOutput(t *testing.T) {
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

	var info map[string]string
	if err := json.Unmarshal(buf.Bytes(), &info); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, buf.String())
	}

	for _, key := range []string{"version", "commit", "date", "go", "os", "arch"} {
		if _, ok := info[key]; !ok {
			t.Errorf("expected key %q in JSON output", key)
		}
	}
	if info["version"] != "1.2.3" {
		t.Errorf("expected version=1.2.3, got %q", info["version"])
	}
}

func TestVersionCmd_NoDoubleV_WhenVersionStartsWithV(t *testing.T) {
	// given: Version starts with "v" (git describe output)
	origVersion, origCommit, origDate := Version, Commit, Date
	Version, Commit, Date = "v0.0.11-test", "def5678", "2026-04-03T00:00:00Z"
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
	out := buf.String()
	if strings.Contains(out, "vv") {
		t.Errorf("version output contains double-v 'vv': %s", out)
	}
	if !strings.Contains(out, "amadeus v0.0.11-test") {
		t.Errorf("expected 'amadeus v0.0.11-test' in output, got: %s", out)
	}
}
