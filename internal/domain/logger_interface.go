package domain

// Logger provides structured log output. Implementations must be goroutine-safe.
type Logger interface {
	Info(format string, args ...any)
	OK(format string, args ...any)
	Warn(format string, args ...any)
	Error(format string, args ...any)
	Debug(format string, args ...any)
}

// NopLogger is a no-op logger for testing and quiet mode.
type NopLogger struct{}

func (*NopLogger) Info(string, ...any)  {}
func (*NopLogger) OK(string, ...any)    {}
func (*NopLogger) Warn(string, ...any)  {}
func (*NopLogger) Error(string, ...any) {}
func (*NopLogger) Debug(string, ...any) {}
