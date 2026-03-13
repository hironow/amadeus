package platform

import (
	"fmt"
	"strings"
)

// IsNDJSON returns true if the first non-empty line of s starts with '{',
// indicating the content is likely NDJSON (newline-delimited JSON).
func IsNDJSON(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		return strings.HasPrefix(trimmed, "{")
	}
	return false
}

// SummarizeNDJSON returns a human-readable summary indicating the number of
// non-empty lines in the NDJSON content, directing the user to --verbose.
func SummarizeNDJSON(s string) string {
	count := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return fmt.Sprintf("(%d lines of stream-json output, use --verbose to see)", count)
}
