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
	"strings"
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

// checkTool verifies that a CLI tool is installed and executable.
// Supports shell-like command strings with leading KEY=VALUE env vars and tilde paths.
func checkTool(ctx context.Context, name string) domain.DoctorCheckResult {
	path, err := lookPathShell(name)
	if err != nil {
		return domain.DoctorCheckResult{
			Name:    name,
			Status:  domain.CheckFail,
			Message: "command not found",
			Hint:    fmt.Sprintf("install %s and ensure it is in PATH", name),
		}
	}

	out, err := newShellCmd(ctx, name, "--version").Output()
	if err != nil {
		return domain.DoctorCheckResult{
			Name:    name,
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("found at %s but --version failed: %v", path, err),
			Hint:    fmt.Sprintf("%s may be corrupted; reinstall it", name),
		}
	}

	version := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	return domain.DoctorCheckResult{
		Name:    name,
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s (%s)", path, version),
	}
}

// checkGitRepo verifies the given directory is inside a git repository.
// Uses exec.Command directly (not newShellCmd) because cmd.Dir must be set,
// and tests use real git repos via git init.
func checkGitRepo(dir string) domain.DoctorCheckResult {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return domain.DoctorCheckResult{
			Name:    "Git Repository",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s is not a git repository", dir),
			Hint:    `run "git init" or navigate to a git repository`,
		}
	}
	return domain.DoctorCheckResult{
		Name:    "Git Repository",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s is a git repository", dir),
	}
}

// checkGitRemote verifies that at least one git remote is configured.
// amadeus reads Pull Requests for divergence checks, so a remote is required.
func checkGitRemote(dir string) domain.DoctorCheckResult {
	cmd := exec.Command("git", "remote")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return domain.DoctorCheckResult{
			Name:    "Git Remote",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("failed to check git remote in %s", dir),
			Hint:    "ensure the directory is a git repository",
		}
	}
	if strings.TrimSpace(string(out)) == "" {
		return domain.DoctorCheckResult{
			Name:    "Git Remote",
			Status:  domain.CheckFail,
			Message: "no remote configured",
			Hint:    `amadeus reads Pull Requests for divergence checks — run "git remote add origin <url>" to connect to GitHub`,
		}
	}
	remotes := strings.Fields(strings.TrimSpace(string(out)))
	return domain.DoctorCheckResult{
		Name:    "Git Remote",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%d remote(s): %s", len(remotes), strings.Join(remotes, ", ")),
	}
}

// checkGateDir verifies .gate/ directory exists and is writable.
func checkGateDir(repoRoot string) domain.DoctorCheckResult {
	dir := filepath.Join(repoRoot, domain.StateDir)
	info, err := os.Stat(dir)
	if err != nil {
		return domain.DoctorCheckResult{
			Name:    ".gate/",
			Status:  domain.CheckFail,
			Message: "not found — run 'amadeus init' first",
			Hint:    `run "amadeus init" first`,
		}
	}
	if !info.IsDir() {
		return domain.DoctorCheckResult{
			Name:    ".gate/",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s exists but is not a directory", dir),
			Hint:    `remove the .gate file and run "amadeus init"`,
		}
	}
	probe := filepath.Join(dir, ".doctor_probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return domain.DoctorCheckResult{
			Name:    ".gate/",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("not writable: %v", err),
			Hint:    "check file permissions on the .gate/ directory",
		}
	}
	if err := os.Remove(probe); err != nil {
		return domain.DoctorCheckResult{
			Name:    ".gate/",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("probe cleanup failed: %v", err),
			Hint:    "check file permissions on the .gate/ directory",
		}
	}
	return domain.DoctorCheckResult{
		Name:    ".gate/",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s writable", dir),
	}
}

// checkClaudeLogin verifies the Claude CLI session is active by parsing
// stream-json output from a minimal --print invocation. Detects expired
// sessions ("Not logged in", "authentication_failed") and provides the
// correct login command including any CLAUDE_CONFIG_DIR prefix from config.
func checkClaudeLogin(stdout string, runErr error, claudeCmd string) domain.DoctorCheckResult {
	loginHint := buildLoginHint(claudeCmd)

	isAuthFailure := strings.Contains(stdout, "authentication_failed") ||
		strings.Contains(stdout, "Not logged in")

	if isAuthFailure {
		return domain.DoctorCheckResult{
			Name:    "claude-login",
			Status:  domain.CheckFail,
			Message: "not logged in",
			Hint:    loginHint,
		}
	}

	if runErr != nil {
		return domain.DoctorCheckResult{
			Name:    "claude-login",
			Status:  domain.CheckFail,
			Message: "login check failed: " + runErr.Error(),
			Hint:    loginHint,
		}
	}

	return domain.DoctorCheckResult{
		Name:    "claude-login",
		Status:  domain.CheckOK,
		Message: "logged in",
	}
}

// buildLoginHint constructs the correct login command from claudeCmd,
// preserving any CLAUDE_CONFIG_DIR=... prefix so the user can copy-paste.
func buildLoginHint(claudeCmd string) string {
	env, _, _ := platform.ParseShellCommand(claudeCmd)
	var prefix string
	for _, e := range env {
		if strings.HasPrefix(e, "CLAUDE_CONFIG_DIR=") {
			prefix = e + " "
			break
		}
	}
	return fmt.Sprintf("run: %sclaude /login", prefix)
}

// checkClaudeAuth determines if the Claude CLI is authenticated by
// interpreting the result of running `claude mcp list`. A successful
// command execution (no error) indicates the CLI is authenticated.
func checkClaudeAuth(mcpOutput string, mcpErr error) domain.DoctorCheckResult {
	if mcpErr != nil {
		return domain.DoctorCheckResult{
			Name:    "claude-auth",
			Status:  domain.CheckFail,
			Message: "not authenticated: " + mcpErr.Error(),
			Hint:    `run "claude login" to authenticate (in Docker: set CLAUDE_CONFIG_DIR=~/.claude to use host credentials)`,
		}
	}
	return domain.DoctorCheckResult{
		Name:    "claude-auth",
		Status:  domain.CheckOK,
		Message: "authenticated",
	}
}

// checkLinearMCP verifies Linear MCP is connected by parsing `claude mcp list` output.
// Looks for a line containing "linear", "✓", and "connected" (case-insensitive).
// Requires "✓" to avoid false positives from "disconnected" or "not connected".
func checkLinearMCP(mcpOutput string, mcpErr error) domain.DoctorCheckResult {
	if mcpErr != nil {
		return domain.DoctorCheckResult{
			Name:    "Linear MCP",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("claude mcp list failed: %v", mcpErr),
			Hint:    `ensure Claude CLI is authenticated with "claude login" (in Docker: set CLAUDE_CONFIG_DIR=~/.claude to use host credentials)`,
		}
	}

	output := strings.ToLower(mcpOutput)
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "linear") && strings.Contains(line, "✓") && strings.Contains(line, "connected") {
			return domain.DoctorCheckResult{
				Name:    "Linear MCP",
				Status:  domain.CheckOK,
				Message: "Linear MCP connected",
			}
		}
	}

	return domain.DoctorCheckResult{
		Name:    "Linear MCP",
		Status:  domain.CheckFail,
		Message: "Linear MCP not found or not connected in claude mcp list output",
		Hint: "run \"claude mcp add --transport http --scope project linear https://mcp.linear.app/mcp\" in your project root\n" +
			"  (a fully compatible local-only Linear MCP alternative is planned — check the project README for updates)",
	}
}

// checkClaudeInference determines if the Claude CLI can perform inference
// by interpreting the result of a minimal "1+1=" prompt.
func checkClaudeInference(output string, err error) domain.DoctorCheckResult {
	if err != nil {
		return domain.DoctorCheckResult{
			Name:    "claude-inference",
			Status:  domain.CheckFail,
			Message: "inference failed: " + err.Error(),
			Hint: `"signal: killed" = CLI startup too slow (timeout 60s); ` +
				`"nested session" = CLAUDECODE env var leaked (doctor should filter it); ` +
				`otherwise check API key, quota, and model access`,
		}
	}
	if strings.TrimSpace(output) != "2" {
		return domain.DoctorCheckResult{
			Name:    "claude-inference",
			Status:  domain.CheckFail,
			Message: "unexpected response: " + strings.TrimSpace(output),
			Hint:    "model returned unexpected output; check model access and API quota",
		}
	}
	return domain.DoctorCheckResult{
		Name:    "claude-inference",
		Status:  domain.CheckOK,
		Message: "inference OK",
	}
}

// checkSkillMD verifies that both dmail-sendable and dmail-readable SKILL.md files exist.
func checkSkillMD(repoRoot string) domain.DoctorCheckResult {
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
		return domain.DoctorCheckResult{
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
			return domain.DoctorCheckResult{
				Name:    "SKILL.md",
				Status:  domain.CheckFail,
				Message: fmt.Sprintf("%s/SKILL.md uses deprecated kind 'feedback'", name),
				Hint:    "deprecated kind 'feedback'; migrate to 'design-feedback' or 'implementation-feedback' (run 'amadeus init --force' to regenerate SKILL.md)",
			}
		}
	}

	return domain.DoctorCheckResult{
		Name:    "SKILL.md",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s (dmail-sendable, dmail-readable)", skillsDir),
	}
}

// runDoctor executes all health checks and returns the results.
// Reads claude_cmd from the config file; falls back to domain.DefaultClaudeCmd on load error.
func runDoctor(ctx context.Context, configPath string, repoRoot string, logger domain.Logger) []domain.DoctorCheckResult {
	claudeCmd := domain.DefaultClaudeCmd
	if cfg, err := loadConfig(configPath); err == nil {
		claudeCmd = cfg.ClaudeCmd
	}
	return runDoctorWithClaudeCmd(ctx, configPath, repoRoot, claudeCmd, logger)
}

// runDoctorWithClaudeCmd executes all health checks with a configurable Claude command.
func runDoctorWithClaudeCmd(ctx context.Context, configPath string, repoRoot string, claudeCmd string, logger domain.Logger) []domain.DoctorCheckResult {
	_, span := platform.Tracer.Start(ctx, "domain.doctor")
	defer span.End()

	var results []domain.DoctorCheckResult

	// --- Binaries ---
	results = append(results, checkTool(ctx, "git"))
	claudeResult := checkTool(ctx, claudeCmd)
	results = append(results, claudeResult)
	results = append(results, checkTool(ctx, "gh"))

	// --- Repository ---
	results = append(results, checkGitRepo(repoRoot))
	results = append(results, checkGitRemote(repoRoot))

	// --- State ---
	results = append(results, checkGateDir(repoRoot))
	results = append(results, checkConfig(configPath))

	// --- Data ---
	results = append(results, checkSkillMD(repoRoot))
	results = append(results, checkEventStore(filepath.Join(repoRoot, domain.StateDir)))
	results = append(results, checkDMailSchema(filepath.Join(repoRoot, domain.StateDir)))
	results = append(results, checkFsnotify())

	// --- Connectivity ---
	if claudeResult.Status != domain.CheckOK {
		results = append(results, domain.DoctorCheckResult{
			Name:    "claude-auth",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
		results = append(results, domain.DoctorCheckResult{
			Name:    "Linear MCP",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
		results = append(results, domain.DoctorCheckResult{
			Name:    "claude-inference",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
	} else {
		// Login check: minimal --print to detect expired sessions
		loginCtx, loginCancel := context.WithTimeout(ctx, 30*time.Second)
		loginCmd := newShellCmd(loginCtx, claudeCmd, "--print", "--output-format", "stream-json", "--max-turns", "1", "1+1=")
		loginCmd.Env = filterEnv(os.Environ(), "CLAUDECODE")
		loginOut, loginErr := loginCmd.Output()
		loginCancel()
		loginResult := checkClaudeLogin(string(loginOut), loginErr, claudeCmd)
		results = append(results, loginResult)

		if loginResult.Status != domain.CheckOK {
			results = append(results, domain.DoctorCheckResult{
				Name:    "claude-auth",
				Status:  domain.CheckSkip,
				Message: "skipped (not logged in)",
			})
			results = append(results, domain.DoctorCheckResult{
				Name:    "Linear MCP",
				Status:  domain.CheckSkip,
				Message: "skipped (not logged in)",
			})
			results = append(results, domain.DoctorCheckResult{
				Name:    "claude-inference",
				Status:  domain.CheckSkip,
				Message: "skipped (not logged in)",
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
				results = append(results, domain.DoctorCheckResult{
					Name:    "Linear MCP",
					Status:  domain.CheckSkip,
					Message: "skipped (claude not authenticated)",
				})
			} else {
				results = append(results, checkLinearMCP(mcpOutput, mcpErr))
			}

			// Inference check: reuse login output if it contains "2"
			inferResult := checkClaudeInference(strings.TrimSpace(extractStreamResult(string(loginOut))), loginErr)
			results = append(results, inferResult)

			// Context budget check: reuse login stream-json output
			results = append(results, checkContextBudget(string(loginOut)))
		}
	}

	// --- Metrics ---
	results = append(results, checkSuccessRate(filepath.Join(repoRoot, domain.StateDir), logger))

	for _, r := range results {
		span.AddEvent("doctor.check", trace.WithAttributes(
			attribute.String("check.name", platform.SanitizeUTF8(r.Name)),
			attribute.String("check.status", platform.SanitizeUTF8(r.Status.StatusLabel())),
		))
	}

	return results
}

// checkEventStore verifies events/ directory exists and all JSONL files are parseable.
func checkEventStore(gateRoot string) domain.DoctorCheckResult {
	eventsDir := filepath.Join(gateRoot, "events")
	if _, err := os.Stat(eventsDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return domain.DoctorCheckResult{
				Name:    "Event Store",
				Status:  domain.CheckSkip,
				Message: "no events directory — run 'amadeus init'",
			}
		}
		return domain.DoctorCheckResult{
			Name:    "Event Store",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("stat events: %v", err),
			Hint:    `run "amadeus init" to create the events directory`,
		}
	}
	count, err := countEventStoreEntries(eventsDir)
	if err != nil {
		return domain.DoctorCheckResult{
			Name:    "Event Store",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("parse error: %v", err),
			Hint:    "check event files in .gate/events/ for corruption",
		}
	}
	return domain.DoctorCheckResult{
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
func checkDMailSchema(gateRoot string) domain.DoctorCheckResult {
	archiveDir := filepath.Join(gateRoot, "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return domain.DoctorCheckResult{
				Name:    "D-Mail Schema",
				Status:  domain.CheckSkip,
				Message: "no archive directory",
			}
		}
		return domain.DoctorCheckResult{
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
		return domain.DoctorCheckResult{
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
		return domain.DoctorCheckResult{
			Name:    "D-Mail Schema",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%d/%d invalid: %s", len(invalid), len(mdFiles), strings.Join(invalid, ", ")),
			Hint:    "re-send affected D-Mails or manually fix the frontmatter",
		}
	}
	return domain.DoctorCheckResult{
		Name:    "D-Mail Schema",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%d D-Mail(s) valid", len(mdFiles)),
	}
}

// checkSuccessRate calculates and reports the event-based success rate.
func checkSuccessRate(gateDir string, logger domain.Logger) domain.DoctorCheckResult {
	eventStore := session.NewEventStore(gateDir, logger)
	rate, clean, total, err := usecase.ComputeSuccessRate(eventStore)
	if err != nil || total == 0 {
		return domain.DoctorCheckResult{
			Name:    "success-rate",
			Status:  domain.CheckOK,
			Message: "no events",
		}
	}

	return domain.DoctorCheckResult{
		Name:    "success-rate",
		Status:  domain.CheckOK,
		Message: domain.FormatSuccessRate(rate, clean, total),
	}
}

// checkFsnotify verifies that the OS file watcher is available.
// On Linux, inotify limits can prevent watcher creation.
func checkFsnotify() domain.DoctorCheckResult {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return domain.DoctorCheckResult{
			Name:    "fsnotify",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("cannot create file watcher: %v", err),
			Hint:    "on Linux, increase inotify limit: sysctl fs.inotify.max_user_watches=524288",
		}
	}
	defer w.Close()
	return domain.DoctorCheckResult{
		Name:    "fsnotify",
		Status:  domain.CheckOK,
		Message: "file watcher available",
	}
}

// checkConfig validates that config.yaml exists and can be loaded.
func checkConfig(path string) domain.DoctorCheckResult {
	if _, err := os.Stat(path); err != nil {
		return domain.DoctorCheckResult{
			Name:    "Config",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s: %v", path, err),
			Hint:    `run "amadeus init" to create config.yaml`,
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.DoctorCheckResult{
			Name:    "Config",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s: %v", path, err),
			Hint:    "check file permissions on config.yaml",
		}
	}
	cfg := domain.DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return domain.DoctorCheckResult{
			Name:    "Config",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s: %v", path, err),
			Hint:    "fix YAML syntax in config.yaml",
		}
	}
	if errs := domain.ValidateConfig(cfg); len(errs) > 0 {
		return domain.DoctorCheckResult{
			Name:    "Config",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s: %s", path, strings.Join(errs, "; ")),
			Hint:    `check config.yaml values; run "amadeus init" to regenerate`,
		}
	}
	return domain.DoctorCheckResult{
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
func checkContextBudget(streamJSON string) domain.DoctorCheckResult {
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

	result := domain.DoctorCheckResult{
		Name:   "context-budget",
		Status: domain.CheckOK,
		Message: fmt.Sprintf("estimated %d tokens (tools=%d, skills=%d, plugins=%d, mcp=%d, hook_bytes=%d)",
			report.EstimatedTokens, report.ToolCount, report.SkillCount,
			report.PluginCount, report.MCPServerCount, report.HookContextBytes),
	}
	if report.Exceeds(platform.DefaultContextBudgetThreshold) {
		result.Hint = "context consumption is high; consider reducing installed plugins/skills or using an allowlist"
	}
	return result
}

// filterEnv returns a copy of env with the named variable removed.
// Used to unset CLAUDECODE so that doctor's inference check does not
// trigger the nested-session guard in Claude Code.
func filterEnv(env []string, name string) []string {
	prefix := name + "="
	out := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return out
}
