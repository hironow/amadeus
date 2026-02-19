package amadeus

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, false)
	log.Info("hello %s", "world")
	if !strings.Contains(buf.String(), "hello world") {
		t.Errorf("expected 'hello world' in output, got %q", buf.String())
	}
}

func TestLogger_Verbose_Suppressed(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, false)
	log.Debug("hidden")
	if buf.Len() != 0 {
		t.Errorf("expected no output in non-verbose mode, got %q", buf.String())
	}
}

func TestLogger_Verbose_Shown(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, true)
	log.Debug("shown")
	if !strings.Contains(buf.String(), "shown") {
		t.Errorf("expected 'shown' in output, got %q", buf.String())
	}
}
