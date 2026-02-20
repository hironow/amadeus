package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// These tests verify cobra flag parsing for the resolve command.
// They replace the old parseResolveArgs tests from cmd/amadeus/main_test.go.
// Each test creates a fresh root command, sets args, and checks that cobra
// correctly separates positional args from interspersed flags.

var errTestSentinel = &sentinelError{}

type sentinelError struct{}

func (e *sentinelError) Error() string { return "test sentinel" }

func isSentinel(err error) bool {
	_, ok := err.(*sentinelError)
	return ok
}

type testResolveResult struct {
	root       *cobra.Command
	approve    bool
	reject     bool
	reason     string
	names      []string
	configPath string
	verbose    bool
	jsonOut    bool
}

func newTestResolveCmd() *testResolveResult {
	r := &testResolveResult{}
	root := NewRootCommand("test")
	for _, sub := range root.Commands() {
		if sub.Name() == "resolve" {
			sub.RunE = func(cmd *cobra.Command, args []string) error {
				r.approve, _ = cmd.Flags().GetBool("approve")
				r.reject, _ = cmd.Flags().GetBool("reject")
				r.reason, _ = cmd.Flags().GetString("reason")
				r.configPath, _ = cmd.Flags().GetString("config")
				r.verbose, _ = cmd.Flags().GetBool("verbose")
				r.jsonOut, _ = cmd.Flags().GetBool("json")
				r.names = args
				return errTestSentinel
			}
		}
	}
	r.root = root
	return r
}

func (r *testResolveResult) run(args ...string) error {
	r.root.SetArgs(append([]string{"resolve"}, args...))
	return r.root.Execute()
}

func TestResolve_FlagsAfterName(t *testing.T) {
	r := newTestResolveCmd()
	err := r.run("feedback-001", "--approve")
	if !isSentinel(err) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.approve {
		t.Error("expected approve=true")
	}
	if r.reject {
		t.Error("expected reject=false")
	}
	if len(r.names) != 1 || r.names[0] != "feedback-001" {
		t.Errorf("expected names=[feedback-001], got %v", r.names)
	}
}

func TestResolve_FlagsBeforeName(t *testing.T) {
	r := newTestResolveCmd()
	err := r.run("--approve", "feedback-001")
	if !isSentinel(err) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.approve {
		t.Error("expected approve=true")
	}
	if len(r.names) != 1 || r.names[0] != "feedback-001" {
		t.Errorf("expected names=[feedback-001], got %v", r.names)
	}
}

func TestResolve_RejectWithReason(t *testing.T) {
	r := newTestResolveCmd()
	err := r.run("feedback-001", "--reject", "--reason", "not aligned")
	if !isSentinel(err) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.reject {
		t.Error("expected reject=true")
	}
	if r.reason != "not aligned" {
		t.Errorf("expected reason='not aligned', got %q", r.reason)
	}
	if len(r.names) != 1 || r.names[0] != "feedback-001" {
		t.Errorf("expected names=[feedback-001], got %v", r.names)
	}
}

func TestResolve_RejectWithReasonEquals(t *testing.T) {
	r := newTestResolveCmd()
	err := r.run("feedback-001", "--reject", "--reason=not aligned")
	if !isSentinel(err) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.reject {
		t.Error("expected reject=true")
	}
	if r.reason != "not aligned" {
		t.Errorf("expected reason='not aligned', got %q", r.reason)
	}
}

func TestResolve_MultipleNames(t *testing.T) {
	r := newTestResolveCmd()
	err := r.run("feedback-001", "feedback-002", "--approve")
	if !isSentinel(err) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.approve {
		t.Error("expected approve=true")
	}
	if len(r.names) != 2 {
		t.Fatalf("expected 2 names, got %d: %v", len(r.names), r.names)
	}
	if r.names[0] != "feedback-001" || r.names[1] != "feedback-002" {
		t.Errorf("expected [feedback-001, feedback-002], got %v", r.names)
	}
}

func TestResolve_NoFlags(t *testing.T) {
	r := newTestResolveCmd()
	err := r.run("feedback-001")
	if !isSentinel(err) {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.approve || r.reject {
		t.Error("expected both approve and reject to be false")
	}
	if len(r.names) != 1 || r.names[0] != "feedback-001" {
		t.Errorf("expected names=[feedback-001], got %v", r.names)
	}
}

func TestResolve_Empty(t *testing.T) {
	r := newTestResolveCmd()
	err := r.run()
	if !isSentinel(err) {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.names) != 0 {
		t.Errorf("expected empty names, got %v", r.names)
	}
}

func TestResolve_CommonFlags(t *testing.T) {
	r := newTestResolveCmd()
	err := r.run("--approve", "-v", "--json", "-c", "custom.yaml", "feedback-001")
	if !isSentinel(err) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.approve {
		t.Error("expected approve=true")
	}
	if !r.verbose {
		t.Error("expected verbose=true")
	}
	if !r.jsonOut {
		t.Error("expected jsonOut=true")
	}
	if r.configPath != "custom.yaml" {
		t.Errorf("expected configPath='custom.yaml', got %q", r.configPath)
	}
	if len(r.names) != 1 || r.names[0] != "feedback-001" {
		t.Errorf("expected names=[feedback-001], got %v", r.names)
	}
}

func TestResolve_CommonFlagsLongForm(t *testing.T) {
	r := newTestResolveCmd()
	err := r.run("--verbose", "--config", "path.yaml", "feedback-001", "--reject", "--reason", "bad")
	if !isSentinel(err) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.reject {
		t.Error("expected reject=true")
	}
	if !r.verbose {
		t.Error("expected verbose=true")
	}
	if r.configPath != "path.yaml" {
		t.Errorf("expected configPath='path.yaml', got %q", r.configPath)
	}
	if r.reason != "bad" {
		t.Errorf("expected reason='bad', got %q", r.reason)
	}
}
