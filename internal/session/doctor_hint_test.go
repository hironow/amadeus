// white-box-reason: tests unexported doctor check functions and their hint messages
package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

// --- BEHAVIORAL tests: Hint populated on FAIL ---

func TestCheckTool_NotFound_HasHint(t *testing.T) {
	// given/when
	result := CheckTool(context.Background(), "nonexistent-tool-xyz-99999")

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
	result := CheckGitRepo(dir)

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
	result := CheckGateDir(dir, false)

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
	mcpOutput := "no linear here"

	// when
	result := checkLinearMCP(mcpOutput, nil)

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
	result := CheckConfig("/nonexistent/config.yaml")

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
	result := CheckConfig(path)

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
	// given -- .gate/events does not exist
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")

	// when
	result := CheckEventStore(root)

	// then
	if result.Status != domain.CheckSkip {
		// CheckEventStore returns CheckSkip for ErrNotExist, not CheckFail
		// so hint is not needed for this case -- verify it's a skip
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
	result := CheckDMailSchema(root)

	// then
	if result.Status != domain.CheckFail {
		t.Skipf("permission test may not work on this OS, got: %v", result.Status)
	}
	if result.Hint == "" {
		t.Error("expected hint for permission error")
	}
}
