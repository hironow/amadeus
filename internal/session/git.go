package session

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	amadeus "github.com/hironow/amadeus"
)

// GitClient implements amadeus.Git using subprocess execution.
type GitClient struct {
	Dir string
}

// Compile-time check that GitClient implements amadeus.Git.
var _ amadeus.Git = (*GitClient)(nil)

func NewGitClient(dir string) *GitClient {
	return &GitClient{Dir: dir}
}

func (g *GitClient) CurrentCommit() (string, error) {
	out, err := g.run("rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

var (
	// Standard merge commit: "Merge pull request #42 from user/branch"
	prMergePattern = regexp.MustCompile(`Merge pull request #(\d+)`)
	// Squash merge: "feat: add something (#123)"
	prSquashPattern = regexp.MustCompile(`\(#(\d+)\)`)
)

// parseMergedPRs extracts PR numbers from git log output.
// It detects both standard merge commits and squash merges,
// deduplicating when a single line matches both patterns.
func parseMergedPRs(log string) []amadeus.MergedPR {
	trimmed := strings.TrimSpace(log)
	if trimmed == "" {
		return nil
	}
	var prs []amadeus.MergedPR
	for _, line := range strings.Split(trimmed, "\n") {
		seen := make(map[string]bool)
		// Try standard merge commit pattern first
		if m := prMergePattern.FindStringSubmatch(line); len(m) >= 2 {
			num := "#" + m[1]
			seen[num] = true
			prs = append(prs, amadeus.MergedPR{Number: num, Title: line})
		}
		// Try squash merge pattern
		if m := prSquashPattern.FindStringSubmatch(line); len(m) >= 2 {
			num := "#" + m[1]
			if !seen[num] {
				prs = append(prs, amadeus.MergedPR{Number: num, Title: line})
			}
		}
	}
	return prs
}

func (g *GitClient) MergedPRsSince(since string) ([]amadeus.MergedPR, error) {
	out, err := g.run("log", "--first-parent", fmt.Sprintf("%s..HEAD", since), "--oneline")
	if err != nil {
		return nil, err
	}
	return parseMergedPRs(out), nil
}

func (g *GitClient) DiffSince(since string) (string, error) {
	return g.run("diff", since+"..HEAD")
}

func (g *GitClient) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), nil
}
