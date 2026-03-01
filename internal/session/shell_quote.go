package session

import (
	"runtime"
	"strings"
)

// ShellQuoteUnix wraps a string in single quotes with proper escaping for sh.
// Single quotes within the string are escaped by ending the current quote,
// inserting an escaped single quote, and reopening the quote.
func ShellQuoteUnix(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// ShellQuoteCmd wraps a string in double quotes with proper escaping for cmd.exe.
// Double quotes are escaped as "" and percent signs as %%.
func ShellQuoteCmd(s string) string {
	s = strings.ReplaceAll(s, `"`, `""`)
	s = strings.ReplaceAll(s, `%`, `%%`)
	return `"` + s + `"`
}

// ShellQuote quotes s for safe interpolation into shell commands.
// Uses single-quote escaping on Unix and double-quote escaping on Windows.
func ShellQuote(s string) string {
	if runtime.GOOS == "windows" {
		return ShellQuoteCmd(s)
	}
	return ShellQuoteUnix(s)
}
