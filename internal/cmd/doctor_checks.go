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

	"github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/session"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"
)

// CheckStatus represents the outcome of a single doctor check.
type CheckStatus int

const (
	CheckOK CheckStatus = iota
	CheckFail
	CheckSkip
)

// DoctorCheckResult holds the outcome of a single doctor check.
type DoctorCheckResult struct {
	Name    string
	Status  CheckStatus
	Message string
}

// StatusLabel returns a display string for the check status.
func (s CheckStatus) StatusLabel() string {
	switch s {
	case CheckOK:
		return "OK"
	case CheckFail:
		return "FAIL"
	case CheckSkip:
		return "SKIP"
	default:
		return "?"
	}
}

// execCommand is a package-level variable for creating exec.Cmd.
// Override in tests to mock command execution.
var execCommand = exec.CommandContext

// checkTool verifies that a CLI tool is installed and executable.
func checkTool(ctx context.Context, name string) DoctorCheckResult {
	path, err := exec.LookPath(name)
	if err != nil {
		return DoctorCheckResult{
			Name:    name,
			Status:  CheckFail,
			Message: "command not found",
		}
	}

	out, err := execCommand(ctx, path, "--version").Output()
	if err != nil {
		return DoctorCheckResult{
			Name:    name,
			Status:  CheckFail,
			Message: fmt.Sprintf("found at %s but --version failed: %v", path, err),
		}
	}

	version := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	return DoctorCheckResult{
		Name:    name,
		Status:  CheckOK,
		Message: fmt.Sprintf("%s (%s)", path, version),
	}
}

// checkGitRepo verifies the given directory is inside a git repository.
// Uses exec.Command directly (not execCommand) because cmd.Dir must be set,
// and tests use real git repos via git init.
func checkGitRepo(dir string) DoctorCheckResult {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return DoctorCheckResult{
			Name:    "Git Repository",
			Status:  CheckFail,
			Message: fmt.Sprintf("%s is not a git repository", dir),
		}
	}
	return DoctorCheckResult{
		Name:    "Git Repository",
		Status:  CheckOK,
		Message: fmt.Sprintf("%s is a git repository", dir),
	}
}

// checkGateDir verifies .gate/ directory exists and is writable.
func checkGateDir(repoRoot string) DoctorCheckResult {
	dir := filepath.Join(repoRoot, ".gate")
	info, err := os.Stat(dir)
	if err != nil {
		return DoctorCheckResult{
			Name:    ".gate/",
			Status:  CheckFail,
			Message: "not found — run 'amadeus init' first",
		}
	}
	if !info.IsDir() {
		return DoctorCheckResult{
			Name:    ".gate/",
			Status:  CheckFail,
			Message: fmt.Sprintf("%s exists but is not a directory", dir),
		}
	}
	probe := filepath.Join(dir, ".doctor_probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return DoctorCheckResult{
			Name:    ".gate/",
			Status:  CheckFail,
			Message: fmt.Sprintf("not writable: %v", err),
		}
	}
	if err := os.Remove(probe); err != nil {
		return DoctorCheckResult{
			Name:    ".gate/",
			Status:  CheckFail,
			Message: fmt.Sprintf("probe cleanup failed: %v", err),
		}
	}
	return DoctorCheckResult{
		Name:    ".gate/",
		Status:  CheckOK,
		Message: fmt.Sprintf("%s writable", dir),
	}
}

// checkLinearMCP verifies Linear MCP is connected by parsing `claude mcp list` output.
// Looks for a line containing "linear", "✓", and "connected" (case-insensitive).
// Requires "✓" to avoid false positives from "disconnected" or "not connected".
func checkLinearMCP(ctx context.Context, claudeCmd string) DoctorCheckResult {
	cmd := execCommand(ctx, claudeCmd, "mcp", "list")
	out, err := cmd.Output()
	if err != nil {
		return DoctorCheckResult{
			Name:    "Linear MCP",
			Status:  CheckFail,
			Message: fmt.Sprintf("claude mcp list failed: %v", err),
		}
	}

	output := strings.ToLower(string(out))
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "linear") && strings.Contains(line, "✓") && strings.Contains(line, "connected") {
			return DoctorCheckResult{
				Name:    "Linear MCP",
				Status:  CheckOK,
				Message: "Linear MCP connected",
			}
		}
	}

	return DoctorCheckResult{
		Name:    "Linear MCP",
		Status:  CheckFail,
		Message: "Linear MCP not found or not connected in claude mcp list output",
	}
}

// checkSkillMD verifies that both dmail-sendable and dmail-readable SKILL.md files exist.
func checkSkillMD(repoRoot string) DoctorCheckResult {
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
		return DoctorCheckResult{
			Name:    "SKILL.md",
			Status:  CheckFail,
			Message: fmt.Sprintf("missing: %s — run 'amadeus init'", strings.Join(missing, ", ")),
		}
	}
	return DoctorCheckResult{
		Name:    "SKILL.md",
		Status:  CheckOK,
		Message: fmt.Sprintf("%s (dmail-sendable, dmail-readable)", skillsDir),
	}
}

// runDoctor executes all health checks and returns the results.
// Uses "claude" as the default Claude CLI command name.
func runDoctor(ctx context.Context, configPath string, repoRoot string) []DoctorCheckResult {
	return runDoctorWithClaudeCmd(ctx, configPath, repoRoot, "claude")
}

// runDoctorWithClaudeCmd executes all health checks with a configurable Claude command.
func runDoctorWithClaudeCmd(ctx context.Context, configPath string, repoRoot string, claudeCmd string) []DoctorCheckResult {
	_, span := amadeus.Tracer.Start(ctx, "amadeus.doctor")
	defer span.End()

	var results []DoctorCheckResult

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
	results = append(results, checkSuccessRate(filepath.Join(repoRoot, ".gate")))

	// 10. Linear MCP (skip if claude unavailable)
	if claudeResult.Status != CheckOK {
		results = append(results, DoctorCheckResult{
			Name:    "Linear MCP",
			Status:  CheckSkip,
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
func checkEventStore(gateRoot string) DoctorCheckResult {
	eventsDir := filepath.Join(gateRoot, "events")
	if _, err := os.Stat(eventsDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return DoctorCheckResult{
				Name:    "Event Store",
				Status:  CheckSkip,
				Message: "no events directory — run 'amadeus init'",
			}
		}
		return DoctorCheckResult{
			Name:    "Event Store",
			Status:  CheckFail,
			Message: fmt.Sprintf("stat events: %v", err),
		}
	}
	count, err := countEventStoreEntries(eventsDir)
	if err != nil {
		return DoctorCheckResult{
			Name:    "Event Store",
			Status:  CheckFail,
			Message: fmt.Sprintf("parse error: %v", err),
		}
	}
	return DoctorCheckResult{
		Name:    "Event Store",
		Status:  CheckOK,
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
			var ev amadeus.Event
			if parseErr := json.Unmarshal([]byte(line), &ev); parseErr != nil {
				return 0, fmt.Errorf("parse %s: %w", e.Name(), parseErr)
			}
			count++
		}
	}
	return count, nil
}

// checkDMailSchema validates all D-Mails in archive/ conform to schema v1.
func checkDMailSchema(gateRoot string) DoctorCheckResult {
	archiveDir := filepath.Join(gateRoot, "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return DoctorCheckResult{
				Name:    "D-Mail Schema",
				Status:  CheckSkip,
				Message: "no archive directory",
			}
		}
		return DoctorCheckResult{
			Name:    "D-Mail Schema",
			Status:  CheckFail,
			Message: fmt.Sprintf("read archive: %v", err),
		}
	}

	var mdFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			mdFiles = append(mdFiles, e.Name())
		}
	}
	if len(mdFiles) == 0 {
		return DoctorCheckResult{
			Name:    "D-Mail Schema",
			Status:  CheckSkip,
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
		dmail, parseErr := amadeus.ParseDMail(data)
		if parseErr != nil {
			invalid = append(invalid, fmt.Sprintf("%s: %v", name, parseErr))
			continue
		}
		if errs := amadeus.ValidateDMail(dmail); len(errs) > 0 {
			invalid = append(invalid, fmt.Sprintf("%s: %s", name, strings.Join(errs, "; ")))
		}
	}

	if len(invalid) > 0 {
		return DoctorCheckResult{
			Name:    "D-Mail Schema",
			Status:  CheckFail,
			Message: fmt.Sprintf("%d/%d invalid: %s", len(invalid), len(mdFiles), strings.Join(invalid, ", ")),
		}
	}
	return DoctorCheckResult{
		Name:    "D-Mail Schema",
		Status:  CheckOK,
		Message: fmt.Sprintf("%d D-Mail(s) valid", len(mdFiles)),
	}
}

// checkSuccessRate calculates and reports the event-based success rate.
func checkSuccessRate(gateDir string) DoctorCheckResult {
	store := session.NewEventStore(gateDir)
	events, err := store.LoadAll()
	if err != nil || len(events) == 0 {
		return DoctorCheckResult{
			Name:    "success-rate",
			Status:  CheckOK,
			Message: "no events",
		}
	}

	rate := amadeus.SuccessRate(events)
	var clean, total int
	for _, ev := range events {
		if ev.Type != amadeus.EventCheckCompleted {
			continue
		}
		var data amadeus.CheckCompletedData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			continue
		}
		total++
		if len(data.Result.DMails) == 0 {
			clean++
		}
	}

	return DoctorCheckResult{
		Name:    "success-rate",
		Status:  CheckOK,
		Message: amadeus.FormatSuccessRate(rate, clean, total),
	}
}

// checkConfig validates that config.yaml exists and can be loaded.
func checkConfig(path string) DoctorCheckResult {
	if _, err := os.Stat(path); err != nil {
		return DoctorCheckResult{
			Name:    "Config",
			Status:  CheckFail,
			Message: fmt.Sprintf("%s: %v", path, err),
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return DoctorCheckResult{
			Name:    "Config",
			Status:  CheckFail,
			Message: fmt.Sprintf("%s: %v", path, err),
		}
	}
	cfg := amadeus.DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DoctorCheckResult{
			Name:    "Config",
			Status:  CheckFail,
			Message: fmt.Sprintf("%s: %v", path, err),
		}
	}
	if errs := amadeus.ValidateConfig(cfg); len(errs) > 0 {
		return DoctorCheckResult{
			Name:    "Config",
			Status:  CheckFail,
			Message: fmt.Sprintf("%s: %s", path, strings.Join(errs, "; ")),
		}
	}
	return DoctorCheckResult{
		Name:    "Config",
		Status:  CheckOK,
		Message: fmt.Sprintf("%s loaded and validated", path),
	}
}
