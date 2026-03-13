package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"
)

// newShellCmd is a package-level variable for creating shell-aware exec.Cmd.
// Override in tests via OverrideShellCmd.
var newShellCmd = platform.NewShellCmd

// OverrideShellCmd replaces the command constructor for testing and returns a
// cleanup function.
func OverrideShellCmd(fn func(ctx context.Context, name string, args ...string) *exec.Cmd) func() {
	old := newShellCmd
	newShellCmd = fn
	return func() { newShellCmd = old }
}

// lookPathShell is a package-level variable for shell-aware LookPath.
// Override in tests via OverrideLookPath.
var lookPathShell = platform.LookPathShell

// OverrideLookPath replaces the path lookup function for testing and returns a
// cleanup function.
func OverrideLookPath(fn func(cmd string) (string, error)) func() {
	old := lookPathShell
	lookPathShell = fn
	return func() { lookPathShell = old }
}

// installSkillsRefFn runs "uv tool install skills-ref". Injectable for testing.
var installSkillsRefFn = func() error {
	cmd := exec.Command("uv", "tool", "install", "skills-ref")
	return cmd.Run()
}

// findSkillsRefDirFn searches for skills-ref submodule directory.
var findSkillsRefDirFn = findSkillsRefDir

// generateSkillsFn regenerates SKILL.md files. Injectable for testing.
var generateSkillsFn = func(repoRoot string, logger domain.Logger) error {
	return session.InitGateDir(filepath.Join(repoRoot, domain.StateDir), logger)
}

func findSkillsRefDir() string {
	candidates := []string{
		filepath.Join("..", "skills-ref"),
		filepath.Join("..", "..", "skills-ref"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return ""
}

// OverrideInstallSkillsRef replaces the skills-ref installer for testing and
// returns a cleanup function.
func OverrideInstallSkillsRef(fn func() error) func() {
	old := installSkillsRefFn
	installSkillsRefFn = fn
	return func() { installSkillsRefFn = old }
}

// OverrideFindSkillsRefDir replaces the skills-ref directory finder for testing
// and returns a cleanup function.
func OverrideFindSkillsRefDir(fn func() string) func() {
	old := findSkillsRefDirFn
	findSkillsRefDirFn = fn
	return func() { findSkillsRefDirFn = old }
}

// OverrideGenerateSkills replaces the skill generator for testing and returns a
// cleanup function.
func OverrideGenerateSkills(fn func(string, domain.Logger) error) func() {
	old := generateSkillsFn
	generateSkillsFn = fn
	return func() { generateSkillsFn = old }
}

// checkTool verifies that a CLI tool is installed and executable.
// Supports shell-like command strings with leading KEY=VALUE env vars and tilde paths.
func checkTool(ctx context.Context, name string) domain.DoctorCheck {
	path, err := lookPathShell(name)
	if err != nil {
		return domain.DoctorCheck{
			Name:    name,
			Status:  domain.CheckFail,
			Message: "command not found",
			Hint:    fmt.Sprintf("install %s and ensure it is in PATH", name),
		}
	}

	out, err := newShellCmd(ctx, name, "--version").Output()
	if err != nil {
		return domain.DoctorCheck{
			Name:    name,
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("found at %s but --version failed: %v", path, err),
			Hint:    fmt.Sprintf("%s may be corrupted; reinstall it", name),
		}
	}

	version := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	return domain.DoctorCheck{
		Name:    name,
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s (%s)", path, version),
	}
}

// checkGitRepo verifies the given directory is inside a git repository.
// Uses exec.Command directly (not newShellCmd) because cmd.Dir must be set,
// and tests use real git repos via git init.
func checkGitRepo(dir string) domain.DoctorCheck {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return domain.DoctorCheck{
			Name:    "Git Repository",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s is not a git repository", dir),
			Hint:    `run "git init" or navigate to a git repository`,
		}
	}
	return domain.DoctorCheck{
		Name:    "Git Repository",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s is a git repository", dir),
	}
}

// checkGitRemote verifies that at least one git remote is configured.
// amadeus reads Pull Requests for divergence checks, so a remote is required.
func checkGitRemote(dir string) domain.DoctorCheck {
	cmd := exec.Command("git", "remote")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return domain.DoctorCheck{
			Name:    "Git Remote",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("failed to check git remote in %s", dir),
			Hint:    "ensure the directory is a git repository",
		}
	}
	if strings.TrimSpace(string(out)) == "" {
		return domain.DoctorCheck{
			Name:    "Git Remote",
			Status:  domain.CheckFail,
			Message: "no remote configured",
			Hint:    `amadeus reads Pull Requests for divergence checks — run "git remote add origin <url>" to connect to GitHub`,
		}
	}
	remotes := strings.Fields(strings.TrimSpace(string(out)))
	return domain.DoctorCheck{
		Name:    "Git Remote",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%d remote(s): %s", len(remotes), strings.Join(remotes, ", ")),
	}
}

// checkGateDir verifies .gate/ directory exists and is writable.
// When repair is true and the directory is missing, it creates it.
func checkGateDir(repoRoot string, repair bool) domain.DoctorCheck {
	dir := filepath.Join(repoRoot, domain.StateDir)
	info, err := os.Stat(dir)
	if err != nil {
		if !repair {
			return domain.DoctorCheck{
				Name:    ".gate/",
				Status:  domain.CheckFail,
				Message: "not found — run 'amadeus init' first",
				Hint:    `run "amadeus init" or "amadeus doctor --repair"`,
			}
		}
		if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
			return domain.DoctorCheck{
				Name:    ".gate/",
				Status:  domain.CheckFail,
				Message: fmt.Sprintf("cannot create %s: %v", dir, mkErr),
				Hint:    `check directory permissions or run "amadeus init"`,
			}
		}
		return domain.DoctorCheck{
			Name:    ".gate/",
			Status:  domain.CheckFixed,
			Message: fmt.Sprintf("created %s", dir),
		}
	}
	if !info.IsDir() {
		return domain.DoctorCheck{
			Name:    ".gate/",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s exists but is not a directory", dir),
			Hint:    `remove the .gate file and run "amadeus init"`,
		}
	}
	probe := filepath.Join(dir, ".doctor_probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return domain.DoctorCheck{
			Name:    ".gate/",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("not writable: %v", err),
			Hint:    "check file permissions on the .gate/ directory",
		}
	}
	if err := os.Remove(probe); err != nil {
		return domain.DoctorCheck{
			Name:    ".gate/",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("probe cleanup failed: %v", err),
			Hint:    "check file permissions on the .gate/ directory",
		}
	}
	return domain.DoctorCheck{
		Name:    ".gate/",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s writable", dir),
	}
}

// checkClaudeAuth determines if the Claude CLI is authenticated by
// interpreting the result of running `claude mcp list`. A successful
// command execution (no error) indicates the CLI is authenticated.
func checkClaudeAuth(mcpOutput string, mcpErr error) domain.DoctorCheck {
	if mcpErr != nil {
		return domain.DoctorCheck{
			Name:    "claude-auth",
			Status:  domain.CheckWarn,
			Message: "not authenticated: " + mcpErr.Error(),
			Hint:    `run "claude login" to authenticate`,
		}
	}
	return domain.DoctorCheck{
		Name:    "claude-auth",
		Status:  domain.CheckOK,
		Message: "authenticated",
	}
}

// checkLinearMCP verifies Linear MCP is connected by parsing `claude mcp list` output.
// Looks for a line containing "linear", "✓", and "connected" (case-insensitive).
// Requires "✓" to avoid false positives from "disconnected" or "not connected".
func checkLinearMCP(mcpOutput string, mcpErr error) domain.DoctorCheck {
	if mcpErr != nil {
		return domain.DoctorCheck{
			Name:    "linear-mcp",
			Status:  domain.CheckWarn,
			Message: fmt.Sprintf("claude mcp list failed: %v", mcpErr),
			Hint:    `run "claude login" to authenticate`,
		}
	}

	output := strings.ToLower(mcpOutput)
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "linear") && strings.Contains(line, "✓") && strings.Contains(line, "connected") {
			return domain.DoctorCheck{
				Name:    "linear-mcp",
				Status:  domain.CheckOK,
				Message: "Linear MCP connected",
			}
		}
	}

	return domain.DoctorCheck{
		Name:    "linear-mcp",
		Status:  domain.CheckWarn,
		Message: "Linear MCP not found or not connected",
		Hint: "run \"claude mcp add --transport http --scope project linear https://mcp.linear.app/mcp\" in your project root\n" +
			"  (a fully compatible local-only Linear MCP alternative is planned — check the project README for updates)",
	}
}

// checkClaudeInference determines if the Claude CLI can perform inference
// by interpreting the result of a minimal "1+1=" prompt.
func checkClaudeInference(output string, err error) domain.DoctorCheck {
	if err != nil {
		return domain.DoctorCheck{
			Name:    "claude-inference",
			Status:  domain.CheckWarn,
			Message: "inference failed: " + err.Error(),
			Hint: `"signal: killed" = CLI startup too slow (timeout 3m); ` +
				`"nested session" = CLAUDECODE env var leaked (doctor should filter it); ` +
				`otherwise check API key, quota, and model access`,
		}
	}
	if strings.TrimSpace(output) != "2" {
		return domain.DoctorCheck{
			Name:    "claude-inference",
			Status:  domain.CheckWarn,
			Message: "unexpected response: " + strings.TrimSpace(output),
			Hint:    "model returned unexpected output; check model access and API quota",
		}
	}
	return domain.DoctorCheck{
		Name:    "claude-inference",
		Status:  domain.CheckOK,
		Message: "inference OK",
	}
}

// checkSkillMD verifies that both dmail-sendable and dmail-readable SKILL.md files exist.
func checkSkillMD(repoRoot string) domain.DoctorCheck {
	skillsDir := filepath.Join(repoRoot, domain.StateDir, "skills")
	required := []string{"dmail-sendable", "dmail-readable"}
	var missing []string
	for _, name := range required {
		path := filepath.Join(skillsDir, name, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return domain.DoctorCheck{
			Name:    "SKILL.md",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("missing: %s — run 'amadeus init'", strings.Join(missing, ", ")),
			Hint:    `run "amadeus init" to regenerate skill files`,
		}
	}
	// Check for deprecated "kind: feedback" (split into design-feedback / implementation-feedback)
	for _, name := range required {
		path := filepath.Join(skillsDir, name, "SKILL.md")
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "kind: feedback") &&
			!strings.Contains(content, "kind: design-feedback") &&
			!strings.Contains(content, "kind: implementation-feedback") {
			return domain.DoctorCheck{
				Name:    "SKILL.md",
				Status:  domain.CheckFail,
				Message: fmt.Sprintf("%s/SKILL.md uses deprecated kind 'feedback'", name),
				Hint:    "deprecated kind 'feedback'; migrate to 'design-feedback' or 'implementation-feedback' (run 'amadeus init --force' to regenerate SKILL.md)",
			}
		}
	}

	return domain.DoctorCheck{
		Name:    "SKILL.md",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s (dmail-sendable, dmail-readable)", skillsDir),
	}
}

// runDoctor executes all health checks and returns the results.
// Reads claude_cmd from the config file; falls back to domain.DefaultClaudeCmd on load error.
func runDoctor(ctx context.Context, configPath string, repoRoot string, logger domain.Logger, repair bool) []domain.DoctorCheck {
	claudeCmd := domain.DefaultClaudeCmd
	if cfg, err := loadConfig(configPath); err == nil {
		claudeCmd = cfg.ClaudeCmd
	}
	return runDoctorWithClaudeCmd(ctx, configPath, repoRoot, claudeCmd, logger, repair)
}

// runDoctorWithClaudeCmd executes all health checks with a configurable Claude command.
func runDoctorWithClaudeCmd(ctx context.Context, configPath string, repoRoot string, claudeCmd string, logger domain.Logger, repair bool) []domain.DoctorCheck {
	_, span := platform.Tracer.Start(ctx, "domain.doctor")
	defer span.End()

	var results []domain.DoctorCheck

	// --- Binaries ---
	results = append(results, checkTool(ctx, "git"))
	claudeResult := checkTool(ctx, claudeCmd)
	results = append(results, claudeResult)
	results = append(results, checkTool(ctx, "gh"))

	// --- Repository ---
	results = append(results, checkGitRepo(repoRoot))
	results = append(results, checkGitRemote(repoRoot))

	// --- State ---
	results = append(results, checkGateDir(repoRoot, repair))
	results = append(results, checkConfig(configPath))

	// --- Data ---
	skillResult := checkSkillMD(repoRoot)
	if repair && skillResult.Status == domain.CheckFail {
		if err := generateSkillsFn(repoRoot, logger); err == nil {
			recheck := checkSkillMD(repoRoot)
			if recheck.Status == domain.CheckOK {
				results = append(results, domain.DoctorCheck{
					Name: "SKILL.md", Status: domain.CheckFixed,
					Message: "regenerated SKILL.md files",
				})
			} else {
				results = append(results, skillResult)
			}
		} else {
			results = append(results, skillResult)
		}
	} else {
		results = append(results, skillResult)
	}
	results = append(results, checkEventStore(filepath.Join(repoRoot, domain.StateDir)))
	results = append(results, checkDMailSchema(filepath.Join(repoRoot, domain.StateDir)))
	results = append(results, checkFsnotify())

	// --- Connectivity ---
	if claudeResult.Status != domain.CheckOK {
		results = append(results, domain.DoctorCheck{
			Name:    "claude-auth",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
		results = append(results, domain.DoctorCheck{
			Name:    "linear-mcp",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
		results = append(results, domain.DoctorCheck{
			Name:    "claude-inference",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
		results = append(results, domain.DoctorCheck{
			Name:    "context-budget",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
	} else {
		mcpCtx, mcpCancel := context.WithTimeout(ctx, 10*time.Second)
		cmd := newShellCmd(mcpCtx, claudeCmd, "mcp", "list")
		out, mcpErr := cmd.Output()
		mcpCancel()
		mcpOutput := string(out)

		authResult := checkClaudeAuth(mcpOutput, mcpErr)
		results = append(results, authResult)

		if authResult.Status != domain.CheckOK {
			results = append(results, domain.DoctorCheck{
				Name:    "linear-mcp",
				Status:  domain.CheckSkip,
				Message: "skipped (auth failed)",
			})
			results = append(results, domain.DoctorCheck{
				Name:    "claude-inference",
				Status:  domain.CheckSkip,
				Message: "skipped (auth failed)",
			})
			results = append(results, domain.DoctorCheck{
				Name:    "context-budget",
				Status:  domain.CheckSkip,
				Message: "skipped (auth failed)",
			})
		} else {
			results = append(results, checkLinearMCP(mcpOutput, mcpErr))

			inferCtx, inferCancel := context.WithTimeout(ctx, 3*time.Minute)
			inferCmd := newShellCmd(inferCtx, claudeCmd, "--print", "--verbose", "--output-format", "stream-json", "--max-turns", "1", "1+1=")
			inferOut, inferErr := inferCmd.Output()
			inferCancel()
			inferOutput := string(inferOut)
			inferResult := checkClaudeInference(strings.TrimSpace(extractStreamResult(inferOutput)), inferErr)
			results = append(results, inferResult)

			// Context budget check: skip if inference failed
			if inferResult.Status != domain.CheckOK {
				results = append(results, domain.DoctorCheck{
					Name:    "context-budget",
					Status:  domain.CheckSkip,
					Message: "skipped (inference failed)",
				})
			} else {
				results = append(results, checkContextBudget(inferOutput, repoRoot))
			}
		}
	}

	// --- skills-ref toolchain ---
	results = append(results, checkSkillsRefToolchain(repair)...)

	// --- Metrics ---
	results = append(results, checkSuccessRate(filepath.Join(repoRoot, domain.StateDir), logger))

	// --- Repair: stale PID cleanup ---
	if repair {
		pidPath := filepath.Join(repoRoot, domain.StateDir, "watch.pid")
		if data, err := os.ReadFile(pidPath); err == nil {
			pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if pid > 0 {
				proc, _ := os.FindProcess(pid)
				if proc == nil || proc.Signal(syscall.Signal(0)) != nil {
					os.Remove(pidPath)
					results = append(results, domain.DoctorCheck{
						Name: "stale-pid", Status: domain.CheckFixed,
						Message: "removed stale PID file",
					})
				}
			}
		}
	}

	for _, r := range results {
		span.AddEvent("doctor.check", trace.WithAttributes(
			attribute.String("check.name", platform.SanitizeUTF8(r.Name)),
			attribute.String("check.status", platform.SanitizeUTF8(r.Status.StatusLabel())),
		))
	}

	return results
}

// checkEventStore verifies events/ directory exists and all JSONL files are parseable.
func checkEventStore(gateRoot string) domain.DoctorCheck {
	eventsDir := filepath.Join(gateRoot, "events")
	if _, err := os.Stat(eventsDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return domain.DoctorCheck{
				Name:    "Event Store",
				Status:  domain.CheckSkip,
				Message: "no events directory — run 'amadeus init'",
			}
		}
		return domain.DoctorCheck{
			Name:    "Event Store",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("stat events: %v", err),
			Hint:    `run "amadeus init" to create the events directory`,
		}
	}
	count, err := countEventStoreEntries(eventsDir)
	if err != nil {
		return domain.DoctorCheck{
			Name:    "Event Store",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("parse error: %v", err),
			Hint:    "check event files in .gate/events/ for corruption",
		}
	}
	return domain.DoctorCheck{
		Name:    "Event Store",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%d event(s) loaded", count),
	}
}

// countEventStoreEntries reads all .jsonl files in the events directory
// and counts valid event entries. Returns an error if any line fails to parse.
func countEventStoreEntries(eventsDir string) (int, error) {
	entries, err := os.ReadDir(eventsDir)
	if err != nil {
		return 0, fmt.Errorf("read events dir: %w", err)
	}
	count := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(eventsDir, e.Name()))
		if readErr != nil {
			return 0, fmt.Errorf("read %s: %w", e.Name(), readErr)
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var ev domain.Event
			if parseErr := json.Unmarshal([]byte(line), &ev); parseErr != nil {
				return 0, fmt.Errorf("parse %s: %w", e.Name(), parseErr)
			}
			count++
		}
	}
	return count, nil
}

// checkDMailSchema validates all D-Mails in archive/ conform to schema v1.
func checkDMailSchema(gateRoot string) domain.DoctorCheck {
	archiveDir := filepath.Join(gateRoot, "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return domain.DoctorCheck{
				Name:    "D-Mail Schema",
				Status:  domain.CheckSkip,
				Message: "no archive directory",
			}
		}
		return domain.DoctorCheck{
			Name:    "D-Mail Schema",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("read archive: %v", err),
			Hint:    "check file permissions on the .gate/archive/ directory",
		}
	}

	var mdFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			mdFiles = append(mdFiles, e.Name())
		}
	}
	if len(mdFiles) == 0 {
		return domain.DoctorCheck{
			Name:    "D-Mail Schema",
			Status:  domain.CheckSkip,
			Message: "no D-Mails in archive",
		}
	}

	var invalid []string
	for _, name := range mdFiles {
		data, readErr := os.ReadFile(filepath.Join(archiveDir, name))
		if readErr != nil {
			invalid = append(invalid, fmt.Sprintf("%s: read error", name))
			continue
		}
		dmail, parseErr := domain.ParseDMail(data)
		if parseErr != nil {
			invalid = append(invalid, fmt.Sprintf("%s: %v", name, parseErr))
			continue
		}
		if errs := domain.ValidateDMail(dmail); len(errs) > 0 {
			invalid = append(invalid, fmt.Sprintf("%s: %s", name, strings.Join(errs, "; ")))
		}
	}

	if len(invalid) > 0 {
		return domain.DoctorCheck{
			Name:    "D-Mail Schema",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%d/%d invalid: %s", len(invalid), len(mdFiles), strings.Join(invalid, ", ")),
			Hint:    "re-send affected D-Mails or manually fix the frontmatter",
		}
	}
	return domain.DoctorCheck{
		Name:    "D-Mail Schema",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%d D-Mail(s) valid", len(mdFiles)),
	}
}

// checkSuccessRate calculates and reports the event-based success rate.
func checkSuccessRate(gateDir string, logger domain.Logger) domain.DoctorCheck {
	eventStore := session.NewEventStore(gateDir, logger)
	rate, clean, total, err := usecase.ComputeSuccessRate(eventStore)
	if err != nil || total == 0 {
		return domain.DoctorCheck{
			Name:    "success-rate",
			Status:  domain.CheckOK,
			Message: "no events",
		}
	}

	return domain.DoctorCheck{
		Name:    "success-rate",
		Status:  domain.CheckOK,
		Message: domain.FormatSuccessRate(rate, clean, total),
	}
}

// checkFsnotify verifies that the OS file watcher is available.
// On Linux, inotify limits can prevent watcher creation.
func checkFsnotify() domain.DoctorCheck {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return domain.DoctorCheck{
			Name:    "fsnotify",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("cannot create file watcher: %v", err),
			Hint:    "on Linux, increase inotify limit: sysctl fs.inotify.max_user_watches=524288",
		}
	}
	defer w.Close()
	return domain.DoctorCheck{
		Name:    "fsnotify",
		Status:  domain.CheckOK,
		Message: "file watcher available",
	}
}

// checkSkillsRefToolchain verifies that the skills-ref tool is available.
func checkSkillsRefToolchain(repair bool) []domain.DoctorCheck {
	if _, err := lookPathShell("skills-ref"); err == nil {
		return []domain.DoctorCheck{{
			Name: "skills-ref", Status: domain.CheckOK,
			Message: "skills-ref found on PATH",
		}}
	}
	_, uvErr := lookPathShell("uv")
	if uvErr != nil {
		return []domain.DoctorCheck{{
			Name: "skills-ref", Status: domain.CheckWarn,
			Message: "uv not found on PATH: SKILL.md spec validation is unavailable",
			Hint:    `install uv (https://docs.astral.sh/uv/) or "uv tool install skills-ref"`,
		}}
	}
	subDir := findSkillsRefDirFn()
	if subDir != "" {
		return []domain.DoctorCheck{{
			Name: "skills-ref", Status: domain.CheckOK,
			Message: "uv + submodule ready",
		}}
	}
	if repair {
		if err := installSkillsRefFn(); err != nil {
			return []domain.DoctorCheck{{
				Name: "skills-ref", Status: domain.CheckWarn,
				Message: fmt.Sprintf("uv tool install skills-ref failed: %v", err),
				Hint:    `try manually: "uv tool install skills-ref"`,
			}}
		}
		return []domain.DoctorCheck{{
			Name: "skills-ref", Status: domain.CheckFixed,
			Message: "installed skills-ref via uv tool install",
		}}
	}
	return []domain.DoctorCheck{{
		Name: "skills-ref", Status: domain.CheckWarn,
		Message: "uv found but skills-ref not installed",
		Hint:    `run "amadeus doctor --repair" or "uv tool install skills-ref"`,
	}}
}

// checkConfig validates that config.yaml exists and can be loaded.
func checkConfig(path string) domain.DoctorCheck {
	if _, err := os.Stat(path); err != nil {
		return domain.DoctorCheck{
			Name:    "Config",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s: %v", path, err),
			Hint:    `run "amadeus init" to create config.yaml`,
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.DoctorCheck{
			Name:    "Config",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s: %v", path, err),
			Hint:    "check file permissions on config.yaml",
		}
	}
	cfg := domain.DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return domain.DoctorCheck{
			Name:    "Config",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s: %v", path, err),
			Hint:    "fix YAML syntax in config.yaml",
		}
	}
	if errs := domain.ValidateConfig(cfg); len(errs) > 0 {
		return domain.DoctorCheck{
			Name:    "Config",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s: %s", path, strings.Join(errs, "; ")),
			Hint:    `check config.yaml values; run "amadeus init" to regenerate`,
		}
	}
	return domain.DoctorCheck{
		Name:    "Config",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s loaded and validated", path),
	}
}

// extractStreamResult parses stream-json output and returns the "result" field
// from the result message. Used to reuse login check output for inference check.
func extractStreamResult(streamJSON string) string {
	for _, line := range strings.Split(streamJSON, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg struct {
			Type   string `json:"type"`
			Result string `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &msg); err == nil && msg.Type == "result" {
			return msg.Result
		}
	}
	return ""
}

// checkContextBudget parses stream-json output from a Claude CLI invocation
// and reports context budget health based on hooks, plugins, skills, and MCP servers.
func checkContextBudget(streamJSON string, baseDir string) domain.DoctorCheck {
	var messages []*platform.StreamMessage
	for _, line := range strings.Split(streamJSON, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		msg, err := platform.ParseStreamMessage([]byte(line))
		if err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	report := platform.CalculateContextBudget(messages)
	breakdown := report.DetailedBreakdown()

	// Build detailed message lines
	var lines []string
	for _, item := range breakdown {
		marker := ""
		if item.Heaviest {
			marker = " <- heaviest"
		}
		if item.Category == "hooks" {
			if item.Bytes > 0 {
				lines = append(lines, fmt.Sprintf("  hooks: %d bytes (%d tok)%s", item.Bytes, item.Tokens, marker))
			}
		} else {
			if item.Count > 0 {
				lines = append(lines, fmt.Sprintf("  %s: %d (%d tok)%s", item.Category, item.Count, item.Tokens, marker))
			}
		}
	}

	status := domain.CheckOK
	msg := fmt.Sprintf("estimated %d tokens", report.EstimatedTokens)
	if report.Exceeds(platform.DefaultContextBudgetThreshold) {
		status = domain.CheckWarn
		msg = fmt.Sprintf("estimated %d tokens (threshold: %d)", report.EstimatedTokens, platform.DefaultContextBudgetThreshold)
	}
	if len(lines) > 0 {
		msg += "\n" + strings.Join(lines, "\n")
	}

	result := domain.DoctorCheck{
		Name:    "context-budget",
		Status:  status,
		Message: msg,
	}

	// Hint logic: only when threshold exceeded
	if report.Exceeds(platform.DefaultContextBudgetThreshold) {
		projectSettings := filepath.Join(baseDir, ".claude", "settings.json")
		if _, err := os.Stat(projectSettings); err == nil {
			result.Hint = ".claude/settings.json の設定を見直してください"
		} else {
			var heaviest string
			for _, item := range breakdown {
				if item.Heaviest {
					heaviest = item.Category
					break
				}
			}
			switch heaviest {
			case "mcp_servers":
				result.Hint = ".claude/settings.json をプロジェクトに作成し、必要な MCP server のみ定義を推奨"
			case "tools":
				result.Hint = "tools は plugins/MCP 由来 → .claude/settings.json で plugins/MCP を絞ることを推奨"
			default:
				result.Hint = ".claude/settings.json をプロジェクトに作成し、必要なプラグインのみ有効化を推奨"
			}
		}
	}

	return result
}

