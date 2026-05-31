//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gopkg.in/yaml.v3"
)

// buildTestContainer starts an amadeus test container once.
func buildTestContainer(t *testing.T, ctx context.Context) testcontainers.Container {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image: sharedImage,
		Cmd:   []string{"sleep", "infinity"},
		WaitingFor: wait.ForExec([]string{"amadeus", "--version"}).
			WithStartupTimeout(10 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("buildTestContainer: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(ctx); err != nil {
			t.Errorf("terminate container: %v", err)
		}
	})
	return c
}

// execInContainer executes a command inside the test container and returns stdout.
func execInContainer(t *testing.T, ctx context.Context, c testcontainers.Container, cmd []string) string {
	t.Helper()
	code, stdout, stderr := execInContainerWithExitCode(t, ctx, c, cmd)
	if code != 0 {
		t.Fatalf("exec %v failed with code %d\nstdout: %s\nstderr: %s", cmd, code, stdout, stderr)
	}
	return stdout
}

// execInContainerWithExitCode executes a command inside the test container and returns (exitCode, stdout, stderr).
func execInContainerWithExitCode(t *testing.T, ctx context.Context, c testcontainers.Container, cmd []string) (int, string, string) {
	t.Helper()
	code, stdoutReader, err := c.Exec(ctx, cmd)
	if err != nil {
		t.Fatalf("container exec failed: %v", err)
	}
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(stdoutReader)
	return code, buf.String(), ""
}

// heredocWrite writes file content inside the container.
func heredocWrite(t *testing.T, ctx context.Context, c testcontainers.Container, path, content string) {
	t.Helper()
	cmd := []string{"sh", "-c", fmt.Sprintf("cat << 'EOF' > %s\n%s\nEOF", path, content)}
	execInContainer(t, ctx, c, cmd)
}

// runCmd executes amadeus inside the test container.
func runCmd(t *testing.T, ctx context.Context, c testcontainers.Container, dir string, args ...string) (string, string, error) {
	t.Helper()
	fullCmd := []string{"sh", "-c", fmt.Sprintf("cd %s && /usr/local/bin/amadeus %s", dir, strings.Join(args, " "))}
	code, stdout, _ := execInContainerWithExitCode(t, ctx, c, fullCmd)
	var err error
	if code != 0 {
		err = fmt.Errorf("exit code %d", code)
	}
	return stdout, "", err
}

// runCmdStdin executes amadeus inside the test container, piping data to stdin.
func runCmdStdin(t *testing.T, ctx context.Context, c testcontainers.Container, dir, stdin string, args ...string) (string, string, error) {
	t.Helper()
	fullCmd := []string{"sh", "-c", fmt.Sprintf("cat << 'EOF' | (cd %s && /usr/local/bin/amadeus %s)\n%s\nEOF", dir, strings.Join(args, " "), stdin)}
	code, stdout, _ := execInContainerWithExitCode(t, ctx, c, fullCmd)
	var err error
	if code != 0 {
		err = fmt.Errorf("exit code %d", code)
	}
	return stdout, "", err
}

// fileExistsInContainer checks if a file exists inside the container.
func fileExistsInContainer(t *testing.T, ctx context.Context, c testcontainers.Container, path string) bool {
	t.Helper()
	code, _, _ := execInContainerWithExitCode(t, ctx, c, []string{"test", "-f", path})
	return code == 0
}

// dirExistsInContainer checks if a directory exists inside the container.
func dirExistsInContainer(t *testing.T, ctx context.Context, c testcontainers.Container, path string) bool {
	t.Helper()
	code, _, _ := execInContainerWithExitCode(t, ctx, c, []string{"test", "-d", path})
	return code == 0
}

// initTestRepo creates a workspace inside the container, git init, and runs `amadeus init`.
func initTestRepo(t *testing.T, ctx context.Context, c testcontainers.Container, dir string) {
	t.Helper()
	execInContainer(t, ctx, c, []string{"mkdir", "-p", dir})
	execInContainer(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("cd %s && git init --initial-branch=main", dir)})
	execInContainer(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("cd %s && git config user.name 'E2E Test' && git config user.email 'e2e@test.local'", dir)})
	execInContainer(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("cd %s && echo '# test' > README.md && git add . && git commit -m 'initial'", dir)})
	execInContainer(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("cd %s && amadeus init", dir)})
}

// writeConfig writes a custom config.yaml to .gate/ inside the container.
func writeConfig(t *testing.T, ctx context.Context, c testcontainers.Container, dir string, cfg map[string]any) {
	t.Helper()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	configPath := fmt.Sprintf("%s/.gate/config.yaml", dir)
	heredocWrite(t, ctx, c, configPath, string(data))
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

// writeDMail writes a D-Mail file to the specified subdirectory of .gate/ inside the container.
func writeDMail(t *testing.T, ctx context.Context, c testcontainers.Container, dir, subdir, name string, fm map[string]any, body string) {
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
	mailDir := fmt.Sprintf("%s/.gate/%s", dir, subdir)
	execInContainer(t, ctx, c, []string{"mkdir", "-p", mailDir})
	
	path := fmt.Sprintf("%s/%s.md", mailDir, name)
	heredocWrite(t, ctx, c, path, buf.String())
}

// listDirInContainer returns sorted filenames in a directory in the container.
func listDirInContainer(t *testing.T, ctx context.Context, c testcontainers.Container, dir string) []string {
	t.Helper()
	code, stdout, _ := execInContainerWithExitCode(t, ctx, c, []string{"ls", "-1", dir})
	if code != 0 {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	var names []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			names = append(names, l)
		}
	}
	sort.Strings(names)
	return names
}

// readJSONFromContainer reads and unmarshals a JSON file inside the container.
func readJSONFromContainer(t *testing.T, ctx context.Context, c testcontainers.Container, path string, v any) {
	t.Helper()
	stdout := execInContainer(t, ctx, c, []string{"cat", path})
	if err := json.Unmarshal([]byte(stdout), v); err != nil {
		t.Fatalf("unmarshal %s: %v\ncontent: %s", path, err, stdout)
	}
}

// parseJSONOutput parses JSON.
func parseJSONOutput(t *testing.T, stdout string, v any) {
	t.Helper()
	start := strings.Index(stdout, "{")
	if start < 0 {
		t.Fatalf("no JSON object found: %s", stdout)
	}
	end := strings.LastIndex(stdout, "}")
	if end < 0 || end < start {
		t.Fatalf("no closing JSON brace found: %s", stdout)
	}
	jsonStr := stdout[start : end+1]
	if err := json.Unmarshal([]byte(jsonStr), v); err != nil {
		t.Fatalf("parse JSON: %v\nraw: %s", err, jsonStr)
	}
}

// countEventsOfTypeInContainer counts events matching the given type in .gate/events/ JSONL files in the container.
func countEventsOfTypeInContainer(t *testing.T, ctx context.Context, c testcontainers.Container, dir, eventType string) int {
	t.Helper()
	eventsDir := fmt.Sprintf("%s/.gate/events", dir)
	names := listDirInContainer(t, ctx, c, eventsDir)
	count := 0
	for _, name := range names {
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		path := fmt.Sprintf("%s/%s", eventsDir, name)
		data := execInContainer(t, ctx, c, []string{"cat", path})
		for _, line := range strings.Split(data, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var ev struct {
				Type string `json:"type"`
			}
			if jsonErr := json.Unmarshal([]byte(line), &ev); jsonErr == nil && ev.Type == eventType {
				count++
			}
		}
	}
	return count
}

// seedDMails writes multiple D-Mail files to archive/ inside the container.
func seedDMails(t *testing.T, ctx context.Context, c testcontainers.Container, dir string, dmails []seedDMailSpec) {
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
		writeDMail(t, ctx, c, dir, "archive", spec.Name, fm, body)
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
