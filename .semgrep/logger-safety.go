package semgreptest

import "strings"

// --- ruleid: logger-safety-no-newline-join-in-exported ---

// BAD: function returning newline-joined string
func FormatHistory(entries []string) string {
	// ruleid: logger-safety-no-newline-join-in-exported
	return strings.Join(entries, "\n")
}

// BAD: method returning newline-joined string
type Gauge struct{ log []string }

func (g *Gauge) FormatLog() string {
	// ruleid: logger-safety-no-newline-join-in-exported
	return strings.Join(g.log, "\n")
}

// --- ok: logger-safety-no-newline-join-in-exported ---

// OK: using comma separator (single line)
func FormatCSV(entries []string) string {
	// ok: logger-safety-no-newline-join-in-exported
	return strings.Join(entries, ", ")
}

// OK: using space separator
func FormatWords(entries []string) string {
	// ok: logger-safety-no-newline-join-in-exported
	return strings.Join(entries, " ")
}
