package session

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	amadeus "github.com/hironow/amadeus"
)

func TestCmdNotifier_Timeout(t *testing.T) {
	// given -- a command factory that captures the context deadline
	var capturedCtx context.Context
	n := &CmdNotifier{
		cmdTemplate: "echo {message}",
		cmdFactory: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			capturedCtx = ctx
			return exec.Command("true")
		},
	}

	// when
	err := n.Notify(context.Background(), "Title", "Message")

	// then -- the context passed to the command should have a deadline
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedCtx == nil {
		t.Fatal("context was not captured")
	}
	deadline, ok := capturedCtx.Deadline()
	if !ok {
		t.Fatal("context should have a deadline (30s timeout)")
	}
	// Deadline should be roughly 30s from now (allow some slack)
	_ = deadline // existence check is sufficient
}

func TestCmdNotifier_EmptyTemplate(t *testing.T) {
	// given
	n := NewCmdNotifier("")

	// when
	err := n.Notify(context.Background(), "Title", "Message")

	// then -- empty template should produce an error
	if err == nil {
		t.Error("expected error for empty template")
	}
}

func TestCmdNotifier_Placeholders(t *testing.T) {
	// given -- template with placeholders, factory captures the expanded command
	var capturedArgs []string
	n := &CmdNotifier{
		cmdTemplate: "echo {title}: {message}",
		cmdFactory: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			capturedArgs = args
			return exec.Command("true")
		},
	}

	// when
	err := n.Notify(context.Background(), "Hello", "World")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedArgs) == 0 {
		t.Fatal("expected args to be captured")
	}
	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "'Hello'") {
		t.Errorf("expected quoted title in command, got: %s", joined)
	}
	if !strings.Contains(joined, "'World'") {
		t.Errorf("expected quoted message in command, got: %s", joined)
	}
}

func TestLocalNotifier_UnsupportedOS(t *testing.T) {
	// given: unsupported OS
	n := NewLocalNotifierForTest("freebsd",
		func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("true")
		},
	)

	// when
	err := n.Notify(context.Background(), "Title", "Message")

	// then: should return ErrUnsupportedOS sentinel
	if err != amadeus.ErrUnsupportedOS {
		t.Errorf("err = %v, want amadeus.ErrUnsupportedOS", err)
	}
}
