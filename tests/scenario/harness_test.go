//go:build scenario

package scenario_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// Workspace represents a temporary directory with all 4 tool state dirs initialized.
type Workspace struct {
	Root     string // t.TempDir()
	RepoPath string // workspace 内の simulated repo
	BinDir   string
	Env      []string
}

// ToolProcess wraps a running tool process.
type ToolProcess struct {
	Cmd    *exec.Cmd
	Cancel context.CancelFunc
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
}

// NewWorkspace creates and initializes a Workspace for scenario tests.
// It creates a temporary directory, initializes a git repo, runs each tool's
// init command, and verifies phonewave's route derivation.
func NewWorkspace(t *testing.T, level string) *Workspace {
	t.Helper()

	root := t.TempDir()
	repoPath := filepath.Join(root, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}

	// Initialize a git repo (tools require a git repository)
	runCmd(t, repoPath, "git", "init")
	runCmd(t, repoPath, "git", "config", "user.email", "test@scenario.test")
	runCmd(t, repoPath, "git", "config", "user.name", "Scenario Test")
	runCmd(t, repoPath, "git", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "init")

	// Resolve testdata/fixtures directory
	here, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	fixtureDir := filepath.Join(here, "testdata", "fixtures", level)

	// Create prompt log directory
	promptLogDir := filepath.Join(root, "prompt-logs")
	if err := os.MkdirAll(promptLogDir, 0o755); err != nil {
		t.Fatalf("create prompt-log dir: %v", err)
	}

	w := &Workspace{
		Root:     root,
		RepoPath: repoPath,
		BinDir:   binDir,
		Env: []string{
			"FAKE_CLAUDE_FIXTURE_SET=" + level,
			"FAKE_CLAUDE_FIXTURE_DIR=" + fixtureDir,
			"FAKE_CLAUDE_PROMPT_LOG_DIR=" + promptLogDir,
		},
	}

	// Initialize each tool in the repo
	w.initSightjack(t)
	w.initPaintress(t)
	w.initAmadeus(t)
	w.initPhonewave(t)

	// Override tool configs to ensure claude commands point to fake-claude.
	w.overrideSightjackClaudeCommand(t)
	w.overrideAmadeusClaudeCommand(t)

	// Verify phonewave route derivation
	w.verifyPhonewaveRoutes(t)

	return w
}

// initSightjack runs sightjack init with default flags.
func (w *Workspace) initSightjack(t *testing.T) {
	t.Helper()
	cmd := w.runToolCmd(context.Background(), "sightjack", "init", w.RepoPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sightjack init failed: %v\n%s", err, out)
	}
}

// initPaintress runs paintress init with --team and --project flags.
func (w *Workspace) initPaintress(t *testing.T) {
	t.Helper()
	cmd := w.runToolCmd(context.Background(), "paintress", "init", "--team", "TEST", "--project", "TEST", w.RepoPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("paintress init failed: %v\n%s", err, out)
	}
}

// initAmadeus runs amadeus init.
func (w *Workspace) initAmadeus(t *testing.T) {
	t.Helper()
	cmd := w.runToolCmd(context.Background(), "amadeus", "init", w.RepoPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("amadeus init failed: %v\n%s", err, out)
	}
}

// initPhonewave runs phonewave init, pointing to the repo path.
// The config is written to the workspace root (not inside the repo).
func (w *Workspace) initPhonewave(t *testing.T) {
	t.Helper()
	cfgPath := filepath.Join(w.Root, "phonewave.yaml")
	cmd := w.runToolCmd(context.Background(), "phonewave", "init", "--config", cfgPath, w.RepoPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("phonewave init failed: %v\n%s", err, out)
	}
}

// overrideSightjackClaudeCommand reads .siren/config.yaml and ensures the
// claude.command field is set to "claude" (the fake-claude binary in PATH).
// The init config omits the claude section, so DefaultConfig fills it with
// "claude" already, but we write it explicitly to be safe.
func (w *Workspace) overrideSightjackClaudeCommand(t *testing.T) {
	t.Helper()
	cfgPath := filepath.Join(w.RepoPath, ".siren", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read sightjack config: %v", err)
	}

	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse sightjack config: %v", err)
	}

	// Ensure claude section exists with command = "claude"
	claudeSection, ok := cfg["claude"].(map[string]any)
	if !ok {
		claudeSection = make(map[string]any)
	}
	claudeSection["command"] = "claude"
	cfg["claude"] = claudeSection

	out, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal sightjack config: %v", err)
	}
	if err := os.WriteFile(cfgPath, out, 0o644); err != nil {
		t.Fatalf("write sightjack config: %v", err)
	}
}

// overrideAmadeusClaudeCommand is a no-op verification that amadeus can find
// the claude binary. Unlike sightjack, amadeus resolves claude via PATH
// (hardcoded "claude" in defaultClaudeRunner.Run), not via config.yaml.
// TestMain prepends the binDir (containing our fake-claude) to PATH, so no
// config modification is needed. This function verifies that invariant.
func (w *Workspace) overrideAmadeusClaudeCommand(t *testing.T) {
	t.Helper()
	// Verify .gate/config.yaml exists (amadeus init succeeded).
	cfgPath := filepath.Join(w.RepoPath, ".gate", "config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("amadeus config not found at %s: %v", cfgPath, err)
	}
	// No modification needed: amadeus discovers claude via PATH,
	// which TestMain configures to include the fake-claude binary.
}

// phonewaveConfigPath returns the path to the phonewave.yaml config file.
func (w *Workspace) phonewaveConfigPath() string {
	return filepath.Join(w.Root, "phonewave.yaml")
}

// verifyPhonewaveRoutes reads phonewave.yaml and verifies that routes exist
// for specification, report, and feedback kinds.
func (w *Workspace) verifyPhonewaveRoutes(t *testing.T) {
	t.Helper()
	cfgPath := w.phonewaveConfigPath()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read phonewave.yaml: %v", err)
	}

	// Parse the routes section
	var cfg struct {
		Routes []struct {
			Kind string   `yaml:"kind"`
			From string   `yaml:"from"`
			To   []string `yaml:"to"`
		} `yaml:"routes"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse phonewave.yaml: %v", err)
	}

	requiredKinds := map[string]bool{
		"specification":          false,
		"report":                 false,
		"design-feedback":       false,
		"implementation-feedback": false,
	}
	for _, route := range cfg.Routes {
		if _, ok := requiredKinds[route.Kind]; ok {
			requiredKinds[route.Kind] = true
		}
	}

	for kind, found := range requiredKinds {
		if !found {
			t.Fatalf("phonewave.yaml missing required route kind: %s\nconfig content:\n%s", kind, string(data))
		}
	}
}

// --- Tool Lifecycle Helpers ---

// StartPhonewave starts the phonewave daemon in background.
// Returns a ToolProcess that can be stopped with StopPhonewave.
func (w *Workspace) StartPhonewave(t *testing.T, ctx context.Context) *ToolProcess {
	t.Helper()
	daemonCtx, cancel := context.WithCancel(ctx)

	cfgPath := w.phonewaveConfigPath()
	cmd := w.runToolCmd(daemonCtx, "phonewave", "run", "--verbose", "--config", cfgPath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start phonewave: %v", err)
	}

	tp := &ToolProcess{
		Cmd:    cmd,
		Cancel: cancel,
		Stdout: &stdout,
		Stderr: &stderr,
	}

	// Wait for the PID file to appear, confirming the daemon started
	stateDir := filepath.Join(w.Root, ".phonewave")
	pidFile := filepath.Join(stateDir, "watch.pid")
	deadline := time.After(15 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for phonewave PID file\nstderr: %s", stderr.String())
		default:
			if _, err := os.Stat(pidFile); err == nil {
				return tp
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// StopPhonewave stops the phonewave daemon gracefully.
func (w *Workspace) StopPhonewave(t *testing.T, tp *ToolProcess) {
	t.Helper()
	tp.Cancel()
	// Wait for the process to exit (ignore error since cancel causes signal)
	_ = tp.Cmd.Wait()

	// Wait for PID file removal (graceful shutdown)
	pidFile := filepath.Join(w.Root, ".phonewave", "watch.pid")
	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-deadline:
			// PID file not removed, but process is dead -- acceptable
			return
		default:
			if _, err := os.Stat(pidFile); os.IsNotExist(err) {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// RunSightjack runs sightjack with the given args and waits for completion.
func (w *Workspace) RunSightjack(t *testing.T, ctx context.Context, args ...string) error {
	t.Helper()
	cmd := w.runToolCmd(ctx, "sightjack", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("sightjack %v failed: %v\n%s", args, err, out)
	}
	return err
}

// RunSightjackScan runs sightjack run with --auto-approve.
// With the extended --auto-approve semantics, sightjack auto-selects the
// first available wave and auto-approves all actions without stdin input.
func (w *Workspace) RunSightjackScan(t *testing.T, ctx context.Context, extraArgs ...string) error {
	t.Helper()
	args := []string{"run", "--auto-approve"}
	args = append(args, extraArgs...)
	args = append(args, w.RepoPath)

	cmd := w.runToolCmd(ctx, "sightjack", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("sightjack %v failed: %v\n%s", args, err, out)
	}
	return err
}

// RunPaintress runs paintress with the given args and waits for completion.
func (w *Workspace) RunPaintress(t *testing.T, ctx context.Context, args ...string) error {
	t.Helper()
	cmd := w.runToolCmd(ctx, "paintress", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("paintress %v failed: %v\n%s", args, err, out)
	}
	return err
}

// RunPaintressExpedition runs paintress run with auto-approve, no-dev, workers 0,
// and max-expeditions 1 (sufficient for scenario tests that inject D-Mails one at a time).
func (w *Workspace) RunPaintressExpedition(t *testing.T, ctx context.Context, extraArgs ...string) error {
	t.Helper()
	args := []string{"run", "--auto-approve", "--no-dev", "--workers", "0", "--max-expeditions", "1"}
	args = append(args, extraArgs...)
	args = append(args, w.RepoPath)
	return w.RunPaintress(t, ctx, args...)
}

// RunAmadeus runs amadeus with the given args and waits for completion.
func (w *Workspace) RunAmadeus(t *testing.T, ctx context.Context, args ...string) error {
	t.Helper()
	cmd := w.runToolCmd(ctx, "amadeus", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("amadeus %v failed: %v\n%s", args, err, out)
	}
	return err
}

// RunAmadeusCheck runs amadeus check with --auto-approve and waits for completion.
func (w *Workspace) RunAmadeusCheck(t *testing.T, ctx context.Context, extraArgs ...string) error {
	t.Helper()
	args := []string{"check", "--auto-approve"}
	args = append(args, extraArgs...)
	args = append(args, w.RepoPath)
	return w.RunAmadeus(t, ctx, args...)
}

// --- Observation Helpers ---

// WaitForDMail polls a tool's mailbox subdirectory until at least one .md file
// appears. Returns the full path of the first .md file found.
func (w *Workspace) WaitForDMail(t *testing.T, toolDir, sub string, timeout time.Duration) string {
	t.Helper()
	dir := filepath.Join(w.RepoPath, toolDir, sub)
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for D-Mail in %s/%s", toolDir, sub)
			return ""
		default:
			entries, err := os.ReadDir(dir)
			if err == nil {
				for _, e := range entries {
					if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
						return filepath.Join(dir, e.Name())
					}
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// WaitForDMailCount polls until at least minCount .md files exist in a mailbox.
// Use this when a mailbox already has files and you're waiting for additional deliveries.
func (w *Workspace) WaitForDMailCount(t *testing.T, toolDir, sub string, minCount int, timeout time.Duration) {
	t.Helper()
	dir := filepath.Join(w.RepoPath, toolDir, sub)
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			actual := countMDFiles(dir)
			t.Fatalf("timeout waiting for %d D-Mails in %s/%s (got %d)", minCount, toolDir, sub, actual)
		default:
			if countMDFiles(dir) >= minCount {
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// countMDFiles counts .md files in a directory (returns 0 if dir doesn't exist).
func countMDFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			count++
		}
	}
	return count
}

// WaitForAbsent polls until a directory contains no .md files.
func (w *Workspace) WaitForAbsent(t *testing.T, toolDir, sub string, timeout time.Duration) {
	t.Helper()
	dir := filepath.Join(w.RepoPath, toolDir, sub)
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			files := w.ListFiles(t, dir)
			t.Fatalf("timeout waiting for %s/%s to be empty; files: %v", toolDir, sub, files)
		default:
			entries, err := os.ReadDir(dir)
			if err != nil {
				// Directory gone or unreadable = absent
				return
			}
			hasMD := false
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
					hasMD = true
					break
				}
			}
			if !hasMD {
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// CountFiles returns the count of non-directory entries in a directory.
func (w *Workspace) CountFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("read dir %s: %v", dir, err)
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	return count
}

// ListFiles returns names of non-directory entries in a directory.
func (w *Workspace) ListFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read dir %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// InjectDMail atomically writes a D-Mail file to a tool's mailbox.
// Uses write-to-temp-then-rename to properly trigger fsnotify.
func (w *Workspace) InjectDMail(t *testing.T, toolDir, sub, filename string, content []byte) {
	t.Helper()
	dir := filepath.Join(w.RepoPath, toolDir, sub)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	tmpFile := filepath.Join(dir, ".tmp-"+filename)
	finalPath := filepath.Join(dir, filename)
	if err := os.WriteFile(tmpFile, content, 0o644); err != nil {
		t.Fatalf("write temp file %s: %v", tmpFile, err)
	}
	if err := os.Rename(tmpFile, finalPath); err != nil {
		t.Fatalf("rename %s -> %s: %v", tmpFile, finalPath, err)
	}
}

// ReadDMail parses a D-Mail file's YAML frontmatter and body.
// Frontmatter is delimited by --- markers. Body follows the second ---.
func (w *Workspace) ReadDMail(t *testing.T, path string) (frontmatter map[string]any, body string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read D-Mail %s: %v", path, err)
	}

	content := string(data)
	frontmatter, body = parseFrontmatter(t, content)
	return frontmatter, body
}

// parseFrontmatter splits YAML frontmatter (between --- markers) from body.
func parseFrontmatter(t *testing.T, content string) (map[string]any, string) {
	t.Helper()
	const sep = "---"

	// Must start with ---
	if !strings.HasPrefix(strings.TrimSpace(content), sep) {
		t.Fatalf("D-Mail missing frontmatter delimiter at start")
	}

	// Find the second ---
	trimmed := strings.TrimSpace(content)
	firstSep := strings.Index(trimmed, sep)
	rest := trimmed[firstSep+len(sep):]
	secondSep := strings.Index(rest, sep)
	if secondSep < 0 {
		t.Fatalf("D-Mail missing closing frontmatter delimiter")
	}

	fmStr := rest[:secondSep]
	bodyStr := rest[secondSep+len(sep):]

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(fmStr), &fm); err != nil {
		t.Fatalf("parse frontmatter: %v\ncontent: %s", err, fmStr)
	}

	return fm, strings.TrimSpace(bodyStr)
}

// --- Private Helpers ---

// runToolCmd creates an exec.Cmd with the correct working directory and environment.
func (w *Workspace) runToolCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = w.RepoPath
	cmd.Env = append(os.Environ(), w.Env...)
	return cmd
}

// runCmd executes a command in the given directory and fatals on error.
func runCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}

// PhonewaveStateDir returns the path to the phonewave state directory (.phonewave/).
func (w *Workspace) PhonewaveStateDir() string {
	return filepath.Join(w.Root, ".phonewave")
}

// ToolDir returns the absolute path to a tool's state directory within the repo.
// Examples: w.ToolDir(".siren"), w.ToolDir(".expedition"), w.ToolDir(".gate")
func (w *Workspace) ToolDir(toolDir string) string {
	return filepath.Join(w.RepoPath, toolDir)
}

// PromptLogDir returns the path to the prompt log directory for this workspace.
func (w *Workspace) PromptLogDir() string {
	return filepath.Join(w.Root, "prompt-logs")
}

// DumpPhonewaveLog writes the phonewave daemon's stderr to the test log.
// Useful for debugging when a scenario test fails.
func (w *Workspace) DumpPhonewaveLog(t *testing.T, tp *ToolProcess) {
	t.Helper()
	if tp.Stderr.Len() > 0 {
		t.Logf("phonewave stderr:\n%s", tp.Stderr.String())
	}
	if tp.Stdout.Len() > 0 {
		t.Logf("phonewave stdout:\n%s", tp.Stdout.String())
	}
}

// FormatDMail creates a D-Mail file content with the given frontmatter fields and body.
// Integer-like fields (priority) are written unquoted; strings are quoted for YAML safety.
func FormatDMail(fields map[string]string, body string) []byte {
	// Fields that should be written as unquoted integers
	intFields := map[string]bool{"priority": true}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	// Always write dmail-schema-version first for consistency
	if v, ok := fields["dmail-schema-version"]; ok {
		fmt.Fprintf(&buf, "dmail-schema-version: %q\n", v)
	}
	for k, v := range fields {
		if k == "dmail-schema-version" {
			continue
		}
		if intFields[k] {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		} else {
			fmt.Fprintf(&buf, "%s: %q\n", k, v)
		}
	}
	buf.WriteString("---\n\n")
	buf.WriteString(body)
	buf.WriteString("\n")
	return buf.Bytes()
}
