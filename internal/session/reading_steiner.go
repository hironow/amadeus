package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/port"
)

type ShiftReport struct {
	Significant       bool
	MergedPRs         []domain.MergedPR
	Diff              string
	CodebaseStructure string
}

type ReadingSteiner struct {
	Git port.Git
}

func (rs *ReadingSteiner) DetectShift(sinceCommit string) (ShiftReport, error) {
	prs, err := rs.Git.MergedPRsSince(sinceCommit)
	if err != nil {
		return ShiftReport{}, fmt.Errorf("reading steiner: merged PRs: %w", err)
	}
	diff, err := rs.Git.DiffSince(sinceCommit)
	if err != nil {
		return ShiftReport{}, fmt.Errorf("reading steiner: diff: %w", err)
	}
	significant := len(prs) > 0 || strings.TrimSpace(diff) != ""
	return ShiftReport{
		Significant: significant,
		MergedPRs:   prs,
		Diff:        diff,
	}, nil
}

func (rs *ReadingSteiner) DetectShiftFull(repoRoot string) (ShiftReport, error) {
	structure, err := buildDirectoryTree(repoRoot, 3)
	if err != nil {
		return ShiftReport{}, fmt.Errorf("reading steiner: directory tree: %w", err)
	}
	return ShiftReport{
		Significant:       true,
		CodebaseStructure: structure,
	}, nil
}

func buildDirectoryTree(root string, maxDepth int) (string, error) {
	var sb strings.Builder
	err := walkDir(root, "", maxDepth, 0, &sb)
	return sb.String(), err
}

func walkDir(path, prefix string, maxDepth, depth int, sb *strings.Builder) error {
	if depth >= maxDepth {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		fmt.Fprintf(sb, "%s%s\n", prefix, name)
		if e.IsDir() {
			if err := walkDir(filepath.Join(path, name), prefix+"  ", maxDepth, depth+1, sb); err != nil {
				return err
			}
		}
	}
	return nil
}
