package session

import (
	"bufio"
	"os"
	"strings"
)

const maxSummaryLen = 100

// ExtractSummary returns the first markdown heading (# ...) found after
// skipping optional YAML frontmatter. If no heading is found, it returns the
// first non-empty line. The result is truncated to maxSummaryLen characters.
func ExtractSummary(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inFrontmatter := false
	firstLine := true
	var fallback string

	for scanner.Scan() {
		line := scanner.Text()
		if firstLine && strings.TrimSpace(line) == "---" {
			inFrontmatter = true
			firstLine = false
			continue
		}
		firstLine = false
		if inFrontmatter {
			if strings.TrimSpace(line) == "---" {
				inFrontmatter = false
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			return truncate(strings.TrimPrefix(trimmed, "# "), maxSummaryLen)
		}
		if fallback == "" {
			fallback = trimmed
		}
	}
	return truncate(fallback, maxSummaryLen)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
