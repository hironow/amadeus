package port_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/usecase/port" // nosemgrep: layer-port-no-import-upper — _test package importing its own public API [permanent]
)

func TestWithResume_SetsResumeSessionID(t *testing.T) {
	t.Parallel()
	rc := port.ApplyOptions(port.WithResume("session-123"))
	if rc.ResumeSessionID != "session-123" {
		t.Errorf("ResumeSessionID = %q, want %q", rc.ResumeSessionID, "session-123")
	}
	if rc.Continue {
		t.Error("Continue should be false when WithResume is used")
	}
}

func TestWithContinue_DoesNotSetResume(t *testing.T) {
	t.Parallel()
	rc := port.ApplyOptions(port.WithContinue())
	if rc.ResumeSessionID != "" {
		t.Errorf("ResumeSessionID should be empty, got %q", rc.ResumeSessionID)
	}
	if !rc.Continue {
		t.Error("Continue should be true")
	}
}
