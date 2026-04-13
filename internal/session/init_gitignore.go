package session

import (
	"os"
	"strings"
)

// EnsureGitignoreEntries reads an existing .gitignore (if any) and appends
// any missing entries from required. Creates the file with all entries if it
// does not exist. Existing user entries are preserved (append-only pattern).
func EnsureGitignoreEntries(path string, required []string) error {
	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}

	var missing []string
	for _, entry := range required {
		if !strings.Contains(existing, entry) {
			missing = append(missing, entry)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	// Ensure trailing newline before appending
	sep := ""
	if len(existing) > 0 && !strings.HasSuffix(existing, "\n") {
		sep = "\n"
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	content := sep + strings.Join(missing, "\n") + "\n"
	_, err = f.WriteString(content)
	return err
}
