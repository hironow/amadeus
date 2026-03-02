//go:build scenario

package scenario_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var binDir string

func TestMain(m *testing.M) {
	var err error
	binDir, err = buildAllBinaries()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build binaries: %v\n", err)
		os.Exit(1)
	}
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)
	defer os.Setenv("PATH", origPath)

	code := m.Run()
	os.RemoveAll(binDir)
	os.Exit(code)
}

func buildAllBinaries() (string, error) {
	dir, err := os.MkdirTemp("", "scenario-bin-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	tools := []struct {
		name    string
		envKey  string
		cmdPath string // relative to repo root
	}{
		{"phonewave", "PHONEWAVE_REPO", "cmd/phonewave"},
		{"sightjack", "SIGHTJACK_REPO", "cmd/sightjack"},
		{"paintress", "PAINTRESS_REPO", "cmd/paintress"},
		{"amadeus", "AMADEUS_REPO", "cmd/amadeus"},
	}

	for _, tool := range tools {
		repo := repoPath(tool.envKey, tool.name)
		src := filepath.Join(repo, tool.cmdPath)
		dst := filepath.Join(dir, tool.name)
		cmd := exec.Command("go", "build", "-o", dst, "./"+tool.cmdPath)
		cmd.Dir = repo

		if out, err := cmd.CombinedOutput(); err != nil {
			return dir, fmt.Errorf("build %s: %w\n%s", tool.name, err, out)
		}
		_ = src // used for documentation
	}

	// Build fake-claude as "claude"
	here, _ := os.Getwd()
	fakeClaude := filepath.Join(here, "testdata", "fake-claude")
	cmd := exec.Command("go", "build", "-o", filepath.Join(dir, "claude"), ".")
	cmd.Dir = fakeClaude
	if out, err := cmd.CombinedOutput(); err != nil {
		return dir, fmt.Errorf("build fake-claude: %w\n%s", err, out)
	}

	// Build fake-gh as "gh"
	fakeGH := filepath.Join(here, "testdata", "fake-gh")
	cmd = exec.Command("go", "build", "-o", filepath.Join(dir, "gh"), ".")
	cmd.Dir = fakeGH
	if out, err := cmd.CombinedOutput(); err != nil {
		return dir, fmt.Errorf("build fake-gh: %w\n%s", err, out)
	}

	return dir, nil
}

func repoPath(envKey, name string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	// go test sets cwd to package dir: /Users/nino/tap/amadeus/tests/scenario
	// Dir(1): /Users/nino/tap/amadeus/tests
	// Dir(2): /Users/nino/tap/amadeus
	// Dir(3): /Users/nino/tap              <- sibling repos live here
	here, _ := os.Getwd()
	tapRoot := filepath.Dir(filepath.Dir(filepath.Dir(here)))
	return filepath.Join(tapRoot, name)
}
