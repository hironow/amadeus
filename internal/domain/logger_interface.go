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
	Header(toolName, version string)
	Section(title string)
}

// LogBanner calls Banner if the logger supports it (type assertion).
// Safe to call with any Logger including NopLogger.
func LogBanner(logger Logger, dir BannerDirection, kind, name, description string) { // nosemgrep: domain-primitives.multiple-string-params-go — kind/name/description are semantically distinct display fields [permanent]
	if bl, ok := logger.(BannerLogger); ok {
		bl.Banner(dir, kind, name, description)
	}
}

// LogHeader calls Header if the logger supports it (type assertion).
// Prints a single-line startup header with tool name and version.
func LogHeader(logger Logger, toolName, version string) {
	if bl, ok := logger.(BannerLogger); ok {
		bl.Header(toolName, version)
	}
}

// LogSection calls Section if the logger supports it (type assertion).
// Prints a single-line section separator for phase transitions.
func LogSection(logger Logger, title string) {
	if bl, ok := logger.(BannerLogger); ok {
		bl.Section(title)
	}
}
