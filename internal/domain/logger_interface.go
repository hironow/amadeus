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

// BannerDirection indicates the direction of a D-Mail intent log.
type BannerDirection int

const (
	BannerSend BannerDirection = iota
	BannerRecv
)

// BannerLogger is an optional extension for loggers that support
// inverted-color banner lines for D-Mail intent logging.
type BannerLogger interface {
	Banner(dir BannerDirection, kind, name, description string)
}

// LogBanner calls Banner if the logger supports it (type assertion).
// Safe to call with any Logger including NopLogger.
func LogBanner(logger Logger, dir BannerDirection, kind, name, description string) {
	if bl, ok := logger.(BannerLogger); ok {
		bl.Banner(dir, kind, name, description)
	}
}
