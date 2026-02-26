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

var prMergePattern = regexp.MustCompile(`Merge pull request #(\d+)`)

func (g *GitClient) MergedPRsSince(since string) ([]amadeus.MergedPR, error) {
	out, err := g.run("log", fmt.Sprintf("%s..HEAD", since), "--oneline", "--grep=Merge pull request")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	var prs []amadeus.MergedPR
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		matches := prMergePattern.FindStringSubmatch(line)
		if len(matches) >= 2 {
			prs = append(prs, amadeus.MergedPR{
				Number: "#" + matches[1],
				Title:  line,
			})
		}
	}
	return prs, nil
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
