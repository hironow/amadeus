package amadeus

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// checkDivergenceDir verifies .divergence/ directory exists and is writable.
func checkDivergenceDir(repoRoot string) DoctorCheckResult {
	dir := filepath.Join(repoRoot, ".divergence")
	info, err := os.Stat(dir)
	if err != nil {
		return DoctorCheckResult{
			Name:    ".divergence/",
			Status:  CheckFail,
			Message: "not found — run 'amadeus init' first",
		}
	}
	if !info.IsDir() {
		return DoctorCheckResult{
			Name:    ".divergence/",
			Status:  CheckFail,
			Message: fmt.Sprintf("%s exists but is not a directory", dir),
		}
	}
	probe := filepath.Join(dir, ".doctor_probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return DoctorCheckResult{
			Name:    ".divergence/",
			Status:  CheckFail,
			Message: fmt.Sprintf("not writable: %v", err),
		}
	}
	os.Remove(probe)
	return DoctorCheckResult{
		Name:    ".divergence/",
		Status:  CheckOK,
		Message: fmt.Sprintf("%s writable", dir),
	}
}

// checkLinearMCP verifies Linear MCP is connected by parsing `claude mcp list` output.
// Looks for a line containing "linear" (case-insensitive) and "Connected".
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
		if strings.Contains(line, "linear") && strings.Contains(line, "connected") {
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

// checkConfig validates that config.yaml exists and can be loaded.
func checkConfig(path string) DoctorCheckResult {
	if _, err := os.Stat(path); err != nil {
		return DoctorCheckResult{
			Name:    "Config",
			Status:  CheckFail,
			Message: fmt.Sprintf("%s: %v", path, err),
		}
	}
	_, err := LoadConfig(path)
	if err != nil {
		return DoctorCheckResult{
			Name:    "Config",
			Status:  CheckFail,
			Message: fmt.Sprintf("%s: %v", path, err),
		}
	}
	return DoctorCheckResult{
		Name:    "Config",
		Status:  CheckOK,
		Message: fmt.Sprintf("%s loaded successfully", path),
	}
}
