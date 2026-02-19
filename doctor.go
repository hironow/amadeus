package amadeus

import (
	"context"
	"fmt"
	"os/exec"
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
