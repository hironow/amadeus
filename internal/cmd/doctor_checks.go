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

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"
)

// execCommand is a package-level variable for creating exec.Cmd.
// Override in tests to mock command execution.
var execCommand = exec.CommandContext

// checkTool verifies that a CLI tool is installed and executable.
func checkTool(ctx context.Context, name string) domain.DoctorCheckResult {
	path, err := exec.LookPath(name)
	if err != nil {
		return domain.DoctorCheckResult{
			Name:    name,
			Status:  domain.CheckFail,
			Message: "command not found",
			Hint:    fmt.Sprintf("install %s and ensure it is in PATH", name),
		}
	}

	out, err := execCommand(ctx, path, "--version").Output()
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
// Uses exec.Command directly (not execCommand) because cmd.Dir must be set,
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

// checkGateDir verifies .gate/ directory exists and is writable.
func checkGateDir(repoRoot string) domain.DoctorCheckResult {
	dir := filepath.Join(repoRoot, ".gate")
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

// checkLinearMCP verifies Linear MCP is connected by parsing `claude mcp list` output.
// Looks for a line containing "linear", "✓", and "connected" (case-insensitive).
// Requires "✓" to avoid false positives from "disconnected" or "not connected".
func checkLinearMCP(ctx context.Context, claudeCmd string) domain.DoctorCheckResult {
	cmd := execCommand(ctx, claudeCmd, "mcp", "list")
	out, err := cmd.Output()
	if err != nil {
		return domain.DoctorCheckResult{
			Name:    "Linear MCP",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("claude mcp list failed: %v", err),
			Hint:    `ensure Claude CLI is authenticated with "claude login"`,
		}
	}

	output := strings.ToLower(string(out))
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
		Hint:    `run "claude mcp add --transport http --scope project linear https://mcp.linear.app/mcp" in your project root`,
	}
}

// checkSkillMD verifies that both dmail-sendable and dmail-readable SKILL.md files exist.
func checkSkillMD(repoRoot string) domain.DoctorCheckResult {
	skillsDir := filepath.Join(repoRoot, ".gate", "skills")
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
	return domain.DoctorCheckResult{
		Name:    "SKILL.md",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s (dmail-sendable, dmail-readable)", skillsDir),
	}
}

// runDoctor executes all health checks and returns the results.
// Uses "claude" as the default Claude CLI command name.
func runDoctor(ctx context.Context, configPath string, repoRoot string, logger domain.Logger) []domain.DoctorCheckResult {
	return runDoctorWithClaudeCmd(ctx, configPath, repoRoot, "claude", logger)
}

// runDoctorWithClaudeCmd executes all health checks with a configurable Claude command.
func runDoctorWithClaudeCmd(ctx context.Context, configPath string, repoRoot string, claudeCmd string, logger domain.Logger) []domain.DoctorCheckResult {
	_, span := platform.Tracer.Start(ctx, "domain.doctor")
	defer span.End()

	var results []domain.DoctorCheckResult

	// 1. git binary
	results = append(results, checkTool(ctx, "git"))

	// 2. git repository
	results = append(results, checkGitRepo(repoRoot))

	// 3. claude CLI
	claudeResult := checkTool(ctx, claudeCmd)
	results = append(results, claudeResult)

	// 4. .gate/ directory
	results = append(results, checkGateDir(repoRoot))

	// 5. config.yaml
	results = append(results, checkConfig(configPath))

	// 6. SKILL.md files
	results = append(results, checkSkillMD(repoRoot))

	// 7. Event Store integrity
	results = append(results, checkEventStore(filepath.Join(repoRoot, ".gate")))

	// 8. D-Mail schema v1 validation
	results = append(results, checkDMailSchema(filepath.Join(repoRoot, ".gate")))

	// 9. Success rate (informational)
	results = append(results, checkSuccessRate(filepath.Join(repoRoot, ".gate"), logger))

	// 10. Linear MCP (skip if claude unavailable)
	if claudeResult.Status != domain.CheckOK {
		results = append(results, domain.DoctorCheckResult{
			Name:    "Linear MCP",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
	} else {
		results = append(results, checkLinearMCP(ctx, claudeCmd))
	}

	for _, r := range results {
		span.AddEvent("doctor.check", trace.WithAttributes(
			attribute.String("check.name", r.Name),
			attribute.String("check.status", r.Status.StatusLabel()),
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
