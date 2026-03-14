package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/hironow/amadeus/internal/domain"
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

var (
	issueIDRe = regexp.MustCompile(`[A-Z]+-\d+`)
	dateRe    = regexp.MustCompile(`(\d{4}-\d{2}-\d{2})[T ](\d{2}:\d{2}:\d{2})`)
	statusRe  = regexp.MustCompile(`\*\*Status\*\*:\s*(success|failed|skipped)`)
)

var opFromTool = map[string]string{
	"paintress": "expedition",
	"amadeus":   "divergence",
	"sightjack": "wave",
	"phonewave": "dmail",
}

// ExtractMeta populates a domain.IndexEntry from a markdown file, extracting
// operation type, issue ID, status, timestamp, and summary.
func ExtractMeta(filePath, stateDir, tool string) domain.IndexEntry {
	relPath, err := filepath.Rel(stateDir, filePath)
	if err != nil {
		relPath = filePath
	}

	entry := domain.IndexEntry{
		Tool:    tool,
		Path:    relPath,
		Summary: ExtractSummary(filePath),
	}

	// Determine operation from first path segment.
	firstSeg := relPath
	if idx := strings.IndexByte(relPath, filepath.Separator); idx >= 0 {
		firstSeg = relPath[:idx]
	}
	if idx := strings.IndexByte(firstSeg, '/'); idx >= 0 {
		firstSeg = firstSeg[:idx]
	}
	switch firstSeg {
	case "archive":
		entry.Operation = "dmail"
	default:
		entry.Operation = opFromTool[tool]
	}

	data, readErr := os.ReadFile(filePath)
	if readErr != nil {
		return entry
	}
	content := string(data)

	if m := issueIDRe.FindString(content); m != "" {
		entry.Issue = m
	}

	if m := statusRe.FindStringSubmatch(content); len(m) > 1 {
		entry.Status = m[1]
	} else {
		entry.Status = "unknown"
	}

	entry.Timestamp = extractTimestamp(filePath, content)
	return entry
}

func extractTimestamp(filePath, content string) string {
	lines := strings.Split(content, "\n")
	inFM := false
	for i, line := range lines {
		if i == 0 && strings.TrimSpace(line) == "---" {
			inFM = true
			continue
		}
		if inFM {
			if strings.TrimSpace(line) == "---" {
				break
			}
			trimmed := strings.TrimSpace(line)
			// Priority 1a: updated_at in frontmatter
			if strings.HasPrefix(trimmed, "updated_at:") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "updated_at:"))
				val = strings.Trim(val, "\"'")
				if val != "" {
					return val
				}
			}
			// Priority 1b: date in frontmatter (normalize bare YYYY-MM-DD to RFC3339)
			if strings.HasPrefix(trimmed, "date:") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "date:"))
				val = strings.Trim(val, "\"'")
				if val != "" {
					return normalizeToRFC3339(val)
				}
			}
		}
	}

	// Priority 2: **Date**: line in body
	for _, line := range lines {
		if strings.Contains(line, "**Date**:") {
			if m := dateRe.FindStringSubmatch(line); len(m) > 2 {
				return fmt.Sprintf("%sT%sZ", m[1], m[2])
			}
		}
	}

	// Priority 3: date in filename
	dateOnlyRe := regexp.MustCompile(`(\d{4}-\d{2}-\d{2})`)
	fname := filepath.Base(filePath)
	if m := dateOnlyRe.FindStringSubmatch(fname); len(m) > 1 {
		return fmt.Sprintf("%sT00:00:00Z", m[1])
	}

	// Priority 4: mtime
	info, statErr := os.Stat(filePath)
	if statErr != nil {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return info.ModTime().UTC().Format(time.RFC3339)
}

func normalizeToRFC3339(s string) string {
	dateOnlyRe := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	if dateOnlyRe.MatchString(s) {
		return s + "T00:00:00Z"
	}
	return s
}

// IndexWriter writes domain.IndexEntry records to a JSONL index file.
type IndexWriter struct{}

// Append appends entries to the index file using flock for concurrency safety.
func (w *IndexWriter) Append(indexPath string, entries []domain.IndexEntry) error {
	if len(entries) == 0 {
		return nil
	}

	var buf []byte
	for _, e := range entries {
		line, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("marshal index entry: %w", err)
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}

	lockPath := indexPath + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("open lock: %w", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("flock: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	f, err := os.OpenFile(indexPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open index: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(buf); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	return nil
}

var indexDirs = []string{"archive", "journal", "journals", "insights"}

// Rebuild scans known subdirectories for .md files, extracts metadata, and
// overwrites the index file atomically using a temp+rename strategy under flock.
func (w *IndexWriter) Rebuild(indexPath, stateDir, tool string) (int, error) {
	indexDir := filepath.Dir(indexPath)
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return 0, fmt.Errorf("mkdir index dir: %w", err)
	}

	lockPath := indexPath + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return 0, fmt.Errorf("open lock: %w", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return 0, fmt.Errorf("flock: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	var entries []domain.IndexEntry
	for _, sub := range indexDirs {
		dir := filepath.Join(stateDir, sub)
		if _, statErr := os.Stat(dir); statErr != nil {
			continue
		}
		filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".md" {
				return nil
			}
			entries = append(entries, ExtractMeta(path, stateDir, tool))
			return nil
		})
	}

	tmpPath := indexPath + ".tmp"
	var buf []byte
	for _, e := range entries {
		line, marshalErr := json.Marshal(e)
		if marshalErr != nil {
			return 0, fmt.Errorf("marshal: %w", marshalErr)
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}

	if err := os.WriteFile(tmpPath, buf, 0644); err != nil {
		return 0, fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmpPath, indexPath); err != nil {
		return 0, fmt.Errorf("rename: %w", err)
	}

	return len(entries), nil
}
