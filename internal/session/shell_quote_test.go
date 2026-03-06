package session_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/session"
)

func TestShellQuoteUnix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "'hello'"},
		{"it's", "'it'\\''s'"},
		{"normal text", "'normal text'"},
		{"", "''"},
		{"$(rm -rf /)", "'$(rm -rf /)'"},
		{`has "double" quotes`, `'has "double" quotes'`},
		{`"; rm -rf /`, `'"; rm -rf /'`},
	}
	for _, tt := range tests {
		got := session.ShellQuoteUnix(tt.input)
		if got != tt.want {
			t.Errorf("ShellQuoteUnix(%q): got %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestShellQuoteCmd(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", `"hello"`},
		{`say "hi"`, `"say ""hi"""`},
		{"normal text", `"normal text"`},
		{"", `""`},
		{"100%", `"100%%"`},
		{`"quoted" & piped`, `"""quoted"" & piped"`},
	}
	for _, tt := range tests {
		got := session.ShellQuoteCmd(tt.input)
		if got != tt.want {
			t.Errorf("ShellQuoteCmd(%q): got %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestShellQuote_DelegatesToPlatformFunction(t *testing.T) {
	// given
	input := "test value"

	// when
	got := session.ShellQuote(input)

	// then — on non-windows, should delegate to ShellQuoteUnix
	want := session.ShellQuoteUnix(input)
	if got != want {
		t.Errorf("ShellQuote(%q) = %q, want %q (same as ShellQuoteUnix)", input, got, want)
	}
}
