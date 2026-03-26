package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// WriteClaudeLog persists raw stream-json events to .run/claude-logs/.
func WriteClaudeLog(repoDir string, rawEvents []string) error {
	if len(rawEvents) == 0 {
		return nil
	}
	logDir := filepath.Join(repoDir, domain.StateDir, ".run", "claude-logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("mkdir claude-logs: %w", err)
	}
	filename := fmt.Sprintf("%s.jsonl", time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join(logDir, filename)
	var buf strings.Builder
	for _, event := range rawEvents {
		buf.WriteString(event)
		buf.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(buf.String()), 0o644)
}
