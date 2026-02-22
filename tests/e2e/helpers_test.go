//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// amadeusBin returns the path to the amadeus binary.
// In Docker, it's installed at /usr/local/bin/amadeus.
// Locally, it falls back to PATH lookup.
func amadeusBin() string {
	if _, err := os.Stat("/usr/local/bin/amadeus"); err == nil {
		return "/usr/local/bin/amadeus"
	}
	p, err := exec.LookPath("amadeus")
	if err != nil {
		return "amadeus"
	}
	return p
}

// runCmd executes the amadeus binary with args in the given directory.
// Returns stdout, stderr, and error.
func runCmd(t *testing.T, dir string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(amadeusBin(), args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// runCmdStdin executes amadeus with args and pipes data to stdin.
func runCmdStdin(t *testing.T, dir string, stdin string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(amadeusBin(), args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// initTestRepo creates a temp dir with a git repo and runs `amadeus init`.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Initialize git repo (required for check command)
	gitInit := exec.Command("git", "init")
	gitInit.Dir = dir
	if out, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	// Configure git user for commits
	for _, kv := range [][2]string{
		{"user.name", "E2E Test"},
		{"user.email", "e2e@test.local"},
	} {
		c := exec.Command("git", "config", kv[0], kv[1])
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git config %s: %v\n%s", kv[0], err, out)
		}
	}

	// Create initial commit so HEAD exists
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd := exec.Command("git", "add", ".")
	gitAdd.Dir = dir
	if out, err := gitAdd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	gitCommit := exec.Command("git", "commit", "-m", "initial")
	gitCommit.Dir = dir
	if out, err := gitCommit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	// Run amadeus init
	stdout, stderr, err := runCmd(t, dir, "init")
	if err != nil {
		t.Fatalf("amadeus init: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	return dir
}

// writeConfig writes a custom config.yaml to .gate/.
func writeConfig(t *testing.T, dir string, cfg map[string]any) {
	t.Helper()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, ".gate", "config.yaml")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// defaultTestConfig returns a minimal valid config for E2E tests.
func defaultTestConfig() map[string]any {
	return map[string]any{
		"lang": "en",
		"weights": map[string]any{
			"adr_integrity":        0.35,
			"dod_fulfillment":      0.25,
			"dependency_integrity": 0.25,
			"implicit_constraints": 0.15,
		},
		"thresholds": map[string]any{
			"low_max":    0.25,
			"medium_max": 0.50,
		},
		"per_axis_override": map[string]any{
			"adr_integrity_force_high":          60,
			"dod_fulfillment_force_high":        70,
			"dependency_integrity_force_medium": 80,
		},
		"full_check": map[string]any{
			"interval":           10,
			"on_divergence_jump": 0.15,
		},
		"convergence": map[string]any{
			"window_days": 14,
			"threshold":   3,
		},
	}
}

// writeDMail writes a D-Mail file to the specified subdirectory of .gate/.
func writeDMail(t *testing.T, dir, subdir, name string, fm map[string]any, body string) {
	t.Helper()
	fmData, err := yaml.Marshal(fm)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(fmData)
	buf.WriteString("---\n")
	if body != "" {
		buf.WriteString("\n")
		buf.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			buf.WriteString("\n")
		}
	}
	mailDir := filepath.Join(dir, ".gate", subdir)
	if err := os.MkdirAll(mailDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mailDir, name+".md"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// listDir returns sorted filenames in a directory.
func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// readJSON reads and unmarshals a JSON file.
func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("unmarshal %s: %v\ncontent: %s", path, err, data)
	}
}

// parseJSONOutput unmarshals stdout JSON output.
func parseJSONOutput(t *testing.T, stdout string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(stdout), v); err != nil {
		t.Fatalf("parse JSON output: %v\nraw: %s", err, stdout)
	}
}

// assertFileExists fails if the file does not exist.
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file to exist: %s", path)
	}
}

// assertFileNotExists fails if the file exists.
func assertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected file NOT to exist: %s", path)
	}
}

// assertExitCode checks the exit code of a command error.
func assertExitCode(t *testing.T, err error, expected int) {
	t.Helper()
	if expected == 0 {
		if err != nil {
			t.Fatalf("expected exit code 0, got error: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("expected exit code %d, got 0", expected)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got: %T %v", err, err)
	}
	if exitErr.ExitCode() != expected {
		t.Fatalf("expected exit code %d, got %d", expected, exitErr.ExitCode())
	}
}

// seedDMails writes multiple D-Mail files to archive/.
func seedDMails(t *testing.T, dir string, dmails []seedDMailSpec) {
	t.Helper()
	for _, spec := range dmails {
		fm := map[string]any{
			"name":        spec.Name,
			"kind":        spec.Kind,
			"description": spec.Description,
			"severity":    spec.Severity,
		}
		if len(spec.Issues) > 0 {
			fm["issues"] = spec.Issues
		}
		if len(spec.Targets) > 0 {
			fm["targets"] = spec.Targets
		}
		if spec.Metadata != nil {
			fm["metadata"] = spec.Metadata
		}
		body := spec.Body
		if body == "" {
			body = fmt.Sprintf("Detail for %s.\n", spec.Name)
		}
		writeDMail(t, dir, "archive", spec.Name, fm, body)
	}
}

type seedDMailSpec struct {
	Name        string
	Kind        string
	Description string
	Severity    string
	Issues      []string
	Targets     []string
	Metadata    map[string]string
	Body        string
}
