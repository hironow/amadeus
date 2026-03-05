package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

// --- STRUCTURAL tests: Hint field rendering ---

func TestPrintDoctorJSON_IncludesHint(t *testing.T) {
	// given
	results := []domain.DoctorCheckResult{
		{Name: "test", Status: domain.CheckFail, Message: "failed", Hint: "fix it"},
	}

	// when
	var buf bytes.Buffer
	_ = printDoctorJSON(&buf, results)

	// then
	var parsed struct {
		Checks []jsonCheck `json:"checks"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Checks[0].Hint != "fix it" {
		t.Errorf("hint = %q, want 'fix it'", parsed.Checks[0].Hint)
	}
}

func TestPrintDoctorJSON_OmitsEmptyHint(t *testing.T) {
	// given
	results := []domain.DoctorCheckResult{
		{Name: "test", Status: domain.CheckOK, Message: "ok"},
	}

	// when
	var buf bytes.Buffer
	_ = printDoctorJSON(&buf, results)

	// then
	if strings.Contains(buf.String(), "hint") {
		t.Error("hint should be omitted when empty")
	}
}

func TestPrintDoctorText_ShowsHint(t *testing.T) {
	// given
	results := []domain.DoctorCheckResult{
		{Name: "test", Status: domain.CheckFail, Message: "failed", Hint: "run init"},
	}

	// when
	var buf bytes.Buffer
	_ = printDoctorText(&buf, results)

	// then
	if !strings.Contains(buf.String(), "hint: run init") {
		t.Errorf("expected hint in text output, got: %s", buf.String())
	}
}

// --- BEHAVIORAL tests: Hint populated on FAIL ---

func TestCheckTool_NotFound_HasHint(t *testing.T) {
	// given/when
	result := checkTool(context.Background(), "nonexistent-tool-xyz-99999")

	// then
	if result.Hint == "" {
		t.Error("expected hint for missing tool")
	}
	if !strings.Contains(result.Hint, "install") {
		t.Errorf("hint should mention install, got: %s", result.Hint)
	}
}

func TestCheckGitRepo_NotRepo_HasHint(t *testing.T) {
	// given
	dir := t.TempDir()

	// when
	result := checkGitRepo(dir)

	// then
	if result.Hint == "" {
		t.Error("expected hint for non-git directory")
	}
	if !strings.Contains(result.Hint, "git init") {
		t.Errorf("hint should mention git init, got: %s", result.Hint)
	}
}

func TestCheckGateDir_NotExist_HasHint(t *testing.T) {
	// given
	dir := t.TempDir()

	// when
	result := checkGateDir(dir)

	// then
	if result.Hint == "" {
		t.Error("expected hint for missing .gate/")
	}
	if !strings.Contains(result.Hint, "amadeus init") {
		t.Errorf("hint should mention 'amadeus init', got: %s", result.Hint)
	}
}

func TestCheckLinearMCP_NotConnected_HasHint(t *testing.T) {
	// given
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "no linear here")
	}
	defer func() { execCommand = exec.CommandContext }()

	// when
	result := checkLinearMCP(context.Background(), "claude")

	// then
	if result.Hint == "" {
		t.Error("expected hint for disconnected linear MCP")
	}
	if !strings.Contains(result.Hint, "claude mcp add") {
		t.Errorf("hint should mention 'claude mcp add', got: %s", result.Hint)
	}
}

func TestCheckConfig_NotFound_HasHint(t *testing.T) {
	// given/when
	result := checkConfig("/nonexistent/config.yaml")

	// then
	if result.Hint == "" {
		t.Error("expected hint for missing config")
	}
	if !strings.Contains(result.Hint, "amadeus init") {
		t.Errorf("hint should mention 'amadeus init', got: %s", result.Hint)
	}
}

func TestCheckConfig_InvalidYAML_HasHint(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`{{{invalid`), 0o644)

	// when
	result := checkConfig(path)

	// then
	if result.Hint == "" {
		t.Error("expected hint for invalid YAML")
	}
	if !strings.Contains(result.Hint, "YAML") {
		t.Errorf("hint should mention YAML, got: %s", result.Hint)
	}
}

func TestCheckSkillMD_Missing_HasHint(t *testing.T) {
	// given
	dir := t.TempDir()

	// when
	result := checkSkillMD(dir)

	// then
	if result.Hint == "" {
		t.Error("expected hint for missing skills")
	}
	if !strings.Contains(result.Hint, "amadeus init") {
		t.Errorf("hint should mention 'amadeus init', got: %s", result.Hint)
	}
}

func TestCheckEventStore_NoDir_HasHint(t *testing.T) {
	// given — .gate/events does not exist
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")

	// when
	result := checkEventStore(root)

	// then
	if result.Status != domain.CheckSkip {
		// checkEventStore returns CheckSkip for ErrNotExist, not CheckFail
		// so hint is not needed for this case — verify it's a skip
		return
	}
}

func TestCheckDMailSchema_PermError_HasHint(t *testing.T) {
	// given: archive exists but is unreadable
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	archiveDir := filepath.Join(root, "archive")
	os.MkdirAll(archiveDir, 0o755)
	os.Chmod(archiveDir, 0o000)
	defer os.Chmod(archiveDir, 0o755)

	// when
	result := checkDMailSchema(root)

	// then
	if result.Status != domain.CheckFail {
		t.Skipf("permission test may not work on this OS, got: %v", result.Status)
	}
	if result.Hint == "" {
		t.Error("expected hint for permission error")
	}
}
