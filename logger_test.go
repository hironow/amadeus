package amadeus

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, false)
	log.Info("hello %s", "world")

	got := buf.String()
	if !strings.Contains(got, "INFO hello world") {
		t.Errorf("expected INFO prefix, got %q", got)
	}
	if !regexp.MustCompile(`\[\d{2}:\d{2}:\d{2}\]`).MatchString(got) {
		t.Errorf("expected timestamp, got %q", got)
	}
}

func TestLogger_OK(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, false)
	log.OK("done")
	if !strings.Contains(buf.String(), " OK  done") {
		t.Errorf("expected OK prefix, got %q", buf.String())
	}
}

func TestLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, false)
	log.Warn("careful")
	if !strings.Contains(buf.String(), "WARN careful") {
		t.Errorf("expected WARN prefix, got %q", buf.String())
	}
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, false)
	log.Error("bad")
	if !strings.Contains(buf.String(), " ERR bad") {
		t.Errorf("expected ERR prefix, got %q", buf.String())
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
	if !strings.Contains(buf.String(), "DBUG shown") {
		t.Errorf("expected DBUG prefix, got %q", buf.String())
	}
}

func TestLogger_SetExtraWriter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	var buf bytes.Buffer
	log := NewLogger(&buf, false)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer f.Close()

	log.SetExtraWriter(f)

	log.Info("dual")

	// Check buffer output
	if !strings.Contains(buf.String(), "INFO dual") {
		t.Errorf("expected buffer output, got %q", buf.String())
	}

	// Check file output
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "INFO dual") {
		t.Errorf("expected file output, got %q", string(data))
	}
}

func TestLogger_SetExtraWriter_Nil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	var buf bytes.Buffer
	log := NewLogger(&buf, false)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	log.SetExtraWriter(f)
	f.Close()

	// Disconnect extra writer
	log.SetExtraWriter(nil)

	// After disconnect, should write only to buf, not crash
	log.Info("after disconnect")
	if !strings.Contains(buf.String(), "INFO after disconnect") {
		t.Errorf("expected output after disconnect, got %q", buf.String())
	}
}
