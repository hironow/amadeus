package amadeus

import (
	"fmt"
	"io"
)

type Logger struct {
	out     io.Writer
	verbose bool
}

func NewLogger(out io.Writer, verbose bool) *Logger {
	return &Logger{out: out, verbose: verbose}
}

func (l *Logger) Info(format string, args ...any) {
	fmt.Fprintf(l.out, "  "+format+"\n", args...)
}

func (l *Logger) Warn(format string, args ...any) {
	fmt.Fprintf(l.out, "  [WARN] "+format+"\n", args...)
}

func (l *Logger) Error(format string, args ...any) {
	fmt.Fprintf(l.out, "  [ERROR] "+format+"\n", args...)
}

func (l *Logger) Debug(format string, args ...any) {
	if l.verbose {
		fmt.Fprintf(l.out, "  [debug] "+format+"\n", args...)
	}
}

func (l *Logger) OK(format string, args ...any) {
	fmt.Fprintf(l.out, "  [OK] "+format+"\n", args...)
}

// Writer returns the underlying io.Writer.
func (l *Logger) Writer() io.Writer {
	return l.out
}
