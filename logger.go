package amadeus

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// Logger provides structured, timestamped log output.
// All methods are safe for concurrent use.
type Logger struct {
	out         io.Writer
	mu          sync.Mutex
	extraWriter io.Writer
	verbose     bool
}

// NewLogger creates a Logger that writes human-readable progress to out.
// If out is nil, it defaults to io.Discard.
func NewLogger(out io.Writer, verbose bool) *Logger {
	if out == nil {
		out = io.Discard
	}
	return &Logger{out: out, verbose: verbose}
}

func (l *Logger) logLine(prefix, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s %s\n", ts, prefix, msg)

	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Fprint(l.out, line)
	if l.extraWriter != nil {
		fmt.Fprint(l.extraWriter, line)
	}
}

// Info prints an informational message.
func (l *Logger) Info(format string, args ...any) { l.logLine("INFO", format, args...) }

// OK prints a success message.
func (l *Logger) OK(format string, args ...any) { l.logLine(" OK ", format, args...) }

// Warn prints a warning message.
func (l *Logger) Warn(format string, args ...any) { l.logLine("WARN", format, args...) }

// Error prints an error message.
func (l *Logger) Error(format string, args ...any) { l.logLine(" ERR", format, args...) }

// Debug prints a debug message only when verbose mode is enabled.
func (l *Logger) Debug(format string, args ...any) {
	if l.verbose {
		l.logLine("DBUG", format, args...)
	}
}

// Writer returns the underlying io.Writer.
func (l *Logger) Writer() io.Writer { return l.out }

// SetExtraWriter sets an additional writer for dual-write logging.
// The caller is responsible for closing the writer when done.
func (l *Logger) SetExtraWriter(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.extraWriter = w
}
