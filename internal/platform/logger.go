package platform

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// compile-time interface checks
var _ domain.Logger = (*Logger)(nil)
var _ domain.BannerLogger = (*Logger)(nil)

// ANSI color codes — CVD (Color Vision Deficiency) friendly palette.
//
// Design rationale (Wong 2011, Nature Methods "Points of view: Color blindness"):
//   - Blue-Yellow axis is preserved by protanopia/deuteranopia (~8% of males)
//   - Bold weight provides brightness cue independent of hue perception
//   - Text prefixes (INFO/OK/WARN/ERR/DBUG) ensure non-color redundancy
//
// Respects NO_COLOR (https://no-color.org/) and auto-detects terminal output.
const (
	ansiReset     = "\033[0m"
	ansiCyan      = "\033[36m"    // INFO — blue axis, universally visible
	ansiBoldGreen = "\033[1;32m"  // OK   — convention + bold brightness for CVD
	ansiYellow    = "\033[33m"    // WARN — yellow axis, safe for common CVD
	ansiBoldRed   = "\033[1;31m"  // ERR  — convention + bold brightness for CVD
	ansiGray        = "\033[90m"    // DBUG — brightness-only, no hue dependency
	ansiInvertGreen = "\033[7;32m" // SEND banner — CVD-safe green inversion
	ansiInvertCyan  = "\033[7;36m" // RECV banner — CVD-safe cyan inversion
)

// Logger provides structured, timestamped log output.
// All methods are safe for concurrent use.
type Logger struct {
	out         io.Writer
	mu          sync.Mutex
	extraWriter io.Writer
	verbose     bool
	noColor     bool
}

// NewLogger creates a Logger that writes human-readable progress to out.
// If out is nil, it defaults to io.Discard.
// Color is enabled by default when writing to a terminal and NO_COLOR
// environment variable is not set.
func NewLogger(out io.Writer, verbose bool) *Logger {
	if out == nil {
		out = io.Discard
	}
	nc := os.Getenv("NO_COLOR") != "" || !isTerminal(out)
	return &Logger{out: out, verbose: verbose, noColor: nc}
}

// isTerminal returns true if w is connected to a terminal (character device).
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

// SetNoColor explicitly enables or disables color output.
// This overrides the auto-detection performed at construction time.
func (l *Logger) SetNoColor(v bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.noColor = v
}

func (l *Logger) colorPrefix(prefix, color string) string {
	if l.noColor {
		return prefix
	}
	return color + prefix + ansiReset
}

func (l *Logger) logLine(prefix, color, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("15:04:05")
	coloredPrefix := l.colorPrefix(prefix, color)
	line := fmt.Sprintf("[%s] %s %s\n", ts, coloredPrefix, msg)

	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Fprint(l.out, line)
	if l.extraWriter != nil {
		// Extra writer (log file) always gets plain text — no ANSI codes.
		plainLine := fmt.Sprintf("[%s] %s %s\n", ts, prefix, msg)
		fmt.Fprint(l.extraWriter, plainLine)
	}
}

// Info prints an informational message.
func (l *Logger) Info(format string, args ...any) {
	l.logLine("INFO", ansiCyan, format, args...)
}

// OK prints a success message.
func (l *Logger) OK(format string, args ...any) {
	l.logLine(" OK ", ansiBoldGreen, format, args...)
}

// Warn prints a warning message.
func (l *Logger) Warn(format string, args ...any) {
	l.logLine("WARN", ansiYellow, format, args...)
}

// Error prints an error message.
func (l *Logger) Error(format string, args ...any) {
	l.logLine(" ERR", ansiBoldRed, format, args...)
}

// Debug prints a debug message only when verbose mode is enabled.
func (l *Logger) Debug(format string, args ...any) {
	if l.verbose {
		l.logLine("DBUG", ansiGray, format, args...)
	}
}

// Writer returns the underlying io.Writer.
func (l *Logger) Writer() io.Writer { return l.out }

// SetExtraWriter sets an additional writer for dual-write logging.
// The caller is responsible for closing the writer when done.
// Extra writer always receives plain text (no ANSI color codes).
func (l *Logger) SetExtraWriter(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.extraWriter = w
}

// Banner prints an inverted-color D-Mail intent line to stderr.
// It satisfies domain.BannerLogger.
func (l *Logger) Banner(dir domain.BannerDirection, kind, name, description string) {
	desc := description
	if len(desc) > 50 {
		desc = desc[:47] + "..."
	}

	var arrow, label, color, plainArrow string
	switch dir {
	case domain.BannerSend:
		arrow = "▶"
		label = "D-MAIL SEND"
		color = ansiInvertGreen
		plainArrow = ">>>"
	default:
		arrow = "◀"
		label = "D-MAIL RECV"
		color = ansiInvertCyan
		plainArrow = "<<<"
	}

	ts := time.Now().Format("15:04:05")
	content := fmt.Sprintf("%s %s %s (%s) %q", arrow, label, kind, name, desc)
	plainContent := fmt.Sprintf("%s %s %s (%s) %q", plainArrow, label, kind, name, desc)

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.noColor {
		fmt.Fprintf(l.out, "[%s] %s\n", ts, plainContent)
	} else {
		fmt.Fprintf(l.out, "%s[%s] %s%s\n", color, ts, content, ansiReset)
	}
	if l.extraWriter != nil {
		fmt.Fprintf(l.extraWriter, "[%s] %s\n", ts, plainContent)
	}
}
