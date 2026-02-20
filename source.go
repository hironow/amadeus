package amadeus

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// collectMarkdownFiles reads all .md files from dir, sorted by name,
// and returns them concatenated with filename headers.
// Returns ("", nil) if the directory does not exist.
func collectMarkdownFiles(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	var mdFiles []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		mdFiles = append(mdFiles, e.Name())
	}
	sort.Strings(mdFiles)

	if len(mdFiles) == 0 {
		return "", nil
	}

	var sb strings.Builder
	for _, name := range mdFiles {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", fmt.Errorf("read %s: %w", name, err)
		}
		fmt.Fprintf(&sb, "### %s\n%s\n\n", name, strings.TrimRight(string(data), "\n"))
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

// CollectADRs reads all .md files from {repoRoot}/docs/adr/
// and returns them concatenated with filename headers.
// Returns ("", nil) if the directory does not exist.
func CollectADRs(repoRoot string) (string, error) {
	return collectMarkdownFiles(filepath.Join(repoRoot, "docs", "adr"))
}

// CollectDoDs reads all .md files from {repoRoot}/docs/dod/
// and returns them concatenated with filename headers.
// Returns ("", nil) if the directory does not exist.
func CollectDoDs(repoRoot string) (string, error) {
	return collectMarkdownFiles(filepath.Join(repoRoot, "docs", "dod"))
}

// CollectDependencyMap reads {repoRoot}/go.mod and returns its content.
// Returns ("", nil) if the file does not exist or is empty.
func CollectDependencyMap(repoRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", nil
	}
	return content, nil
}
