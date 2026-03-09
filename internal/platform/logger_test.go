package platform_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
)

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	log := platform.NewLogger(&buf, false)
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
	log := platform.NewLogger(&buf, false)
	log.OK("done")
	if !strings.Contains(buf.String(), " OK  done") {
		t.Errorf("expected OK prefix, got %q", buf.String())
	}
}

func TestLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	log := platform.NewLogger(&buf, false)
	log.Warn("careful")
	if !strings.Contains(buf.String(), "WARN careful") {
		t.Errorf("expected WARN prefix, got %q", buf.String())
	}
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	log := platform.NewLogger(&buf, false)
	log.Error("bad")
	if !strings.Contains(buf.String(), " ERR bad") {
		t.Errorf("expected ERR prefix, got %q", buf.String())
	}
}

func TestLogger_Verbose_Suppressed(t *testing.T) {
	var buf bytes.Buffer
	log := platform.NewLogger(&buf, false)
	log.Debug("hidden")
	if buf.Len() != 0 {
		t.Errorf("expected no output in non-verbose mode, got %q", buf.String())
	}
}

func TestLogger_Verbose_Shown(t *testing.T) {
	var buf bytes.Buffer
	log := platform.NewLogger(&buf, true)
	log.Debug("shown")
	if !strings.Contains(buf.String(), "DBUG shown") {
		t.Errorf("expected DBUG prefix, got %q", buf.String())
	}
}

func TestLogger_SetExtraWriter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	var buf bytes.Buffer
	log := platform.NewLogger(&buf, false)

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

func TestLogger_NoColorWhenNotTerminal(t *testing.T) {
	var buf bytes.Buffer
	log := platform.NewLogger(&buf, false)
	log.Info("no terminal")
	if strings.Contains(buf.String(), "\033[") {
		t.Errorf("expected no ANSI codes for non-terminal writer, got %q", buf.String())
	}
}

func TestLogger_ColorWhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	log := platform.NewLogger(&buf, false)
	log.SetNoColor(false)
	log.Info("colored")
	if !strings.Contains(buf.String(), "\033[") {
		t.Errorf("expected ANSI codes when color enabled, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "\033[0m") {
		t.Errorf("expected reset code, got %q", buf.String())
	}
}

func TestLogger_SetNoColor(t *testing.T) {
	var buf bytes.Buffer
	log := platform.NewLogger(&buf, false)
	log.SetNoColor(false)
	log.Info("on")
	colored := buf.String()

	buf.Reset()
	log.SetNoColor(true)
	log.Info("off")
	plain := buf.String()

	if !strings.Contains(colored, "\033[") {
		t.Errorf("expected color codes when on, got %q", colored)
	}
	if strings.Contains(plain, "\033[") {
		t.Errorf("expected no color codes when off, got %q", plain)
	}
}

func TestLogger_NoColorEnvVar(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	log := platform.NewLogger(&buf, false)
	log.Info("env test")
	if strings.Contains(buf.String(), "\033[") {
		t.Errorf("NO_COLOR=1 should disable color, got %q", buf.String())
	}
}

func TestLogger_ExtraWriterPlainText(t *testing.T) {
	var primary bytes.Buffer
	log := platform.NewLogger(&primary, false)
	log.SetNoColor(false) // color on for primary

	var extra bytes.Buffer
	log.SetExtraWriter(&extra)

	log.Info("dual")

	if !strings.Contains(primary.String(), "\033[") {
		t.Errorf("primary should have ANSI codes, got %q", primary.String())
	}
	if strings.Contains(extra.String(), "\033[") {
		t.Errorf("extra writer should be plain text, got %q", extra.String())
	}
}

func TestLogger_SetExtraWriter_Nil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	var buf bytes.Buffer
	log := platform.NewLogger(&buf, false)

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

func TestLogger_ConcurrentSetExtraWriterAndWrite(t *testing.T) {
	logger := platform.NewLogger(io.Discard, false)

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(3)
		go func() {
			defer wg.Done()
			var buf bytes.Buffer
			logger.SetExtraWriter(&buf)
		}()
		go func(n int) {
			defer wg.Done()
			logger.Info("race test info %d", n)
			logger.Warn("race test warn %d", n)
		}(i)
		go func() {
			defer wg.Done()
			logger.SetExtraWriter(nil)
		}()
	}
	wg.Wait()

	// Clean up
	logger.SetExtraWriter(nil)
}

func TestBanner_Send_Format(t *testing.T) {
	var buf bytes.Buffer
	log := platform.NewLogger(&buf, false)
	log.SetNoColor(false)

	log.Banner(domain.BannerSend, "feedback", "fb-001", "test description")

	got := buf.String()
	if !strings.Contains(got, "\033[7;32m") {
		t.Errorf("expected green inversion ANSI code, got %q", got)
	}
	if !strings.Contains(got, "D-MAIL SEND") {
		t.Errorf("expected D-MAIL SEND label, got %q", got)
	}
	if !strings.Contains(got, "feedback") {
		t.Errorf("expected kind, got %q", got)
	}
	if !strings.Contains(got, "fb-001") {
		t.Errorf("expected name, got %q", got)
	}
}

func TestBanner_Recv_Format(t *testing.T) {
	var buf bytes.Buffer
	log := platform.NewLogger(&buf, false)
	log.SetNoColor(false)

	log.Banner(domain.BannerRecv, "report", "rpt-001", "incoming report")

	got := buf.String()
	if !strings.Contains(got, "\033[7;36m") {
		t.Errorf("expected cyan inversion ANSI code, got %q", got)
	}
	if !strings.Contains(got, "D-MAIL RECV") {
		t.Errorf("expected D-MAIL RECV label, got %q", got)
	}
}

func TestBanner_NoColor(t *testing.T) {
	var buf bytes.Buffer
	log := platform.NewLogger(&buf, false)
	log.SetNoColor(true)

	log.Banner(domain.BannerSend, "feedback", "fb-001", "test")

	got := buf.String()
	if strings.Contains(got, "\033[") {
		t.Errorf("expected no ANSI codes in no-color mode, got %q", got)
	}
	if !strings.Contains(got, ">>> D-MAIL SEND") {
		t.Errorf("expected plain arrow prefix, got %q", got)
	}
}

func TestBanner_LongDescription_Truncated(t *testing.T) {
	var buf bytes.Buffer
	log := platform.NewLogger(&buf, false)
	log.SetNoColor(true)

	long := strings.Repeat("x", 60)
	log.Banner(domain.BannerSend, "feedback", "fb-001", long)

	got := buf.String()
	if !strings.Contains(got, "...") {
		t.Errorf("expected truncation ellipsis, got %q", got)
	}
	if strings.Contains(got, long) {
		t.Errorf("expected description to be truncated, got full string in %q", got)
	}
}

func TestBanner_ExtraWriter_PlainText(t *testing.T) {
	var primary bytes.Buffer
	log := platform.NewLogger(&primary, false)
	log.SetNoColor(false)

	var extra bytes.Buffer
	log.SetExtraWriter(&extra)

	log.Banner(domain.BannerSend, "feedback", "fb-001", "test")

	if !strings.Contains(primary.String(), "\033[7;32m") {
		t.Errorf("primary should have ANSI codes, got %q", primary.String())
	}
	if strings.Contains(extra.String(), "\033[") {
		t.Errorf("extra writer should be plain text, got %q", extra.String())
	}
	if !strings.Contains(extra.String(), ">>> D-MAIL SEND") {
		t.Errorf("extra writer should have plain content, got %q", extra.String())
	}
}
