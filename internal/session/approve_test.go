package session

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestStdinApprover_Yes(t *testing.T) {
	// given
	in := strings.NewReader("y\n")
	out := new(bytes.Buffer)
	a := NewStdinApprover(in, out)

	// when
	approved, err := a.RequestApproval(context.Background(), "Continue check?")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("expected approved=true for 'y' input")
	}
	if !strings.Contains(out.String(), "Continue? [y/N]") {
		t.Errorf("prompt not shown, got: %q", out.String())
	}
}

func TestStdinApprover_YesFull(t *testing.T) {
	// given
	in := strings.NewReader("yes\n")
	out := new(bytes.Buffer)
	a := NewStdinApprover(in, out)

	// when
	approved, err := a.RequestApproval(context.Background(), "msg")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("expected approved=true for 'yes' input")
	}
}

func TestStdinApprover_No(t *testing.T) {
	// given
	in := strings.NewReader("n\n")
	out := new(bytes.Buffer)
	a := NewStdinApprover(in, out)

	// when
	approved, err := a.RequestApproval(context.Background(), "msg")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved {
		t.Error("expected approved=false for 'n' input")
	}
}

func TestStdinApprover_EmptyDefault(t *testing.T) {
	// given: empty enter = default = deny (safe side)
	in := strings.NewReader("\n")
	out := new(bytes.Buffer)
	a := NewStdinApprover(in, out)

	// when
	approved, err := a.RequestApproval(context.Background(), "msg")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved {
		t.Error("expected approved=false for empty input (safe default)")
	}
}

func TestStdinApprover_NilInput(t *testing.T) {
	// given: StdinApprover with nil input (library/non-interactive usage)
	a := NewStdinApprover(nil, new(bytes.Buffer))

	// when: should not panic
	approved, err := a.RequestApproval(context.Background(), "msg")

	// then: safe default = deny, no error
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if approved {
		t.Error("expected denial for nil input")
	}
}

func TestStdinApprover_EOFTerminatedYes(t *testing.T) {
	// given: piped input "y" without trailing newline (echo -n "y" | amadeus check)
	in := strings.NewReader("y")
	out := new(bytes.Buffer)
	a := NewStdinApprover(in, out)

	// when
	approved, err := a.RequestApproval(context.Background(), "msg")

	// then: should approve even without trailing newline
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("expected approval for EOF-terminated 'y' input")
	}
}

func TestStdinApprover_EOFTerminatedNo(t *testing.T) {
	// given: piped "n" without trailing newline — should deny (not error)
	in := strings.NewReader("n")
	out := new(bytes.Buffer)
	a := NewStdinApprover(in, out)

	// when
	approved, err := a.RequestApproval(context.Background(), "msg")

	// then
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if approved {
		t.Error("expected denial for EOF-terminated 'n' input")
	}
}

func TestStdinApprover_SharedReader(t *testing.T) {
	// given: a shared reader with approval line + subsequent data.
	// After RequestApproval consumes "y\n", the remaining "next-line\n"
	// must still be readable from the same reader.
	in := strings.NewReader("y\nnext-line\n")
	out := new(bytes.Buffer)
	a := NewStdinApprover(in, out)

	// when
	approved, err := a.RequestApproval(context.Background(), "msg")

	// then: approved
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Fatal("expected approval")
	}

	// then: remaining data is still available from the shared reader
	remaining := make([]byte, 64)
	n, _ := in.Read(remaining)
	got := string(remaining[:n])
	if got != "next-line\n" {
		t.Errorf("shared reader lost data: got %q, want %q", got, "next-line\n")
	}
}

func TestStdinApprover_ContextCancel(t *testing.T) {
	// given: context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	in := new(blockingReader)
	out := new(bytes.Buffer)
	a := NewStdinApprover(in, out)

	// when
	approved, err := a.RequestApproval(ctx, "msg")

	// then
	if approved {
		t.Error("expected approved=false when context is cancelled")
	}
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestStdinApprover_Timeout(t *testing.T) {
	// given
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	in := new(blockingReader)
	out := new(bytes.Buffer)
	a := NewStdinApprover(in, out)

	// when
	approved, err := a.RequestApproval(ctx, "msg")

	// then
	if approved {
		t.Error("expected approved=false on timeout")
	}
	if err == nil {
		t.Error("expected error on timeout")
	}
}

func TestCmdApprover_EmptyTemplate(t *testing.T) {
	// given
	a := NewCmdApprover("")

	// when
	approved, err := a.RequestApproval(context.Background(), "msg")

	// then -- empty template should produce an error and deny
	if err == nil {
		t.Error("expected error for empty template")
	}
	if approved {
		t.Error("expected approved=false for empty template")
	}
}

func TestCmdApprover_FactoryDI(t *testing.T) {
	// given -- inject a factory that records the expanded command
	var capturedArgs []string
	a := &CmdApprover{
		cmdTemplate: "echo {message}",
		cmdFactory: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			capturedArgs = args
			return exec.Command("true")
		},
	}

	// when
	approved, err := a.RequestApproval(context.Background(), "hello world")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("expected approved=true for exit code 0")
	}
	if len(capturedArgs) == 0 {
		t.Fatal("expected args to be captured by factory")
	}
	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "'hello world'") {
		t.Errorf("expected quoted message in command, got: %s", joined)
	}
}

// blockingReader never returns data, simulating a blocking stdin.
type blockingReader struct{}

func (r *blockingReader) Read(p []byte) (int, error) {
	select {} //nolint:staticcheck // intentional blocking for test
}
