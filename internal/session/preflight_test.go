package session_test

import (
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/session"
)

func TestPreflightCheck_NotFound_ShowsResolvedBinary(t *testing.T) {
	bin := "CLAUDE_CONFIG_DIR=/tmp/test /nonexistent/fake-claude"
	err := session.PreflightCheck(bin)
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
	msg := err.Error()
	if !strings.Contains(msg, "/nonexistent/fake-claude") {
		t.Errorf("error should contain resolved binary path, got: %s", msg)
	}
	if !strings.Contains(msg, "from") {
		t.Errorf("error should contain 'from' context, got: %s", msg)
	}
}

func TestPreflightCheck_SimpleBinary_NotFound(t *testing.T) {
	err := session.PreflightCheck("nonexistent-binary-xyz")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "nonexistent-binary-xyz") {
		t.Errorf("error should contain binary name, got: %s", err.Error())
	}
}
